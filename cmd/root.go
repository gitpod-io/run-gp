package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	gitpod "github.com/gitpod-io/gitpod/gitpod-protocol"
	"github.com/gitpod-io/gitpod/run-gp/pkg/builder"
	"github.com/gitpod-io/gitpod/run-gp/pkg/runtime"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rungp",
	Short: "start a local dev-environment using a .gitpdod.yaml file",
}

var rootOpts struct {
	Workdir      string
	GitpodYamlFN string
	Verbose      bool
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
}

func getConfig() (*gitpod.GitpodConfig, error) {
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

func getBuilder(workdir string) (builder.Builder, error) {
	return &builder.DockerBuilder{
		Workdir: workdir,
		Images:  builder.DefaultImages,
	}, nil
}

func getRuntime(workdir string) (runtime.Runtime, error) {
	return &runtime.DockerRuntime{Workdir: workdir}, nil
}
