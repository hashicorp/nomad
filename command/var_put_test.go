package command

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestVarPutCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VarPutCommand{}
}

func TestVarPutCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &VarPutCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	out := ui.ErrorWriter.String()
	require.Equal(t, 1, code, "expected exit code 1, got: %d")
	require.Contains(t, out, commandErrorText(cmd), "expected help output, got: %s", out)

	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope", "foo"})
	out = ui.ErrorWriter.String()
	require.Equal(t, 1, code, "expected exit code 1, got: %d")
	require.Contains(t, out, "putting variable", "connection error, got: %s", out)
	ui.ErrorWriter.Reset()
}

func TestVarPutCommand_GoodJson(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &VarPutCommand{Meta: Meta{Ui: ui}}

	// Get the variable
	code := cmd.Run([]string{"-address=" + url, "-out=json", "test/var", "k1=v1", "k2=v2"})
	require.Equal(t, 0, code, "expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())

	t.Cleanup(func() {
		_, _ = client.Variables().Delete("test/var", nil)
	})

	var outVar api.Variable
	b := ui.OutputWriter.Bytes()
	err := json.Unmarshal(b, &outVar)
	require.NoError(t, err, "error unmarshaling json: %v\nb: %s", err, b)
	require.Equal(t, "default", outVar.Namespace)
	require.Equal(t, "test/var", outVar.Path)
	require.Equal(t, api.VariableItems{"k1": "v1", "k2": "v2"}, outVar.Items)
}

func TestVarPutCommand_AutocompleteArgs(t *testing.T) {
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
