// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkIndex_Copy(t *testing.T) {
	ci.Parallel(t)

	n := &Node{
		NodeResources: &NodeResources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					IP:     "192.168.0.100",
					MBits:  1000,
				},
			},
			NodeNetworks: []*NodeNetworkResource{
				{
					Mode:   "host",
					Device: "eth0",
					Speed:  1000,
					Addresses: []NodeNetworkAddress{
						{
							Alias:   "default",
							Address: "192.168.0.100",
							Family:  NodeNetworkAF_IPv4,
						},
					},
				},
			},
		},
		Reserved: &Resources{
			Networks: []*NetworkResource{
				{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []Port{{Label: "ssh", Value: 22}},
					MBits:         1,
				},
			},
		},
		ReservedResources: &NodeReservedResources{
			Networks: NodeReservedNetworkResources{
				ReservedHostPorts: "22",
			},
		},
	}

	allocs := []*Allocation{
		{
			AllocatedResources: &AllocatedResources{
				Tasks: map[string]*AllocatedTaskResources{
					"web": {
						Networks: []*NetworkResource{
							{
								Device:        "eth0",
								IP:            "192.168.0.100",
								MBits:         20,
								ReservedPorts: []Port{{Label: "one", Value: 8000}, {Label: "two", Value: 9000}},
							},
						},
					},
				},
			},
		},
		{
			AllocatedResources: &AllocatedResources{
				Tasks: map[string]*AllocatedTaskResources{
					"api": {
						Networks: []*NetworkResource{
							{
								Device:        "eth0",
								IP:            "192.168.0.100",
								MBits:         50,
								ReservedPorts: []Port{{Label: "one", Value: 10000}},
							},
						},
					},
				},
			},
		},
	}

	netIdx := NewNetworkIndex()
	netIdx.SetNode(n)
	netIdx.AddAllocs(allocs)

	// Copy must be equal.
	netIdxCopy := netIdx.Copy()
	require.Equal(t, netIdx, netIdxCopy)

	// Modifying copy should not affect original value.
	n.NodeResources.Networks[0].Device = "eth1"
	n.ReservedResources.Networks.ReservedHostPorts = "22,80"
	allocs = append(allocs, &Allocation{
		AllocatedResources: &AllocatedResources{
			Tasks: map[string]*AllocatedTaskResources{
				"db": {
					Networks: []*NetworkResource{
						{
							Device:        "eth1",
							IP:            "192.168.0.104",
							MBits:         50,
							ReservedPorts: []Port{{Label: "one", Value: 4567}},
						},
					},
				},
			},
		},
	})
	netIdxCopy.SetNode(n)
	netIdxCopy.AddAllocs(allocs)
	netIdxCopy.MinDynamicPort = 1000
	netIdxCopy.MaxDynamicPort = 2000
	require.NotEqual(t, netIdx, netIdxCopy)
}

func TestNetworkIndex_Overcommitted(t *testing.T) {
	t.Skip()
	ci.Parallel(t)
	idx := NewNetworkIndex()

	// Consume some network
	reserved := &NetworkResource{
		Device:        "eth0",
		IP:            "192.168.0.100",
		MBits:         505,
		ReservedPorts: []Port{{Label: "one", Value: 8000}, {Label: "two", Value: 9000}},
	}
	collide, reasons := idx.AddReserved(reserved)
	if collide || len(reasons) != 0 {
		t.Fatalf("bad")
	}
	if !idx.Overcommitted() {
		t.Fatalf("have no resources")
	}

	// Add resources
	n := &Node{
		NodeResources: &NodeResources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
		},
	}
	idx.SetNode(n)
	if idx.Overcommitted() {
		t.Fatalf("have resources")
	}

	// Double up our usage
	idx.AddReserved(reserved)
	if !idx.Overcommitted() {
		t.Fatalf("should be overcommitted")
	}
}

func TestNetworkIndex_SetNode(t *testing.T) {
	ci.Parallel(t)

	idx := NewNetworkIndex()
	n := &Node{
		NodeResources: &NodeResources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					IP:     "192.168.0.100",
					MBits:  1000,
				},
			},
		},
		ReservedResources: &NodeReservedResources{
			Networks: NodeReservedNetworkResources{
				ReservedHostPorts: "22",
			},
		},
	}
	require.NoError(t, idx.SetNode(n))
	require.Len(t, idx.TaskNetworks, 1)
	require.Equal(t, 1000, idx.AvailBandwidth["eth0"])
	require.True(t, idx.UsedPorts["192.168.0.100"].Check(22))
}

