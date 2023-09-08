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
	"github.com/stretchr/testify/require"
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
		require.Equal(t, 1, code, "expected exit code 1, got: %d")
		require.Contains(t, out, commandErrorText(cmd), "expected help output, got: %s", out)
	})
	t.Run("bad_address", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{"-address=nope", "foo"})
		out := ui.ErrorWriter.String()
		require.Equal(t, 1, code, "expected exit code 1, got: %d")
		require.Contains(t, ui.ErrorWriter.String(), "purging variable", "connection error, got: %s", out)
		require.Zero(t, ui.OutputWriter.String())
	})
	t.Run("bad_check_index/syntax", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{`-check-index=a`, "foo"})
		out := strings.TrimSpace(ui.ErrorWriter.String())
		require.Equal(t, 1, code, "expected exit code 1, got: %d", code)
		require.Equal(t, `Invalid -check-index value "a": not parsable as uint64`, out)
		require.Zero(t, ui.OutputWriter.String())
	})
	t.Run("bad_check_index/range", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}
		code := cmd.Run([]string{`-check-index=18446744073709551616`, "foo"})
		out := strings.TrimSpace(ui.ErrorWriter.String())
		require.Equal(t, 1, code, "expected exit code 1, got: %d", code)
		require.Equal(t, `Invalid -check-index value "18446744073709551616": out of range for uint64`, out)
		require.Zero(t, ui.OutputWriter.String())
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
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = client.Variables().Delete(sv.Path, nil) })

		// Delete the variable
		code := cmd.Run([]string{"-address=" + url, sv.Path})
		require.Equal(t, 0, code, "expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())

		vars, _, err := client.Variables().List(nil)
		require.NoError(t, err)
		require.Len(t, vars, 0)
	})

	t.Run("unchecked", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}

		// Create a var to delete
		sv := testVariable()
		sv, _, err := client.Variables().Create(sv, nil)
		require.NoError(t, err)

		// Delete a variable
		code := cmd.Run([]string{"-address=" + url, "-check-index=1", sv.Path})
		stderr := ui.ErrorWriter.String()
		require.Equal(t, 1, code, "expected exit 1, got: %d; %v", code, stderr)
		require.Contains(t, stderr, "\nCheck-and-Set conflict\n\n    Your provided check-index (1)")

		code = cmd.Run([]string{"-address=" + url, fmt.Sprintf("-check-index=%v", sv.ModifyIndex), sv.Path})
		require.Equal(t, 0, code, "expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())

		vars, _, err := client.Variables().List(nil)
		require.NoError(t, err)
		require.Len(t, vars, 0)
	})

	t.Run("autocompleteArgs", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarPurgeCommand{Meta: Meta{Ui: ui, flagAddress: url}}

		// Create a var
		sv := testVariable()
		sv.Path = "autocomplete/test"
		_, _, err := client.Variables().Create(sv, nil)
		require.NoError(t, err)
		t.Cleanup(func() { client.Variables().Delete(sv.Path, nil) })

		args := complete.Args{Last: "aut"}
		predictor := cmd.AutocompleteArgs()

		res := predictor.Predict(args)
		require.Equal(t, 1, len(res))
		require.Equal(t, sv.Path, res[0])
	})
}
