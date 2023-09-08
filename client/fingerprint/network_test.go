// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"net"
	"os"
	"sort"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
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

	// One link local address
	eth3 = net.Interface{
		Index:        4,
		MTU:          1500,
		Name:         "eth3",
		HardwareAddr: []byte{23, 44, 54, 71},
		Flags:        net.FlagUp | net.FlagBroadcast | net.FlagMulticast,
	}

	// One link local address and one globally routable address
	eth4 = net.Interface{
		Index:        4,
		MTU:          1500,
		Name:         "eth4",
		HardwareAddr: []byte{23, 44, 54, 72},
		Flags:        net.FlagUp | net.FlagBroadcast | net.FlagMulticast,
	}
)

// A fake network detector which returns no devices
type NetworkInterfaceDetectorNoDevices struct {
}

func (f *NetworkInterfaceDetectorNoDevices) Interfaces() ([]net.Interface, error) {
	return make([]net.Interface, 0), nil
}

func (f *NetworkInterfaceDetectorNoDevices) InterfaceByName(name string) (*net.Interface, error) {
	return nil, fmt.Errorf("Device with name %s doesn't exist", name)
}

func (f *NetworkInterfaceDetectorNoDevices) Addrs(intf *net.Interface) ([]net.Addr, error) {
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
	// Return link local first to test we don't prefer it
	return []net.Interface{lo, eth0, eth1, eth2, eth3, eth4}, nil
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
	case "eth3":
		intf = &eth3
	case "eth4":
		intf = &eth4
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

	if intf.Name == "eth3" {
		_, ipnet1, _ := net.ParseCIDR("169.254.155.20/32")
		return []net.Addr{ipnet1}, nil
	}

	if intf.Name == "eth4" {
		_, ipnet1, _ := net.ParseCIDR("169.254.155.20/32")
		_, ipnet2, _ := net.ParseCIDR("100.64.0.10/10")
		return []net.Addr{ipnet1, ipnet2}, nil
	}

	return nil, fmt.Errorf("Can't find addresses for device: %v", intf.Name)
}

