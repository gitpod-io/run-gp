// Copyright (c) 2020 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/gitpod-io/gitpod/run-gp/pkg/console"
	"github.com/gitpod-io/gitpod/run-gp/pkg/runtime"
	"github.com/gitpod-io/gitpod/run-gp/pkg/runtime/assets"
	"github.com/spf13/cobra"
)

type noopWriteCloser struct{ io.Writer }

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Just builds the workspace image and runs the tasks as if the workspace",

	RunE: func(cmd *cobra.Command, args []string) error {

		log := console.NewConsoleLog(os.Stdout)
		console.Init(log)

		asts := assets.WithinGitpodWorkspace
		if !asts.Available() {
			asts = assets.Embedded
		}

		idePort := 25000
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
			err = rt.BuildImage(ctx, ref, cfg, runtime.BuildOpts{
				Assets: asts,
			})
			if err != nil {
				buildingPhase.Failure(err.Error())
				return
			}
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
	preflightCmd.PersistentFlags().StringVarP(&rootOpts.Output, "output", "u", "plain", "UI mode (fancy, json, plain)")
}
