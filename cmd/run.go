// Copyright (c) 2020 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitpod-io/gitpod/run-gp/pkg/console"
	"github.com/gitpod-io/gitpod/run-gp/pkg/runtime"
	"github.com/gitpod-io/gitpod/run-gp/pkg/telemetry"
	"github.com/gitpod-io/gitpod/run-gp/pkg/update"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Starts a workspace",

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
			ref := filepath.Join("workspace-image:latest")
			bldLog := log.Writer()
			err = rt.BuildImage(ctx, ref, cfg, runtime.BuildOpts{
				Logs: bldLog,
			})
			if err != nil {
				buildingPhase.Failure(err.Error())
				return
			}
			bldLog.Discard()
			buildingPhase.Success()

			var (
				publicSSHKey   string
				publicSSHKeyFN = runOpts.SSHPublicKeyPath
			)
			if strings.HasPrefix(publicSSHKeyFN, "~") {
				home, err := os.UserHomeDir()
				if err != nil {
					log.Warnf("cannot find user home directory: %v", err)
					return
				}
				publicSSHKeyFN = filepath.Join(home, strings.TrimPrefix(publicSSHKeyFN, "~"))
			}

			if fc, err := ioutil.ReadFile(publicSSHKeyFN); err == nil {
				publicSSHKey = string(fc)
			} else if rootOpts.Verbose {
				log.Warnf("cannot read public SSH key from %s: %v", publicSSHKeyFN, err)
			}

			recordFailure := func() {
				if !telemetry.Enabled() {
					return
				}

				telemetry.RecordWorkspaceFailure(telemetry.GetGitRemoteOriginURI(rootOpts.Workdir), "running", rt.Name())
			}

			runLogs := console.Observe(log, console.WorkspaceAccessInfo{
				WorkspaceFolder: filepath.Join("/workspace", cfg.WorkspaceLocation),
				HTTPPort:        runOpts.StartOpts.IDEPort,
				SSHPort:         runOpts.StartOpts.SSHPort,
			}, recordFailure)
			opts := runOpts.StartOpts
			opts.Logs = runLogs
			opts.SSHPublicKey = publicSSHKey
			err := rt.StartWorkspace(ctx, ref, cfg, opts)
			if err != nil {
				return
			}
			runLogs.Discard()
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

var runOpts struct {
	StartOpts        runtime.StartOpts
	SSHPublicKeyPath string
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().BoolVar(&runOpts.StartOpts.NoPortForwarding, "no-port-forwarding", false, "disable port-forwarding for ports in the .gitpod.yml")
	runCmd.Flags().IntVar(&runOpts.StartOpts.PortOffset, "port-offset", 0, "shift exposed ports by this number")
	runCmd.Flags().IntVar(&runOpts.StartOpts.IDEPort, "ide-port", 8080, "port to expose open vs code server")
	runCmd.Flags().IntVar(&runOpts.StartOpts.SSHPort, "ssh-port", 8082, "port to expose SSH on (set to 0 to disable SSH)")
	runCmd.Flags().StringVar(&runOpts.SSHPublicKeyPath, "ssh-public-key-path", "~/.ssh/id_rsa.pub", "path to the user's public SSH key")
}
