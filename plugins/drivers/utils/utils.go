package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/nomad/structs"
)

// CgroupsMounted returns true if the cgroups are mounted on a system otherwise
// returns false
func CgroupsMounted(node *structs.Node) bool {
	_, ok := node.Attributes["unique.cgroup.mountpoint"]
	return ok
}

// CreateExecutor launches an executor plugin and returns an instance of the
// Executor interface
func CreateExecutor(w io.Writer, level hclog.Level, CMinPort, CMaxPort uint,
	executorConfig *dstructs.ExecutorConfig) (executor.Executor, *plugin.Client, error) {

	c, err := json.Marshal(executorConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create executor config: %v", err)
	}
	bin, err := discover.NomadExecutable()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to find the nomad binary: %v", err)
	}

	config := &plugin.ClientConfig{
		Cmd: exec.Command(bin, "executor", string(c)),
	}
	config.HandshakeConfig = driver.HandshakeConfig
	config.Plugins = driver.GetPluginMap(w, level, executorConfig.FSIsolation)
	config.MaxPort = CMaxPort
	config.MinPort = CMinPort

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

func CreateExecutorWithConfig(config *plugin.ClientConfig, w io.Writer) (executor.Executor, *plugin.Client, error) {
	config.HandshakeConfig = driver.HandshakeConfig

	// Setting this to DEBUG since the log level at the executor server process
	// is already set, and this effects only the executor client.
	config.Plugins = driver.GetPluginMap(w, hclog.Debug, false)

	executorClient := plugin.NewClient(config)
	rpcClient, err := executorClient.Client()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating rpc client for executor plugin: %v", err)
	}

	raw, err := rpcClient.Dispense("executor")
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dispense the executor plugin: %v", err)
	}
	executorPlugin, ok := raw.(*driver.ExecutorRPC)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected executor rpc type: %T", raw)
	}
	return executorPlugin, executorClient, nil
}

// KillProcess kills a process with the given pid
func KillProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
