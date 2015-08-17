package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/testutil"
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

func TestClient_RPC(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()

	c1 := testClient(t, func(c *Config) {
		c.ServerAddress = []string{s1.config.RPCAddr.String()}
	})
	defer c1.Shutdown()

	// RPC should succeed
	testutil.WaitForResult(func() (bool, error) {
		var out struct{}
		err := c1.RPC("Status.Ping", struct{}{}, &out)
		return err == nil, err
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}
