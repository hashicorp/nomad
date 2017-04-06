package fingerprint

import (
	"fmt"
	"log"
	"net"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// defaultNetworkSpeed is the speed set if the network link speed could not
	// be detected.
	defaultNetworkSpeed = 1000
)

// NetworkFingerprint is used to fingerprint the Network capabilities of a node
type NetworkFingerprint struct {
	StaticFingerprinter
	logger            *log.Logger
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
func NewNetworkFingerprint(logger *log.Logger) Fingerprint {
	f := &NetworkFingerprint{logger: logger, interfaceDetector: &DefaultNetworkInterfaceDetector{}}
	return f
}

func (f *NetworkFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	if node.Resources == nil {
		node.Resources = &structs.Resources{}
	}

	// Find the named interface
	intf, err := f.findInterface(cfg.NetworkInterface)
	switch {
	case err != nil:
		return false, fmt.Errorf("Error while detecting network interface during fingerprinting: %v", err)
	case intf == nil:
		// No interface could be found
		return false, nil
	}

	// Record the throughput of the interface
	var mbits int
	throughput := f.linkSpeed(intf.Name)
	if cfg.NetworkSpeed != 0 {
		mbits = cfg.NetworkSpeed
		f.logger.Printf("[DEBUG] fingerprint.network: setting link speed to user configured speed: %d", mbits)
	} else if throughput != 0 {
		mbits = throughput
		f.logger.Printf("[DEBUG] fingerprint.network: link speed for %v set to %v", intf.Name, mbits)
	} else {
		mbits = defaultNetworkSpeed
		f.logger.Printf("[DEBUG] fingerprint.network: link speed could not be detected and no speed specified by user. Defaulting to %d", defaultNetworkSpeed)
	}

	var ips []net.IP
	if ips, err = f.ipAddrs(intf); err != nil || len(ips) == 0 {
		return false, fmt.Errorf("Unable to find IP address of interface: %s, err: %v", intf.Name, err)
	}

	for _, ip := range ips {
		newNetwork := &structs.NetworkResource{}
		newNetwork.Device = intf.Name
		newNetwork.IP = ip.String()
		if ip.To4() != nil {
			newNetwork.CIDR = newNetwork.IP + "/32"
		} else {
			newNetwork.CIDR = newNetwork.IP + "/64"
		}
		newNetwork.MBits = mbits
		node.Resources.Networks = append(node.Resources.Networks, newNetwork)

		f.logger.Printf("[DEBUG] fingerprint.network: Detected interface %v with IP %v during fingerprinting", intf.Name, ip)
	}

	// return true, because we have a network connection
	return true, nil
}

// ipAddrs gets all the ipv addresses associated with a network interface
func (f *NetworkFingerprint) ipAddrs(intf *net.Interface) ([]net.IP, error) {
	var addrs []net.Addr
	var err error

	if addrs, err = f.interfaceDetector.Addrs(intf); err != nil {
		return nil, err
	}

	ips := make([]net.IP, 0)
	for _, addr := range addrs {
		var ip net.IP
		switch v := (addr).(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		ips = append(ips, ip)
	}

	return ips, nil

}

// Checks if the device is marked UP by the operator
func (f *NetworkFingerprint) isDeviceEnabled(intf *net.Interface) bool {
	return intf.Flags&net.FlagUp != 0
}

// Checks if the device has any IP address configured
func (f *NetworkFingerprint) deviceHasIpAddress(intf *net.Interface) bool {
	_, err := f.ipAddrs(intf)
	return err == nil
}

func (n *NetworkFingerprint) isDeviceLoopBackOrPointToPoint(intf *net.Interface) bool {
	return intf.Flags&(net.FlagLoopback|net.FlagPointToPoint) != 0
}

// Returns the interface with the name passed by user
// If the name is blank then it iterates through all the devices
// and finds one which is routable and marked as UP
// It excludes PPP and lo devices unless they are specifically asked
func (f *NetworkFingerprint) findInterface(deviceName string) (*net.Interface, error) {
	var interfaces []net.Interface
	var err error

	if deviceName != "" {
		return f.interfaceDetector.InterfaceByName(deviceName)
	}

	var intfs []net.Interface

	if intfs, err = f.interfaceDetector.Interfaces(); err != nil {
		return nil, err
	}

	for _, intf := range intfs {
		if f.isDeviceEnabled(&intf) && !f.isDeviceLoopBackOrPointToPoint(&intf) && f.deviceHasIpAddress(&intf) {
			interfaces = append(interfaces, intf)
		}
	}

	if len(interfaces) == 0 {
		return nil, nil
	}
	return &interfaces[0], nil
}
