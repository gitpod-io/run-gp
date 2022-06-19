#!/bin/bash
# Copyright (c) 2022 Gitpod GmbH. All rights reserved.
# Licensed under the GNU Affero General Public License (AGPL).
# See License-AGPL.txt in the project root for license information.

lpwd="$(pwd)"
if ! [ -x "$(command -v addlicense)" ]; then
    tmpdir="$(mktemp -d)"
    cd "${tmpdir}"
    git clone https://github.com/gitpod-io/gitpod.git .
    cd dev/addlicense
    go install
    rm -rf "${tmpdir}"
fi
cd "${lpwd}"
pwd

addlicense "$(realpath "$(dirname "$0")/..")"