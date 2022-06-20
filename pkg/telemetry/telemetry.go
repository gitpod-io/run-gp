// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package telemetry

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"os/exec"
	"runtime"
	"strings"
	"time"

	segment "github.com/segmentio/analytics-go/v3"
)

// Injected at build time
var segmentKey = "TgiJIVvFsBGwmxbnnt5NeeDaian9nr3n"

var opts struct {
	Enabled  bool
	Identity string

	client segment.Client
}

// Init initialises the telemetry
func Init(enabled bool, identity string) {
	opts.Enabled = enabled
	if !enabled {
		return
	}

	if identity == "" {
		rand.Seed(time.Now().UnixNano())
		letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
		b := make([]rune, 32)
		for i := range b {
			b[i] = letters[rand.Intn(len(letters))]
		}
		identity = string(b)
	}
	opts.Identity = identity

	if segmentKey != "" {
		opts.client = segment.New(segmentKey)
	}
}

func Close() {
	if opts.client != nil {
		opts.client.Close()
	}
}

// Identity returns the identity
func Identity() string {
	return opts.Identity
}

// Enabled returns true if the telemetry is enabled
func Enabled() bool {
	return opts.Enabled && opts.Identity != "" && opts.client != nil
}

func track(event string, props segment.Properties) {
	if !Enabled() {
		return
	}

	opts.client.Enqueue(segment.Track{
		AnonymousId: opts.Identity,
		Event:       event,
		Timestamp:   time.Now(),
		Properties:  props,
	})
}

// RecordWorkspaceStarted sends telemetry when a workspace is started
func RecordWorkspaceStarted(remoteURI string, containerRuntime string) {
	uriHash := sha256.New()
	_, _ = uriHash.Write([]byte(remoteURI))

	track("rungp_start_workspace", segment.NewProperties().
		Set("runtime", containerRuntime).
		Set("remoteURIHash", fmt.Sprintf("sha256:%x", uriHash.Sum(nil))).
		Set("GOOS", runtime.GOOS).
		Set("GOARCH", runtime.GOARCH),
	)
}

// RecordWorkspaceFailure sets telemetry when a workspace fails
func RecordWorkspaceFailure(remoteURI string, phase string, containerRuntime string) {
	uriHash := sha256.New()
	_, _ = uriHash.Write([]byte(remoteURI))

	track("rungp_workspace_failure", segment.NewProperties().
		Set("runtime", containerRuntime).
		Set("phase", phase).
		Set("remoteURIHash", fmt.Sprintf("sha256:%x", uriHash.Sum(nil))).
		Set("GOOS", runtime.GOOS).
		Set("GOARCH", runtime.GOARCH),
	)
}

// GetGitRemoteOriginURI returns the remote origin URI for the specified working directory.
func GetGitRemoteOriginURI(wd string) string {
	git := exec.Command("git", "remote", "get-uri", "origin")
	git.Dir = wd
	gitout, _ := git.CombinedOutput()
	return strings.TrimSpace(string(gitout))
}
