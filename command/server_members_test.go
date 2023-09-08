// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestServerMembersCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &ServerMembersCommand{}
}

func TestServerMembersCommand_Run(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &ServerMembersCommand{Meta: Meta{Ui: ui}}

	// Get our own node name
	name, err := client.Agent().NodeName()
	must.NoError(t, err)

	// Query the members
	code := cmd.Run([]string{"-address=" + url})

	if out := ui.OutputWriter.String(); !strings.Contains(out, name) {
		t.Fatalf("expected %q in output, got: %s", name, out)
	}
	ui.OutputWriter.Reset()

	// Query members with verbose output
	code = cmd.Run([]string{"-address=" + url, "-verbose"})
	must.Zero(t, code)

	// Still support previous detailed flag
	code = cmd.Run([]string{"-address=" + url, "-detailed"})
	must.Zero(t, code)

	must.StrContains(t, ui.OutputWriter.String(), "Tags")

	ui.OutputWriter.Reset()

	// List json
	code = cmd.Run([]string{"-address=" + url, "-json"})
	must.Zero(t, code)

	outJson := []api.AgentMember{}
	err = json.Unmarshal(ui.OutputWriter.Bytes(), &outJson)
	must.NoError(t, err)

	ui.OutputWriter.Reset()

	// Go template to format the output
	code = cmd.Run([]string{"-address=" + url, "-t", "{{range .}}{{ .Status }}{{end}}"})
	must.Zero(t, code)

	out := ui.OutputWriter.String()
	must.StrContains(t, out, "alive")

	ui.ErrorWriter.Reset()
}

func TestMembersCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &ServerMembersCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	must.One(t, code)

	must.StrContains(t, ui.ErrorWriter.String(), commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	code = cmd.Run([]string{"-address=nope"})
	must.One(t, code)

	must.StrContains(t, ui.ErrorWriter.String(), "Error querying servers")
}

// Tests that a single server region that left should still
// not return an error and list other members in other regions
func TestServerMembersCommand_MultiRegion_Leave(t *testing.T) {
	ci.Parallel(t)

	config1 := func(c *agent.Config) {
		c.Region = "r1"
		c.Datacenter = "d1"
	}

	srv1, client1, url := testServer(t, false, config1)
	defer srv1.Shutdown()

	config2 := func(c *agent.Config) {
		c.Region = "r2"
		c.Datacenter = "d2"
	}

	srv2, _, _ := testServer(t, false, config2)
	defer srv2.Shutdown()

	// Join with srv1
	addr := fmt.Sprintf("127.0.0.1:%d",
		srv1.Agent.Server().GetConfig().SerfConfig.MemberlistConfig.BindPort)

	_, err := srv2.Agent.Server().Join([]string{addr})
	must.NoError(t, err)

	ui := cli.NewMockUi()
	cmd := &ServerMembersCommand{Meta: Meta{Ui: ui}}

	// Get our own node name
	name, err := client1.Agent().NodeName()
	must.NoError(t, err)

	// Query the members
	code := cmd.Run([]string{"-address=" + url})
	must.Zero(t, code)

	must.StrContains(t, ui.OutputWriter.String(), name)
	ui.OutputWriter.Reset()

	// Make one of the servers leave
	srv2.Agent.Leave()

	// Query again, should still contain expected output
	code = cmd.Run([]string{"-address=" + url})
	must.Zero(t, code)

	must.StrContains(t, ui.OutputWriter.String(), name)
}
