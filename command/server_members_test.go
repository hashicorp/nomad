package command

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/mitchellh/cli"
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
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Query the members
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, name) {
		t.Fatalf("expected %q in output, got: %s", name, out)
	}
	ui.OutputWriter.Reset()

	// Query members with verbose output
	if code := cmd.Run([]string{"-address=" + url, "-verbose"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	// Still support previous detailed flag
	if code := cmd.Run([]string{"-address=" + url, "-detailed"}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, "Tags") {
		t.Fatalf("expected tags in output, got: %s", out)
	}
}

func TestMembersCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &ServerMembersCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	if code := cmd.Run([]string{"some", "bad", "args"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, commandErrorText(cmd)) {
		t.Fatalf("expected help output, got: %s", out)
	}
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	if code := cmd.Run([]string{"-address=nope"}); code != 1 {
		t.Fatalf("expected exit code 1, got: %d", code)
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Error querying servers") {
		t.Fatalf("expected failed query error, got: %s", out)
	}
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

	if _, err := srv2.Agent.Server().Join([]string{addr}); err != nil {
		t.Fatalf("Join err: %v", err)
	}
	ui := cli.NewMockUi()
	cmd := &ServerMembersCommand{Meta: Meta{Ui: ui}}

	// Get our own node name
	name, err := client1.Agent().NodeName()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Query the members
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, name) {
		t.Fatalf("expected %q in output, got: %s", name, out)
	}
	ui.OutputWriter.Reset()

	// Make one of the servers leave
	srv2.Agent.Leave()

	// Query again, should still contain expected output
	if code := cmd.Run([]string{"-address=" + url}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}
	if out := ui.OutputWriter.String(); !strings.Contains(out, name) {
		t.Fatalf("expected %q in output, got: %s", name, out)
	}
}
