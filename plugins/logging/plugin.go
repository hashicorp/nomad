package logging

import (
	"context"
	"fmt"

	plugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/hashicorp/nomad/plugins/base"
	loglib "github.com/hashicorp/nomad/plugins/logging/lib"
	"github.com/hashicorp/nomad/plugins/logging/proto"
)

const (
	// ApiVersion010 is the initial API version for logging plugins
	ApiVersion010 = "v0.1.0"
)

var (
	// ErrPluginDisabled indicates that the logging plugin is disabled
	ErrPluginDisabled = fmt.Errorf("plugin is not enabled")
)

// LoggingPlugin is the interface which logging plugins will implement. It is
// also implemented by a plugin client which proxies the calls to go-plugin. See
// the proto/logging.proto file for detailed information about each RPC and
// message structure.
type LoggingPlugin interface {
	base.BasePlugin

	Start(*loglib.LogConfig) error
	Stop(*loglib.LogConfig) error
	Fingerprint(ctx context.Context) (<-chan *FingerprintResponse, error)
}

// TODO
type FingerprintResponse struct {
	Error error
}

type Plugin struct {
	plugin.NetRPCUnsupportedPlugin
	impl LoggingPlugin
}

// NewPlugin should be called when the plugin's main function calls
// go-plugin.Serve; the plugin main will pass in the concrete implementation of
// the LoggingPlugin interface.
func NewPlugin(i LoggingPlugin) plugin.Plugin {
	return &Plugin{impl: i}
}

// GRPCServer is needed for the go-plugin interface
func (p *Plugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterLoggingPluginServer(s, &LoggingPluginServer{
		impl:   p.impl,
		broker: broker,
	})
	return nil
}

// GRPCClient is needed for the go-plugin interface
func (p *Plugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &LoggingPluginClient{
		doneCtx: ctx,
		client:  proto.NewLoggingPluginClient(c),
	}, nil
}
