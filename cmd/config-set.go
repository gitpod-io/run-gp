// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"github.com/gitpod-io/gitpod/run-gp/pkg/config"
	"github.com/spf13/cobra"
)

var configSetCmd = &cobra.Command{
	Use:   "set <path> <value>",
	Short: "set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.ReadInConfig()
		if err != nil {
			return err
		}

		err = cfg.Set(args[0], args[1])
		if err != nil {
			return err
		}

		err = cfg.Write()
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	configCmd.AddCommand(configSetCmd)
}
