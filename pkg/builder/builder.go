// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package builder

import (
	"io"

	gitpod "github.com/gitpod-io/gitpod/gitpod-protocol"
)

type Result struct {
	Ref string
	Err error
}

type Builder interface {
	BuildImage(logs io.WriteCloser, ref string, cfg *gitpod.GitpodConfig) (err error)
}
