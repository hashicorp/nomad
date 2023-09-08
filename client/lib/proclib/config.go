// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proclib

import (
	"github.com/hashicorp/go-hclog"
)

// Configs is used to pass along values from client configuration that are
// build-tag specific. These are not the final representative values, just what
// was set in agent configuration.
type Configs struct {
	Logger hclog.Logger
}
