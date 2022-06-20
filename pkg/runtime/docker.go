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
	"strings"
	"time"

	gitpod "github.com/gitpod-io/gitpod/gitpod-protocol"
	"github.com/gitpod-io/gitpod/run-gp/pkg/runtime/assets"
	"github.com/gitpod-io/gitpod/run-gp/pkg/telemetry"
)

type docker struct {
	Workdir string
	Command string
}

// BuildImage builds the workspace image
func (dr docker) BuildImage(logs io.WriteCloser, ref string, cfg *gitpod.GitpodConfig) (err error) {
	tmpdir, err := os.MkdirTemp("", "rungp-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	var (
		assetsHeader string
		assetsCmds   string
	)
	if assets.IsEmbedded() {
		err := assets.Extract(tmpdir)
		if err != nil {
			return err
		}
		assetsCmds = `
		COPY supervisor/ /.supervisor/
		COPY ide/ /ide/
		`
	} else {
		return fmt.Errorf("missing assets - please make sure you ran go:generate before")
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

	df := `
	` + assetsHeader + `
	` + baseimage + `
	` + assetsCmds + `

	USER root
	RUN rm /usr/bin/gp-vncsession || true
	RUN mkdir -p /workspace && \
		chown -R 33333:33333 /workspace

	RUN ln -s /usr/bin/dockerd /usr/bin/docker-up
	`

	fmt.Fprintf(logs, "\nDockerfile:%s\n", df)

	err = ioutil.WriteFile(filepath.Join(tmpdir, "Dockerfile"), []byte(df), 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command(dr.Command, "build", "-t", ref, ".")
	cmd.Dir = tmpdir
	cmd.Stdout = logs
	cmd.Stderr = logs
	err = cmd.Run()
	if _, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("workspace image build failed")
	} else if err != nil {
		return err
	}

	return nil
}

// Startworkspace actually runs a workspace using a previously built image
func (dr docker) StartWorkspace(ctx context.Context, workspaceImage string, cfg *gitpod.GitpodConfig, opts StartOpts) error {
	var logs io.Writer
	if opts.Logs != nil {
		logs = opts.Logs
		defer opts.Logs.Close()
	} else {
		logs = io.Discard
	}

	checkoutLocation := cfg.CheckoutLocation
	if checkoutLocation == "" {
		checkoutLocation = filepath.Base(dr.Workdir)
	}
	workspaceLocation := cfg.WorkspaceLocation
	if workspaceLocation == "" {
		workspaceLocation = checkoutLocation
	}

	name := fmt.Sprintf("rungp-%d", time.Now().UnixNano())
	args := []string{"run", "--rm", "--user", "root", "--privileged", "-p", fmt.Sprintf("%d:22999", opts.IDEPort), "-v", fmt.Sprintf("%s:%s", dr.Workdir, filepath.Join("/workspace", checkoutLocation)), "--name", name}

	tasks, err := json.Marshal(cfg.Tasks)
	if err != nil {
		return err
	}

	envs := map[string]string{
		"GITPOD_WORKSPACE_URL":           "http://localhost",
		"GITPOD_THEIA_PORT":              "23000",
		"GITPOD_IDE_ALIAS":               "code",
		"THEIA_WORKSPACE_ROOT":           filepath.Join("/workspace", workspaceLocation),
		"GITPOD_REPO_ROOT":               filepath.Join("/workspace", checkoutLocation),
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
		git := exec.Command("git", "remote", "get-uri", "origin")
		git.Dir = dr.Workdir
		gitout, _ := git.CombinedOutput()

		telemetry.RecordWorkspaceStarted(strings.TrimSpace(string(gitout)), dr.Command)
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
	}()

	return cmd.Run()
}