func TestNetworkIndex_AddAllocs(t *testing.T) {
	ci.Parallel(t)

	idx := NewNetworkIndex()
	allocs := []*Allocation{
		{
			ClientStatus:  AllocClientStatusRunning,
			DesiredStatus: AllocDesiredStatusRun,
			AllocatedResources: &AllocatedResources{
				Tasks: map[string]*AllocatedTaskResources{
					"web": {
						Networks: []*NetworkResource{
							{
								Device:        "eth0",
								IP:            "192.168.0.100",
								MBits:         20,
								ReservedPorts: []Port{{Label: "one", Value: 8000}, {Label: "two", Value: 9000}},
							},
						},
					},
				},
			},
		},
		{
			ClientStatus:  AllocClientStatusRunning,
			DesiredStatus: AllocDesiredStatusRun,
			AllocatedResources: &AllocatedResources{
				Tasks: map[string]*AllocatedTaskResources{
					"api": {
						Networks: []*NetworkResource{
							{
								Device:        "eth0",
								IP:            "192.168.0.100",
								MBits:         50,
								ReservedPorts: []Port{{Label: "one", Value: 10000}},
							},
						},
					},
				},
			},
		},
		{
			// Allocations running on clients should have their
			// ports counted even if their DesiredStatus=stop
			ClientStatus:  AllocClientStatusRunning,
			DesiredStatus: AllocDesiredStatusStop,
			AllocatedResources: &AllocatedResources{
				Tasks: map[string]*AllocatedTaskResources{
					"api": {
						Networks: []*NetworkResource{
							{
								Device:        "eth0",
								IP:            "192.168.0.100",
								MBits:         50,
								ReservedPorts: []Port{{Label: "one", Value: 10001}},
							},
						},
					},
				},
			},
		},
		{
			// Allocations *not* running on clients should *not*
			// have their ports counted even if their
			// DesiredStatus=run
			ClientStatus:  AllocClientStatusFailed,
			DesiredStatus: AllocDesiredStatusRun,
			AllocatedResources: &AllocatedResources{
				Tasks: map[string]*AllocatedTaskResources{
					"api": {
						Networks: []*NetworkResource{
							{
								Device:        "eth0",
								IP:            "192.168.0.100",
								MBits:         50,
								ReservedPorts: []Port{{Label: "one", Value: 10001}},
							},
						},
					},
				},
			},
		},
	}
	collide, reason := idx.AddAllocs(allocs)
	assert.False(t, collide)
	assert.Empty(t, reason)

	assert.True(t, idx.UsedPorts["192.168.0.100"].Check(8000))
	assert.True(t, idx.UsedPorts["192.168.0.100"].Check(9000))
	assert.True(t, idx.UsedPorts["192.168.0.100"].Check(10000))
	assert.True(t, idx.UsedPorts["192.168.0.100"].Check(10001))
}

func TestNetworkIndex_AddReserved(t *testing.T) {
	ci.Parallel(t)

	idx := NewNetworkIndex()

	reserved := &NetworkResource{
		Device:        "eth0",
		IP:            "192.168.0.100",
		MBits:         20,
		ReservedPorts: []Port{{Label: "one", Value: 8000}, {Label: "two", Value: 9000}},
	}
	collide, reasons := idx.AddReserved(reserved)
	if collide || len(reasons) > 0 {
		t.Fatalf("bad")
	}

	if idx.UsedBandwidth["eth0"] != 20 {
		t.Fatalf("Bad")
	}
	if !idx.UsedPorts["192.168.0.100"].Check(8000) {
		t.Fatalf("Bad")
	}
	if !idx.UsedPorts["192.168.0.100"].Check(9000) {
		t.Fatalf("Bad")
	}

	// Try to reserve the same network
	collide, reasons = idx.AddReserved(reserved)
	if !collide || len(reasons) == 0 {
		t.Fatalf("bad")
	}
}

// XXX Reserving ports doesn't work when yielding from a CIDR block. This is
// okay for now since we do not actually fingerprint CIDR blocks.
func TestNetworkIndex_yieldIP(t *testing.T) {
	ci.Parallel(t)

	idx := NewNetworkIndex()
	n := &Node{
		NodeResources: &NodeResources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/30",
					MBits:  1000,
				},
			},
		},
	}
	idx.SetNode(n)

	var out []string
	idx.yieldIP(func(n *NetworkResource, ip net.IP) (stop bool) {
		out = append(out, ip.String())
		return
	})

	expect := []string{"192.168.0.100", "192.168.0.101",
		"192.168.0.102", "192.168.0.103"}
	if !reflect.DeepEqual(out, expect) {
		t.Fatalf("bad: %v", out)
	}
}

