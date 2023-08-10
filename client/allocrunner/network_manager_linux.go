// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"
	"os"
	"path"
	"strings"
	"syscall"

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

	groupIsolationMode := netModeToIsolationMode(tgNetMode)

	// Setting the hostname is only possible where the task groups networking
	// mode is group; meaning bridge or none.
	if len(tg.Networks) > 0 &&
		(groupIsolationMode != drivers.NetIsolationModeGroup && tg.Networks[0].Hostname != "") {
		return nil, fmt.Errorf("hostname cannot be set on task group using %q networking mode",
			groupIsolationMode)
	}

	// networkInitiator tracks the task driver which needs to create the network
	// to check for multiple drivers needing to create the network.
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
		} else if len(tg.Networks) > 0 && tg.Networks[0].Hostname != "" {
			// TODO jrasell: remove once the default linux network manager
			//  supports setting the hostname in bridged mode. This currently
			//  indicates only Docker supports this, which is true unless a
			//  custom driver can which means this check still holds as true as
			//  we can tell.
			//  Please see: https://github.com/hashicorp/nomad/issues/11180
			return nil, fmt.Errorf("hostname is not currently supported on driver %s", task.Driver)
		}

		// mark this driver's capabilities as checked
		driverCaps[task.Driver] = struct{}{}
	}

	return nm, nil
}

// defaultNetworkManager creates a network namespace for the alloc
type defaultNetworkManager struct{}

// CreateNetwork is the CreateNetwork implementation of the
// drivers.DriverNetworkManager interface function. It does not currently
// support setting the hostname of the network namespace.
func (*defaultNetworkManager) CreateNetwork(allocID string, _ *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
	netns, err := nsutil.NewNS(allocID)
	if err != nil {
		// when a client restarts, the namespace will already exist and
		// there will be a namespace file in use by the task process
		if e, ok := err.(*os.PathError); ok && e.Err == syscall.EPERM {
			nsPath := path.Join(nsutil.NetNSRunDir, allocID)
			_, err := os.Stat(nsPath)
			if err == nil {
				// Let's return a spec that points to the tested nspath, but indicate
				// that we didn't make the namespace. That will stop the network_hook
				// from calling its networkConfigurator.Setup function in the reconnect
				// case, but provide the spec value necessary for the network_hook's
				// Postrun function to not fast exit.
				spec := &drivers.NetworkIsolationSpec{
					Mode:   drivers.NetIsolationModeGroup,
					Path:   nsPath,
					Labels: make(map[string]string),
				}

				return spec, false, nil
			}
		}
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
	if spec == nil {
		return nil
	}
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

	// Check if network block is given
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
		c, err := newBridgeNetworkConfigurator(log, config.BridgeNetworkName, config.BridgeNetworkAllocSubnet, config.BridgeNetworkHairpinMode, config.CNIPath, ignorePortMappingHostIP)
		if err != nil {
			return nil, err
		}
		return &synchronizedNetworkConfigurator{c}, nil
	case strings.HasPrefix(netMode, "cni/"):
		c, err := newCNINetworkConfigurator(log, config.CNIPath, config.CNIInterfacePrefix, config.CNIConfigDir, netMode[4:], ignorePortMappingHostIP)
		if err != nil {
			return nil, err
		}
		return &synchronizedNetworkConfigurator{c}, nil
	default:
		return &hostNetworkConfigurator{}, nil
	}
}
