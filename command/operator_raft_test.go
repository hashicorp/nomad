// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
)

func TestOperator_Raft_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &OperatorRaftCommand{}
}
