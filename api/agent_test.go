package api

import (
	"testing"
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

	// Ensure we got a node name back
	if res == "" {
		t.Fatalf("expected node name, got nothing")
	}

	// Check that we cached the node name
	if a.nodeName == "" {
		t.Fatalf("should have cached node name")
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

	// Check that we got the DC name back
	if dc != "dc1" {
		t.Fatalf("expected dc1, got: %q", dc)
	}

	// Check that the datacenter name was cached
	if a.datacenter == "" {
		t.Fatalf("should have cached datacenter")
	}
}
