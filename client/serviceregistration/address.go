// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package serviceregistration

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

// GetAddress returns the IP (or custom advertise address) and port to use for a
// service or check registration. If no port label is specified (an empty value)
// and no custom address is specified, zero values are returned because no address
// could be resolved.
func GetAddress(
	address, // custom address, if set
	addressMode,
	portLabel string,
	networks structs.Networks,
	driverNet *drivers.DriverNetwork,
	ports structs.AllocatedPorts,
	netStatus *structs.AllocNetworkStatus,
) (string, int, error) {
	switch addressMode {
	case structs.AddressModeAuto:
		switch {
		case address != "":
			// No port no problem, just return the advertise address.
			if portLabel == "" {
				return address, 0, nil
			}
			// A custom advertise address can be used with a port map; using the
			// Value and ignoring the IP. The routing from your custom address to
			// the group network address is DIY. (e.g. EC2 public address)
			if mapping, exists := ports.Get(portLabel); exists {
				return address, mapping.Value, nil
			}
			// If not a port map we can interpolate a numeric port for you.
			port, err := strconv.Atoi(portLabel)
			if err != nil {
				return "", 0, fmt.Errorf("invalid port: %q: not a valid port mapping or numeric port", portLabel)
			}
			return address, port, nil
		case driverNet.Advertise():
			return GetAddress("", structs.AddressModeDriver, portLabel, networks, driverNet, ports, netStatus)
		default:
			return GetAddress("", structs.AddressModeHost, portLabel, networks, driverNet, ports, netStatus)
		}
	case structs.AddressModeHost:
		// Cannot use address mode host with custom advertise address.
		if address != "" {
			return "", 0, fmt.Errorf("cannot use custom advertise address with %q address mode", structs.AddressModeHost)
		}

		if portLabel == "" {
			if len(networks) != 1 {
				// If no networks are specified return zero
				// values. Consul will advertise the host IP
				// with no port. This is the pre-0.7.1 behavior
				// some people rely on.
				return "", 0, nil
			}
			return networks[0].IP, 0, nil
		}

		// Default path: use host ip:port
		// Try finding port in the AllocatedPorts struct first
		// Check in Networks struct for backwards compatibility if not found
		mapping, ok := ports.Get(portLabel)
		if !ok {
			mapping = networks.Port(portLabel)
			if mapping.Value > 0 {
				return mapping.HostIP, mapping.Value, nil
			}

			// If port isn't a label, try to parse it as a literal port number
			port, err := strconv.Atoi(portLabel)
			if err != nil {
				// Don't include Atoi error message as user likely
				// never intended it to be a numeric and it creates a
				// confusing error message
				return "", 0, fmt.Errorf("invalid port %q: port label not found", portLabel)
			}
			if port <= 0 {
				return "", 0, fmt.Errorf("invalid port: %q: port must be >0", portLabel)
			}

			// A number was given which will use the Consul agent's address and the given port
			// Returning a blank string as an address will use the Consul agent's address
			return "", port, nil
		}
		return mapping.HostIP, mapping.Value, nil

	case structs.AddressModeDriver:
		// Cannot use address mode driver with custom advertise address.
		if address != "" {
			return "", 0, fmt.Errorf("cannot use custom advertise address with %q address mode", structs.AddressModeDriver)
		}

		// Require a driver network if driver address mode is used
		if driverNet == nil {
			return "", 0, fmt.Errorf(`cannot use address_mode="driver": no driver network exists`)
		}

		// If no port label is specified just return the IP
		if portLabel == "" {
			return driverNet.IP, 0, nil
		}

		// If the port is a label, use the driver's port (not the host's)
		if port, ok := ports.Get(portLabel); ok {
			return driverNet.IP, port.To, nil
		}

		// Check if old style driver portmap is used
		if port, ok := driverNet.PortMap[portLabel]; ok {
			return driverNet.IP, port, nil
		}

		// If port isn't a label, try to parse it as a literal port number
		port, err := strconv.Atoi(portLabel)
		if err != nil {
			// Don't include Atoi error message as user likely
			// never intended it to be a numeric and it creates a
			// confusing error message
			return "", 0, fmt.Errorf("invalid port label %q: port labels in driver address_mode must be numeric or in the driver's port map", portLabel)
		}
		if port <= 0 {
			return "", 0, fmt.Errorf("invalid port: %q: port must be >0", portLabel)
		}

		return driverNet.IP, port, nil

	case structs.AddressModeAlloc:
		// Cannot use address mode alloc with custom advertise address.
		if address != "" {
			return "", 0, fmt.Errorf("cannot use custom advertise address with %q address mode", structs.AddressModeAlloc)
		}

		// Going to need a network for this.
		if netStatus == nil {
			return "", 0, fmt.Errorf(`cannot use address_mode="alloc": no allocation network status reported`)
		}

		// If no port label is specified just return the IP
		if portLabel == "" {
			return netStatus.Address, 0, nil
		}

		// If port is a label and is found then return it
		if port, ok := ports.Get(portLabel); ok {
			// Use port.To value unless not set
			if port.To > 0 {
				return netStatus.Address, port.To, nil
			}
			return netStatus.Address, port.Value, nil
		}

		// Check if port is a literal number
		port, err := strconv.Atoi(portLabel)
		if err != nil {
			// User likely specified wrong port label here
			return "", 0, fmt.Errorf("invalid port %q: port label not found or is not numeric", portLabel)
		}
		if port <= 0 {
			return "", 0, fmt.Errorf("invalid port: %q: port must be >0", portLabel)
		}
		return netStatus.Address, port, nil

	default:
		// Shouldn't happen due to validation, but enforce invariants
		return "", 0, fmt.Errorf("invalid address mode %q", addressMode)
	}
}
