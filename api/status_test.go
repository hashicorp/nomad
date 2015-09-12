package api

import (
	"testing"

	"github.com/hashicorp/nomad/testutil"
)

func TestStatus_Leader(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	status := c.Status()

	// Query for leader status should return a result
	out, err := status.Leader()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if out == "" {
		t.Fatalf("expected leader, got: %q", out)
	}
}

func TestStatus_Leader_NoLeader(t *testing.T) {
	// Start a server without bootstrap mode. This prevents
	// the leadership from being acquired.
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.Server.Bootstrap = false
	})
	defer s.Stop()
	status := c.Status()

	// Query for leader status should return nothing.
	out, err := status.Leader()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if out != "" {
		t.Fatalf("expected no leader, got: %q", out)
	}
}
