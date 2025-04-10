// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestOperator_Raft_RemovePeer(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &OperatorRaftRemoveCommand{}

	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := cli.NewMockUi()
	c := &OperatorRaftRemoveCommand{Meta: Meta{Ui: ui}}
	args := []string{"-address=" + addr}
	code := c.Run(args)
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), "Missing peer id required")

	ui.ErrorWriter.Reset()
	args = []string{"-address=" + addr, "-peer-id=nope"}
	code = c.Run(args)
	must.One(t, code)
	// If we get this error, it proves we sent the peer ID all the way thru
	must.StrContains(t, ui.ErrorWriter.String(), "id \"nope\" was not found in the Raft configuration")
}
