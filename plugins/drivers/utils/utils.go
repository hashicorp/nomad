package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
)

const (
	// ExecutorDefaultMaxPort is the default max port used by the executor for
	// searching for an available port
	ExecutorDefaultMaxPort = 14512

	// ExecutorDefaultMinPort is the default min port used by the executor for
	// searching for an available port
	ExecutorDefaultMinPort = 14000
)

// SetEnvvars sets path and host env vars depending on the FS isolation used.
func SetEnvvars(envBuilder *env.Builder, fsi cstructs.FSIsolation, taskDir *allocdir.TaskDir, conf *config.Config) {
	// Set driver-specific environment variables
	switch fsi {
	case cstructs.FSIsolationNone:
		// Use host paths
		envBuilder.SetAllocDir(taskDir.SharedAllocDir)
		envBuilder.SetTaskLocalDir(taskDir.LocalDir)
		envBuilder.SetSecretsDir(taskDir.SecretsDir)
	default:
		// filesystem isolation; use container paths
		envBuilder.SetAllocDir(allocdir.SharedAllocContainerPath)
		envBuilder.SetTaskLocalDir(allocdir.TaskLocalContainerPath)
		envBuilder.SetSecretsDir(allocdir.TaskSecretsContainerPath)
	}

	// Set the host environment variables for non-image based drivers
	if fsi != cstructs.FSIsolationImage {
		filter := strings.Split(conf.ReadDefault("env.blacklist", config.DefaultEnvBlacklist), ",")
		envBuilder.SetHostEnvvars(filter)
	}
}

// CgroupsMounted returns true if the cgroups are mounted on a system otherwise
// returns false
func CgroupsMounted(node *structs.Node) bool {
	_, ok := node.Attributes["unique.cgroup.mountpoint"]
	return ok
}

// CreateExecutor launches an executor plugin and returns an instance of the
// Executor interface
func CreateExecutor(w io.Writer, level hclog.Level, driverConfig *base.ClientDriverConfig,
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

	if driverConfig != nil {
		config.MaxPort = driverConfig.ClientMaxPort
		config.MinPort = driverConfig.ClientMinPort
	} else {
		config.MaxPort = ExecutorDefaultMaxPort
		config.MinPort = ExecutorDefaultMinPort
	}

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

// CreateExecutorWithConfig launches a plugin with a given plugin config
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
