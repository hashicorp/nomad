// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ui

// This version of the ui package is used when building Nomad without the UI
// embeded in the binary, which can be done by setting NOMAD_NO_UI=1 before
// doing a 'make dev' build step.
package ui

import (
	"embed"
)

const Included = false

//go:embed empty/index.html
var Files embed.FS
