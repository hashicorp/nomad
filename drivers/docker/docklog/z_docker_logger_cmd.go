// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docklog

import (
	"os"

	log "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base"
)

// Install a plugin cli handler to ease working with tests
// and external plugins.
// This init() must be initialized last in package required by the child plugin
// process. It's recommended to avoid any other `init()` or inline any necessary calls
// here. See eeaa95d commit message for more details.
func init() {
	if len(os.Args) > 1 && os.Args[1] == PluginName {
		logger := log.New(&log.LoggerOptions{
			Level:      log.Trace,
			JSONFormat: true,
			Name:       PluginName,
		})

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: base.Handshake,
			Plugins: map[string]plugin.Plugin{
				PluginName: NewPlugin(NewDockerLogger(logger)),
			},
			GRPCServer: plugin.DefaultGRPCServer,
			Logger:     logger,
		})
		os.Exit(0)
	}
}
