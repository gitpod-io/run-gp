// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	gitpod "github.com/gitpod-io/gitpod/gitpod-protocol"
	"github.com/gitpod-io/gitpod/run-gp/pkg/console"
	"github.com/gitpod-io/gitpod/run-gp/pkg/runtime/assets"
	"github.com/gitpod-io/gitpod/run-gp/pkg/telemetry"
)

type docker struct {
	Workdir string
	Command string
}

func (dr docker) Name() string {
	return dr.Command
}

// BuildImage builds the workspace image
func (dr docker) BuildImage(ctx context.Context, ref string, cfg *gitpod.GitpodConfig, opts BuildOpts) (err error) {
	tmpdir, err := os.MkdirTemp("", "rungp-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	defer func() {
		if err != nil && telemetry.Enabled() {
			telemetry.RecordWorkspaceFailure(telemetry.GetGitRemoteOriginURI(dr.Workdir), "build", dr.Command)
		}
	}()

	logs := opts.Logs
	if logs == nil {
		logs = console.DiscardLogs
	}

	actualAssets := opts.Assets
	if actualAssets == nil {
		actualAssets = assets.Embedded
	}

	var baseimage string
	switch img := cfg.Image.(type) {
	case nil:
		baseimage = "FROM gitpod/workspace-full:latest"
	case string:
		baseimage = "FROM " + img
	case map[string]interface{}:
		fc, err := json.Marshal(img)
		if err != nil {
			return err
		}
		var obj gitpod.Image_object
		err = json.Unmarshal(fc, &obj)
		if err != nil {
			return err
		}
		fc, err = ioutil.ReadFile(filepath.Join(dr.Workdir, obj.Context, obj.File))
		if err != nil {
			// TODO(cw): make error actionable
			return err
		}
		baseimage = "\n" + string(fc) + "\n"
	default:
		return fmt.Errorf("unsupported image: %v", img)
	}

	df := baseimage + `

	USER root
	RUN rm /usr/bin/gp-vncsession || true
	RUN mkdir -p /workspace && \
		chown -R 33333:33333 /workspace
	`
	if actualAssets != nil {
		df += strings.Join(assetEnvVars(actualAssets.EnvVars()), "\n")
	}

	fmt.Fprintf(logs, "\nDockerfile:%s\n", df)

	err = ioutil.WriteFile(filepath.Join(tmpdir, "Dockerfile"), []byte(df), 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command(dr.Command, "build", "-t", ref, ".")
	cmd.Dir = tmpdir
	cmd.Stdout = logs
	cmd.Stderr = logs

	go func() {
		<-ctx.Done()
		if proc := cmd.Process; proc != nil {
			proc.Kill()
		}
	}()

	err = cmd.Run()
	if _, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("workspace image build failed")
	} else if err != nil {
		return err
	}

	return nil
}

func assetEnvVars(input []string) []string {
	res := make([]string, 0, len(input))

	for _, env := range input {
		segs := strings.Split(env, "=")
		if len(segs) != 2 {
			continue
		}
		name, value := segs[0], segs[1]
		switch {
		case strings.HasPrefix(name, "GITPOD_ENV_SET_"):
			res = append(res, fmt.Sprintf("ENV %s=\"%s\"", strings.TrimPrefix(name, "GITPOD_ENV_SET_"), value))
		}
	}

	return res
}

