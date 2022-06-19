// Copyright (c) 2020 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

//go:generate sh hack/update-license-header.sh

package main

import (
	"github.com/gitpod-io/gitpod/run-gp/cmd"
)

func main() {
	cmd.Execute()
}
