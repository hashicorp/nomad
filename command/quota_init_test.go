// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
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
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	// Ensure we change the cwd back
	origDir, err := os.Getwd()
	must.NoError(t, err)
	defer os.Chdir(origDir)

	// Create a temp dir and change into it
	dir := t.TempDir()

	err = os.Chdir(dir)
	must.NoError(t, err)

	// Works if the file doesn't exist
	code = cmd.Run([]string{})
	must.Eq(t, "", ui.ErrorWriter.String())
	must.Zero(t, code)

	content, err := os.ReadFile(DefaultHclQuotaInitName)
	must.NoError(t, err)
	must.Eq(t, defaultHclQuotaSpec, string(content))

	// Fails if the file exists
	code = cmd.Run([]string{})
	must.StrContains(t, ui.ErrorWriter.String(), "exists")
	must.One(t, code)
	ui.ErrorWriter.Reset()

	// Works if file is passed
	code = cmd.Run([]string{"mytest.hcl"})
	must.Eq(t, "", ui.ErrorWriter.String())
	must.Zero(t, code)

	content, err = os.ReadFile("mytest.hcl")
	must.NoError(t, err)
	must.Eq(t, defaultHclQuotaSpec, string(content))
}

func TestQuotaInitCommand_Run_JSON(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &QuotaInitCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	must.One(t, code)
	must.StrContains(t, ui.ErrorWriter.String(), commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	// Ensure we change the cwd back
	origDir, err := os.Getwd()
	must.NoError(t, err)
	defer os.Chdir(origDir)

	// Create a temp dir and change into it
	dir := t.TempDir()

	err = os.Chdir(dir)
	must.NoError(t, err)

	// Works if the file doesn't exist
	code = cmd.Run([]string{"-json"})
	must.Eq(t, "", ui.ErrorWriter.String())
	must.Zero(t, code)

	content, err := os.ReadFile(DefaultJsonQuotaInitName)
	must.NoError(t, err)
	must.Eq(t, defaultJsonQuotaSpec, string(content))

	// Fails if the file exists
	code = cmd.Run([]string{"-json"})
	must.StrContains(t, ui.ErrorWriter.String(), "exists")
	must.One(t, code)
	ui.ErrorWriter.Reset()

	// Works if file is passed
	code = cmd.Run([]string{"-json", "mytest.json"})
	must.Eq(t, "", ui.ErrorWriter.String())
	must.Zero(t, code)

	content, err = os.ReadFile("mytest.json")
	must.NoError(t, err)
	must.Eq(t, defaultJsonQuotaSpec, string(content))
}
