package plugin

import (
	"log"
	"net/rpc"
	"sync"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/runner"
)

// DriverPluginClient embeds a driverPluginRPCClient and wraps it's Exit
// method to also call Kill() on the plugin.Client.
type DriverPluginClient struct {
	client *plugin.Client
	sync.Mutex

	*driverPluginRPCClient
}

func (dc *DriverPluginClient) Exit() error {
	err := dc.driverPluginRPCClient.Exit()
	dc.client.Kill()

	return err
}

// newPluginClient returns a driverRPCClient with a connection to a running
// plugin.
func newPluginClient(pluginRunner *runner.PluginRunner, reattach *plugin.ReattachConfig, logger *log.Logger) (Driver, *plugin.ReattachConfig, error) {
	// pluginMap is the map of plugins we can dispense.
	var pluginMap = map[string]plugin.Plugin{
		"driver": new(DriverPlugin),
	}

	client, err := pluginRunner.Run(pluginMap, HandshakeConfig, reattach, logger)
	if err != nil {
		return nil, nil, err
	}

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		return nil, nil, err
	}

	// Request the plugin
	raw, err := rpcClient.Dispense("driver")
	if err != nil {
		return nil, nil, err
	}

	// We should have a driver type now. This feels like a normal interface
	// implementation but is in fact over an RPC connection.
	driverRPC := raw.(*driverPluginRPCClient)

	// Wrap RPC implimentation in DriverPluginClient
	return &DriverPluginClient{
		client:                client,
		driverPluginRPCClient: driverRPC,
	}, client.ReattachConfig(), nil
}

// ---- RPC client domain ----

// driverPluginRPCClient implements Database and is used on the client to
// make RPC calls to a plugin.
type driverPluginRPCClient struct {
	client *rpc.Client
}

func (dr *driverPluginRPCClient) Name() (string, error) {
	var name string
	err := dr.client.Call("Plugin.Name", struct{}{}, &name)
	return name, err
}

func (dr *driverPluginRPCClient) Exit() error {
	err := dr.client.Call("Plugin.Exit", struct{}{}, &struct{}{})
	return err
}
