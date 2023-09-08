// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestAgentCheckCommand_ServerHealth(t *testing.T) {
	ci.Parallel(t)
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &AgentCheckCommand{Meta: Meta{Ui: ui}}
	address := fmt.Sprintf("-address=%s", url)

	code := cmd.Run([]string{address})
	must.Eq(t, HealthPass, code)

	minPeers := fmt.Sprintf("-min-peers=%v", 3)
	code = cmd.Run([]string{address, minPeers})
	must.Eq(t, HealthCritical, code)
}
