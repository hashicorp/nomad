// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestEventCommand_BaseCommand(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &EventCommand{Meta: Meta{Ui: ui}}

	code := cmd.Run([]string{"-address=" + url})

	must.Eq(t, -18511, code)
}
