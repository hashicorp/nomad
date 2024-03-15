// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"testing"
	"time"

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

func TestVarLockCommand_Good(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &VarLockCommand{
		varPutCommand: &VarPutCommand{Meta: Meta{Ui: ui}},
	}

	filePath := fmt.Sprintf("%v.txt", time.Now().Unix())

	// Get the variable
	code := cmd.Run([]string{"-address=" + url, "test/var/shell", "touch ", filePath})
	must.Zero(t, code)

	sv, _, err := srv.APIClient().Variables().Peek("test/var/shell", nil)
	must.NoError(t, err)

	must.NotNil(t, sv)
	must.Eq(t, "test/var/shell", sv.Path)

	// Check for the file
	_, err = os.ReadFile(filePath)
	must.NoError(t, err)

	t.Cleanup(func() {
		os.Remove(filePath)
		_, _ = client.Variables().Delete("test/var/shell", nil)
	})
}

func TestVarLockCommand_Good_NoShell(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &VarLockCommand{
		varPutCommand: &VarPutCommand{Meta: Meta{Ui: ui}},
	}

	filePath := fmt.Sprintf("%v.txt", time.Now().Unix())

	// Get the variable
	code := cmd.Run([]string{"-address=" + url, "-shell=false", "test/var/noShell", "touch", filePath})
	must.Zero(t, code)

	sv, _, err := srv.APIClient().Variables().Peek("test/var/noShell", nil)
	must.NoError(t, err)

	must.NotNil(t, sv)
	must.Eq(t, "test/var/noShell", sv.Path)

	// Check for the file
	_, err = os.ReadFile(filePath)
	must.NoError(t, err)

	t.Cleanup(func() {
		os.Remove(filePath)
		_, _ = client.Variables().Delete("test/var/noShell", nil)
	})
}
