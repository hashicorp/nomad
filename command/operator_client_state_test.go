// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestOperatorClientStateCommand(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &OperatorClientStateCommand{Meta: Meta{Ui: ui}}

	failedCode := cmd.Run([]string{"some", "bad", "args"})
	must.Eq(t, 1, failedCode)
	out := ui.ErrorWriter.String()
	must.StrContains(t, out, commandErrorText(cmd), must.Sprint("expected help output"))
	ui.ErrorWriter.Reset()

	dir := t.TempDir()

	// run against an empty client state directory
	code := cmd.Run([]string{dir})
	must.Eq(t, 0, code)
	must.StrContains(t, ui.OutputWriter.String(), "{}")

	// create a minimal client state db
	db, err := state.NewBoltStateDB(testlog.HCLogger(t), dir)
	must.NoError(t, err)
	alloc := structs.MockAlloc()
	err = db.PutAllocation(alloc)
	must.NoError(t, err)
	must.NoError(t, db.Close())

	// run against an incomplete client state directory
	code = cmd.Run([]string{dir})
	must.Eq(t, 0, code)
	must.StrContains(t, ui.OutputWriter.String(), alloc.ID)
}