// Startworkspace actually runs a workspace using a previously built image
func (dr docker) StartWorkspace(ctx context.Context, workspaceImage string, cfg *gitpod.GitpodConfig, opts StartOpts) (err error) {
	var logs io.Writer
	if opts.Logs != nil {
		logs = opts.Logs
		defer opts.Logs.Close()
	} else {
		logs = io.Discard
	}

	if cfg.CheckoutLocation == "" {
		return fmt.Errorf("missing checkout location")
	}
	if cfg.WorkspaceLocation == "" {
		return fmt.Errorf("missing workspace location")
	}

	actualAssets := opts.Assets
	if actualAssets == nil {
		actualAssets = assets.Embedded
	}
	if !actualAssets.Available() {
		return fmt.Errorf("no assets available - cannot start workspace")
	}
	aps, err := actualAssets.Access()
	if err != nil {
		return fmt.Errorf("error accessing assets: %v", err)
	}
	defer aps.Close()

	name := fmt.Sprintf("rungp-%d", time.Now().UnixNano())
	args := []string{
		"run",
		"--rm",
		"--user", "root",
		"--privileged",
		"-v", fmt.Sprintf("%s:%s", dr.Workdir, filepath.Join("/workspace", cfg.CheckoutLocation)),
		"-v", fmt.Sprintf("%s:%s", aps.IDEPath(), "/ide"),
		"-v", fmt.Sprintf("%s:%s", aps.Supervisor(), "/.supervisor"),
		"--name", name,
	}

	if opts.IDEPort > 0 {
		args = append(args, "-p", fmt.Sprintf("%d:22999", opts.IDEPort))
	}

	if (runtime.GOOS == "darwin" || runtime.GOOS == "linux") && dr.Command == "docker" {
		args = append(args, "-v", "/var/run/docker.sock:/var/run/docker.sock")
	}

	tasks, err := json.Marshal(cfg.Tasks)
	if err != nil {
		return err
	}

	envs := map[string]string{
		"GITPOD_WORKSPACE_URL":           "http://localhost",
		"GITPOD_THEIA_PORT":              "23000",
		"GITPOD_IDE_ALIAS":               "code",
		"THEIA_WORKSPACE_ROOT":           filepath.Join("/workspace", cfg.WorkspaceLocation),
		"GITPOD_REPO_ROOT":               filepath.Join("/workspace", cfg.CheckoutLocation),
		"GITPOD_PREVENT_METADATA_ACCESS": "false",
		"GITPOD_WORKSPACE_ID":            "a-random-name",
		"GITPOD_TASKS":                   string(tasks),
		"GITPOD_HEADLESS":                "false",
		"GITPOD_HOST":                    "gitpod.local",
		"THEIA_SUPERVISOR_TOKENS":        `{"token": "invalid","kind": "gitpod","host": "gitpod.local","scope": [],"expiryDate": ` + time.Now().Format(time.RFC3339) + `,"reuse": 2}`,
		"VSX_REGISTRY_URL":               "https://https://open-vsx.org/",
	}
	tmpf, err := ioutil.TempFile("", "rungp-*.env")
	if err != nil {
		return err
	}
	for k, v := range envs {
		tmpf.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}
	for _, env := range opts.AdditionalEnvVars {
		tmpf.WriteString(env + "\n")
	}
	tmpf.Close()
	args = append(args, "--env-file", tmpf.Name())
	defer os.Remove(tmpf.Name())

	if opts.SSHPublicKey != "" {
		tmpf, err := ioutil.TempFile("", "rungp-*.pub")
		if err != nil {
			return err
		}
		tmpf.WriteString(opts.SSHPublicKey)
		tmpf.Close()
		args = append(args, "-v", fmt.Sprintf("%s:/home/gitpod/.ssh/authorized_keys", tmpf.Name()))
		defer os.Remove(tmpf.Name())
	}
	if opts.SSHPort > 0 {
		args = append(args, "-p", fmt.Sprintf("%d:23001", opts.SSHPort))
	}

	if !opts.NoPortForwarding {
		for _, p := range cfg.Ports {
			args = append(args, "-p", fmt.Sprintf("%d:%d", p.Port.(int)+opts.PortOffset, p.Port))
		}
	}

	args = append(args, workspaceImage)
	args = append(args, "/.supervisor/supervisor", "run", "--rungp")

	if telemetry.Enabled() {
		telemetry.RecordWorkspaceStarted(telemetry.GetGitRemoteOriginURI(dr.Workdir), dr.Command)
	}

	cmd := exec.Command(dr.Command, args...)
	cmd.Dir = dr.Workdir
	cmd.Stdout = logs
	cmd.Stderr = logs

	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			cmd.Process.Kill()
		}

		exec.Command(dr.Command, "kill", name).CombinedOutput()

		if err != nil && telemetry.Enabled() {
			telemetry.RecordWorkspaceFailure(telemetry.GetGitRemoteOriginURI(dr.Workdir), "start", dr.Command)
		}
	}()

	return cmd.Run()
}
