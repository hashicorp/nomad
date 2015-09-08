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
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Agent()

	// Attempting to join a non-existent host returns error
	if err := a.Join("nope"); err == nil {
		t.Fatalf("expected error, got nothing")
	}

	// TODO(ryanuber): This is pretty much a worthless test,
	// since we are just joining ourselves. Once the agent
	// respects config options, change this to actually make
	// two nodes and join them.
	if err := a.Join("127.0.0.1"); err != nil {
		t.Fatalf("err: %s", err)
	}
}
