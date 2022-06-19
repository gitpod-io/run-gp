#!/bin/sh

tmpdir="$(mktemp -d)"
bldname="assets$(date +%s)"
base="$(dirname "$0")/.."

SUPERVISOR="$(jq -r '.supervisor' "$base/pkg/runtime/assets/images.json")"
WEBIDE="$(jq -r '."gitpod-code"' "$base/pkg/runtime/assets/images.json")"
OPENVSCODE="$(jq -r '."open-vscode"' "$base/pkg/runtime/assets/images.json")"

cat <<EOF > "$tmpdir/Dockerfile"
FROM $SUPERVISOR AS supervisor
FROM $WEBIDE AS webide
FROM --platform=linux/amd64 $OPENVSCODE AS openvscode

FROM alpine:3.16 AS staging

RUN mkdir /staging
COPY --from=supervisor /.supervisor /staging/supervisor/
COPY --from=openvscode --chown=33333:33333 /home/.openvscode-server /staging/ide/
COPY --from=webide --chown=33333:33333 /ide/startup.sh /ide/codehelper /staging/ide/
COPY --from=webide --chown=33333:33333 /ide/extensions/gitpod-web /staging/ide/extensions/gitpod-web/
RUN echo '{"entrypoint": "/ide/startup.sh", "entrypointArgs": [ "--port", "{IDEPORT}", "--host", "0.0.0.0", "--without-connection-token", "--server-data-dir", "/workspace/.vscode-remote" ]}' > /staging/ide/supervisor-ide-config.json && \
    (echo '#!/bin/bash -li'; echo 'cd /ide || exit'; echo 'exec /ide/codehelper "\$@"') > /staging/ide/startup.sh && \
    chmod +x /staging/ide/startup.sh && \
    mv /staging/ide/bin/openvscode-server /staging/ide/bin/gitpod-code

RUN cd /staging && tar cvvfz /assets.tar.gz .

FROM alpine:3.16
COPY --from=staging /assets.tar.gz /

EOF

docker build -t "$bldname" "$tmpdir"
docker run --rm -i -v "$(realpath "$base/pkg/runtime/assets"):/out" "$bldname" cp /assets.tar.gz /out/assets.tar.gz