// TestNetworkIndex_AssignPorts exercises assigning ports on group networks.
func TestNetworkIndex_AssignPorts(t *testing.T) {
	ci.Parallel(t)

	// Create a node that only two free dynamic ports
	idx := NewNetworkIndex()
	n := &Node{
		NodeResources: &NodeResources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					IP:     "192.168.0.100",
					MBits:  1000,
				},
			},
			NodeNetworks: []*NodeNetworkResource{
				{
					Mode:   "host",
					Device: "eth0",
					Speed:  1000,
					Addresses: []NodeNetworkAddress{
						{
							Alias:   "default",
							Address: "192.168.0.100",
							Family:  NodeNetworkAF_IPv4,
						},
					},
				},
			},
		},
		ReservedResources: &NodeReservedResources{
			Networks: NodeReservedNetworkResources{
				ReservedHostPorts: fmt.Sprintf("%d-%d", idx.MinDynamicPort, idx.MaxDynamicPort-2),
			},
		},
	}

	idx.SetNode(n)

	// Ask for 2 dynamic ports
	ask := &NetworkResource{
		ReservedPorts: []Port{{Label: "static", Value: 443, To: 443, HostNetwork: "default"}},
		DynamicPorts:  []Port{{Label: "http", To: 80, HostNetwork: "default"}, {Label: "admin", To: 8080, HostNetwork: "default"}},
	}
	offer, err := idx.AssignPorts(ask)
	must.NoError(t, err)
	must.NotNil(t, offer, must.Sprint("did not get an offer"))

	staticPortMapping, ok := offer.Get("static")
	must.True(t, ok)

	httpPortMapping, ok := offer.Get("http")
	must.True(t, ok)

	adminPortMapping, ok := offer.Get("admin")
	must.True(t, ok)

	must.NotEq(t, httpPortMapping.Value, adminPortMapping.Value,
		must.Sprint("assigned dynamic ports must not conflict"))

	must.Eq(t, 443, staticPortMapping.Value)
	must.Between(t, idx.MaxDynamicPort-1, httpPortMapping.Value, idx.MaxDynamicPort)
	must.Between(t, idx.MaxDynamicPort-1, adminPortMapping.Value, idx.MaxDynamicPort)
}

// TestNetworkIndex_AssignPorts_SmallRange exercises assigning ports on group
// networks with small dynamic port ranges configured
func TestNetworkIndex_AssignPortss_SmallRange(t *testing.T) {
	ci.Parallel(t)

	n := &Node{
		NodeResources: &NodeResources{
			NodeNetworks: []*NodeNetworkResource{
				{
					Mode:   "host",
					Device: "eth0",
					Speed:  1000,
					Addresses: []NodeNetworkAddress{
						{
							Alias:   "default",
							Address: "192.168.0.100",
							Family:  NodeNetworkAF_IPv4,
						},
					},
				},
			},
		},
	}

	testCases := []struct {
		name      string
		min       int
		max       int
		ask       []Port
		expectErr string
	}{
		{
			name:      "1 dynamic port avail and 1 port requested",
			min:       20000,
			max:       20000,
			ask:       []Port{{Label: "http", To: 80, HostNetwork: "default"}},
			expectErr: "",
		},
		{
			name:      "1 dynamic port avail and 2 ports requested",
			min:       20000,
			max:       20000,
			ask:       []Port{{Label: "http", To: 80, HostNetwork: "default"}, {Label: "admin", To: 80, HostNetwork: "default"}},
			expectErr: "dynamic port selection failed",
		},
		{
			name:      "2 dynamic ports avail and 2 ports requested",
			min:       20000,
			max:       20001,
			ask:       []Port{{Label: "http", To: 80, HostNetwork: "default"}, {Label: "admin", To: 80, HostNetwork: "default"}},
			expectErr: "",
		},
	}

	for _, tc := range testCases {

		idx := NewNetworkIndex()
		idx.MinDynamicPort = tc.min
		idx.MaxDynamicPort = tc.max
		idx.SetNode(n)

		ask := &NetworkResource{DynamicPorts: tc.ask}
		offer, err := idx.AssignPorts(ask)
		if tc.expectErr != "" {
			must.EqError(t, err, tc.expectErr)
		} else {
			must.NoError(t, err)
			must.NotNil(t, offer, must.Sprint("did not get an offer"))

			for _, port := range tc.ask {
				_, ok := offer.Get(port.Label)
				must.True(t, ok)
			}
		}
	}

}

