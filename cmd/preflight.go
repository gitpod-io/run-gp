// Copyright (c) 2020 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"context"
	"path/filepath"

	"github.com/gitpod-io/gitpod/run-gp/pkg/console"
	"github.com/gitpod-io/gitpod/run-gp/pkg/runtime"
	"github.com/gitpod-io/gitpod/run-gp/pkg/runtime/assets"
	"github.com/spf13/cobra"
)

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Just builds the workspace image and runs the tasks as if the workspace",

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

		asts := assets.WithinGitpodWorkspace
		if !asts.Available() {
			asts = assets.Embedded
		}

		idePort := 9999
		asts = assets.NoopIDE{Assets: asts, SupervisorPort: idePort}

		cfg, err := getGitpodYaml()
		if err != nil {
			return err
		}

		if cfg.CheckoutLocation == "" {
			cfg.CheckoutLocation = filepath.Base(rootOpts.Workdir)
		}
		if cfg.WorkspaceLocation == "" {
			cfg.WorkspaceLocation = cfg.CheckoutLocation
		}

		rt, err := getRuntime(rootOpts.Workdir)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		shutdown := make(chan struct{})
		tasksDone := make(chan struct{})
		go func() {
			defer close(shutdown)

			buildingPhase := log.StartPhase("[building]", "workspace image")
			ref := filepath.Join("workspace-image:latest")
			bldLog := log.Writer()
			err = rt.BuildImage(ctx, ref, cfg, runtime.BuildOpts{
				Logs:   bldLog,
				Assets: asts,
			})
			if err != nil {
				buildingPhase.Failure(err.Error())
				return
			}
			bldLog.Discard()
			buildingPhase.Success()

			var envVars []string
			if !preflightOpts.AllCommands {
				envVars = append(envVars, "GITPOD_HEADLESS=true")
			}

			runLogs := console.Observe(ctx, log, console.WorkspaceAccessInfo{
				WorkspaceFolder: filepath.Join("/workspace", cfg.WorkspaceLocation),
				HTTPPort:        0,
				SSHPort:         0,
				SupervisorPort:  idePort,
			}, console.ObserveOpts{
				ObserveTasks: true,
				OnTasksDone: func() {
					close(tasksDone)
				},
			})
			err := rt.StartWorkspace(ctx, ref, cfg, runtime.StartOpts{
				Network:           runtime.HostNetwork{},
				Logs:              runLogs,
				Assets:            asts,
				AdditionalEnvVars: envVars,
			})
			if err != nil {
				return
			}
			runLogs.Discard()
		}()

		select {
		case <-tasksDone:
			cancel()
			log.Quit()
		case <-done:
			cancel()
			<-shutdown
		case <-shutdown:
			log.Quit()
		}

		return nil
	},
}

var preflightOpts struct {
	AllCommands bool
}

func init() {
	rootCmd.AddCommand(preflightCmd)
	preflightCmd.Flags().BoolVar(&preflightOpts.AllCommands, "all-commands", true, "run all commands - note that run-gp will not exit once they're done")
}
