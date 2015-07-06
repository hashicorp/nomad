package nomad

import (
	"os"
	"reflect"
	"sort"
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
		Attributes: map[string]string{
			"os":            "linux",
			"arch":          "x86",
			"version":       "0.1.0",
			"driver.docker": "1.0.0",
		},
		Resources: &structs.Resources{
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
		Links: map[string]string{
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

func TestStateStore_Nodes(t *testing.T) {
	state := testStateStore(t)
	var nodes []*structs.Node

	for i := 0; i < 10; i++ {
		node := mockNode()
		nodes = append(nodes, node)

		err := state.RegisterNode(1000+uint64(i), node)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	iter, err := state.Nodes()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out []*structs.Node
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Node))
	}

	sort.Sort(NodeIDSort(nodes))
	sort.Sort(NodeIDSort(out))

	if !reflect.DeepEqual(nodes, out) {
		t.Fatalf("bad: %#v %#v", nodes, out)
	}
}

func TestStateStore_RestoreNode(t *testing.T) {
	state := testStateStore(t)

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	node := mockNode()
	err = restore.NodeRestore(node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	restore.Commit()

	out, err := state.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, node) {
		t.Fatalf("Bad: %#v %#v", out, node)
	}
}

// NodeIDSort is used to sort nodes by ID
type NodeIDSort []*structs.Node

func (n NodeIDSort) Len() int {
	return len(n)
}

func (n NodeIDSort) Less(i, j int) bool {
	return n[i].ID < n[j].ID
}

func (n NodeIDSort) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
