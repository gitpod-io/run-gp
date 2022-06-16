// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"context"
	"encoding/json"
	"os"

	"github.com/gitpod-io/gitpod/run-gp/pkg/update"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var debugLatestReleaseCmd = &cobra.Command{
	Use: "get-latest-release",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := update.NewGitHubReleaseDiscovery(context.Background()).DiscoverLatest(context.Background())
		if err != nil {
			return err
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	},
}

func init() {
	debugCmd.AddCommand(debugLatestReleaseCmd)
}
