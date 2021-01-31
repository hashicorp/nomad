package structs

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetworkIndex_Overcommitted(t *testing.T) {
	t.Skip()
	idx := NewNetworkIndex()

	// Consume some network
	reserved := &NetworkResource{
		Device:        "eth0",
		IP:            "192.168.0.100",
		MBits:         505,
		ReservedPorts: []Port{{"one", 8000, 0, ""}, {"two", 9000, 0, ""}},
	}
	collide := idx.AddReserved(reserved)
	if collide {
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
	collide := idx.SetNode(n)
	if collide {
		t.Fatalf("bad")
	}

	if len(idx.AvailNetworks) != 1 {
		t.Fatalf("Bad")
	}
	if idx.AvailBandwidth["eth0"] != 1000 {
		t.Fatalf("Bad")
	}
	if !idx.UsedPorts["192.168.0.100"].Check(22) {
		t.Fatalf("Bad")
	}
}

func TestNetworkIndex_AddAllocs(t *testing.T) {
	idx := NewNetworkIndex()
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
								ReservedPorts: []Port{{"one", 8000, 0, ""}, {"two", 9000, 0, ""}},
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
								ReservedPorts: []Port{{"one", 10000, 0, ""}},
							},
						},
					},
				},
			},
		},
	}
	collide := idx.AddAllocs(allocs)
	if collide {
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

func TestNetworkIndex_AddReserved(t *testing.T) {
	idx := NewNetworkIndex()

	reserved := &NetworkResource{
		Device:        "eth0",
		IP:            "192.168.0.100",
		MBits:         20,
		ReservedPorts: []Port{{"one", 8000, 0, ""}, {"two", 9000, 0, ""}},
	}
	collide := idx.AddReserved(reserved)
	if collide {
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
	collide = idx.AddReserved(reserved)
	if !collide {
		t.Fatalf("bad")
	}
}

// XXX Reserving ports doesn't work when yielding from a CIDR block. This is
// okay for now since we do not actually fingerprint CIDR blocks.
func TestNetworkIndex_yieldIP(t *testing.T) {
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

func TestNetworkIndex_AssignNetwork(t *testing.T) {
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
							ReservedPorts: []Port{{"one", 8000, 0, ""}, {"two", 9000, 0, ""}},
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
							ReservedPorts: []Port{{"main", 10000, 0, ""}},
						},
					},
				},
			},
		},
	}
	idx.AddAllocs(allocs)

	// Ask for a reserved port
	ask := &NetworkResource{
		ReservedPorts: []Port{{"main", 8000, 0, ""}},
	}
	offer, err := idx.AssignNetwork(ask)
	require.NoError(t, err)
	require.NotNil(t, offer)
	require.Equal(t, "192.168.0.101", offer.IP)
	rp := Port{"main", 8000, 0, ""}
	require.Len(t, offer.ReservedPorts, 1)
	require.Exactly(t, rp, offer.ReservedPorts[0])

	// Ask for dynamic ports
	ask = &NetworkResource{
		DynamicPorts: []Port{{"http", 0, 80, ""}, {"https", 0, 443, ""}, {"admin", 0, -1, ""}},
	}
	offer, err = idx.AssignNetwork(ask)
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
		ReservedPorts: []Port{{"main", 2345, 0, ""}},
		DynamicPorts:  []Port{{"http", 0, 80, ""}, {"https", 0, 443, ""}, {"admin", 0, 8080, ""}},
	}
	offer, err = idx.AssignNetwork(ask)
	require.NoError(t, err)
	require.NotNil(t, offer)
	require.Equal(t, "192.168.0.100", offer.IP)

	rp = Port{"main", 2345, 0, ""}
	require.Len(t, offer.ReservedPorts, 1)
	require.Exactly(t, rp, offer.ReservedPorts[0])

	// Ask for too much bandwidth
	ask = &NetworkResource{
		MBits: 1000,
	}
	offer, err = idx.AssignNetwork(ask)
	require.Error(t, err)
	require.Equal(t, "bandwidth exceeded", err.Error())
	require.Nil(t, offer)
}

// This test ensures that even with a small domain of available ports we are
// able to make a dynamic port allocation.
func TestNetworkIndex_AssignNetwork_Dynamic_Contention(t *testing.T) {

	// Create a node that only has one free port
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
				ReservedHostPorts: fmt.Sprintf("%d-%d", MinDynamicPort, MaxDynamicPort-1),
			},
		},
	}
	idx.SetNode(n)

	// Ask for dynamic ports
	ask := &NetworkResource{
		DynamicPorts: []Port{{"http", 0, 80, ""}},
	}
	offer, err := idx.AssignNetwork(ask)
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
		t.Fatalf("There should be one dynamic ports")
	}
	if p := offer.DynamicPorts[0].Value; p != MaxDynamicPort {
		t.Fatalf("Dynamic Port: should have been assigned %d; got %d", p, MaxDynamicPort)
	}
}

