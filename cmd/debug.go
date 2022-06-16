// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "helps develop run-gp",
	// Hidden: true,
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
