package scheduler

import (
	"net"
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestNetworkIndex_SetNode(t *testing.T) {
	idx := NewNetworkIndex()
	n := mock.Node()
	idx.SetNode(n)

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
	allocs := []*structs.Allocation{
		&structs.Allocation{
			TaskResources: map[string]*structs.Resources{
				"web": &structs.Resources{
					Networks: []*structs.NetworkResource{
						&structs.NetworkResource{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         20,
							ReservedPorts: []int{8000, 9000},
						},
					},
				},
			},
		},
		&structs.Allocation{
			TaskResources: map[string]*structs.Resources{
				"api": &structs.Resources{
					Networks: []*structs.NetworkResource{
						&structs.NetworkResource{
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

	reserved := &structs.NetworkResource{
		Device:        "eth0",
		IP:            "192.168.0.100",
		MBits:         20,
		ReservedPorts: []int{8000, 9000},
	}
	idx.AddReserved(reserved)

	if idx.UsedBandwidth["eth0"] != 20 {
		t.Fatalf("Bad")
	}
	if _, ok := idx.UsedPorts["192.168.0.100"][8000]; !ok {
		t.Fatalf("Bad")
	}
	if _, ok := idx.UsedPorts["192.168.0.100"][9000]; !ok {
		t.Fatalf("Bad")
	}
}

func TestNetworkIndex_yieldIP(t *testing.T) {
	idx := NewNetworkIndex()
	n := mock.Node()
	n.Resources.Networks[0].CIDR = "192.168.0.100/30"
	idx.SetNode(n)

	var out []string
	idx.yieldIP(func(n *structs.NetworkResource, ip net.IP) (stop bool) {
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
	n := mock.Node()
	n.Resources.Networks[0].CIDR = "192.168.0.100/30"
	idx.SetNode(n)

	allocs := []*structs.Allocation{
		&structs.Allocation{
			TaskResources: map[string]*structs.Resources{
				"web": &structs.Resources{
					Networks: []*structs.NetworkResource{
						&structs.NetworkResource{
							Device:        "eth0",
							IP:            "192.168.0.100",
							MBits:         20,
							ReservedPorts: []int{8000, 9000},
						},
					},
				},
			},
		},
		&structs.Allocation{
			TaskResources: map[string]*structs.Resources{
				"api": &structs.Resources{
					Networks: []*structs.NetworkResource{
						&structs.NetworkResource{
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
	ask := &structs.NetworkResource{
		ReservedPorts: []int{8000},
	}
	offer := idx.AssignNetwork(ask)
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
	ask = &structs.NetworkResource{
		DynamicPorts: 3,
	}
	offer = idx.AssignNetwork(ask)
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
	ask = &structs.NetworkResource{
		ReservedPorts: []int{12345},
		DynamicPorts:  3,
	}
	offer = idx.AssignNetwork(ask)
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
	ask = &structs.NetworkResource{
		MBits: 1000,
	}
	offer = idx.AssignNetwork(ask)
	if offer != nil {
		t.Fatalf("bad")
	}
}

func TestIntContains(t *testing.T) {
	l := []int{1, 2, 10, 20}
	if intContains(l, 50) {
		t.Fatalf("bad")
	}
	if !intContains(l, 20) {
		t.Fatalf("bad")
	}
	if !intContains(l, 1) {
		t.Fatalf("bad")
	}
}
