package plugins

import (
	"fmt"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/device"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// PluginFactory returns a new plugin instance
type PluginFactory func(log log.Logger) interface{}

// Serve is used to serve a new Nomad plugin
func Serve(f PluginFactory) {
	logger := log.New(&log.LoggerOptions{
		Level:      log.Trace,
		JSONFormat: true,
	})

	plugin := f(logger)
	switch p := plugin.(type) {
	case device.DevicePlugin:
		device.Serve(p, logger)
	case drivers.DriverPlugin:
		drivers.Serve(p, logger)
	default:
		fmt.Println("Unsupported plugin type")
	}
}
