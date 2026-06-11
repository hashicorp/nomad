// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/v2/ci"
)

func TestAllocPauseCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &AllocPauseCommand{}
}
