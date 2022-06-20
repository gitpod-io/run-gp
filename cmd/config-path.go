// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"fmt"

	"github.com/gitpod-io/gitpod/run-gp/pkg/config"
	"github.com/spf13/cobra"
)

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "prints the path to the config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.ReadInConfig()
		if err != nil {
			return err
		}

		if cfg == nil {
			return fmt.Errorf("no config file found")
		}

		fmt.Println(cfg.Filename)

		return nil
	},
}

func init() {
	configCmd.AddCommand(configPathCmd)
}
