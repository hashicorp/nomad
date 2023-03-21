package command

import (
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
)

// testVariable returns a test variable spec
func testVariable() *api.Variable {
	return &api.Variable{
		Namespace: "default",
		Path:      "test/var",
		Items: map[string]string{
			"keyA": "valueA",
			"keyB": "valueB",
		},
	}
}

func testAPIClient(t *testing.T) (srv *agent.TestAgent, client *api.Client, url string, shutdownFn func() error) {
	srv, client, url = testServer(t, true, nil)
	shutdownFn = srv.Shutdown
	return
}