// TestNetworkIndex_AssignPorts_TwoIp exercises assigning ports on group
// networks with two ip matching host network
func TestNetworkIndex_AssignPorts_TwoIp(t *testing.T) {
	ci.Parallel(t)

	n := &Node{
		NodeResources: &NodeResources{
			NodeNetworks: []*NodeNetworkResource{
				{
					Mode:   "host",
					Device: "eth0",
					Speed:  1000,
					Addresses: []NodeNetworkAddress{
						{
							Alias:   "two_ip_test",
							Address: "192.168.0.100",
							Family:  NodeNetworkAF_IPv4,
						},
					},
				},
				{
					Mode:   "host",
					Device: "eth1",
					Speed:  1000,
					Addresses: []NodeNetworkAddress{
						{
							Alias:   "two_ip_test",
							Address: "192.168.0.101",
							Family:  NodeNetworkAF_IPv4,
						},
					},
				},
			},
		},
	}

	testCases := []struct {
		name      string
		allocated []AllocatedPortMapping
		ask       []Port
		expectErr string
	}{
		{
			name:      "1 reserved port asked, 2 ip, 1 already used",
			allocated: []AllocatedPortMapping{{Label: "static", Value: 7000, To: 7000, HostIP: "192.168.0.100"}},
			ask:       []Port{{Label: "static", Value: 7000, To: 7000, HostNetwork: "two_ip_test"}},
			expectErr: "",
		},
		{
			name: "1 reserved port asked, 2 ip, 2 already used",
			allocated: []AllocatedPortMapping{
				{Label: "static", Value: 7000, To: 7000, HostIP: "192.168.0.100"},
				{Label: "static", Value: 7000, To: 7000, HostIP: "192.168.0.101"},
			},
			ask:       []Port{{Label: "static", Value: 7000, To: 7000, HostNetwork: "two_ip_test"}},
			expectErr: "reserved port collision static=7000",
		},
	}

	for _, tc := range testCases {

		idx := NewNetworkIndex()
		idx.SetNode(n)
		idx.AddReservedPorts(tc.allocated)

		ask := &NetworkResource{ReservedPorts: tc.ask}
		offer, err := idx.AssignPorts(ask)
		if tc.expectErr != "" {
			must.EqError(t, err, tc.expectErr)
		} else {
			must.NoError(t, err)
			must.NotNil(t, offer, must.Sprint("did not get an offer"))

			for _, port := range tc.ask {
				_, ok := offer.Get(port.Label)
				must.True(t, ok)
			}
		}
	}

}

// TestNetworkIndex_IgnorePortCollision tests Port.IgnoreCollision.
func TestNetworkIndex_IgnorePortCollision(t *testing.T) {
	ci.Parallel(t)

	// set up some fake resources

	ip := "127.3.2.1"
	net := "test-ignore-port-collision"
	n := &Node{
		NodeResources: &NodeResources{
			NodeNetworks: []*NodeNetworkResource{{
				Addresses: []NodeNetworkAddress{{
					Alias:   net,
					Address: ip,
				}},
			}},
		},
	}

	getPortMappings := func(collideOK bool) []AllocatedPortMapping {
		return []AllocatedPortMapping{{
			HostIP:          ip,
			Label:           "test-port",
			Value:           10,
			To:              10,
			IgnoreCollision: collideOK,
		}}
	}
	getPorts := func(collideOK bool) []Port {
		return []Port{{
			HostNetwork:     net,
			Label:           "test-port",
			Value:           10,
			To:              10,
			IgnoreCollision: collideOK,
		}}
	}
	collidingPortMappings := getPortMappings(false)
	nonCollidingPortMappings := getPortMappings(true)
	collidingPorts := getPorts(false)
	nonCollidingPorts := getPorts(true)

	// now we can get started

	idx := NewNetworkIndex()
	idx.SetNode(n)

	// initial reservation - pretend some other job has already used the port
	// note the behavior below is the same whether this one is a collider or not
	collide, reasons := idx.AddReservedPorts(collidingPortMappings)
	must.False(t, collide, must.Sprint("expect no collision in first reservation"))
	must.Len(t, 0, reasons, must.Sprint("expect no reasons in first reservation"))

	t.Run("AddReservedPorts", func(t *testing.T) {
		collide, reasons = idx.AddReservedPorts(collidingPortMappings)
		must.True(t, collide, must.Sprint("expect collision"))
		must.Eq(t, []string{"port 10 already in use"}, reasons, must.Sprint("expect collision reasons"))

		collide, reasons = idx.AddReservedPorts(nonCollidingPortMappings)
		must.False(t, collide, must.Sprint("expect no collision"))
		must.Len(t, 0, reasons, must.Sprint("expect no collision reasons"))
	})

	t.Run("AssignPorts", func(t *testing.T) {
		ask := &NetworkResource{ReservedPorts: collidingPorts}
		allocated, err := idx.AssignPorts(ask)
		must.ErrorContains(t, err, "reserved port collision test-port=10")
		must.Nil(t, allocated, must.Sprint("expect no ports on AssignPorts error"))

		ask = &NetworkResource{ReservedPorts: nonCollidingPorts}
		allocated, err = idx.AssignPorts(ask)
		must.NoError(t, err)
		must.Eq(t, nonCollidingPortMappings, allocated)
	})
}

