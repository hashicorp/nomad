package plugin

import (
	"net/rpc"

	plugin "github.com/hashicorp/go-plugin"
)

// HandshakeConfigs are used to just do a basic handshake between
// a plugin and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad plugins or executing a plugin
// directory. It is a UX feature, not a security feature.
var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "NOMAD_DRIVER_PLUGIN",
	MagicCookieValue: "f85c4f83-0e60-4f73-9f1b-a7e153b2650f",
}

// DriverPlugin implements go-plugin's Plugin interface. It has methods for
// retrieving a server and a client instance of the plugin.
type DriverPlugin struct {
	impl Driver
}

func NewDriverPlugin(d Driver) *DriverPlugin {
	return &DriverPlugin{impl: d}
}

func (d DriverPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &driverPluginRPCServer{impl: d.impl}, nil
}

func (DriverPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &driverPluginRPCClient{client: c}, nil
}
