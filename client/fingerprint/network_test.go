package fingerprint

import (
	"fmt"
	"net"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	lo = net.Interface{
		Index:        2,
		MTU:          65536,
		Name:         "lo",
		HardwareAddr: []byte{23, 43, 54, 54},
		Flags:        net.FlagUp | net.FlagLoopback,
	}
)

type LoAddrV4 struct {
}

func (l LoAddrV4) Network() string {
	return "ip+net"
}

func (l LoAddrV4) String() string {
	return "127.0.0.1/8"
}

type LoAddrV6 struct {
}

func (l LoAddrV6) Network() string {
	return "ip+net"
}

func (l LoAddrV6) String() string {
	return "::1/128"
}

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

func TestNetworkFingerprint_basic(t *testing.T) {
	f := &NetworkFingerprint{logger: testLogger(), interfaceDetector: &BasicNetworkInterfaceDetector{}}
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

	assertNodeAttributeContains(t, node, "network.ip-address")

	ip := node.Attributes["network.ip-address"]
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

func TestNetworkFingerprint_no_devices(t *testing.T) {
	f := &NetworkFingerprint{logger: testLogger(), interfaceDetector: &NetworkIntefaceDetectorNoDevices{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{NetworkSpeed: 100}

	ok, err := f.Fingerprint(cfg, node)
	if err == nil {
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

	assertNodeAttributeContains(t, node, "network.ip-address")

	ip := node.Attributes["network.ip-address"]
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
