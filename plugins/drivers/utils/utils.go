package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

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
func CreateExecutor(w io.Writer, level string, CMinPort, CMaxPort uint,
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
	config.Plugins = driver.GetPluginMap(w, level)
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
	config.Plugins = driver.GetPluginMap(w, "DEBUG")

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
	// 0.6 Upgrade path: Deregister services from the executor as the Nomad
	// client agent now handles all Consul interactions. Ignore errors as
	// this shouldn't cause the alloc to fail and there's nothing useful to
	// do with them.
	executorPlugin.DeregisterServices()
	return executorPlugin, executorClient, nil
}

// ValidateCommand validates that the command only has a single value and
// returns a user friendly error message telling them to use the passed
// argField.
func ValidateCommand(command, argField string) error {
	trimmed := strings.TrimSpace(command)
	if len(trimmed) == 0 {
		return fmt.Errorf("command empty: %q", command)
	}

	if len(trimmed) != len(command) {
		return fmt.Errorf("command contains extra white space: %q", command)
	}

	return nil
}

// KillProcess kills a process with the given pid
func KillProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
