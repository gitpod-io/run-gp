// Copyright (c) 2020 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gitpod-io/gitpod/run-gp/pkg/console"
	"github.com/gitpod-io/gitpod/run-gp/pkg/runtime"
	"github.com/gitpod-io/gitpod/run-gp/pkg/update"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Starts a workspace",

	RunE: func(cmd *cobra.Command, args []string) error {
		log := console.Default

		printBanner()

		if rootOpts.cfg.AutoUpdate {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := update.Update(ctx, version, update.NewGitHubReleaseDiscovery(ctx), rootOpts.cfg.Filename)
			if err != nil {
				log.Warnf("failed to auto-update: %v", err)
			}
		}

		cfg, err := getGitpodYaml()
		if err != nil {
			return err
		}

		runtime, err := getRuntime(rootOpts.Workdir)
		if err != nil {
			return err
		}

		buildingPhase := log.StartPhase("[building]", "workspace image")
		ref := filepath.Join("local/workspace-image:latest")
		bldLog := log.Writer()
		err = runtime.BuildImage(bldLog, ref, cfg)
		if err != nil {
			buildingPhase.Failure(err.Error())
			bldLog.Show()
			return err
		}
		buildingPhase.Success()

		var (
			publicSSHKey   string
			publicSSHKeyFN = runOpts.SSHPublicKeyPath
		)
		if strings.HasPrefix(publicSSHKeyFN, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			publicSSHKeyFN = filepath.Join(home, strings.TrimPrefix(publicSSHKeyFN, "~"))
		}

		if fc, err := ioutil.ReadFile(publicSSHKeyFN); err == nil {
			publicSSHKey = string(fc)
		} else if rootOpts.Verbose {
			log.Warnf("cannot read public SSH key from %s: %v", publicSSHKeyFN, err)
		}

		shutdown := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			runLogs := console.Observe(log, filepath.Join("/workspace", cfg.WorkspaceLocation))
			opts := runOpts.StartOpts
			opts.Logs = runLogs
			opts.SSHPublicKey = publicSSHKey
			err := runtime.StartWorkspace(ctx, ref, cfg, opts)
			if err != nil {
				runLogs.Show()
				close(shutdown)
				return
			}
		}()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		select {
		case <-sigChan:
			// give things a change to shut down
			log.Infof("\nReceived SIGTERM, shutting down")
			cancel()
			time.Sleep(1 * time.Second)
		case <-shutdown:
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
