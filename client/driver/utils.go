package driver

import (
	"fmt"
	"io"
	"net"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	"github.com/hashicorp/nomad/client/driver/syslog"
)

// createExecutor launches an executor plugin and returns an instance of the
// Executor interface
func createExecutor(config *plugin.ClientConfig, w io.Writer,
	clientConfig *config.Config) (executor.Executor, *plugin.Client, error) {
	config.HandshakeConfig = HandshakeConfig
	config.Plugins = GetPluginMap(w)
	config.MaxPort = clientConfig.ClientMaxPort
	config.MinPort = clientConfig.ClientMinPort

	// setting the setsid of the plugin process so that it doesn't get signals sent to
	// the nomad client.
	if config.Cmd != nil {
		isolateCommand(config.Cmd)
	}

	executorClient := plugin.NewClient(config)
	rpcClient, err := executorClient.Client()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating rpc client for executor plugin: %v", err)
	}

	raw, err := rpcClient.Dispense("executor")
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dispense the executor plugin: %v", err)
	}
	executorPlugin := raw.(executor.Executor)
	return executorPlugin, executorClient, nil
}

func createLogCollector(config *plugin.ClientConfig, w io.Writer,
	clientConfig *config.Config) (syslog.LogCollector, *plugin.Client, error) {
	config.HandshakeConfig = HandshakeConfig
	config.Plugins = GetPluginMap(w)
	config.MaxPort = clientConfig.ClientMaxPort
	config.MinPort = clientConfig.ClientMinPort
	if config.Cmd != nil {
		isolateCommand(config.Cmd)
	}

	syslogClient := plugin.NewClient(config)
	rpcCLient, err := syslogClient.Client()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating rpc client for syslog plugin: %v", err)
	}

	raw, err := rpcCLient.Dispense("syslogcollector")
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dispense the syslog plugin: %v", err)
	}
	logCollector := raw.(syslog.LogCollector)
	return logCollector, syslogClient, nil
}

// getFreePort returns a free port ready to be listened on between upper and
// lower bounds
func getFreePort(lowerBound uint, upperBound uint) (net.Addr, error) {
	for i := lowerBound; i <= upperBound; i++ {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%v", i))
		if err != nil {
			return nil, err
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			continue
		}
		defer l.Close()
		return l.Addr(), nil
	}
	return nil, fmt.Errorf("No free port found")
}

// killProcess kills a process with the given pid
func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

// destroyPlugin kills the plugin with the given pid and also kills the user
// process
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
