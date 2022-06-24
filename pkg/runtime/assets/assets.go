// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package assets

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

//go:generate sh ../../../hack/update-assets.sh

//go:embed *.tar.gz
var assetPack embed.FS

//go:embed images.json
var imagesJSON []byte

type Assets interface {
	// returns the image environment variables embedded which would come from the IDE and supervisor image
	EnvVars() []string

	// Access makes the assets available to be mounted into the container
	Access() (AssetPaths, error)

	// Available returns true if this asset set is available
	Available() bool
}

// AssetPaths represents assets on the disk
type AssetPaths interface {
	Supervisor() string
	IDEPath() string

	Close() error
}

type NoopIDE struct {
	Assets Assets
}

// returns the image environment variables embedded which would come from the IDE and supervisor image
func (np NoopIDE) EnvVars() []string { return nil }

// Access makes the assets available to be mounted into the container
func (np NoopIDE) Access() (AssetPaths, error) {
	actual, err := np.Assets.Access()
	if err != nil {
		return nil, err
	}

	idePath, err := np.prepIDE()
	if err != nil {
		return nil, err
	}

	return compositeAssetPaths{
		supervisor: actual.Supervisor(),
		idePath:    idePath,
		closer: []io.Closer{
			actual,
			closerFunc(func() error { return os.RemoveAll(idePath) }),
		},
	}, nil
}

type closerFunc func() error

func (c closerFunc) Close() error {
	return c()
}

func (NoopIDE) prepIDE() (path string, err error) {
	tmpdir, err := os.MkdirTemp("", "rungp-noopide-*")
	if err != nil {
		return "", err
	}

	var errors []error
	errors = append(errors, ioutil.WriteFile(filepath.Join(tmpdir, "startup.sh"), []byte("#!/bin/sh\nsleep infinity\n"), 0644))
	errors = append(errors, ioutil.WriteFile(filepath.Join(tmpdir, "supervisor-ide-config.json"), []byte(`{"entrypoint": "/ide/startup.sh"}`), 0644))
	for _, err := range errors {
		if err != nil {
			return "", err
		}
	}

	return tmpdir, nil
}

// Available returns true if this asset set is available
func (np NoopIDE) Available() bool {
	return np.Assets.Available()
}

type compositeAssetPaths struct {
	supervisor string
	idePath    string

	closer []io.Closer
}

