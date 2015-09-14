package structs

import (
	"net"
	"reflect"
	"testing"
)

func TestNetworkIndex_Overcommitted(t *testing.T) {
	idx := NewNetworkIndex()

	// Consume some network
	reserved := &NetworkResource{
		Device:        "eth0",
		IP:            "192.168.0.100",
		MBits:         505,
		ReservedPorts: []int{8000, 9000},
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
		Resources: &Resources{
			Networks: []*NetworkResource{
				&NetworkResource{
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

	// Double up our ussage
	idx.AddReserved(reserved)
	if !idx.Overcommitted() {
		t.Fatalf("should be overcommitted")
	}
}

func TestNetworkIndex_SetNode(t *testing.T) {
	idx := NewNetworkIndex()
	n := &Node{
		Resources: &Resources{
			Networks: []*NetworkResource{
				&NetworkResource{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					MBits:  1000,
				},
			},
		},
		Reserved: &Resources{
			Networks: []*NetworkResource{
				&NetworkResource{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []int{22},
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
	if _, ok := idx.UsedPorts["192.168.0.100"][22]; !ok {
		t.Fatalf("Bad")
	}
}

func TestNetworkIndex_AddAllocs(t *testing.T) {
	idx := NewNetworkIndex()
	allocs := []*Allocation{
		&Allocation{
			TaskResources: map[string]*Resources{
				"web": &Resources{
					Networks: []*NetworkResource{
						&NetworkResource{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         20,
							ReservedPorts: []int{8000, 9000},
						},
					},
				},
			},
		},
		&Allocation{
			TaskResources: map[string]*Resources{
				"api": &Resources{
					Networks: []*NetworkResource{
						&NetworkResource{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         50,
							ReservedPorts: []int{10000},
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
	if _, ok := idx.UsedPorts["192.168.0.100"][8000]; !ok {
		t.Fatalf("Bad")
	}
	if _, ok := idx.UsedPorts["192.168.0.100"][9000]; !ok {
		t.Fatalf("Bad")
	}
	if _, ok := idx.UsedPorts["192.168.0.100"][10000]; !ok {
		t.Fatalf("Bad")
	}
}

func TestNetworkIndex_AddReserved(t *testing.T) {
	idx := NewNetworkIndex()

	reserved := &NetworkResource{
		Device:        "eth0",
		IP:            "192.168.0.100",
		MBits:         20,
		ReservedPorts: []int{8000, 9000},
	}
	collide := idx.AddReserved(reserved)
	if collide {
		t.Fatalf("bad")
	}

	if idx.UsedBandwidth["eth0"] != 20 {
		t.Fatalf("Bad")
	}
	if _, ok := idx.UsedPorts["192.168.0.100"][8000]; !ok {
		t.Fatalf("Bad")
	}
	if _, ok := idx.UsedPorts["192.168.0.100"][9000]; !ok {
		t.Fatalf("Bad")
	}

	// Try to reserve the same network
	collide = idx.AddReserved(reserved)
	if !collide {
		t.Fatalf("bad")
	}
}

func TestNetworkIndex_yieldIP(t *testing.T) {
	idx := NewNetworkIndex()
	n := &Node{
		Resources: &Resources{
			Networks: []*NetworkResource{
				&NetworkResource{
					Device: "eth0",
					CIDR:   "192.168.0.100/30",
					MBits:  1000,
				},
			},
		},
		Reserved: &Resources{
			Networks: []*NetworkResource{
				&NetworkResource{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []int{22},
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

func TestNetworkIndex_AssignNetwork(t *testing.T) {
	idx := NewNetworkIndex()
	n := &Node{
		Resources: &Resources{
			Networks: []*NetworkResource{
				&NetworkResource{
					Device: "eth0",
					CIDR:   "192.168.0.100/30",
					MBits:  1000,
				},
			},
		},
		Reserved: &Resources{
			Networks: []*NetworkResource{
				&NetworkResource{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []int{22},
					MBits:         1,
				},
			},
		},
	}
	idx.SetNode(n)

	allocs := []*Allocation{
		&Allocation{
			TaskResources: map[string]*Resources{
				"web": &Resources{
					Networks: []*NetworkResource{
						&NetworkResource{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         20,
							ReservedPorts: []int{8000, 9000},
						},
					},
				},
			},
		},
		&Allocation{
			TaskResources: map[string]*Resources{
				"api": &Resources{
					Networks: []*NetworkResource{
						&NetworkResource{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         50,
							ReservedPorts: []int{10000},
						},
					},
				},
			},
		},
	}
	idx.AddAllocs(allocs)

	// Ask for a reserved port
	ask := &NetworkResource{
		ReservedPorts: []int{8000},
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
	if len(offer.ReservedPorts) != 1 || offer.ReservedPorts[0] != 8000 {
		t.Fatalf("bad: %#v", offer)
	}

	// Ask for dynamic ports
	ask = &NetworkResource{
		DynamicPorts: 3,
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
	if len(offer.ReservedPorts) != 3 {
		t.Fatalf("bad: %#v", offer)
	}

	// Ask for reserved + dynamic ports
	ask = &NetworkResource{
		ReservedPorts: []int{12345},
		DynamicPorts:  3,
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
	if len(offer.ReservedPorts) != 4 || offer.ReservedPorts[0] != 12345 {
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

func TestIntContains(t *testing.T) {
	l := []int{1, 2, 10, 20}
	if IntContains(l, 50) {
		t.Fatalf("bad")
	}
	if !IntContains(l, 20) {
		t.Fatalf("bad")
	}
	if !IntContains(l, 1) {
		t.Fatalf("bad")
	}
}
