package driver

import (
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/go-multierror"
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

func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

func destroyPlugin(pluginPid int, userPid int) error {
	var merr error
	if err := killProcess(pluginPid); err != nil {
		merr = multierror.Append(merr, err)
	}

	if err := killProcess(userPid); err != nil {
		merr = multierror.Append(merr, err)
	}
	return merr
}
