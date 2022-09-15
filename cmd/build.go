// Copyright (c) 2020 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/gitpod-io/gitpod/run-gp/pkg/console"
	"github.com/gitpod-io/gitpod/run-gp/pkg/update"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var buildCmd = &cobra.Command{
	Use:   "build <target-reference>",
	Short: "Builds the workspace image",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uiMode := console.UIModeAuto
		if rootOpts.Verbose {
			uiMode = console.UIModeDaemon
		}
		log, done, err := console.NewBubbleTeaUI(console.BubbleUIOpts{
			UIMode:  uiMode,
			Verbose: rootOpts.Verbose,
		})
		if err != nil {
			return err
		}
		console.Init(log)

		cfg, err := getGitpodYaml()
		if err != nil {
			return err
		}
		if cfg.WorkspaceLocation == "" {
			cfg.WorkspaceLocation = cfg.CheckoutLocation
		}

		runtime, err := getRuntime(rootOpts.Workdir)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		shutdown := make(chan struct{})
		go func() {
			defer close(shutdown)

			if rootOpts.cfg.AutoUpdate.Enabled {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
				defer cancel()
				didUpdate, err := update.Update(ctx, version, update.NewGitHubReleaseDiscovery(ctx), rootOpts.cfg.Filename)
				if errors.Is(err, context.Canceled) {
					return
				} else if err != nil {
					log.Warnf("failed to auto-update: %v", err)
				} else if didUpdate {
					log.Warnf("Updated to new version - update comes into effect with the next start of run-gp")
				}
			}

			buildingPhase := log.StartPhase("[building]", "workspace image")
			ref := args[0]
			bldLog := log.Writer()
			err = runtime.BuildImage(ctx, bldLog, ref, cfg)
			if err != nil {
				buildingPhase.Failure(err.Error())
				return
			}
			bldLog.Discard()
			buildingPhase.Success()
		}()

		select {
		case <-done:
			cancel()
			<-shutdown
		case <-shutdown:
			log.Quit()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
