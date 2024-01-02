// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"net"
	"strings"

	log "github.com/hashicorp/go-hclog"
	sockaddr "github.com/hashicorp/go-sockaddr"
	"github.com/hashicorp/go-sockaddr/template"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// defaultNetworkSpeed is the speed set if the network link speed could not
	// be detected.
	defaultNetworkSpeed = 1000

	// networkDisallowLinkLocalOption/Default are used to allow the operator to
	// decide how the fingerprinter handles an interface that only contains link
	// local addresses.
	networkDisallowLinkLocalOption  = "fingerprint.network.disallow_link_local"
	networkDisallowLinkLocalDefault = false
)

// NetworkFingerprint is used to fingerprint the Network capabilities of a node
type NetworkFingerprint struct {
	StaticFingerprinter
	logger            log.Logger
	interfaceDetector NetworkInterfaceDetector
}

// An interface to isolate calls to various api in net package
// This facilitates testing where we can implement
// fake interfaces and addresses to test varios code paths
type NetworkInterfaceDetector interface {
	Interfaces() ([]net.Interface, error)
	InterfaceByName(name string) (*net.Interface, error)
	Addrs(intf *net.Interface) ([]net.Addr, error)
}

// Implements the interface detector which calls net directly
type DefaultNetworkInterfaceDetector struct {
}

func (b *DefaultNetworkInterfaceDetector) Interfaces() ([]net.Interface, error) {
	return net.Interfaces()
}

func (b *DefaultNetworkInterfaceDetector) InterfaceByName(name string) (*net.Interface, error) {
	return net.InterfaceByName(name)
}

func (b *DefaultNetworkInterfaceDetector) Addrs(intf *net.Interface) ([]net.Addr, error) {
	return intf.Addrs()
}

// NewNetworkFingerprint returns a new NetworkFingerprinter with the given
// logger
func NewNetworkFingerprint(logger log.Logger) Fingerprint {
	f := &NetworkFingerprint{logger: logger.Named("network"), interfaceDetector: &DefaultNetworkInterfaceDetector{}}
	return f
}

func (f *NetworkFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	cfg := req.Config

	// Find the named interface
	intf, err := f.findInterface(cfg.NetworkInterface)
	switch {
	case err != nil:
		return fmt.Errorf("Error while detecting network interface %s during fingerprinting: %v",
			cfg.NetworkInterface,
			err)
	case intf == nil:
		// No interface could be found
		return nil
	}

	// Create a sub-logger with common values to help with debugging
	logger := f.logger.With("interface", intf.Name)

	// Record the throughput of the interface
	var mbits int
	throughput := f.linkSpeed(intf.Name)
	if cfg.NetworkSpeed != 0 {
		mbits = cfg.NetworkSpeed
		logger.Debug("setting link speed to user configured speed", "mbits", mbits)
	} else if throughput != 0 {
		mbits = throughput
		logger.Debug("link speed detected", "mbits", mbits)
	} else {
		mbits = defaultNetworkSpeed
		logger.Debug("link speed could not be detected and no speed specified by user, falling back to default speed", "mbits", defaultNetworkSpeed)
	}

	// Create the network resources from the interface
	disallowLinkLocal := cfg.ReadBoolDefault(networkDisallowLinkLocalOption, networkDisallowLinkLocalDefault)
	nwResources, err := f.createNetworkResources(mbits, intf, disallowLinkLocal)
	if err != nil {
		return err
	}

	// COMPAT(0.10): Remove in 0.10
	resp.Resources = &structs.Resources{
		Networks: nwResources,
	}

	resp.NodeResources = &structs.NodeResources{
		Networks: nwResources,
	}

	for _, nwResource := range nwResources {
		logger.Debug("detected interface IP", "IP", nwResource.IP)
	}

	// Deprecated, setting the first IP as unique IP for the node
	if len(nwResources) > 0 {
		resp.AddAttribute("unique.network.ip-address", nwResources[0].IP)
	}

	ifaces, err := f.interfaceDetector.Interfaces()
	if err != nil {
		return err
	}
	nodeNetResources, err := f.createNodeNetworkResources(ifaces, disallowLinkLocal, req.Config)
	if err != nil {
		return err
	}
	resp.NodeResources.NodeNetworks = nodeNetResources

	resp.Detected = true

	return nil
}

