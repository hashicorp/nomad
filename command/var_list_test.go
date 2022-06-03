package command

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestVarListCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VarListCommand{}
}

func TestVarListCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &VarListCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error retrieving vars") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
	ui.ErrorWriter.Reset()
}

func TestVarListCommand_List(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &VarListCommand{Meta: Meta{Ui: ui}}
	client.Namespaces().Register(&api.Namespace{Name: "ns1"}, nil)
	client.Raw().Write("/v1/var/a/b/c", &api.SecureVariable{Items: map[string]string{"key": "value"}}, nil, nil)
	client.Raw().Write("/v1/var/a/b/c2", &api.SecureVariable{Items: map[string]string{"key": "value"}}, nil, nil)
	client.Raw().Write("/v1/var/a/b/c", &api.SecureVariable{Items: map[string]string{"key": "value"}}, nil, &api.WriteOptions{Namespace: "ns1"})
	client.Raw().Write("/v1/var/a/b/c2", &api.SecureVariable{Items: map[string]string{"key": "value"}}, nil, &api.WriteOptions{Namespace: "ns1"})

	// Create a var
	// qs := testVarSpec()
	// _, err := client.Vars().Register(qs, nil)
	// assert.Nil(err)

	// List should contain the new var
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		require.Equal(t, 0, code, "Expected exit code 0; got: %v, stderr: %s", code, ui.ErrorWriter.String())
	}
	out := ui.OutputWriter.String()
	if !strings.Contains(out, "a/b/c") || !strings.Contains(out, "default") {
		require.FailNowf(t, "expected var, got: %s", out)
	}
	ui.OutputWriter.Reset()

	// List json
	t.Log(url)
	if code := cmd.Run([]string{"-address=" + url, "-json"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}
	out = ui.OutputWriter.String()
	svs := make([]*api.SecureVariableMetadata, 0)
	err := json.Unmarshal([]byte(out), &svs)
	require.NoError(t, err)
	require.Len(t, svs, 2)
	require.Equal(t, "a/b/c", svs[0].Path)
	ui.OutputWriter.Reset()
}
