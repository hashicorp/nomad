package api

import (
	"github.com/hashicorp/nomad/api/internal/testutil"
	"testing"
)

// NewTestClient returns an API client useful for testing.
func NewTestClient(t *testing.T) (*Client, error) {
	// Make client config
	conf := DefaultConfig()
	// Create server
	server := testutil.NewTestServer(t, nil)
	conf.Address = "http://" + server.HTTPAddr

	// Create client
	client, err := NewClient(conf)
	if err != nil {
		return nil, err
	}

	return client, nil
}
