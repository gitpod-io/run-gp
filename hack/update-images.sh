#!/bin/bash

if [ $# -eq 0 ]; then
    echo "usage: $0 <gitpod-version>"
    exit 1
fi

temp_file="$(mktemp)"

cat <<EOF > "$temp_file"
#!/bin/sh
apk add --no-cache yq jq moreutils curl gcompat

export supervisor=\$(yq '.components.workspace.supervisor.version' < versions.yaml)
export webide="\$(yq '.components.workspace.codeImage.version' < versions.yaml)"

curl -qL https://github.com/csweichel/oci-tool/releases/download/v0.2.0/oci-tool_0.2.0_linux_amd64.tar.gz | tar xzv
export openvscode="\$(./oci-tool resolve name docker.io/gitpod/openvscode-server:latest)"

echo
echo "supervisor: \${supervisor}"
echo "web IDE: \${webide}"
echo "Ppen VS Code server: \${openvscode}"


jq '.supervisor="eu.gcr.io/gitpod-core-dev/build/supervisor:'\${supervisor}'"' /wd/images.json | sponge /wd/images.json
jq '."gitpod-code"="eu.gcr.io/gitpod-core-dev/build/ide/code:'\${webide}'"' /wd/images.json | sponge /wd/images.json
jq '."open-vscode"="'\${openvscode}'"' /wd/images.json | sponge /wd/images.json
EOF

echo "$temp_file"

wd="$(realpath $(dirname $0)/../pkg/runtime/assets)"
docker run --rm -it -v "$wd:/wd" -v "$temp_file:/run.sh" "eu.gcr.io/gitpod-core-dev/build/versions:$1" sh /run.sh