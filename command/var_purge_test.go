package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
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
	ui := cli.NewMockUi()
	cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	out := ui.ErrorWriter.String()
	require.Equal(t, 1, code, "expected exit code 1, got: %d")
	require.Contains(t, out, commandErrorText(cmd), "expected help output, got: %s", out)

	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope", "foo"})
	out = ui.ErrorWriter.String()
	require.Equal(t, 1, code, "expected exit code 1, got: %d")
	require.Contains(t, out, "deleting variable", "connection error, got: %s", out)
	ui.ErrorWriter.Reset()
}

func TestVarPurgeCommand_Good(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &VarPurgeCommand{Meta: Meta{Ui: ui}}

	// Create a var to delete
	sv := testVariable()
	_, _, err := client.Variables().Create(sv, nil)
	require.NoError(t, err)

	// Delete a namespace
	code := cmd.Run([]string{"-address=" + url, sv.Path})
	require.Equal(t, 0, code, "expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())

	vars, _, err := client.Variables().List(nil)
	require.NoError(t, err)
	require.Len(t, vars, 0)
}

func TestVarPurgeCommand_Checked(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

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
	require.Contains(t, stderr, "Error purging variable: cas conflict: expected ModifyIndex 1")

	code = cmd.Run([]string{"-address=" + url, fmt.Sprintf("-check-index=%v", sv.ModifyIndex), sv.Path})
	require.Equal(t, 0, code, "expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())

	vars, _, err := client.Variables().List(nil)
	require.NoError(t, err)
	require.Len(t, vars, 0)
}

func TestVarPurgeCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)
	_, client, url, shutdownFn := testAPIClient(t)
	defer shutdownFn()

	ui := cli.NewMockUi()
	cmd := &VarPurgeCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a var
	sv := testVariable()
	_, _, err := client.Variables().Create(sv, nil)
	require.NoError(t, err)

	args := complete.Args{Last: "t"}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	require.Equal(t, 1, len(res))
	require.Equal(t, sv.Path, res[0])
}

// testVariable returns a test variable spec
func testVariable() *api.Variable {
	return &api.Variable{
		Path: "test/var/delete",
		Items: map[string]string{
			"keyA": "valueA",
			"keyB": "valueB",
		},
	}
}

func testAPIClient(t *testing.T) (srv *agent.TestAgent, client *api.Client, url string, shutdownFn func() error) {
	srv, client, url = testServer(t, true, nil)
	shutdownFn = srv.Shutdown
	return
}