func (ca compositeAssetPaths) Supervisor() string { return ca.supervisor }
func (ca compositeAssetPaths) IDEPath() string    { return ca.idePath }
func (ca compositeAssetPaths) Close() error {
	for _, c := range ca.closer {
		err := c.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

type localAssetPaths struct {
	supervisor string
	idePath    string
}

func (la localAssetPaths) Supervisor() string { return la.supervisor }
func (la localAssetPaths) IDEPath() string    { return la.idePath }
func (la localAssetPaths) Close() error       { return nil }

// WithinGitpodWorkspace pull assets from within a Gitpod workspace.
var WithinGitpodWorkspace Assets = &localAssets{
	Paths: localAssetPaths{
		supervisor: "/.supervisor",
		idePath:    "/ide",
	},
}

// localAssets ships assets from a localAssets directory.
// This only works on Linux.
type localAssets struct {
	Paths AssetPaths
}

func (localAssets) EnvVars() []string { return []string{} }

func (l localAssets) Available() bool {
	supervisor := l.Paths.Supervisor()
	if stat, err := os.Stat(supervisor); supervisor != "" && (err != nil || !stat.IsDir()) {
		return false
	}
	ide := l.Paths.IDEPath()
	if stat, err := os.Stat(ide); ide != "" && (err != nil || !stat.IsDir()) {
		return false
	}

	return true
}

func (l localAssets) Access() (AssetPaths, error) {
	return l.Paths, nil
}

// Embedded are the assets embedded in this binary
var Embedded Assets = &embeddedAssets{}

type embeddedAssets struct{}

// EnvVars returns the image environment variables embedded in the images.json file
func (*embeddedAssets) EnvVars() []string {
	var res struct {
		Envs []string `json:"envs"`
	}
	_ = json.Unmarshal(imagesJSON, &res)
	return res.Envs
}

// Available returns true if the assets are embedded in this binary
func (*embeddedAssets) Available() bool {
	f, err := assetPack.Open("assets.tar.gz")
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// CopyTo extracts the assets to the destionation directory
func (*embeddedAssets) Access() (AssetPaths, error) {
	tmpdir, err := os.MkdirTemp("", "rungp-*")
	if err != nil {
		return nil, err
	}

	f, err := assetPack.Open("assets.tar.gz")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	err = untar(f, tmpdir)
	if err != nil {
		return nil, err
	}

	return extractedAssetPaths(tmpdir), nil
}

type extractedAssetPaths string

func (ea extractedAssetPaths) Supervisor() string { return filepath.Join(string(ea), "supervisor") }
func (ea extractedAssetPaths) IDEPath() string    { return filepath.Join(string(ea), "ide") }
func (ea extractedAssetPaths) Close() error       { return os.RemoveAll((string(ea))) }

// untar is copied from https://cs.opensource.google/go/x/build/+/2838fbb2:internal/untar/untar.go;l=27
func untar(r io.Reader, dir string) (err error) {
	t0 := time.Now()
	nFiles := 0
	madeDir := map[string]bool{}
	defer func() {
		td := time.Since(t0)
		if err != nil {
			log.Printf("error extracting tarball into %s after %d files, %d dirs, %v: %v", dir, nFiles, len(madeDir), td, err)
		}
	}()
	zr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("requires gzip-compressed body: %v", err)
	}
	tr := tar.NewReader(zr)
	loggedChtimesError := false
	for {
		f, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("tar reading error: %v", err)
			return fmt.Errorf("tar error: %v", err)
		}
		if !validRelPath(f.Name) {
			return fmt.Errorf("tar contained invalid name error %q", f.Name)
		}
		rel := filepath.FromSlash(f.Name)
		abs := filepath.Join(dir, rel)

		fi := f.FileInfo()
		mode := fi.Mode()
		switch {
		case mode.IsRegular():
			// Make the directory. This is redundant because it should
			// already be made by a directory entry in the tar
			// beforehand. Thus, don't check for errors; the next
			// write will fail with the same error.
			dir := filepath.Dir(abs)
			if !madeDir[dir] {
				if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
					return err
				}
				madeDir[dir] = true
			}
			wf, err := os.OpenFile(abs, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode.Perm())
			if err != nil {
				return err
			}
			n, err := io.Copy(wf, tr)
			if closeErr := wf.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
			if err != nil {
				return fmt.Errorf("error writing to %s: %v", abs, err)
			}
			if n != f.Size {
				return fmt.Errorf("only wrote %d bytes to %s; expected %d", n, abs, f.Size)
			}
			modTime := f.ModTime
			if modTime.After(t0) {
				// Clamp modtimes at system time. See
				// golang.org/issue/19062 when clock on
				// buildlet was behind the gitmirror server
				// doing the git-archive.
				modTime = t0
			}
			if !modTime.IsZero() {
				if err := os.Chtimes(abs, modTime, modTime); err != nil && !loggedChtimesError {
					// benign error. Gerrit doesn't even set the
					// modtime in these, and we don't end up relying
					// on it anywhere (the gomote push command relies
					// on digests only), so this is a little pointless
					// for now.
					log.Printf("error changing modtime: %v (further Chtimes errors suppressed)", err)
					loggedChtimesError = true // once is enough
				}
			}
			nFiles++
		case mode.IsDir():
			if err := os.MkdirAll(abs, 0755); err != nil {
				return err
			}
			madeDir[abs] = true
		default:
			return fmt.Errorf("tar file entry %s contained unsupported file type %v", f.Name, mode)
		}
	}
	return nil
}

func validRelativeDir(dir string) bool {
	if strings.Contains(dir, `\`) || path.IsAbs(dir) {
		return false
	}
	dir = path.Clean(dir)
	if strings.HasPrefix(dir, "../") || strings.HasSuffix(dir, "/..") || dir == ".." {
		return false
	}
	return true
}

func validRelPath(p string) bool {
	if p == "" || strings.Contains(p, `\`) || strings.HasPrefix(p, "/") || strings.Contains(p, "../") {
		return false
	}
	return true
}
