# Copyright (c) 2022 Gitpod GmbH. All rights reserved.
# Licensed under the GNU Affero General Public License (AGPL).
# See License-AGPL.txt in the project root for license information.

FROM alpine:3.16 as base_builder
RUN mkdir /ide

# for debugging
# FROM alpine:3.16
FROM scratch
# ensures right permissions for /ide
COPY --from=base_builder --chown=33333:33333 /ide/ /ide/
COPY --chown=33333:33333 run-gp /ide/run-gp
COPY --chown=33333:33333 bin/gp-run.sh /ide/bin/run-gp-cli/gp-run

ENV GITPOD_ENV_APPEND_PATH=/ide/bin/run-gp-cli: