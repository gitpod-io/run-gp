// Copyright (c) 2020 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
		uiMode := console.UIModeAuto
		log, done, err := console.NewBubbleTeaUI(console.BubbleUIOpts{
			UIMode:     uiMode,
			Verbose:    rootOpts.Verbose,
			WithBanner: false,
		})

		console.Init(console.NewConsoleLog(os.Stdout))

		asts := assets.WithinGitpodWorkspace
		if !asts.Available() {
			asts = assets.Embedded
		}

		supervisorPort := 24999
		asts = assets.DebugIDE{
			Assets:         asts,
			SupervisorPort: supervisorPort,
		}

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

		dockerClient, err := rt.NewClient()
		if err != nil {
			return err
		}

		rt.TerminateExistingRunGPContainer(dockerClient, ctx)

		shutdown := make(chan struct{})
		tasksDone := make(chan struct{})
		go func() {
			defer close(shutdown)

			ref := filepath.Join("workspace-image:latest")
			err = rt.BuildImage(ctx, ref, cfg, runtime.BuildOpts{
				Assets: asts,
			})

			var envVars []string
			if !preflightOpts.AllCommands {
				envVars = append(envVars, "GITPOD_HEADLESS=true")
			}
			envVars = append(envVars, fmt.Sprintf("GITPOD_THEIA_PORT=%d", supervisorPort+1))
			envVars = append(envVars, fmt.Sprintf("GITPOD_REPO_ROOT=%s", os.Getenv("GITPOD_REPO_ROOT")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_WORKSPACE_CLASS_INFO=%s", os.Getenv("GITPOD_WORKSPACE_CLASS_INFO")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_OWNER_ID=%s", os.Getenv("GITPOD_OWNER_ID")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_WORKSPACE_ID=%s", os.Getenv("GITPOD_WORKSPACE_ID")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_WORKSPACE_CONTEXT_URL=%s", os.Getenv("GITPOD_WORKSPACE_CONTEXT_URL")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_WORKSPACE_CLASS=%s", os.Getenv("GITPOD_WORKSPACE_CLASS")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_INSTANCE_ID=%s", os.Getenv("GITPOD_INSTANCE_ID")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_REPO_ROOTS=%s", os.Getenv("GITPOD_REPO_ROOTS")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_WORKSPACE_URL=%s", os.Getenv("GITPOD_WORKSPACE_URL")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_WORKSPACE_CLUSTER_HOST=%s", os.Getenv("GITPOD_WORKSPACE_CLUSTER_HOST")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_HOST=%s", os.Getenv("GITPOD_HOST")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_GIT_USER_NAME=%s", os.Getenv("GITPOD_GIT_USER_NAME")))
			envVars = append(envVars, fmt.Sprintf("GITPOD_MEMORY=%s", os.Getenv("GITPOD_MEMORY")))

			envVars = append(envVars, fmt.Sprintf("VSX_REGISTRY_URL=%s", os.Getenv("VSX_REGISTRY_URL")))
			envVars = append(envVars, fmt.Sprintf("EDITOR=%s", os.Getenv("EDITOR")))
			envVars = append(envVars, fmt.Sprintf("VISUAL=%s", os.Getenv("VISUAL")))
			envVars = append(envVars, fmt.Sprintf("GP_OPEN_EDITOR=%s", os.Getenv("GP_OPEN_EDITOR")))
			envVars = append(envVars, fmt.Sprintf("GIT_EDITOR=%s", os.Getenv("GIT_EDITOR")))
			envVars = append(envVars, fmt.Sprintf("GP_PREVIEW_BROWSER=%s", os.Getenv("GP_PREVIEW_BROWSER")))
			envVars = append(envVars, fmt.Sprintf("GP_EXTERNAL_BROWSER=%s", os.Getenv("GP_EXTERNAL_BROWSER")))

			urlCmd := exec.Command("gp", "url")
			rawUrl, err := urlCmd.Output()
			var workspaceURL *url.URL
			if err == nil {
				workspaceURL, err = url.Parse(strings.TrimSpace(string(rawUrl)))
			}
			if err == nil {
				workspaceURL.Host = "debug-" + workspaceURL.Host
			}

			runLogs := console.Observe(ctx, log, console.WorkspaceAccessInfo{
				WorkspaceURL:    workspaceURL,
				WorkspaceFolder: filepath.Join("/workspace", cfg.WorkspaceLocation),
				HTTPPort:        0,
				SSHPort:         0,
				SupervisorPort:  supervisorPort,
			}, console.ObserveOpts{
				ObserveTasks: false,
				OnTasksDone: func() {
					close(tasksDone)
				},
			})
			err = rt.StartWorkspace(ctx, ref, cfg, runtime.StartOpts{
				Network:           runtime.HostNetwork{},
				Logs:              runLogs,
				Assets:            asts,
				AdditionalEnvVars: envVars,
			})
			if err != nil {
				log.Infof("Failed to start: %v", err)
				return
			}
			runLogs.Discard()
		}()

		select {
		case <-tasksDone:
			cancel()
			log.Quit()
			rt.TerminateExistingRunGPContainer(dockerClient, ctx)
		case <-done:
			cancel()
			rt.TerminateExistingRunGPContainer(dockerClient, ctx)
			<-shutdown
		case <-shutdown:
			log.Quit()
			rt.TerminateExistingRunGPContainer(dockerClient, ctx)
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
