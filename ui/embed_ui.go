// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build ui

// This version of the ui package is used when building Nomad with the UI
// embedded in the binary, which is the default behavior when doing a 'make dev'
// build step.
package ui

import (
	"embed"
)

const Included = true

//go:embed dist/*
var Files embed.FS
