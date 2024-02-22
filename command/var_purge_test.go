// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/shoenig/test/must"
)

func TestVarPurgeCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VarPurgeCommand{}
}

func TestVarPurgeCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	t.Run("bad_args", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{"some", "bad", "args"})
		out := ui.ErrorWriter.String()
		must.One(t, code)
		must.StrContains(t, out, commandErrorText(cmd))
	})
	t.Run("bad_address", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{"-address=nope", "foo"})
		must.One(t, code)
		must.StrContains(t, ui.ErrorWriter.String(), "purging variable")
		must.Eq(t, "", ui.OutputWriter.String())
	})
	t.Run("bad_check_index/syntax", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{`-check-index=a`, "foo"})
		out := strings.TrimSpace(ui.ErrorWriter.String())
		must.One(t, code)
		must.Eq(t, `Invalid -check-index value "a": not parsable as uint64`, out)
		must.Eq(t, "", ui.OutputWriter.String())
	})
	t.Run("bad_check_index/range", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{`-check-index=18446744073709551616`, "foo"})
		out := strings.TrimSpace(ui.ErrorWriter.String())
		must.One(t, code)
		must.Eq(t, `Invalid -check-index value "18446744073709551616": out of range for uint64`, out)
		must.Eq(t, "", ui.OutputWriter.String())
	})
}

func TestVarPurgeCommand_Online(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	t.Run("unchecked", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}

		// Create a var to delete
		sv := testVariable()
		_, _, err := client.Variables().Create(sv, nil)
		must.NoError(t, err)
		t.Cleanup(func() { _, _ = client.Variables().Delete(sv.Path, nil) })

		// Delete the variable
		code := cmd.Run([]string{"-address=" + url, sv.Path})
		must.Zero(t, code)

		vars, _, err := client.Variables().List(nil)
		must.NoError(t, err)
		must.SliceEmpty(t, vars)
	})

	t.Run("unchecked", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}

		// Create a var to delete
		sv := testVariable()
		sv, _, err := client.Variables().Create(sv, nil)
		must.NoError(t, err)

		// Delete a variable
		code := cmd.Run([]string{"-address=" + url, "-check-index=1", sv.Path})
		stderr := ui.ErrorWriter.String()
		must.One(t, code)
		must.StrContains(t, stderr, "\nCheck-and-Set conflict\n\n    Your provided check-index (1)")

		code = cmd.Run([]string{"-address=" + url, fmt.Sprintf("-check-index=%v", sv.ModifyIndex), sv.Path})
		must.Zero(t, code)

		vars, _, err := client.Variables().List(nil)
		must.NoError(t, err)
		must.SliceEmpty(t, vars)
	})

	t.Run("autocompleteArgs", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui, flagAddress: url}}

		// Create a var
		sv := testVariable()
		sv.Path = "autocomplete/test"
		_, _, err := client.Variables().Create(sv, nil)
		must.NoError(t, err)
		t.Cleanup(func() { client.Variables().Delete(sv.Path, nil) })

		args := complete.Args{Last: "aut"}
		predictor := cmd.AutocompleteArgs()

		res := predictor.Predict(args)
		must.Len(t, 1, res)
		must.Eq(t, sv.Path, res[0])
	})
}
