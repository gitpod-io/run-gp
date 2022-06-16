// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	gitpod "github.com/gitpod-io/gitpod/gitpod-protocol"
	"github.com/gitpod-io/gitpod/run-gp/pkg/config"
	"github.com/gitpod-io/gitpod/run-gp/pkg/console"
	"github.com/gitpod-io/gitpod/run-gp/pkg/runtime"
	"github.com/gitpod-io/gitpod/run-gp/pkg/telemetry"
)

var banner = `    
   _______  ______     ____ _____ 
  / ___/ / / / __ \   / __ ` + "`" + `/ __ \
 / /  / /_/ / / / /  / /_/ / /_/ /
/_/   \__,_/_/ /_/   \__, / .___/ 
                     /____/_/      
`

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rungp",
	Short: "start a local dev-environment using a .gitpdod.yaml file",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		console.Init(rootOpts.Verbose)

		cfg, err := config.ReadInConfig()
		if err != nil {
			console.Default.Warnf("%v", err)
		}
		if cfg == nil {
			cfg = &config.Config{
				AutoUpdate: true,
			}
		}

		telemetry.Init(cfg.Telemetry.Disabled || rootOpts.DisableTelemetry, cfg.Telemetry.Identity)
		if cfg.Telemetry.Identity == "" {
			cfg.Telemetry.Identity = telemetry.Identity()
			err := cfg.Write()
			if err != nil {
				console.Default.Warnf("cannot write config file: %v", err)
			}

			console.Default.Debugf("produced new telemetry identity: %s", cfg.Telemetry.Identity)
		}

		rootOpts.cfg = cfg
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		telemetry.Close()
	},
}

func printBanner() {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return
	}
	var res string

	lines := strings.Split(banner, "\n")
	start, _ := pterm.NewRGBFromHEX("#ff8a00")
	end, _ := pterm.NewRGBFromHEX("#ffbe5c")
	for _, line := range lines {
		for i := range line {
			res += start.Fade(0, float32(len(line)), float32(i), end).Sprint(line[i : i+1])
		}
		res += "\n"
	}
	fmt.Println(res)
}

var rootOpts struct {
	Workdir          string
	GitpodYamlFN     string
	Verbose          bool
	DisableTelemetry bool
	Runtime          string

	cfg *config.Config
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cmd, _, err := rootCmd.Find(os.Args[1:])
	// default cmd if no cmd is given
	if err == nil && cmd.Use == rootCmd.Use && cmd.Flags().Parse(os.Args[1:]) != pflag.ErrHelp {
		args := append([]string{runCmd.Use}, os.Args[1:]...)
		rootCmd.SetArgs(args)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	rootCmd.PersistentFlags().StringVarP(&rootOpts.Workdir, "workdir", "w", wd, "Path to the working directory")
	rootCmd.PersistentFlags().StringVarP(&rootOpts.GitpodYamlFN, "gitpod-yaml", "f", ".gitpod.yml", "path to the .gitpod.yml file relative to the working directory")
	rootCmd.PersistentFlags().BoolVarP(&rootOpts.Verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&rootOpts.DisableTelemetry, "disable-telemetry", false, "disable telemetry")
	rootCmd.PersistentFlags().StringVar(&rootOpts.Runtime, "runtime", "auto", "container runtime to use")
}

func getGitpodYaml() (*gitpod.GitpodConfig, error) {
	fn := filepath.Join(rootOpts.Workdir, rootOpts.GitpodYamlFN)
	fc, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	var cfg gitpod.GitpodConfig
	err = yaml.Unmarshal(fc, &cfg)
	if err != nil {
		return nil, fmt.Errorf("unmarshal .gitpod.yml file failed: %v", err)
	}

	return &cfg, nil
}

func getRuntime(workdir string) (runtime.RuntimeBuilder, error) {
	var rt runtime.SupportedRuntime
	switch rootOpts.Runtime {
	case "auto":
		rt = runtime.AutodetectRuntime
	case "docker":
		rt = runtime.DockerRuntime
	case "nerdctl":
		rt = runtime.NerdctlRuntime
	default:
		return nil, fmt.Errorf("unsupported value for --runtime: %s. Only auto, docker and nerdctl are supported", rootOpts.Runtime)
	}

	return runtime.New(workdir, rt)
}
