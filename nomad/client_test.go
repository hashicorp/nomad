package nomad

import (
	"fmt"
	"testing"
)

func testClient(t *testing.T, cb func(c *Config)) *Client {
	// Setup the default settings
	config := DefaultConfig()
	config.Build = "unittest"
	config.DevMode = true
	config.NodeName = fmt.Sprintf("Client %d", config.RPCAddr.Port)

	// Invoke the callback if any
	if cb != nil {
		cb(config)
	}

	// Create client
	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return client
}

func TestClient_StartStop(t *testing.T) {
	client := testClient(t, nil)
	if err := client.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}
}
