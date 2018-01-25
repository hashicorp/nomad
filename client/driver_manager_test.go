package client

import (
	"testing"

	"github.com/hashicorp/nomad/client/testutil"
)

// test that the driver manager updates a node when its attributes change
func TestDriverManager_Fingerprint(t *testing.T) {
	testutil.RequireRoot(t)
	t.Parallel()

	c := testClient(t, nil)
	defer c.Shutdown()

	// Ensure kernel and arch are always present
	node := c.Node()
	if node.Attributes["kernel.name"] == "" {
		t.Fatalf("missing kernel.name")
	}
	if node.Attributes["cpu.arch"] == "" {
		t.Fatalf("missing cpu arch")
	}
}
