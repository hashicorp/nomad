package plugin

import plugin "github.com/hashicorp/go-plugin"

// Serve is called from within a plugin and wraps the provided
// Driver implementation in a driverPluginRPCServer object and starts a
// RPC server.
func Serve(d Driver) {
	driverPlugin := &DriverPlugin{
		impl: d,
	}

	// pluginMap is the map of plugins we can dispense.
	var pluginMap = map[string]plugin.Plugin{
		"driver": driverPlugin,
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: HandshakeConfig,
		Plugins:         pluginMap,
	})
}

// ---- RPC server domain ----

// driverPluginRPCServer implements an RPC version of Driver and is run
// inside a plugin. It wraps an underlying implementation of Driver.
type driverPluginRPCServer struct {
	impl Driver
}

func (ds *driverPluginRPCServer) Name(_ struct{}, resp *string) error {
	var err error
	*resp, err = ds.impl.Name()
	return err
}

func (ds *driverPluginRPCServer) Exit(_ struct{}, _ *struct{}) error {
	ds.impl.Exit()
	return nil
}
