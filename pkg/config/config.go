// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gitpod-io/gitpod/run-gp/pkg/console"
	"gopkg.in/yaml.v3"
)

// Config configures run-gp
type Config struct {
	Filename string `yaml:"-"`

	Telemetry struct {
		Disabled bool   `json:"disabled,omitempty"`
		Identity string `json:"identity,omitempty"`
	} `yaml:"telemetry,omitempty"`
}

var paths = []func() (string, error){
	func() (string, error) {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "run-gp", "config.yaml"), nil
	},
	func() (string, error) {
		base, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, ".run-gp", "config.yaml"), nil
	},
	func() (string, error) {
		if runtime.GOOS == "linux" {
			return "/etc/run-gp/config.yaml", nil
		}
		return "", nil
	},
	func() (string, error) {
		return os.Getenv("RUNGP_CONFIG_PATH"), nil
	},
}

// ReadInConfig tries to read the config from several paths.
// The first path is used.
func ReadInConfig() (*Config, error) {
	var fn string
	for _, pf := range paths {
		path, err := pf()
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		fn = path
		break
	}
	if fn == "" {
		return nil, nil
	}

	fc, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file from %s: %v", fn, err)
	}
	var cfg Config
	err = yaml.Unmarshal(fc, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file: %w", err)
	}
	cfg.Filename = fn

	console.Default.Debugf("read config file: %s", cfg.Filename)

	return &cfg, nil
}

// Write writes the config file back
func (cfg *Config) Write() error {
	if cfg.Filename == "" {
		p, err := paths[0]()
		if err != nil {
			return fmt.Errorf("cannot determine config file name: %w", err)
		}
		cfg.Filename = p
	}

	fc, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(cfg.Filename), 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	err = ioutil.WriteFile(cfg.Filename, fc, 0644)
	if err != nil {
		return err
	}
	console.Default.Debugf("wrote config file: %v", cfg.Filename)
	return nil
}
