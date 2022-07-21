#!/bin/bash
# Copyright (c) 2022 Gitpod GmbH. All rights reserved.
# Licensed under the GNU Affero General Public License (AGPL).
# See License-AGPL.txt in the project root for license information.


if [ $# -eq 0 ]; then
    echo "use \"$0 <gitpod-version>\" to pass a version - trying to find one now"
    if [ ! -f "/tmp/werft" ]; then
        curl -L https://github.com/csweichel/werft/releases/download/v0.3.5/werft-client-linux-amd64.tar.gz | tar xz; chmod +x werft-client-linux-amd64 && mv werft-client-linux-amd64 /tmp/werft
    fi

    version="$(/tmp/werft --host werft-grpc.gitpod-dev.com:443 --tls-mode system job list  --order created:desc --limit 100 -o json | jq -r '.result[] | select((.phase == "PHASE_DONE") and (.metadata.repository.owner == "gitpod-io") and (.metadata.repository.repo == "gitpod") and (.metadata.repository.ref == "refs/heads/main") and (.conditions.success == true)) | .results[] | .payload'  | grep /versions: | head -n 1)"
    echo "found version: $version"
    echo
else
    version="eu.gcr.io/gitpod-core-dev/build/versions:$1"
fi



temp_file="$(mktemp)"

cat <<EOF > "$temp_file"
#!/bin/sh
apk add --no-cache yq jq moreutils curl gcompat

export supervisor=\$(yq '.components.workspace.supervisor.version' < versions.yaml)
export webide="\$(yq '.components.workspace.codeImage.version' < versions.yaml)"

curl -qL https://github.com/csweichel/oci-tool/releases/download/v0.2.0/oci-tool_0.2.0_linux_amd64.tar.gz | tar xzv
export openvscode="\$(./oci-tool resolve name docker.io/gitpod/openvscode-server:latest)"

export supervisorImage='eu.gcr.io/gitpod-core-dev/build/supervisor:'\${supervisor}
export webideImage='eu.gcr.io/gitpod-core-dev/build/ide/code:'\${webide}

echo
echo "supervisor: \${supervisorImage}"
echo "web IDE: \${webideImage}"
echo "Ppen VS Code server: \${openvscode}"

echo '{"supervisor": "", "gitpod-code":"", "open-vscode":"", "envs":[]}' > /wd/images.json
jq '.supervisor="'\${supervisorImage}'"' /wd/images.json | sponge /wd/images.json
jq '."gitpod-code"="'\${webideImage}'"' /wd/images.json | sponge /wd/images.json
jq '."open-vscode"="'\${openvscode}'"' /wd/images.json | sponge /wd/images.json
./oci-tool fetch image "\${webideImage}" | jq '{envs: .config.Env}' | jq -s '.[0] * .[1]' /wd/images.json - | sponge /wd/images.json
EOF

echo "$temp_file"

wd="$(realpath $(dirname $0)/../pkg/runtime/assets)"
docker run --rm -it -v "$wd:/wd" -v "$temp_file:/run.sh" "$version" sh /run.sh