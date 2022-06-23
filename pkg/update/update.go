// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package update

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/gitpod-io/gitpod/run-gp/pkg/console"
	"github.com/gitpod-io/gitpod/run-gp/pkg/telemetry"
	"github.com/google/go-github/v45/github"
	"github.com/inconshreveable/go-update"
	"golang.org/x/oauth2"
)

// Update runs the self-update
func Update(ctx context.Context, currentVersion string, discovery ReleaseDiscovery, cfgFN string) (err error) {
	if currentVersion == "v0.0.0" {
		// development builds don't auto update
		return nil
	}

	cv, err := semver.NewVersion(currentVersion)
	if err != nil {
		return fmt.Errorf("cannot parse current version %s: %v", currentVersion, err)
	}

	discoveryCtx, discoveryCancel := context.WithTimeout(ctx, 30*time.Second)
	defer discoveryCancel()
	latest, err := discovery.DiscoverLatest(discoveryCtx)
	if err != nil {
		telemetry.RecordUpdateStatus("failed-discover-latest", "", err)
		return fmt.Errorf("cannot discover latest version: %v", err)
	}

	if !cv.LessThan(latest.Version) {
		return nil
	}

	console.Default.Warnf("Newer version found: %s", latest.Name)
	console.Default.Warnf("will automatically upgrade. To disable this, set autoUpdate to false in %s", cfgFN)

	var (
		arch = strings.ToLower(runtime.GOARCH)
		os   = strings.ToLower(runtime.GOOS)
	)

	var candidate *Executable
	for _, p := range latest.Platforms {
		if strings.ToLower(p.Platform.OS) != os {
			continue
		}
		if strings.ToLower(p.Platform.Arch) != arch {
			continue
		}
		candidate = &p
		break
	}
	if candidate == nil {
		telemetry.RecordUpdateStatus("failed-no-supported-executable", latest.Name, err)
		return fmt.Errorf("no supported executable found for version %s (OS: %s, Architecture: %s)", latest.Name, os, arch)
	}

	phase := console.Default.StartPhase("self-update", "downloading from "+candidate.URL)
	defer func() {
		if err == nil {
			phase.Success()
		} else {
			phase.Failure(err.Error())
		}
	}()

	checksum, err := hex.DecodeString(candidate.Checksum)
	if err != nil {
		telemetry.RecordUpdateStatus("failed-invalid-checksum", latest.Name, err)
		return fmt.Errorf("candidate %s has invalid checksum: %w", candidate.URL, err)
	}

	if candidate.IsArchive {
		telemetry.RecordUpdateStatus("failed-is-archive", latest.Name, err)
		return fmt.Errorf("not supported")
	}

	dl, err := discovery.Download(ctx, *candidate)
	if err != nil {
		telemetry.RecordUpdateStatus("failed-download", latest.Name, err)
		return err
	}
	defer dl.Close()

	err = update.Apply(dl, update.Options{
		Checksum: checksum,
	})
	if err != nil {
		telemetry.RecordUpdateStatus("failed-apply", latest.Name, err)
		return err
	}

	telemetry.RecordUpdateStatus("success-apply", latest.Name, nil)
	return nil
}

type Updater struct {
	Discovery ReleaseDiscovery
}

type ReleaseDiscovery interface {
	DiscoverLatest(ctx context.Context) (*Version, error)
	Download(ctx context.Context, e Executable) (io.ReadCloser, error)
}

type Version struct {
	Name      string
	Version   *semver.Version
	Platforms []Executable
}

type Platform struct {
	OS   string
	Arch string
}

type Executable struct {
	Platform Platform

	URL       string
	Checksum  string
	Filename  string
	IsArchive bool
}

// NewGitHubReleaseDiscovery returns a new Discovery which uses GitHub
func NewGitHubReleaseDiscovery(ctx context.Context) *GitHubReleaseDiscovery {
	var client *http.Client

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		client = oauth2.NewClient(ctx, ts)

	} else {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	return &GitHubReleaseDiscovery{
		GitHubClient: github.NewClient(client).Repositories,
		HTTPClient:   client,
	}
}

type GitHubReleaseDiscovery struct {
	GitHubClient GithubClient
	HTTPClient   *http.Client
}

type GithubClient interface {
	GetLatestRelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error)
}

func (g *GitHubReleaseDiscovery) Download(ctx context.Context, e Executable) (io.ReadCloser, error) {
	req, err := http.NewRequest("GET", e.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// DiscoverLatest discovers the latest release
func (g *GitHubReleaseDiscovery) DiscoverLatest(ctx context.Context) (*Version, error) {
	rel, _, err := g.GitHubClient.GetLatestRelease(ctx, "gitpod-io", "run-gp")
	if err != nil {
		return nil, err
	}

	var (
		platforms        []Executable
		checksumAssetURL string
	)
	for _, asset := range rel.Assets {
		if asset.GetName() == "checksums.txt" {
			checksumAssetURL = asset.GetURL()
			continue
		}

		segs := strings.Split(asset.GetName(), "_")
		if len(segs) < 4 || 5 < len(segs) {
			continue
		}

		os, arch := segs[2], segs[3]
		if len(segs) == 5 {
			arch += "_" + segs[4]
		}
		arch = strings.TrimSuffix(arch, ".tar.gz")

		platforms = append(platforms, Executable{
			Platform: Platform{
				OS:   os,
				Arch: arch,
			},
			URL:       asset.GetURL(),
			Filename:  asset.GetName(),
			IsArchive: strings.HasSuffix(asset.GetName(), ".tar.gz"),
		})
	}

	if checksumAssetURL != "" {
		console.Default.Debugf("downloading checksums from: %s", checksumAssetURL)

		req, err := http.NewRequest("GET", checksumAssetURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/octet-stream")
		resp, err := g.HTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		fc, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		checksums := parseChecksums(string(fc))
		for i, e := range platforms {
			c, ok := checksums[e.Filename]
			if !ok {
				continue
			}
			e.Checksum = c
			platforms[i] = e
		}
	}

	name := rel.GetName()
	version, err := semver.NewVersion(name)
	if err != nil {
		return nil, err
	}

	return &Version{
		Name:      name,
		Version:   version,
		Platforms: platforms,
	}, nil
}

func parseChecksums(fc string) map[string]string {
	lines := strings.Split(fc, "\n")

	res := make(map[string]string, len(lines))
	for _, line := range lines {
		segs := strings.Fields(line)
		if len(segs) != 2 {
			continue
		}
		res[segs[1]] = segs[0]
	}
	return res
}
