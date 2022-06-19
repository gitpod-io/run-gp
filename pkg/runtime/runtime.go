// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package runtime

import (
	"context"
	"io"

	gitpod "github.com/gitpod-io/gitpod/gitpod-protocol"
)

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
