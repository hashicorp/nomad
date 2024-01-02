// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestQuotaInitCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &QuotaInitCommand{}
}

func TestQuotaInitCommand_Run_HCL(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QuotaInitCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)
	require.Contains(t, ui.ErrorWriter.String(), commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	// Ensure we change the cwd back
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)

	// Create a temp dir and change into it
	dir := t.TempDir()

	err = os.Chdir(dir)
	require.NoError(t, err)

	// Works if the file doesn't exist
	code = cmd.Run([]string{})
	require.Empty(t, ui.ErrorWriter.String())
	require.Zero(t, code)

	content, err := os.ReadFile(DefaultHclQuotaInitName)
	require.NoError(t, err)
	require.Equal(t, defaultHclQuotaSpec, string(content))

	// Fails if the file exists
	code = cmd.Run([]string{})
	require.Contains(t, ui.ErrorWriter.String(), "exists")
	require.Equal(t, 1, code)
	ui.ErrorWriter.Reset()

	// Works if file is passed
	code = cmd.Run([]string{"mytest.hcl"})
	require.Empty(t, ui.ErrorWriter.String())
	require.Zero(t, code)

	content, err = os.ReadFile("mytest.hcl")
	require.NoError(t, err)
	require.Equal(t, defaultHclQuotaSpec, string(content))
}

func TestQuotaInitCommand_Run_JSON(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QuotaInitCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)
	require.Contains(t, ui.ErrorWriter.String(), commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	// Ensure we change the cwd back
	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)

	// Create a temp dir and change into it
	dir := t.TempDir()

	err = os.Chdir(dir)
	require.NoError(t, err)

	// Works if the file doesn't exist
	code = cmd.Run([]string{"-json"})
	require.Empty(t, ui.ErrorWriter.String())
	require.Zero(t, code)

	content, err := os.ReadFile(DefaultJsonQuotaInitName)
	require.NoError(t, err)
	require.Equal(t, defaultJsonQuotaSpec, string(content))

	// Fails if the file exists
	code = cmd.Run([]string{"-json"})
	require.Contains(t, ui.ErrorWriter.String(), "exists")
	require.Equal(t, 1, code)
	ui.ErrorWriter.Reset()

	// Works if file is passed
	code = cmd.Run([]string{"-json", "mytest.json"})
	require.Empty(t, ui.ErrorWriter.String())
	require.Zero(t, code)

	content, err = os.ReadFile("mytest.json")
	require.NoError(t, err)
	require.Equal(t, defaultJsonQuotaSpec, string(content))
}
