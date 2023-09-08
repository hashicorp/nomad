// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestVarLockCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VarLockCommand{}
}

func TestVarLockCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	t.Run("bad_args", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarLockCommand{
			varPutCommand: &VarPutCommand{Meta: Meta{Ui: ui}},
		}
		code := cmd.Run([]string{"-bad-flag"})
		out := ui.ErrorWriter.String()
		must.One(t, code)
		must.StrContains(t, out, commandErrorText(cmd))
	})

	t.Run("bad_address", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarLockCommand{
			varPutCommand: &VarPutCommand{Meta: Meta{Ui: ui}},
		}
		code := cmd.Run([]string{"-address=nope", "foo", "-"})
		out := ui.ErrorWriter.String()
		must.One(t, code)
		must.StrContains(t, out, "unsupported protocol scheme")
	})

	t.Run("missing_args", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarLockCommand{
			varPutCommand: &VarPutCommand{Meta: Meta{Ui: ui}},
		}
		code := cmd.Run([]string{"foo"})
		out := ui.ErrorWriter.String()
		must.One(t, code)
		must.StrContains(t, out, "Not enough arguments (expected >2, got 1)")
	})

	t.Run("invalid_TTL", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarLockCommand{
			varPutCommand: &VarPutCommand{Meta: Meta{Ui: ui}},
		}
		code := cmd.Run([]string{"-ttl=2", "bar", "foo"})
		out := ui.ErrorWriter.String()
		must.One(t, code)
		must.StrContains(t, out, "Invalid TTL: time")
	})

	t.Run("invalid_lock_delay", func(t *testing.T) {
		ci.Parallel(t)
		ui := cli.NewMockUi()
		cmd := &VarLockCommand{
			varPutCommand: &VarPutCommand{Meta: Meta{Ui: ui}},
		}
		code := cmd.Run([]string{"-delay=2", "bar", "foo"})
		out := ui.ErrorWriter.String()
		must.One(t, code)
		must.StrContains(t, out, "Invalid Lock Delay: time")
	})
}

//func TestVarPutCommand_GoodJson(t *testing.T) {
//	ci.Parallel(t)
//
//	// Create a server
//	srv, client, url := testServer(t, true, nil)
//	defer srv.Shutdown()
//
//	ui := cli.NewMockUi()
//	cmd := &VarPutCommand{Meta: Meta{Ui: ui}}
//
//	// Get the variable
//	code := cmd.Run([]string{"-address=" + url, "-out=json", "test/var", "k1=v1", "k2=v2"})
//	require.Equal(t, 0, code, "expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
//
//	t.Cleanup(func() {
//		_, _ = client.Variables().Delete("test/var", nil)
//	})
//
//	var outVar api.Variable
//	b := ui.OutputWriter.Bytes()
//	err := json.Unmarshal(b, &outVar)
//	require.NoError(t, err, "error unmarshaling json: %v\nb: %s", err, b)
//	require.Equal(t, "default", outVar.Namespace)
//	require.Equal(t, "test/var", outVar.Path)
//	require.Equal(t, api.VariableItems{"k1": "v1", "k2": "v2"}, outVar.Items)
//}
