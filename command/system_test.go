// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
)

func TestSystemCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &SystemCommand{}
}
