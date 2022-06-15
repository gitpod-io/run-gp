// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package builder

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gitpod "github.com/gitpod-io/gitpod/gitpod-protocol"
	"github.com/gitpod-io/gitpod/run-gp/pkg/builder/assets"
)

type DockerBuilder struct {
	Workdir string
	Images  GitpodImages
}

func (db DockerBuilder) BuildImage(logs io.WriteCloser, ref string, cfg *gitpod.GitpodConfig) (err error) {
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
		assetsHeader = `
		FROM $SUPERVISOR AS supervisor
		FROM $WEBIDE AS webide
		FROM --platform=linux/amd64 $OPENVSCODE AS openvscode
		`
		assetsHeader = strings.ReplaceAll(assetsHeader, "$SUPERVISOR", db.Images.Supervisor)
		assetsHeader = strings.ReplaceAll(assetsHeader, "$WEBIDE", db.Images.WebIDE)
		assetsHeader = strings.ReplaceAll(assetsHeader, "$OPENVSCODE", db.Images.OpenVSCode)

		assetsCmds = `
		COPY --from=supervisor /.supervisor /.supervisor/
		COPY --from=openvscode --chown=33333:33333 /home/.openvscode-server /ide/
		COPY --from=webide --chown=33333:33333 /ide/startup.sh /ide/codehelper /ide/
		COPY --from=webide --chown=33333:33333 /ide/extensions/gitpod-web /ide/extensions/gitpod-web/
		RUN echo '{"entrypoint": "/ide/startup.sh", "entrypointArgs": [ "--port", "{IDEPORT}", "--host", "0.0.0.0", "--without-connection-token", "--server-data-dir", "/workspace/.vscode-remote" ]}' > /ide/supervisor-ide-config.json && \
			(echo '#!/bin/bash -li'; echo 'cd /ide || exit'; echo 'exec /ide/codehelper "$@"') > /ide/startup.sh && \
			chmod +x /ide/startup.sh && \
			mv /ide/bin/openvscode-server /ide/bin/gitpod-code
		`
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
		fc, err = ioutil.ReadFile(filepath.Join(db.Workdir, obj.Context, obj.File))
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
	`

	fmt.Fprintf(logs, "\nDockerfile:%s\n", df)

	err = ioutil.WriteFile(filepath.Join(tmpdir, "Dockerfile"), []byte(df), 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command("docker", "build", "-t", ref, "--pull=false", ".")
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
