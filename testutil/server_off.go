//+build !bin_test

package testutil

import testing "github.com/mitchellh/go-testing-interface"

// NewTestServer creates a new TestServer, and makes a call to
// an optional callback function to modify the configuration.
func NewTestServer(t testing.T, cb ServerConfigCallback) *TestServer {
	t.Skip(`This test requires the bin_test tag to be set.
	==> Tip: Ensure you've built your latest changes to nomad by
	    running 'make dev' first, then run this test again with
	    "-tags bin_test"`)
	return nil
}