// COMPAT(0.11): Remove in 0.11
func TestNetworkIndex_SetNode_Old(t *testing.T) {
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
					ReservedPorts: []Port{{"ssh", 22, 0, ""}},
					MBits:         1,
				},
			},
		},
	}
	collide := idx.SetNode(n)
	if collide {
		t.Fatalf("bad")
	}

	if len(idx.AvailNetworks) != 1 {
		t.Fatalf("Bad")
	}
	if idx.AvailBandwidth["eth0"] != 1000 {
		t.Fatalf("Bad")
	}
	if idx.UsedBandwidth["eth0"] != 1 {
		t.Fatalf("Bad")
	}
	if !idx.UsedPorts["192.168.0.100"].Check(22) {
		t.Fatalf("Bad")
	}
}

// COMPAT(0.11): Remove in 0.11
func TestNetworkIndex_AddAllocs_Old(t *testing.T) {
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
							ReservedPorts: []Port{{"one", 8000, 0, ""}, {"two", 9000, 0, ""}},
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
							ReservedPorts: []Port{{"one", 10000, 0, ""}},
						},
					},
				},
			},
		},
	}
	collide := idx.AddAllocs(allocs)
	if collide {
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
					ReservedPorts: []Port{{"ssh", 22, 0, ""}},
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
func TestNetworkIndex_AssignNetwork_Old(t *testing.T) {
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
					ReservedPorts: []Port{{"ssh", 22, 0, ""}},
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
							ReservedPorts: []Port{{"one", 8000, 0, ""}, {"two", 9000, 0, ""}},
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
							ReservedPorts: []Port{{"main", 10000, 0, ""}},
						},
					},
				},
			},
		},
	}
	idx.AddAllocs(allocs)

	// Ask for a reserved port
	ask := &NetworkResource{
		ReservedPorts: []Port{{"main", 8000, 0, ""}},
	}
	offer, err := idx.AssignNetwork(ask)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if offer == nil {
		t.Fatalf("bad")
	}
	if offer.IP != "192.168.0.101" {
		t.Fatalf("bad: %#v", offer)
	}
	rp := Port{"main", 8000, 0, ""}
	if len(offer.ReservedPorts) != 1 || offer.ReservedPorts[0] != rp {
		t.Fatalf("bad: %#v", offer)
	}

	// Ask for dynamic ports
	ask = &NetworkResource{
		DynamicPorts: []Port{{"http", 0, 80, ""}, {"https", 0, 443, ""}, {"admin", 0, 8080, ""}},
	}
	offer, err = idx.AssignNetwork(ask)
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
		ReservedPorts: []Port{{"main", 2345, 0, ""}},
		DynamicPorts:  []Port{{"http", 0, 80, ""}, {"https", 0, 443, ""}, {"admin", 0, 8080, ""}},
	}
	offer, err = idx.AssignNetwork(ask)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if offer == nil {
		t.Fatalf("bad")
	}
	if offer.IP != "192.168.0.100" {
		t.Fatalf("bad: %#v", offer)
	}

	rp = Port{"main", 2345, 0, ""}
	if len(offer.ReservedPorts) != 1 || offer.ReservedPorts[0] != rp {
		t.Fatalf("bad: %#v", offer)
	}

	// Ask for too much bandwidth
	ask = &NetworkResource{
		MBits: 1000,
	}
	offer, err = idx.AssignNetwork(ask)
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
func TestNetworkIndex_AssignNetwork_Dynamic_Contention_Old(t *testing.T) {

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
	for i := MinDynamicPort; i < MaxDynamicPort; i++ {
		n.Reserved.Networks[0].ReservedPorts = append(n.Reserved.Networks[0].ReservedPorts, Port{Value: i})
	}

	idx.SetNode(n)

	// Ask for dynamic ports
	ask := &NetworkResource{
		DynamicPorts: []Port{{"http", 0, 80, ""}},
	}
	offer, err := idx.AssignNetwork(ask)
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
	if p := offer.DynamicPorts[0].Value; p != MaxDynamicPort {
		t.Fatalf("Dynamic Port: should have been assigned %d; got %d", p, MaxDynamicPort)
	}
}

func TestIntContains(t *testing.T) {
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