func TestNetworkIndex_AssignTaskNetwork(t *testing.T) {
	ci.Parallel(t)
	idx := NewNetworkIndex()
	n := &Node{
		NodeResources: &NodeResources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/30",
					MBits:  1000,
				},
			},
		},
	}
	idx.SetNode(n)

	allocs := []*Allocation{
		{
			TaskResources: map[string]*Resources{
				"web": {
					Networks: []*NetworkResource{
						{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         20,
							ReservedPorts: []Port{{Label: "one", Value: 8000}, {Label: "two", Value: 9000}},
						},
					},
				},
			},
		},
		{
			TaskResources: map[string]*Resources{
				"api": {
					Networks: []*NetworkResource{
						{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         50,
							ReservedPorts: []Port{{Label: "main", Value: 10000}},
						},
					},
				},
			},
		},
	}
	idx.AddAllocs(allocs)

	// Ask for a reserved port
	ask := &NetworkResource{
		ReservedPorts: []Port{{Label: "main", Value: 8000}},
	}
	offer, err := idx.AssignTaskNetwork(ask)
	require.NoError(t, err)
	require.NotNil(t, offer)
	require.Equal(t, "192.168.0.101", offer.IP)
	rp := Port{Label: "main", Value: 8000}
	require.Len(t, offer.ReservedPorts, 1)
	require.Exactly(t, rp, offer.ReservedPorts[0])

	// Ask for dynamic ports
	ask = &NetworkResource{
		DynamicPorts: []Port{{Label: "http", To: 80}, {Label: "https", To: 443}, {Label: "admin", To: -1}},
	}
	offer, err = idx.AssignTaskNetwork(ask)
	require.NoError(t, err)
	require.NotNil(t, offer)
	require.Equal(t, "192.168.0.100", offer.IP)
	require.Len(t, offer.DynamicPorts, 3)
	var adminPort Port
	for _, port := range offer.DynamicPorts {
		require.NotZero(t, port.Value)
		if port.Label == "admin" {
			adminPort = port
		}
	}
	require.Equal(t, adminPort.Value, adminPort.To)

	// Ask for reserved + dynamic ports
	ask = &NetworkResource{
		ReservedPorts: []Port{{Label: "main", Value: 2345}},
		DynamicPorts:  []Port{{Label: "http", To: 80}, {Label: "https", To: 443}, {Label: "admin", To: 8080}},
	}
	offer, err = idx.AssignTaskNetwork(ask)
	require.NoError(t, err)
	require.NotNil(t, offer)
	require.Equal(t, "192.168.0.100", offer.IP)

	rp = Port{Label: "main", Value: 2345}
	require.Len(t, offer.ReservedPorts, 1)
	require.Exactly(t, rp, offer.ReservedPorts[0])

	// Ask for too much bandwidth
	ask = &NetworkResource{
		MBits: 1000,
	}
	offer, err = idx.AssignTaskNetwork(ask)
	require.Error(t, err)
	require.Equal(t, "bandwidth exceeded", err.Error())
	require.Nil(t, offer)
}

