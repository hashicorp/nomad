package allocrunner

import (
	"fmt"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	clientconfig "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/lib/nsutil"
	"github.com/hashicorp/nomad/client/pluginmanager/drivermanager"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func newNetworkManager(alloc *structs.Allocation, driverManager drivermanager.Manager) (nm drivers.DriverNetworkManager, err error) {
	// The defaultNetworkManager is used if a driver doesn't need to create the network
	nm = &defaultNetworkManager{}
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)

	// default netmode to host, this can be overridden by the task or task group
	tgNetMode := "host"
	if len(tg.Networks) > 0 && tg.Networks[0].Mode != "" {
		tgNetMode = tg.Networks[0].Mode
	}

	// networkInitiator tracks the task driver which needs to create the network
	// to check for multiple drivers needing the create the network
	var networkInitiator string

	// driverCaps tracks which drivers we've checked capabilities for so as not
	// to do extra work
	driverCaps := make(map[string]struct{})
	for _, task := range tg.Tasks {
		// the task's netmode defaults to the the task group but can be overridden
		taskNetMode := tgNetMode
		if len(task.Resources.Networks) > 0 && task.Resources.Networks[0].Mode != "" {
			taskNetMode = task.Resources.Networks[0].Mode
		}

		// netmode host should always work to support backwards compat
		if taskNetMode == "host" {
			continue
		}

		// check to see if capabilities of this task's driver have already been checked
		if _, ok := driverCaps[task.Driver]; ok {
			continue
		}

		driver, err := driverManager.Dispense(task.Driver)
		if err != nil {
			return nil, fmt.Errorf("failed to dispense driver %s: %v", task.Driver, err)
		}

		caps, err := driver.Capabilities()
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve capabilities for driver %s: %v",
				task.Driver, err)
		}

		// check that the driver supports the requested network isolation mode
		netIsolationMode := netModeToIsolationMode(taskNetMode)
		if !caps.HasNetIsolationMode(netIsolationMode) {
			return nil, fmt.Errorf("task %s does not support %q networking mode", task.Name, taskNetMode)
		}

		// check if the driver needs to create the network and if a different
		// driver has already claimed it needs to initiate the network
		if caps.MustInitiateNetwork {
			if networkInitiator != "" {
				return nil, fmt.Errorf("tasks %s and %s want to initiate networking but only one driver can do so", networkInitiator, task.Name)
			}
			netManager, ok := driver.(drivers.DriverNetworkManager)
			if !ok {
				return nil, fmt.Errorf("driver %s does not implement network management RPCs", task.Driver)
			}

			nm = netManager
			networkInitiator = task.Name
		}

		// mark this driver's capabilities as checked
		driverCaps[task.Driver] = struct{}{}
	}

	return nm, nil
}

// defaultNetworkManager creates a network namespace for the alloc
type defaultNetworkManager struct{}

func (*defaultNetworkManager) CreateNetwork(allocID string) (*drivers.NetworkIsolationSpec, bool, error) {
	netns, err := nsutil.NewNS(allocID)
	if err != nil {
		return nil, false, err
	}

	spec := &drivers.NetworkIsolationSpec{
		Mode:   drivers.NetIsolationModeGroup,
		Path:   netns.Path(),
		Labels: make(map[string]string),
	}

	return spec, true, nil
}

func (*defaultNetworkManager) DestroyNetwork(allocID string, spec *drivers.NetworkIsolationSpec) error {
	return nsutil.UnmountNS(spec.Path)
}

func netModeToIsolationMode(netMode string) drivers.NetIsolationMode {
	switch strings.ToLower(netMode) {
	case "host":
		return drivers.NetIsolationModeHost
	case "bridge", "none":
		return drivers.NetIsolationModeGroup
	case "driver":
		return drivers.NetIsolationModeTask
	default:
		if strings.HasPrefix(strings.ToLower(netMode), "cni/") {
			return drivers.NetIsolationModeGroup
		}
		return drivers.NetIsolationModeHost
	}
}

func newNetworkConfigurator(log hclog.Logger, alloc *structs.Allocation, config *clientconfig.Config) (NetworkConfigurator, error) {
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)

	// Check if network stanza is given
	if len(tg.Networks) == 0 {
		return &hostNetworkConfigurator{}, nil
	}

	netMode := strings.ToLower(tg.Networks[0].Mode)
	ignorePortMappingHostIP := config.BindWildcardDefaultHostNetwork
	if len(config.HostNetworks) > 0 {
		ignorePortMappingHostIP = false
	}

	switch {
	case netMode == "bridge":
		return newBridgeNetworkConfigurator(log, config.BridgeNetworkName, config.BridgeNetworkAllocSubnet, config.CNIPath, ignorePortMappingHostIP)
	case strings.HasPrefix(netMode, "cni/"):
		return newCNINetworkConfigurator(log, config.CNIPath, config.CNIInterfacePrefix, config.CNIConfigDir, netMode[4:], ignorePortMappingHostIP)
	default:
		return &hostNetworkConfigurator{}, nil
	}
}
