package main

import (
	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/devices/gpu/nvidia"
	"github.com/hashicorp/nomad/plugins"
)

func main() {
	// Serve the plugin
	plugins.Serve(factory)
}

// factory returns a new instance of the Nvidia GPU plugin
func factory(log log.Logger) interface{} {
	return nvidia.NewNvidiaDevice(log)
}
