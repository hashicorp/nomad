// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"path"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestVarInitCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VarInitCommand{}
}

func TestVarInitCommand_Run(t *testing.T) {
	ci.Parallel(t)
	dir := t.TempDir()
	origDir, err := os.Getwd()
	must.NoError(t, err)
	err = os.Chdir(dir)
	must.NoError(t, err)
	t.Cleanup(func() { os.Chdir(origDir) })

	t.Run("hcl", func(t *testing.T) {
		ci.Parallel(t)
		dir := dir
		ui := cli.NewMockUi()
		cmd := &VarInitCommand{Meta: Meta{Ui: ui}}

		// Fails on misuse
		ec := cmd.Run([]string{"some", "bad", "args"})
		must.One(t, ec)
		must.StrContains(t, ui.ErrorWriter.String(), commandErrorText(cmd))
		must.Eq(t, "", ui.OutputWriter.String())
		reset(ui)

		// Works if the file doesn't exist
		ec = cmd.Run([]string{"-out", "hcl"})
		must.Eq(t, "", ui.ErrorWriter.String())
		must.Eq(t, "Example variable specification written to spec.nv.hcl\n", ui.OutputWriter.String())
		must.Zero(t, ec)
		reset(ui)
		t.Cleanup(func() { os.Remove(path.Join(dir, "spec.nv.hcl")) })

		content, err := os.ReadFile(DefaultHclVarInitName)
		must.NoError(t, err)
		must.Eq(t, defaultHclVarSpec, string(content))

		// Fails if the file exists
		ec = cmd.Run([]string{"-out", "hcl"})
		must.StrContains(t, ui.ErrorWriter.String(), "exists")
		must.Eq(t, "", ui.OutputWriter.String())
		must.One(t, ec)
		reset(ui)

		// Works if file is passed
		ec = cmd.Run([]string{"-out", "hcl", "myTest.hcl"})
		must.Eq(t, "", ui.ErrorWriter.String())
		must.Eq(t, "Example variable specification written to myTest.hcl\n", ui.OutputWriter.String())
		must.Zero(t, ec)
		reset(ui)

		t.Cleanup(func() { os.Remove(path.Join(dir, "myTest.hcl")) })
		content, err = os.ReadFile("myTest.hcl")
		must.NoError(t, err)
		must.Eq(t, defaultHclVarSpec, string(content))
	})
	t.Run("json", func(t *testing.T) {
		ci.Parallel(t)
		dir := dir
		ui := cli.NewMockUi()
		cmd := &VarInitCommand{Meta: Meta{Ui: ui}}

		// Fails on misuse
		code := cmd.Run([]string{"some", "bad", "args"})
		must.One(t, code)
		must.StrContains(t, ui.ErrorWriter.String(), "This command takes no arguments or one")
		must.Eq(t, "", ui.OutputWriter.String())
		reset(ui)

		// Works if the file doesn't exist
		code = cmd.Run([]string{"-out", "json"})
		must.StrContains(t, ui.ErrorWriter.String(), "REMINDER: While keys")
		must.StrContains(t, ui.OutputWriter.String(), "Example variable specification written to spec.nv.json\n")
		must.Zero(t, code)
		reset(ui)

		t.Cleanup(func() { os.Remove(path.Join(dir, "spec.nv.json")) })
		content, err := os.ReadFile(DefaultJsonVarInitName)
		must.NoError(t, err)
		must.Eq(t, defaultJsonVarSpec, string(content))

		// Fails if the file exists
		code = cmd.Run([]string{"-out", "json"})
		must.StrContains(t, ui.ErrorWriter.String(), "exists")
		must.Eq(t, "", ui.OutputWriter.String())
		must.One(t, code)
		reset(ui)

		// Works if file is passed
		code = cmd.Run([]string{"-out", "json", "myTest.json"})
		must.StrContains(t, ui.ErrorWriter.String(), "REMINDER: While keys")
		must.StrContains(t, ui.OutputWriter.String(), "Example variable specification written to myTest.json\n")
		must.Zero(t, code)
		reset(ui)

		t.Cleanup(func() { os.Remove(path.Join(dir, "myTest.json")) })
		content, err = os.ReadFile("myTest.json")
		must.NoError(t, err)
		must.Eq(t, defaultJsonVarSpec, string(content))
	})
}

func reset(ui *cli.MockUi) {
	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
