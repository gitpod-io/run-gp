// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package builder

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gitpod "github.com/gitpod-io/gitpod/gitpod-protocol"
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

	df := `
	FROM $SUPERVISOR AS supervisor
	FROM $WEBIDE AS webide
	FROM --platform=linux/amd64 $OPENVSCODE AS openvscode

	$BASEIMAGE

	COPY --from=supervisor /.supervisor /.supervisor/
	COPY --from=openvscode --chown=33333:33333 /home/.openvscode-server /ide/
	COPY --from=webide --chown=33333:33333 /ide/startup.sh /ide/codehelper /ide/
	COPY --from=webide --chown=33333:33333 /ide/extensions/gitpod-web /ide/extensions/gitpod-web/
	RUN echo '{"entrypoint": "/ide/startup.sh", "entrypointArgs": [ "--port", "{IDEPORT}", "--host", "0.0.0.0", "--without-connection-token", "--server-data-dir", "/workspace/.vscode-remote" ]}' > /ide/supervisor-ide-config.json && \
		(echo '#!/bin/bash -li'; echo 'cd /ide || exit'; echo 'exec /ide/codehelper "$@"') > /ide/startup.sh && \
		chmod +x /ide/startup.sh && \
		mv /ide/bin/openvscode-server /ide/bin/gitpod-code


	USER root
	RUN rm /usr/bin/gp-vncsession || true
	RUN mkdir -p /workspace && \
		chown -R 33333:33333 /workspace
	`
	df = strings.ReplaceAll(df, "$SUPERVISOR", db.Images.Supervisor)
	df = strings.ReplaceAll(df, "$WEBIDE", db.Images.WebIDE)
	df = strings.ReplaceAll(df, "$OPENVSCODE", db.Images.OpenVSCode)

	var baseimage string
	switch img := cfg.Image.(type) {
	case nil:
		baseimage = "FROM gitpod/workspace-full:latest"
	case string:
		baseimage = img
	case gitpod.Image_object:
		fc, err := ioutil.ReadFile(filepath.Join(db.Workdir, img.Context, img.File))
		if err != nil {
			// TODO(cw): make error actionable
			return err
		}
		baseimage = "\n" + string(fc) + "\n"
	}
	df = strings.ReplaceAll(df, "$BASEIMAGE", baseimage)

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
