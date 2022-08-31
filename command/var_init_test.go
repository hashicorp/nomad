package command

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestVarInitCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VarInitCommand{}
}

func TestVarInitCommand_Run_HCL(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &VarInitCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	ec := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, ec)
	require.Contains(t, ui.ErrorWriter.String(), commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	// Capture current working dir
	origDir, err := os.Getwd()
	require.NoError(t, err)
	// Create a temp dir and change into it
	dir := t.TempDir()
	err = os.Chdir(dir)
	require.NoError(t, err)
	// Ensure we change back to the starting dir
	defer os.Chdir(origDir)

	// Works if the file doesn't exist
	ec = cmd.Run([]string{"-out", "hcl"})
	require.Contains(t, ui.ErrorWriter.String(), "REMINDER: While keys in the items")
	require.Zero(t, ec)

	content, err := ioutil.ReadFile(DefaultHclVarInitName)
	require.NoError(t, err)
	require.Equal(t, defaultHclVarSpec, string(content))

	// Fails if the file exists
	ec = cmd.Run([]string{"-out", "hcl"})
	require.Contains(t, ui.ErrorWriter.String(), "exists")
	require.Equal(t, 1, ec)
	ui.ErrorWriter.Reset()

	// Works if file is passed
	ec = cmd.Run([]string{"-out", "hcl", "mytest.hcl"})
	require.Contains(t, ui.ErrorWriter.String(), "REMINDER: While keys in the items")
	require.Zero(t, ec)

	content, err = ioutil.ReadFile("mytest.hcl")
	require.NoError(t, err)
	require.Equal(t, defaultHclVarSpec, string(content))
}

func TestVarInitCommand_Run_JSON(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &VarInitCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)
	require.Contains(t, ui.ErrorWriter.String(), "This command takes no arguments or one")
	ui.ErrorWriter.Reset()

	// Capture current working dir
	origDir, err := os.Getwd()
	require.NoError(t, err)
	// Create a temp dir and change into it
	dir := t.TempDir()
	err = os.Chdir(dir)
	require.NoError(t, err)
	// Ensure we change back to the starting dir
	defer os.Chdir(origDir)

	// Works if the file doesn't exist
	code = cmd.Run([]string{"-out", "json"})
	require.Contains(t, ui.ErrorWriter.String(), "REMINDER: While keys")
	require.Zero(t, code)

	content, err := ioutil.ReadFile(DefaultJsonVarInitName)
	require.NoError(t, err)
	require.Equal(t, defaultJsonVarSpec, string(content))

	// Fails if the file exists
	code = cmd.Run([]string{"-out", "json"})
	require.Contains(t, ui.ErrorWriter.String(), "exists")
	require.Equal(t, 1, code)
	ui.ErrorWriter.Reset()

	// Works if file is passed
	code = cmd.Run([]string{"-out", "json", "mytest.json"})
	require.Contains(t, ui.ErrorWriter.String(), "REMINDER: While keys")
	require.Zero(t, code)

	content, err = ioutil.ReadFile("mytest.json")
	require.NoError(t, err)
	require.Equal(t, defaultJsonVarSpec, string(content))
}
