#!/usr/bin/env sh
# Copyright (c) 2022 Gitpod GmbH. All rights reserved.
# Licensed under the GNU Affero General Public License (AGPL).
# See License-AGPL.txt in the project root for license information.

set -e

ROOT="$(dirname "$(dirname "$(readlink -f "$0")")")"

sh $ROOT/scripts/build.sh
docker login --username "$DOCKER_HUB_USERNAME" --password "$DOCKER_HUB_PASSWORD"
docker push gitpod/gp-run:ak-test
