package client

import "testing"

func testClient(t *testing.T, cb func(c *Config)) *Client {
	conf := DefaultConfig()
	if cb != nil {
		cb(conf)
	}

	client, err := NewClient(conf)
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
