package driver

import (
	"fmt"
	"io"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/plugins"
)

func createExecutor(config *plugin.ClientConfig, w io.Writer) (plugins.Executor, *plugin.Client, error) {
	config.HandshakeConfig = plugins.HandshakeConfig
	config.Plugins = plugins.PluginMap
	config.SyncStdout = w
	config.SyncStderr = w
	executorClient := plugin.NewClient(config)
	rpcClient, err := executorClient.Client()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating rpc client for executor plugin: %v", err)
	}

	raw, err := rpcClient.Dispense("executor")
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dispense the executor plugin: %v", err)
	}
	executorPlugin := raw.(plugins.Executor)
	return executorPlugin, executorClient, nil
}
