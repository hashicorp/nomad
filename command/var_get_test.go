package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestVarGetCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VarGetCommand{}
}

func TestVarGetCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &VarGetCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	out := ui.ErrorWriter.String()
	require.Equal(t, 1, code, "expected exit code 1, got: %d")
	require.Contains(t, out, commandErrorText(cmd), "expected help output, got: %s", out)

	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope", "foo"})
	out = ui.ErrorWriter.String()
	require.Equal(t, 1, code, "expected exit code 1, got: %d")
	require.Contains(t, out, "retrieving variable", "connection error, got: %s", out)
	ui.ErrorWriter.Reset()
}

func TestVarGetCommand_GoodJson(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &VarGetCommand{Meta: Meta{Ui: ui}}

	// Create a var to get
	sv := testVariable()
	var err error
	sv, _, err = client.Variables().Create(sv, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = client.Variables().Delete(sv.Path, nil)
	})

	// Get the variable
	code := cmd.Run([]string{"-out=json", "-address=" + url, sv.Path})
	require.Equal(t, 0, code, "expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	require.Equal(t, sv.AsJSON(), strings.TrimSpace(ui.OutputWriter.String()))
}

func TestVarGetCommand_GoodTable(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &VarGetCommand{Meta: Meta{Ui: ui}}

	// Create a var to get
	sv := testVariable()
	var err error
	sv, _, err = client.Variables().Create(sv, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = client.Variables().Delete(sv.Path, nil)
	})

	// Get the variable
	code := cmd.Run([]string{"-out=table", "-address=" + url, sv.Path})
	require.Equal(t, 0, code, "expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	out := ui.OutputWriter.String()
	outs := strings.Split(out, "\n")
	require.Len(t, outs, 9)
	require.Equal(t, "Namespace   = default", outs[0])
	require.Equal(t, "Path        = test/var", outs[1])
	//require.Equal(t, "", strings.TrimSpace(ui.OutputWriter.String()))
}

func TestVarGetCommand_AutocompleteArgs(t *testing.T) {
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