func (f *NetworkFingerprint) createNodeNetworkResources(ifaces []net.Interface, disallowLinkLocal bool, conf *config.Config) ([]*structs.NodeNetworkResource, error) {
	nets := make([]*structs.NodeNetworkResource, 0)
	for _, iface := range ifaces {
		speed := f.linkSpeed(iface.Name)
		if speed == 0 {
			speed = defaultNetworkSpeed
			f.logger.Debug("link speed could not be detected, falling back to default speed", "interface", iface.Name, "mbits", defaultNetworkSpeed)
		}

		newNetwork := &structs.NodeNetworkResource{
			Mode:       "host",
			Device:     iface.Name,
			MacAddress: iface.HardwareAddr.String(),
			Speed:      speed,
		}
		addrs, err := f.interfaceDetector.Addrs(&iface)
		if err != nil {
			return nil, err
		}
		var networkAddrs, linkLocalAddrs []structs.NodeNetworkAddress
		for _, addr := range addrs {
			// Find the IP Addr and the CIDR from the Address
			var ip net.IP
			var family structs.NodeNetworkAF
			switch v := (addr).(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip.To4() != nil {
				family = structs.NodeNetworkAF_IPv4
			} else {
				family = structs.NodeNetworkAF_IPv6
			}
			for _, alias := range deriveAddressAliases(iface, ip, conf) {
				newAddr := structs.NodeNetworkAddress{
					Address: ip.String(),
					Family:  family,
					Alias:   alias,
				}

				if hostNetwork, ok := conf.HostNetworks[alias]; ok {
					newAddr.ReservedPorts = hostNetwork.ReservedPorts
				}

				if newAddr.Alias != "" {
					if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
						linkLocalAddrs = append(linkLocalAddrs, newAddr)
					} else {
						networkAddrs = append(networkAddrs, newAddr)
					}
				}
			}
		}

		if len(networkAddrs) == 0 && len(linkLocalAddrs) > 0 {
			if disallowLinkLocal {
				f.logger.Debug("ignoring detected link-local address on interface", "interface", iface.Name)
			} else {
				newNetwork.Addresses = linkLocalAddrs
			}
		} else {
			newNetwork.Addresses = networkAddrs
		}

		if len(newNetwork.Addresses) > 0 {
			nets = append(nets, newNetwork)
		}
	}
	return nets, nil
}

func deriveAddressAliases(iface net.Interface, addr net.IP, config *config.Config) (aliases []string) {
	for name, conf := range config.HostNetworks {
		var cidrMatch, ifaceMatch bool
		if conf.CIDR != "" {
			for _, cidr := range strings.Split(conf.CIDR, ",") {
				_, ipnet, err := net.ParseCIDR(cidr)
				if err != nil {
					continue
				}

				if ipnet.Contains(addr) {
					cidrMatch = true
					break
				}
			}
		} else {
			cidrMatch = true
		}
		if conf.Interface != "" {
			ifaceName, err := template.Parse(conf.Interface)
			if err != nil {
				continue
			}

			if ifaceName == iface.Name {
				ifaceMatch = true
			}
		} else {
			ifaceMatch = true
		}
		if cidrMatch && ifaceMatch {
			aliases = append(aliases, name)
		}
	}

	if config.NetworkInterface != "" {
		if config.NetworkInterface == iface.Name {
			aliases = append(aliases, "default")
		}
	} else if ri, err := sockaddr.NewRouteInfo(); err == nil {
		defaultIface, err := ri.GetDefaultInterfaceName()
		if err == nil && iface.Name == defaultIface {
			aliases = append(aliases, "default")
		}
	}

	return
}

// createNetworkResources creates network resources for every IP
func (f *NetworkFingerprint) createNetworkResources(throughput int, intf *net.Interface, disallowLinkLocal bool) ([]*structs.NetworkResource, error) {
	// Find the interface with the name
	addrs, err := f.interfaceDetector.Addrs(intf)
	if err != nil {
		return nil, err
	}

	nwResources := make([]*structs.NetworkResource, 0)
	linkLocals := make([]*structs.NetworkResource, 0)

	for _, addr := range addrs {
		// Create a new network resource
		newNetwork := &structs.NetworkResource{
			Mode:   "host",
			Device: intf.Name,
			MBits:  throughput,
		}

		// Find the IP Addr and the CIDR from the Address
		var ip net.IP
		switch v := (addr).(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}

		newNetwork.IP = ip.String()
		if ip.To4() != nil {
			newNetwork.CIDR = newNetwork.IP + "/32"
		} else {
			newNetwork.CIDR = newNetwork.IP + "/128"
		}

		// If the ip is link-local then we ignore it unless the user allows it
		// and we detect nothing else
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			linkLocals = append(linkLocals, newNetwork)
			continue
		}

		nwResources = append(nwResources, newNetwork)
	}

	if len(nwResources) == 0 && len(linkLocals) != 0 {
		if disallowLinkLocal {
			f.logger.Debug("ignoring detected link-local address on interface", "interface", intf.Name)
			return nwResources, nil
		}

		return linkLocals, nil
	}

	return nwResources, nil
}

// Returns the interface with the name passed by user. If the name is blank, we
// use the interface attached to the default route.
func (f *NetworkFingerprint) findInterface(deviceName string) (*net.Interface, error) {
	// If we aren't given a device, look it up by using the interface with the default route
	if deviceName == "" {
		ri, err := sockaddr.NewRouteInfo()
		if err != nil {
			return nil, err
		}

		defaultIfName, err := ri.GetDefaultInterfaceName()
		if err != nil {
			return nil, err
		}
		if defaultIfName == "" {
			return nil, fmt.Errorf("no network_interface given and failed to determine interface attached to default route")
		}
		deviceName = defaultIfName
	}

	return f.interfaceDetector.InterfaceByName(deviceName)
}