// This test ensures that even with a small domain of available ports we are
// able to make a dynamic port allocation.
func TestNetworkIndex_AssignTaskNetwork_Dynamic_Contention(t *testing.T) {
	ci.Parallel(t)

	// Create a node that only has two free dynamic ports
	idx := NewNetworkIndex()
	n := &Node{
		NodeResources: &NodeResources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					IP:     "192.168.0.100",
					MBits:  1000,
				},
			},
		},
		ReservedResources: &NodeReservedResources{
			Networks: NodeReservedNetworkResources{
				// leave only 2 available ports
				ReservedHostPorts: fmt.Sprintf("%d-%d", idx.MinDynamicPort, idx.MaxDynamicPort-2),
			},
		},
	}

	idx.SetNode(n)

	// Ask for 2 dynamic ports
	ask := &NetworkResource{
		DynamicPorts: []Port{{Label: "http", To: 80}, {Label: "admin", To: 443}},
	}
	offer, err := idx.AssignTaskNetwork(ask)
	must.NoError(t, err)
	must.NotNil(t, offer, must.Sprint("did not get an offer"))
	must.Eq(t, "192.168.0.100", offer.IP)
	must.Len(t, 2, offer.DynamicPorts, must.Sprint("There should be two dynamic ports"))

	must.NotEq(t, offer.DynamicPorts[0].Value, offer.DynamicPorts[1].Value,
		must.Sprint("assigned dynamic ports must not conflict"))
	must.Between(t, idx.MaxDynamicPort-1, offer.DynamicPorts[0].Value, idx.MaxDynamicPort)
	must.Between(t, idx.MaxDynamicPort-1, offer.DynamicPorts[1].Value, idx.MaxDynamicPort)
}

// COMPAT(0.11): Remove in 0.11
func TestNetworkIndex_SetNode_Old(t *testing.T) {
	ci.Parallel(t)

	idx := NewNetworkIndex()
	n := &Node{
		Resources: &Resources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
		},
		Reserved: &Resources{
			Networks: []*NetworkResource{
				{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []Port{{Label: "ssh", Value: 22}},
					MBits:         1,
				},
			},
		},
	}
	require.NoError(t, idx.SetNode(n))
	require.Len(t, idx.TaskNetworks, 1)
	require.Equal(t, 1000, idx.AvailBandwidth["eth0"])
	require.Equal(t, 1, idx.UsedBandwidth["eth0"])
	require.True(t, idx.UsedPorts["192.168.0.100"].Check(22))
}

// COMPAT(0.11): Remove in 0.11
func TestNetworkIndex_AddAllocs_Old(t *testing.T) {
	ci.Parallel(t)

	idx := NewNetworkIndex()
	allocs := []*Allocation{
		{
			TaskResources: map[string]*Resources{
				"web": {
					Networks: []*NetworkResource{
						{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         20,
							ReservedPorts: []Port{{Label: "one", Value: 8000}, {Label: "two", Value: 9000}},
						},
					},
				},
			},
		},
		{
			TaskResources: map[string]*Resources{
				"api": {
					Networks: []*NetworkResource{
						{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         50,
							ReservedPorts: []Port{{Label: "one", Value: 10000}},
						},
					},
				},
			},
		},
	}
	collide, reason := idx.AddAllocs(allocs)
	if collide || reason != "" {
		t.Fatalf("bad")
	}

	if idx.UsedBandwidth["eth0"] != 70 {
		t.Fatalf("Bad")
	}
	if !idx.UsedPorts["192.168.0.100"].Check(8000) {
		t.Fatalf("Bad")
	}
	if !idx.UsedPorts["192.168.0.100"].Check(9000) {
		t.Fatalf("Bad")
	}
	if !idx.UsedPorts["192.168.0.100"].Check(10000) {
		t.Fatalf("Bad")
	}
}

// COMPAT(0.11): Remove in 0.11
func TestNetworkIndex_yieldIP_Old(t *testing.T) {
	ci.Parallel(t)

	idx := NewNetworkIndex()
	n := &Node{
		Resources: &Resources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/30",
					MBits:  1000,
				},
			},
		},
		Reserved: &Resources{
			Networks: []*NetworkResource{
				{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []Port{{Label: "ssh", Value: 22}},
					MBits:         1,
				},
			},
		},
	}
	idx.SetNode(n)

	var out []string
	idx.yieldIP(func(n *NetworkResource, ip net.IP) (stop bool) {
		out = append(out, ip.String())
		return
	})

	expect := []string{"192.168.0.100", "192.168.0.101",
		"192.168.0.102", "192.168.0.103"}
	if !reflect.DeepEqual(out, expect) {
		t.Fatalf("bad: %v", out)
	}
}

