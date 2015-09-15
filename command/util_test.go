package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/testutil"
)

// seen is used to track which tests we have already
// marked as parallel. Marking twice causes panic.
var seen map[*testing.T]struct{}

func init() {
	seen = make(map[*testing.T]struct{})
}

func testServer(
	t *testing.T,
	cb testutil.ServerConfigCallback) (*testutil.TestServer, *api.Client, string) {

	// Always run these tests in parallel.
	if _, ok := seen[t]; !ok {
		seen[t] = struct{}{}
		t.Parallel()
	}

	// Make a new test server
	srv := testutil.NewTestServer(t, cb)

	// Make a client
	clientConf := api.DefaultConfig()
	clientConf.Address = "http://" + srv.HTTPAddr
	client, err := api.NewClient(clientConf)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	return srv, client, clientConf.Address
}
