// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"encoding/json"
	"os"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/base"
)

// Install a plugin cli handler to ease working with tests
// and external plugins.
// This init() must be initialized last in package required by the child plugin
// process. It's recommended to avoid any other `init()` or inline any necessary calls
// here. See eeaa95d commit message for more details.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "executor" {
		if len(os.Args) != 3 {
			hclog.L().Error("json configuration not provided")
			os.Exit(1)
		}

		config := os.Args[2]
		var executorConfig ExecutorConfig
		if err := json.Unmarshal([]byte(config), &executorConfig); err != nil {
			os.Exit(1)
		}

		f, err := os.OpenFile(executorConfig.LogFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
		if err != nil {
			hclog.L().Error(err.Error())
			os.Exit(1)
		}

		// Create the logger
		logger := hclog.New(&hclog.LoggerOptions{
			Level:      hclog.LevelFromString(executorConfig.LogLevel),
			JSONFormat: true,
			Output:     f,
		})

		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: base.Handshake,
			Plugins: GetPluginMap(
				logger,
				executorConfig.FSIsolation,
			),
			GRPCServer: plugin.DefaultGRPCServer,
			Logger:     logger,
		})
		os.Exit(0)
	}
}
