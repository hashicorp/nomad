package fingerprint

import (
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Set skipOnlineTestEnvVar to a non-empty value to skip network tests.  Useful
// when working offline (e.g. an airplane).
const skipOnlineTestsEnvVar = "TEST_NOMAD_SKIP_ONLINE_NET"

var (
	lo = net.Interface{
		Index:        2,
		MTU:          65536,
		Name:         "lo",
		HardwareAddr: []byte{23, 43, 54, 54},
		Flags:        net.FlagUp | net.FlagLoopback,
	}

	eth0 = net.Interface{
		Index:        3,
		MTU:          1500,
		Name:         "eth0",
		HardwareAddr: []byte{23, 44, 54, 67},
		Flags:        net.FlagUp | net.FlagMulticast | net.FlagBroadcast,
	}

	eth1 = net.Interface{
		Index:        4,
		MTU:          1500,
		Name:         "eth1",
		HardwareAddr: []byte{23, 44, 54, 69},
		Flags:        net.FlagMulticast | net.FlagBroadcast,
	}

	eth2 = net.Interface{
		Index:        4,
		MTU:          1500,
		Name:         "eth2",
		HardwareAddr: []byte{23, 44, 54, 70},
		Flags:        net.FlagUp | net.FlagBroadcast | net.FlagMulticast,
	}
)

// A fake network detector which returns no devices
type NetworkIntefaceDetectorNoDevices struct {
}

func (f *NetworkIntefaceDetectorNoDevices) Interfaces() ([]net.Interface, error) {
	return make([]net.Interface, 0), nil
}

func (f *NetworkIntefaceDetectorNoDevices) InterfaceByName(name string) (*net.Interface, error) {
	return nil, fmt.Errorf("Device with name %s doesn't exist", name)
}

func (f *NetworkIntefaceDetectorNoDevices) Addrs(intf *net.Interface) ([]net.Addr, error) {
	return nil, fmt.Errorf("No interfaces found for device %v", intf.Name)
}

// A fake network detector which returns only loopback
type NetworkInterfaceDetectorOnlyLo struct {
}

func (n *NetworkInterfaceDetectorOnlyLo) Interfaces() ([]net.Interface, error) {
	return []net.Interface{lo}, nil
}

func (n *NetworkInterfaceDetectorOnlyLo) InterfaceByName(name string) (*net.Interface, error) {
	if name == "lo" {
		return &lo, nil
	}

	return nil, fmt.Errorf("No device with name %v found", name)
}

func (n *NetworkInterfaceDetectorOnlyLo) Addrs(intf *net.Interface) ([]net.Addr, error) {
	if intf.Name == "lo" {
		_, ipnet1, _ := net.ParseCIDR("127.0.0.1/8")
		_, ipnet2, _ := net.ParseCIDR("2001:DB8::/48")
		return []net.Addr{ipnet1, ipnet2}, nil
	}

	return nil, fmt.Errorf("Can't find addresses for device: %v", intf.Name)
}

// A fake network detector which simulates the presence of multiple interfaces
type NetworkInterfaceDetectorMultipleInterfaces struct {
}

func (n *NetworkInterfaceDetectorMultipleInterfaces) Interfaces() ([]net.Interface, error) {
	return []net.Interface{lo, eth0, eth1, eth2}, nil
}

func (n *NetworkInterfaceDetectorMultipleInterfaces) InterfaceByName(name string) (*net.Interface, error) {
	var intf *net.Interface
	switch name {
	case "lo":
		intf = &lo
	case "eth0":
		intf = &eth0
	case "eth1":
		intf = &eth1
	case "eth2":
		intf = &eth2
	}
	if intf != nil {
		return intf, nil
	}

	return nil, fmt.Errorf("No device with name %v found", name)
}

func (n *NetworkInterfaceDetectorMultipleInterfaces) Addrs(intf *net.Interface) ([]net.Addr, error) {
	if intf.Name == "lo" {
		_, ipnet1, _ := net.ParseCIDR("127.0.0.1/8")
		_, ipnet2, _ := net.ParseCIDR("2001:DB8::/48")
		return []net.Addr{ipnet1, ipnet2}, nil
	}

	if intf.Name == "eth0" {
		_, ipnet1, _ := net.ParseCIDR("100.64.0.11/10")
		_, ipnet2, _ := net.ParseCIDR("2001:0db8:85a3:0000:0000:8a2e:0370:7334/64")
		ipAddr, _ := net.ResolveIPAddr("ip6", "fe80::140c:9579:8037:f565")
		return []net.Addr{ipnet1, ipnet2, ipAddr}, nil
	}

	if intf.Name == "eth1" {
		_, ipnet1, _ := net.ParseCIDR("100.64.0.10/10")
		_, ipnet2, _ := net.ParseCIDR("2003:DB8::/48")
		return []net.Addr{ipnet1, ipnet2}, nil
	}

	if intf.Name == "eth2" {
		return []net.Addr{}, nil
	}
	return nil, fmt.Errorf("Can't find addresses for device: %v", intf.Name)
}

func TestNetworkFingerprint_basic(t *testing.T) {
	if v := os.Getenv(skipOnlineTestsEnvVar); v != "" {
		t.Skipf("Environment variable %+q not empty, skipping test", skipOnlineTestsEnvVar)
	}

	f := &NetworkFingerprint{logger: testLogger(), interfaceDetector: &DefaultNetworkInterfaceDetector{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{NetworkSpeed: 101}

	ok, err := f.Fingerprint(cfg, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should apply (HINT: working offline? Set env %q=y", skipOnlineTestsEnvVar)
	}

	assertNodeAttributeContains(t, node, "unique.network.ip-address")

	ip := node.Attributes["unique.network.ip-address"]
	match := net.ParseIP(ip)
	if match == nil {
		t.Fatalf("Bad IP match: %s", ip)
	}

	if node.Resources == nil || len(node.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net := node.Resources.Networks[0]
	if net.IP == "" {
		t.Fatal("Expected Network Resource to not be empty")
	}
	if net.CIDR == "" {
		t.Fatal("Expected Network Resource to have a CIDR")
	}
	if net.Device == "" {
		t.Fatal("Expected Network Resource to have a Device Name")
	}
	if net.MBits != 101 {
		t.Fatalf("Expected Network Resource to have bandwith %d; got %d", 101, net.MBits)
	}
}

func TestNetworkFingerprint_no_devices(t *testing.T) {
	f := &NetworkFingerprint{logger: testLogger(), interfaceDetector: &NetworkIntefaceDetectorNoDevices{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{NetworkSpeed: 100}

	ok, err := f.Fingerprint(cfg, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if ok {
		t.Fatalf("ok: %v", ok)
	}
}

func TestNetworkFingerprint_default_device_absent(t *testing.T) {
	f := &NetworkFingerprint{logger: testLogger(), interfaceDetector: &NetworkInterfaceDetectorOnlyLo{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{NetworkSpeed: 100, NetworkInterface: "eth0"}

	ok, err := f.Fingerprint(cfg, node)
	if err == nil {
		t.Fatalf("err: %v", err)
	}

	if ok {
		t.Fatalf("ok: %v", ok)
	}
}

func TestNetworkFingerPrint_default_device(t *testing.T) {
	f := &NetworkFingerprint{logger: testLogger(), interfaceDetector: &NetworkInterfaceDetectorOnlyLo{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{NetworkSpeed: 100, NetworkInterface: "lo"}

	ok, err := f.Fingerprint(cfg, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should apply")
	}

	assertNodeAttributeContains(t, node, "unique.network.ip-address")

	ip := node.Attributes["unique.network.ip-address"]
	match := net.ParseIP(ip)
	if match == nil {
		t.Fatalf("Bad IP match: %s", ip)
	}

	if node.Resources == nil || len(node.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net := node.Resources.Networks[0]
	if net.IP == "" {
		t.Fatal("Expected Network Resource to not be empty")
	}
	if net.CIDR == "" {
		t.Fatal("Expected Network Resource to have a CIDR")
	}
	if net.Device == "" {
		t.Fatal("Expected Network Resource to have a Device Name")
	}
	if net.MBits == 0 {
		t.Fatal("Expected Network Resource to have a non-zero bandwith")
	}
}

func TestNetworkFingerPrint_excludelo_down_interfaces(t *testing.T) {
	f := &NetworkFingerprint{logger: testLogger(), interfaceDetector: &NetworkInterfaceDetectorMultipleInterfaces{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{NetworkSpeed: 100}

	ok, err := f.Fingerprint(cfg, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should apply")
	}

	assertNodeAttributeContains(t, node, "unique.network.ip-address")

	ip := node.Attributes["unique.network.ip-address"]
	match := net.ParseIP(ip)
	if match == nil {
		t.Fatalf("Bad IP match: %s", ip)
	}

	if node.Resources == nil || len(node.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net := node.Resources.Networks[0]
	if net.IP == "" {
		t.Fatal("Expected Network Resource to have an IP")
	}
	if net.CIDR == "" {
		t.Fatal("Expected Network Resource to have a CIDR")
	}
	if net.Device != "eth0" {
		t.Fatal("Expected Network Resource to be eth0. Actual: ", net.Device)
	}
	if net.MBits == 0 {
		t.Fatal("Expected Network Resource to have a non-zero bandwith")
	}

	// Test the CIDR of the IPs
	if node.Resources.Networks[0].CIDR != "100.64.0.0/32" {
		t.Fatalf("bad CIDR: %v", node.Resources.Networks[0].CIDR)
	}
	if node.Resources.Networks[1].CIDR != "2001:db8:85a3::/128" {
		t.Fatalf("bad CIDR: %v", node.Resources.Networks[1].CIDR)
	}
	// Ensure that the link local address isn't fingerprinted
	if len(node.Resources.Networks) != 2 {
		t.Fatalf("bad number of IPs %v", len(node.Resources.Networks))
	}
}
