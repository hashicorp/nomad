package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/testutil"
)

func testServer(t *testing.T) (*testutil.TestServer, *api.Client, string) {
	// Always run these tests in parallel.
	t.Parallel()

	// Make a new test server
	srv := testutil.NewTestServer(t, nil)

	// Make a client
	clientConf := api.DefaultConfig()
	clientConf.Address = "http://" + srv.HTTPAddr
	client, err := api.NewClient(clientConf)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	return srv, client, clientConf.Address
}
