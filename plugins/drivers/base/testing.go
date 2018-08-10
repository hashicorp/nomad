package base

import (
	"github.com/mitchellh/go-testing-interface"

	plugin "github.com/hashicorp/go-plugin"
)

type DriverHarness struct {
	DriverClient
	client *plugin.GRPCClient
	server *plugin.GRPCServer
	t      testing.T
}

func NewDriverHarness(t testing.T, d Driver) *DriverHarness {

	client, server := plugin.TestPluginGRPCConn(t, map[string]plugin.Plugin{
		DriverGoPlugin: &DriverPlugin{impl: d},
	})

	raw, err := client.Dispense(DriverGoPlugin)
	if err != nil {
		t.Fatalf("err dispensing plugin: %v", err)
	}

	dClient := raw.(DriverClient)
	h := &DriverHarness{
		client:       client,
		server:       server,
		DriverClient: dClient,
	}

	return h
}

func (h *DriverHarness) Kill() {
	h.client.Close()
	h.server.Stop()
}
