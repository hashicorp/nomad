// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
)

func TestSystemGCCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &SystemGCCommand{}
}

func TestSystemGCCommand_Good(t *testing.T) {
	ci.Parallel(t)

	// Create a server
	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &SystemGCCommand{Meta: Meta{Ui: ui}}

	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d; %v", code, ui.ErrorWriter.String())
	}
}
