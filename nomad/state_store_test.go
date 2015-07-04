package nomad

import (
	"os"
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
)

func testStateStore(t *testing.T) *StateStore {
	state, err := NewStateStore(os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if state == nil {
		t.Fatalf("missing state")
	}
	return state
}

func mockNode() *structs.Node {
	node := &structs.Node{
		ID:         generateUUID(),
		Datacenter: "dc1",
		Name:       "foobar",
		Attributes: map[string]interface{}{
			"os":            "linux",
			"arch":          "x86",
			"version":       "0.1.0",
			"driver.docker": 1,
		},
		Resouces: &structs.Resources{
			CPU:      4.0,
			MemoryMB: 8192,
			DiskMB:   100 * 1024,
			IOPS:     150,
			Networks: []*structs.NetworkResource{
				&structs.NetworkResource{
					Public:        true,
					CIDR:          "192.168.0.100/32",
					ReservedPorts: []int{22},
					MBits:         1000,
				},
			},
		},
		Reserved: &structs.Resources{
			CPU:      0.1,
			MemoryMB: 256,
			DiskMB:   4 * 1024,
		},
		Links: map[string]interface{}{
			"consul": "foobar.dc1",
		},
		Meta: map[string]string{
			"pci-dss": "true",
		},
		NodeClass: "linux-medium-pci",
		Status:    structs.NodeStatusInit,
	}
	return node
}

func TestStateStore_RegisterNode_GetNode(t *testing.T) {
	state := testStateStore(t)
	node := mockNode()

	err := state.RegisterNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(node, out) {
		t.Fatalf("bad: %#v %#v", node, out)
	}
}

func TestStateStore_DeregisterNode_GetNode(t *testing.T) {
	state := testStateStore(t)
	node := mockNode()

	err := state.RegisterNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.DeregisterNode(1001, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != nil {
		t.Fatalf("bad: %#v %#v", node, out)
	}
}

func TestStateStore_UpdateNode_GetNode(t *testing.T) {
	state := testStateStore(t)
	node := mockNode()

	err := state.RegisterNode(1000, node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = state.UpdateNodeStatus(1001, node.ID, structs.NodeStatusReady)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.Status != structs.NodeStatusReady {
		t.Fatalf("bad: %#v", out)
	}
	if out.ModifyIndex != 1001 {
		t.Fatalf("bad: %#v", out)
	}
}
