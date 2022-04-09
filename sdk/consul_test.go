package sdk

import (
	"strings"
	"testing"
)

func TestConsul_Basic(t *testing.T) {
	c, wait, stop := NewConsul(t, nil)
	t.Cleanup(stop)

	// wait for leader
	wait()

	// get consul api client object
	client := c.Client(t)

	// query consul api for node name
	name, err := client.Agent().NodeName()
	if err != nil {
		t.Fatalf("failed to get node name: %v", err)
	}

	ok := strings.HasPrefix(name, "test-")
	if !ok {
		t.Fatalf("expected node name to have 'test-' prefix, got: %s", name)
	}
}
