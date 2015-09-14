package api

import (
	"testing"

	"github.com/hashicorp/nomad/testutil"
)

func TestAgent_Self(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	// Get a handle on the Agent endpoints
	a := c.Agent()

	// Query the endpoint
	res, err := a.Self()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that we got a valid response
	if name, ok := res["member"]["Name"]; !ok || name == "" {
		t.Fatalf("bad member name in response: %#v", res)
	}

	// Local cache was populated
	if a.nodeName == "" || a.datacenter == "" || a.region == "" {
		t.Fatalf("cache should be populated, got: %#v", a)
	}
}

func TestAgent_NodeName(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Query the agent for the node name
	res, err := a.NodeName()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if res == "" {
		t.Fatalf("expected node name, got nothing")
	}
}

func TestAgent_Datacenter(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Query the agent for the datacenter
	dc, err := a.Datacenter()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if dc != "dc1" {
		t.Fatalf("expected dc1, got: %q", dc)
	}
}

func TestAgent_Join(t *testing.T) {
	c1, s1 := makeClient(t, nil, nil)
	defer s1.Stop()
	a1 := c1.Agent()

	_, s2 := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.Server.Bootstrap = false
	})
	defer s2.Stop()

	// Attempting to join a non-existent host returns error
	n, err := a1.Join("nope")
	if err == nil {
		t.Fatalf("expected error, got nothing")
	}
	if n != 0 {
		t.Fatalf("expected 0 nodes, got: %d", n)
	}

	// Returns correctly if join succeeds
	n, err = a1.Join(s2.SerfAddr)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 node, got: %d", n)
	}
}

func TestAgent_Members(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Query nomad for all the known members
	mem, err := a.Members()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that we got the expected result
	if n := len(mem); n != 1 {
		t.Fatalf("expected 1 member, got: %d", n)
	}
	if m := mem[0]; m.Name == "" || m.Addr == "" || m.Port == 0 {
		t.Fatalf("bad member: %#v", m)
	}
}

func TestAgent_ForceLeave(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Force-leave on a non-existent node does not error
	if err := a.ForceLeave("nope"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// TODO: test force-leave on an existing node
}
