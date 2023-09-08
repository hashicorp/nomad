// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"path"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestVarInitCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VarInitCommand{}
}

func TestVarInitCommand_Run(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(dir)
	require.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origDir) })

	t.Run("hcl", func(t *testing.T) {
		ci.Parallel(t)
		dir := dir
		ui := cli.NewMockUi()
		cmd := &VarInitCommand{Meta: Meta{Ui: ui}}

		// Fails on misuse
		ec := cmd.Run([]string{"some", "bad", "args"})
		require.Equal(t, 1, ec)
		require.Contains(t, ui.ErrorWriter.String(), commandErrorText(cmd))
		require.Empty(t, ui.OutputWriter.String())
		reset(ui)

		// Works if the file doesn't exist
		ec = cmd.Run([]string{"-out", "hcl"})
		require.Empty(t, ui.ErrorWriter.String())
		require.Equal(t, "Example variable specification written to spec.nv.hcl\n", ui.OutputWriter.String())
		require.Zero(t, ec)
		reset(ui)
		t.Cleanup(func() { os.Remove(path.Join(dir, "spec.nv.hcl")) })

		content, err := os.ReadFile(DefaultHclVarInitName)
		require.NoError(t, err)
		require.Equal(t, defaultHclVarSpec, string(content))

		// Fails if the file exists
		ec = cmd.Run([]string{"-out", "hcl"})
		require.Contains(t, ui.ErrorWriter.String(), "exists")
		require.Empty(t, ui.OutputWriter.String())
		require.Equal(t, 1, ec)
		reset(ui)

		// Works if file is passed
		ec = cmd.Run([]string{"-out", "hcl", "myTest.hcl"})
		require.Empty(t, ui.ErrorWriter.String())
		require.Equal(t, "Example variable specification written to myTest.hcl\n", ui.OutputWriter.String())
		require.Zero(t, ec)
		reset(ui)

		t.Cleanup(func() { os.Remove(path.Join(dir, "myTest.hcl")) })
		content, err = os.ReadFile("myTest.hcl")
		require.NoError(t, err)
		require.Equal(t, defaultHclVarSpec, string(content))
	})
	t.Run("json", func(t *testing.T) {
		ci.Parallel(t)
		dir := dir
		ui := cli.NewMockUi()
		cmd := &VarInitCommand{Meta: Meta{Ui: ui}}

		// Fails on misuse
		code := cmd.Run([]string{"some", "bad", "args"})
		require.Equal(t, 1, code)
		require.Contains(t, ui.ErrorWriter.String(), "This command takes no arguments or one")
		require.Empty(t, ui.OutputWriter.String())
		reset(ui)

		// Works if the file doesn't exist
		code = cmd.Run([]string{"-out", "json"})
		require.Contains(t, ui.ErrorWriter.String(), "REMINDER: While keys")
		require.Contains(t, ui.OutputWriter.String(), "Example variable specification written to spec.nv.json\n")
		require.Zero(t, code)
		reset(ui)

		t.Cleanup(func() { os.Remove(path.Join(dir, "spec.nv.json")) })
		content, err := os.ReadFile(DefaultJsonVarInitName)
		require.NoError(t, err)
		require.Equal(t, defaultJsonVarSpec, string(content))

		// Fails if the file exists
		code = cmd.Run([]string{"-out", "json"})
		require.Contains(t, ui.ErrorWriter.String(), "exists")
		require.Empty(t, ui.OutputWriter.String())
		require.Equal(t, 1, code)
		reset(ui)

		// Works if file is passed
		code = cmd.Run([]string{"-out", "json", "myTest.json"})
		require.Contains(t, ui.ErrorWriter.String(), "REMINDER: While keys")
		require.Contains(t, ui.OutputWriter.String(), "Example variable specification written to myTest.json\n")
		require.Zero(t, code)
		reset(ui)

		t.Cleanup(func() { os.Remove(path.Join(dir, "myTest.json")) })
		content, err = os.ReadFile("myTest.json")
		require.NoError(t, err)
		require.Equal(t, defaultJsonVarSpec, string(content))
	})
}

func reset(ui *cli.MockUi) {
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
