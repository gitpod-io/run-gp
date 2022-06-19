// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package runtime

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	gitpod "github.com/gitpod-io/gitpod/gitpod-protocol"
	"github.com/gitpod-io/gitpod/run-gp/pkg/console"
)

type SupportedRuntime int

const (
	AutodetectRuntime SupportedRuntime = iota
	DockerRuntime
	NerdctlRuntime
)

func New(wd string, rt SupportedRuntime) (RuntimeBuilder, error) {
	if rt == AutodetectRuntime {
		nrt, err := detectRuntime()
		if err != nil {
			return nil, err
		}
		rt = nrt
	}

	switch rt {
	case DockerRuntime:
		console.Default.Debugf("using docker as container runtime")
		return &docker{Workdir: wd, Command: "docker"}, nil
	case NerdctlRuntime:
		console.Default.Debugf("using nerdctl as container runtime")
		return &docker{Workdir: wd, Command: "nerdctl"}, nil
	default:
		return nil, fmt.Errorf("unsupported runtime: %v", rt)
	}
}

func detectRuntime() (SupportedRuntime, error) {
	if _, err := exec.LookPath("docker"); err == nil {
		return DockerRuntime, nil
	}
	if _, err := exec.LookPath("nerdctl"); err == nil {
		return NerdctlRuntime, nil
	}
	return AutodetectRuntime, fmt.Errorf("no supported container runtime detected")
}

type RuntimeBuilder interface {
	Runtime
	Builder
}

type Runtime interface {
	StartWorkspace(ctx context.Context, imageRef string, cfg *gitpod.GitpodConfig, opts StartOpts) error
}

type Builder interface {
	BuildImage(logs io.WriteCloser, ref string, cfg *gitpod.GitpodConfig) (err error)
}

type StartOpts struct {
	PortOffset       int
	NoPortForwarding bool
	IDEPort          int
	SSHPort          int
	SSHPublicKey     string
	Logs             io.WriteCloser
}
