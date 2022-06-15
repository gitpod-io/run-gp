// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package builder

import (
	_ "embed"
	"encoding/json"
)

type GitpodImages struct {
	Supervisor string `json:"supervisor"`
	WebIDE     string `json:"gitpod-code"`
	OpenVSCode string `json:"open-vscode"`
}

var (
	//go:embed images.json
	imagesFC string

	DefaultImages GitpodImages
)

func init() {
	err := json.Unmarshal([]byte(imagesFC), &DefaultImages)
	if err != nil {
		panic(err)
	}
}
