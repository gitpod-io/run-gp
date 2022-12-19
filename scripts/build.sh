#!/usr/bin/env sh
# Copyright (c) 2022 Gitpod GmbH. All rights reserved.
# Licensed under the GNU Affero General Public License (AGPL).
# See License-AGPL.txt in the project root for license information.

set -e

ROOT="$(dirname "$(dirname "$(readlink -f "$0")")")"

go build -o run-gp .
docker build -t gitpod/gp-run:ak-test $ROOT