// COMPAT(0.11): Remove in 0.11
func TestNetworkIndex_AssignTaskNetwork_Old(t *testing.T) {
	ci.Parallel(t)

	idx := NewNetworkIndex()
	n := &Node{
		Resources: &Resources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/30",
					MBits:  1000,
				},
			},
		},
		Reserved: &Resources{
			Networks: []*NetworkResource{
				{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []Port{{Label: "ssh", Value: 22}},
					MBits:         1,
				},
			},
		},
	}
	idx.SetNode(n)

	allocs := []*Allocation{
		{
			TaskResources: map[string]*Resources{
				"web": {
					Networks: []*NetworkResource{
						{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         20,
							ReservedPorts: []Port{{Label: "one", Value: 8000}, {Label: "two", Value: 9000}},
						},
					},
				},
			},
		},
		{
			TaskResources: map[string]*Resources{
				"api": {
					Networks: []*NetworkResource{
						{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         50,
							ReservedPorts: []Port{{Label: "main", Value: 10000}},
						},
					},
				},
			},
		},
	}
	idx.AddAllocs(allocs)

	// Ask for a reserved port
	ask := &NetworkResource{
		ReservedPorts: []Port{{Label: "main", Value: 8000}},
	}
	offer, err := idx.AssignTaskNetwork(ask)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if offer == nil {
		t.Fatalf("bad")
	}
	if offer.IP != "192.168.0.101" {
		t.Fatalf("bad: %#v", offer)
	}
	rp := Port{Label: "main", Value: 8000}
	if len(offer.ReservedPorts) != 1 || offer.ReservedPorts[0] != rp {
		t.Fatalf("bad: %#v", offer)
	}

	// Ask for dynamic ports
	ask = &NetworkResource{
		DynamicPorts: []Port{{Label: "http", To: 80}, {Label: "https", To: 443}, {Label: "admin", To: 8080}},
	}
	offer, err = idx.AssignTaskNetwork(ask)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if offer == nil {
		t.Fatalf("bad")
	}
	if offer.IP != "192.168.0.100" {
		t.Fatalf("bad: %#v", offer)
	}
	if len(offer.DynamicPorts) != 3 {
		t.Fatalf("There should be three dynamic ports")
	}
	for _, port := range offer.DynamicPorts {
		if port.Value == 0 {
			t.Fatalf("Dynamic Port: %v should have been assigned a host port", port.Label)
		}
	}

	// Ask for reserved + dynamic ports
	ask = &NetworkResource{
		ReservedPorts: []Port{{Label: "main", Value: 2345}},
		DynamicPorts:  []Port{{Label: "http", To: 80}, {Label: "https", To: 443}, {Label: "admin", To: 8080}},
	}
	offer, err = idx.AssignTaskNetwork(ask)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if offer == nil {
		t.Fatalf("bad")
	}
	if offer.IP != "192.168.0.100" {
		t.Fatalf("bad: %#v", offer)
	}

	rp = Port{Label: "main", Value: 2345}
	if len(offer.ReservedPorts) != 1 || offer.ReservedPorts[0] != rp {
		t.Fatalf("bad: %#v", offer)
	}

	// Ask for too much bandwidth
	ask = &NetworkResource{
		MBits: 1000,
	}
	offer, err = idx.AssignTaskNetwork(ask)
	if err.Error() != "bandwidth exceeded" {
		t.Fatalf("err: %v", err)
	}
	if offer != nil {
		t.Fatalf("bad")
	}
}

