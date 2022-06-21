// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"fmt"
	"time"

	"github.com/gitpod-io/gitpod/run-gp/pkg/console"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var debugUICmd = &cobra.Command{
	Use:   "ui",
	Short: "runs the bubble UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		lg, _, err := console.NewBubbleTeaUI(console.BubbleUIOpts{
			UIMode:  console.UIModeFancy,
			Verbose: false,
		})
		if err != nil {
			return err
		}
		defer lg.Quit()

		p := lg.StartPhase("", "doing something")
		time.Sleep(200 * time.Millisecond)
		p.Success()
		p = lg.StartPhase("", "doing some more")
		time.Sleep(1 * time.Second)
		p.Success()

		p = lg.StartPhase("", "yet more work")
		w := lg.Writer()
		for i := 0; i < 30; i++ {
			fmt.Fprintf(w, "line %03d\n", i)
			time.Sleep(100 * time.Millisecond)
		}
		w.Close()
		p.Failure("no good reason")
		w.Discard()

		time.Sleep(200 * time.Millisecond)

		return nil
	},
}

func init() {
	debugCmd.AddCommand(debugUICmd)
}