func TestNetworkFingerprint_basic(t *testing.T) {
	ci.Parallel(t)

	if v := os.Getenv(skipOnlineTestsEnvVar); v != "" {
		t.Skipf("Environment variable %+q not empty, skipping test", skipOnlineTestsEnvVar)
	}

	f := &NetworkFingerprint{logger: testlog.HCLogger(t), interfaceDetector: &DefaultNetworkInterfaceDetector{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{NetworkSpeed: 101}

	request := &FingerprintRequest{Config: cfg, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	attributes := response.Attributes
	if len(attributes) == 0 {
		t.Fatalf("should apply (HINT: working offline? Set env %q=y", skipOnlineTestsEnvVar)
	}

	assertNodeAttributeContains(t, attributes, "unique.network.ip-address")

	ip := attributes["unique.network.ip-address"]
	match := net.ParseIP(ip)
	if match == nil {
		t.Fatalf("Bad IP match: %s", ip)
	}

	if response.Resources == nil || len(response.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net := response.Resources.Networks[0]
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
		t.Fatalf("Expected Network Resource to have bandwidth %d; got %d", 101, net.MBits)
	}
}

func TestNetworkFingerprint_default_device_absent(t *testing.T) {
	ci.Parallel(t)

	f := &NetworkFingerprint{logger: testlog.HCLogger(t), interfaceDetector: &NetworkInterfaceDetectorOnlyLo{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{NetworkSpeed: 100, NetworkInterface: "eth0"}

	request := &FingerprintRequest{Config: cfg, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err == nil {
		t.Fatalf("err: %v", err)
	}

	if response.Detected {
		t.Fatalf("expected response to not be applicable")
	}

	if len(response.Attributes) != 0 {
		t.Fatalf("attributes should be zero but instead are: %v", response.Attributes)
	}
}

func TestNetworkFingerPrint_default_device(t *testing.T) {
	ci.Parallel(t)

	f := &NetworkFingerprint{logger: testlog.HCLogger(t), interfaceDetector: &NetworkInterfaceDetectorOnlyLo{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{NetworkSpeed: 100, NetworkInterface: "lo"}

	request := &FingerprintRequest{Config: cfg, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	attributes := response.Attributes
	if len(attributes) == 0 {
		t.Fatalf("should apply")
	}

	assertNodeAttributeContains(t, attributes, "unique.network.ip-address")

	ip := attributes["unique.network.ip-address"]
	match := net.ParseIP(ip)
	if match == nil {
		t.Fatalf("Bad IP match: %s", ip)
	}

	if response.Resources == nil || len(response.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net := response.Resources.Networks[0]
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
		t.Fatal("Expected Network Resource to have a non-zero bandwidth")
	}
}

func TestNetworkFingerPrint_LinkLocal_Allowed(t *testing.T) {
	ci.Parallel(t)

	f := &NetworkFingerprint{logger: testlog.HCLogger(t), interfaceDetector: &NetworkInterfaceDetectorMultipleInterfaces{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{NetworkSpeed: 100, NetworkInterface: "eth3"}

	request := &FingerprintRequest{Config: cfg, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	attributes := response.Attributes
	assertNodeAttributeContains(t, attributes, "unique.network.ip-address")

	ip := attributes["unique.network.ip-address"]
	match := net.ParseIP(ip)
	if match == nil {
		t.Fatalf("Bad IP match: %s", ip)
	}

	if response.Resources == nil || len(response.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net := response.Resources.Networks[0]
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
		t.Fatal("Expected Network Resource to have a non-zero bandwidth")
	}
}

func TestNetworkFingerPrint_LinkLocal_Allowed_MixedIntf(t *testing.T) {
	ci.Parallel(t)

	f := &NetworkFingerprint{logger: testlog.HCLogger(t), interfaceDetector: &NetworkInterfaceDetectorMultipleInterfaces{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{NetworkSpeed: 100, NetworkInterface: "eth4"}

	request := &FingerprintRequest{Config: cfg, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	attributes := response.Attributes
	if len(attributes) == 0 {
		t.Fatalf("should apply attributes")
	}

	assertNodeAttributeContains(t, attributes, "unique.network.ip-address")

	ip := attributes["unique.network.ip-address"]
	match := net.ParseIP(ip)
	if match == nil {
		t.Fatalf("Bad IP match: %s", ip)
	}

	if response.Resources == nil || len(response.Resources.Networks) == 0 {
		t.Fatal("Expected to find Network Resources")
	}

	// Test at least the first Network Resource
	net := response.Resources.Networks[0]
	if net.IP == "" {
		t.Fatal("Expected Network Resource to not be empty")
	}
	if net.IP == "169.254.155.20" {
		t.Fatalf("expected non-link local address; got %v", net.IP)
	}
	if net.CIDR == "" {
		t.Fatal("Expected Network Resource to have a CIDR")
	}
	if net.Device == "" {
		t.Fatal("Expected Network Resource to have a Device Name")
	}
	if net.MBits == 0 {
		t.Fatal("Expected Network Resource to have a non-zero bandwidth")
	}
}

func TestNetworkFingerPrint_LinkLocal_Disallowed(t *testing.T) {
	ci.Parallel(t)

	f := &NetworkFingerprint{logger: testlog.HCLogger(t), interfaceDetector: &NetworkInterfaceDetectorMultipleInterfaces{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{
		NetworkSpeed:     100,
		NetworkInterface: "eth3",
		Options: map[string]string{
			networkDisallowLinkLocalOption: "true",
		},
	}

	request := &FingerprintRequest{Config: cfg, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	if len(response.Attributes) != 0 {
		t.Fatalf("should not apply attributes")
	}
}

func TestNetworkFingerPrint_MultipleAliases(t *testing.T) {
	ci.Parallel(t)

	f := &NetworkFingerprint{logger: testlog.HCLogger(t), interfaceDetector: &NetworkInterfaceDetectorMultipleInterfaces{}}
	node := &structs.Node{
		Attributes: make(map[string]string),
	}
	cfg := &config.Config{
		NetworkSpeed:     100,
		NetworkInterface: "eth3",
		HostNetworks: map[string]*structs.ClientHostNetworkConfig{
			"alias1": {
				Name:      "alias1",
				Interface: "eth3",
				CIDR:      "169.254.155.20/32",
			},
			"alias2": {
				Name:      "alias2",
				Interface: "eth3",
				CIDR:      "169.254.155.20/32",
			},
			"alias3": {
				Name:      "alias3",
				Interface: "eth0",
				CIDR:      "100.64.0.11/10",
			},
		},
	}

	request := &FingerprintRequest{Config: cfg, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	require.NoError(t, err)

	aliases := []string{}
	for _, network := range response.NodeResources.NodeNetworks {
		for _, address := range network.Addresses {
			aliases = append(aliases, address.Alias)
		}
	}
	expected := []string{}
	for alias := range cfg.HostNetworks {
		expected = append(expected, alias)
	}
	// eth3 matches the NetworkInterface and will then generate the 'default'
	// alias
	expected = append(expected, "default")
	sort.Strings(expected)
	sort.Strings(aliases)
	require.Equal(t, expected, aliases, "host networks should match aliases")
}

func TestNetworkFingerPrint_HostNetworkReservedPorts(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name         string
		hostNetworks map[string]*structs.ClientHostNetworkConfig
		expected     []string
	}{
		{
			name:         "no host networks",
			hostNetworks: map[string]*structs.ClientHostNetworkConfig{},
			expected:     []string{""},
		},
		{
			name: "no reserved ports",
			hostNetworks: map[string]*structs.ClientHostNetworkConfig{
				"alias1": {
					Name:      "alias1",
					Interface: "eth3",
					CIDR:      "169.254.155.20/32",
				},
				"alias2": {
					Name:      "alias2",
					Interface: "eth3",
					CIDR:      "169.254.155.20/32",
				},
				"alias3": {
					Name:      "alias3",
					Interface: "eth0",
					CIDR:      "100.64.0.11/10",
				},
			},
			expected: []string{"", "", "", ""},
		},
		{
			name: "reserved ports in some aliases",
			hostNetworks: map[string]*structs.ClientHostNetworkConfig{
				"alias1": {
					Name:          "alias1",
					Interface:     "eth3",
					CIDR:          "169.254.155.20/32",
					ReservedPorts: "22",
				},
				"alias2": {
					Name:          "alias2",
					Interface:     "eth3",
					CIDR:          "169.254.155.20/32",
					ReservedPorts: "80,3000-4000",
				},
				"alias3": {
					Name:      "alias3",
					Interface: "eth0",
					CIDR:      "100.64.0.11/10",
				},
			},
			expected: []string{"22", "80,3000-4000", "", ""},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := &NetworkFingerprint{
				logger:            testlog.HCLogger(t),
				interfaceDetector: &NetworkInterfaceDetectorMultipleInterfaces{},
			}
			node := &structs.Node{
				Attributes: make(map[string]string),
			}
			cfg := &config.Config{
				NetworkInterface: "eth3",
				HostNetworks:     tc.hostNetworks,
			}

			request := &FingerprintRequest{Config: cfg, Node: node}
			var response FingerprintResponse
			err := f.Fingerprint(request, &response)
			require.NoError(t, err)

			got := []string{}
			for _, network := range response.NodeResources.NodeNetworks {
				for _, address := range network.Addresses {
					got = append(got, address.ReservedPorts)
				}
			}

			sort.Strings(tc.expected)
			sort.Strings(got)
			require.Equal(t, tc.expected, got, "host networks should match reserved ports")
		})
	}
}