// COMPAT(0.11): Remove in 0.11
// This test ensures that even with a small domain of available ports we are
// able to make a dynamic port allocation.
func TestNetworkIndex_AssignTaskNetwork_Dynamic_Contention_Old(t *testing.T) {
	ci.Parallel(t)

	// Create a node that only has one free port
	idx := NewNetworkIndex()
	n := &Node{
		Resources: &Resources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
		},
		Reserved: &Resources{
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					IP:     "192.168.0.100",
					MBits:  1,
				},
			},
		},
	}
	for i := idx.MinDynamicPort; i < idx.MaxDynamicPort; i++ {
		n.Reserved.Networks[0].ReservedPorts = append(n.Reserved.Networks[0].ReservedPorts, Port{Value: i})
	}

	idx.SetNode(n)

	// Ask for dynamic ports
	ask := &NetworkResource{
		DynamicPorts: []Port{{Label: "http", To: 80}},
	}
	offer, err := idx.AssignTaskNetwork(ask)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if offer == nil {
		t.Fatalf("bad")
	}
	if offer.IP != "192.168.0.100" {
		t.Fatalf("bad: %#v", offer)
	}
	if len(offer.DynamicPorts) != 1 {
		t.Fatalf("There should be three dynamic ports")
	}
	if p := offer.DynamicPorts[0].Value; p != idx.MaxDynamicPort {
		t.Fatalf("Dynamic Port: should have been assigned %d; got %d", p, idx.MaxDynamicPort)
	}
}

func TestIntContains(t *testing.T) {
	ci.Parallel(t)

	l := []int{1, 2, 10, 20}
	if isPortReserved(l, 50) {
		t.Fatalf("bad")
	}
	if !isPortReserved(l, 20) {
		t.Fatalf("bad")
	}
	if !isPortReserved(l, 1) {
		t.Fatalf("bad")
	}
}

func TestNetworkIndex_SetNode_HostNets(t *testing.T) {
	ci.Parallel(t)

	idx := NewNetworkIndex()
	n := &Node{
		NodeResources: &NodeResources{
			Networks: []*NetworkResource{
				// As of Nomad v1.3 bridge networks get
				// registered with only their mode set.
				{
					Mode: "bridge",
				},

				// Localhost (agent interface)
				{
					CIDR:   "127.0.0.1/32",
					Device: "lo",
					IP:     "127.0.0.1",
					MBits:  1000,
					Mode:   "host",
				},
				{
					CIDR:   "::1/128",
					Device: "lo",
					IP:     "::1",
					MBits:  1000,
					Mode:   "host",
				},

				// Node.NodeResources.Networks does *not*
				// contain host_networks.
			},
			NodeNetworks: []*NodeNetworkResource{
				// As of Nomad v1.3 bridge networks get
				// registered with only their mode set.
				{
					Mode: "bridge",
				},
				{
					Addresses: []NodeNetworkAddress{
						{
							Address: "127.0.0.1",
							Alias:   "default",
							Family:  "ipv4",
						},
						{
							Address: "::1",
							Alias:   "default",
							Family:  "ipv6",
						},
					},
					Device: "lo",
					Mode:   "host",
					Speed:  1000,
				},
				{
					Addresses: []NodeNetworkAddress{
						{
							Address:       "192.168.0.1",
							Alias:         "eth0",
							Family:        "ipv4",
							ReservedPorts: "22",
						},
					},
					Device:     "enxaaaaaaaaaaaa",
					MacAddress: "aa:aa:aa:aa:aa:aa",
					Mode:       "host",
					Speed:      1000,
				},
				{
					Addresses: []NodeNetworkAddress{
						{
							Address:       "192.168.1.1",
							Alias:         "eth1",
							Family:        "ipv4",
							ReservedPorts: "80",
						},
					},
					Device:     "enxbbbbbbbbbbbb",
					MacAddress: "bb:bb:bb:bb:bb:bb",
					Mode:       "host",
					Speed:      1000,
				},
			},
		},
		ReservedResources: &NodeReservedResources{
			Networks: NodeReservedNetworkResources{
				ReservedHostPorts: "22",
			},
		},
	}

	require.NoError(t, idx.SetNode(n))

	// TaskNetworks should only contain the bridge and agent network
	require.Len(t, idx.TaskNetworks, 2)

	// Ports should be used across all 4 IPs
	require.Equal(t, 4, len(idx.UsedPorts))

	// 22 should be reserved on all IPs
	require.True(t, idx.UsedPorts["127.0.0.1"].Check(22))
	require.True(t, idx.UsedPorts["::1"].Check(22))
	require.True(t, idx.UsedPorts["192.168.0.1"].Check(22))
	require.True(t, idx.UsedPorts["192.168.1.1"].Check(22))

	// 80 should only be reserved on eth1's address
	require.False(t, idx.UsedPorts["127.0.0.1"].Check(80))
	require.False(t, idx.UsedPorts["::1"].Check(80))
	require.False(t, idx.UsedPorts["192.168.0.1"].Check(80))
	require.True(t, idx.UsedPorts["192.168.1.1"].Check(80))
}
