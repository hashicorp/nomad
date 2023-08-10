// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"os"
	"path"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/asset"
)

func TestNodePoolInitCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &NodePoolInitCommand{}
}

func TestNodePoolInitCommand_Run(t *testing.T) {
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
		cmd := &NodePoolInitCommand{Meta: Meta{Ui: ui}}

		// Fails on misuse
		ec := cmd.Run([]string{"some", "bad", "args"})
		must.Eq(t, 1, ec)
		must.StrContains(t, ui.ErrorWriter.String(), commandErrorText(cmd))
		must.Eq(t, "", ui.OutputWriter.String())
		reset(ui)

		// Works if the file doesn't exist
		ec = cmd.Run([]string{"-out", "hcl"})
		must.Eq(t, "", ui.ErrorWriter.String())
		must.Eq(t, "Example node pool specification written to pool.nomad.hcl\n", ui.OutputWriter.String())
		must.Zero(t, ec)
		reset(ui)
		t.Cleanup(func() { os.Remove(path.Join(dir, "pool.nomad.hcl")) })

		content, err := os.ReadFile(DefaultHclNodePoolInitName)
		must.NoError(t, err)
		must.Eq(t, asset.NodePoolSpec, content)

		// Fails if the file exists
		ec = cmd.Run([]string{"-out", "hcl"})
		must.StrContains(t, ui.ErrorWriter.String(), "exists")
		must.Eq(t, "", ui.OutputWriter.String())
		must.Eq(t, 1, ec)
		reset(ui)

		// Works if file is passed
		ec = cmd.Run([]string{"-out", "hcl", "myTest.hcl"})
		must.Eq(t, "", ui.ErrorWriter.String())
		must.Eq(t, "Example node pool specification written to myTest.hcl\n", ui.OutputWriter.String())
		must.Zero(t, ec)
		reset(ui)

		t.Cleanup(func() { os.Remove(path.Join(dir, "myTest.hcl")) })
		content, err = os.ReadFile("myTest.hcl")
		must.NoError(t, err)
		must.Eq(t, asset.NodePoolSpec, content)
	})

	t.Run("json", func(t *testing.T) {
		ci.Parallel(t)
		dir := dir
		ui := cli.NewMockUi()
		cmd := &NodePoolInitCommand{Meta: Meta{Ui: ui}}

		// Fails on misuse
		code := cmd.Run([]string{"some", "bad", "args"})
		must.Eq(t, 1, code)
		must.StrContains(t, ui.ErrorWriter.String(), "This command takes no arguments or one")
		must.Eq(t, "", ui.OutputWriter.String())
		reset(ui)

		// Works if the file doesn't exist
		code = cmd.Run([]string{"-out", "json"})
		must.StrContains(t, ui.OutputWriter.String(), "Example node pool specification written to pool.nomad.json\n")
		must.Zero(t, code)
		reset(ui)

		t.Cleanup(func() { os.Remove(path.Join(dir, "pool.nomad.json")) })
		content, err := os.ReadFile(DefaultJsonNodePoolInitName)
		must.NoError(t, err)
		must.Eq(t, asset.NodePoolSpecJSON, content)

		// Fails if the file exists
		code = cmd.Run([]string{"-out", "json"})
		must.StrContains(t, ui.ErrorWriter.String(), "exists")
		must.Eq(t, "", ui.OutputWriter.String())
		must.Eq(t, 1, code)
		reset(ui)

		// Works if file is passed
		code = cmd.Run([]string{"-out", "json", "myTest.json"})
		must.StrContains(t, ui.OutputWriter.String(), "Example node pool specification written to myTest.json\n")
		must.Zero(t, code)
		reset(ui)

		t.Cleanup(func() { os.Remove(path.Join(dir, "myTest.json")) })
		content, err = os.ReadFile("myTest.json")
		must.NoError(t, err)
		must.Eq(t, asset.NodePoolSpecJSON, content)
	})
}
