// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/ci"
)

func TestSentinelReadCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &SentinelReadCommand{}
}
