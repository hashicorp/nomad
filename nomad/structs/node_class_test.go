// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

// TODO Test
func testNode() *Node {
	return &Node{
		ID:         uuid.Generate(),
		Datacenter: "dc1",
		Name:       "foobar",
		Attributes: map[string]string{
			"kernel.name": "linux",
			"arch":        "x86",
			"version":     "0.1.0",
			"driver.exec": "1",
		},
		NodeResources: &NodeResources{
			Cpu: NodeCpuResources{
				CpuShares: 4000,
			},
			Memory: NodeMemoryResources{
				MemoryMB: 8192,
			},
			Disk: NodeDiskResources{
				DiskMB: 100 * 1024,
			},
			Networks: []*NetworkResource{
				{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
					IP:     "192.168.0.100",
					MBits:  1000,
				},
			},
		},
		Links: map[string]string{
			"consul": "foobar.dc1",
		},
		Meta: map[string]string{
			"pci-dss": "true",
		},
		NodeClass: "linux-medium-pci",
		NodePool:  "dev",
		Status:    NodeStatusReady,
	}
}

func TestNode_ComputedClass(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Create a node and gets it computed class
	n := testNode()
	require.NoError(n.ComputeClass())
	require.NotEmpty(n.ComputedClass)
	old := n.ComputedClass

	// Compute again to ensure determinism
	require.NoError(n.ComputeClass())
	require.Equal(n.ComputedClass, old)

	// Modify a field and compute the class again.
	n.Datacenter = "New DC"
	require.NoError(n.ComputeClass())
	require.NotEqual(n.ComputedClass, old)
	old = n.ComputedClass

	// Add a device
	n.NodeResources.Devices = append(n.NodeResources.Devices, &NodeDeviceResource{
		Vendor: "foo",
		Type:   "gpu",
		Name:   "bam",
	})
	require.NoError(n.ComputeClass())
	require.NotEqual(n.ComputedClass, old)
}

func TestNode_ComputedClass_Ignore(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Create a node and gets it computed class
	n := testNode()
	require.NoError(n.ComputeClass())
	require.NotEmpty(n.ComputedClass)
	old := n.ComputedClass

	// Modify an ignored field and compute the class again.
	n.ID = "New ID"
	require.NoError(n.ComputeClass())
	require.NotEmpty(n.ComputedClass)
	require.Equal(n.ComputedClass, old)

}

func TestNode_ComputedClass_Device_Attr(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Create a node and gets it computed class
	n := testNode()
	d := &NodeDeviceResource{
		Vendor: "foo",
		Type:   "gpu",
		Name:   "bam",
		Attributes: map[string]*psstructs.Attribute{
			"foo": psstructs.NewBoolAttribute(true),
		},
	}
	n.NodeResources.Devices = append(n.NodeResources.Devices, d)
	require.NoError(n.ComputeClass())
	require.NotEmpty(n.ComputedClass)
	old := n.ComputedClass

	// Update the attributes to be have a unique value
	d.Attributes["unique.bar"] = psstructs.NewBoolAttribute(false)
	require.NoError(n.ComputeClass())
	require.Equal(n.ComputedClass, old)
}

func TestNode_ComputedClass_Attr(t *testing.T) {
	ci.Parallel(t)

	// Create a node and gets it computed class
	n := testNode()
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == "" {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	old := n.ComputedClass

	// Add a unique addr and compute the class again
	n.Attributes["unique.foo"] = "bar"
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if old != n.ComputedClass {
		t.Fatal("ComputeClass() didn't ignore unique attr suffix")
	}

	// Modify an attribute and compute the class again.
	n.Attributes["version"] = "New Version"
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == "" {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	if old == n.ComputedClass {
		t.Fatal("ComputeClass() ignored attribute change")
	}

	// Remove and attribute and compute the class again.
	old = n.ComputedClass
	delete(n.Attributes, "driver.exec")
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputedClass() failed: %v", err)
	}
	if n.ComputedClass == "" {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	if old == n.ComputedClass {
		t.Fatalf("ComputedClass() ignored removal of attribute key")
	}
}

func TestNode_ComputedClass_Meta(t *testing.T) {
	ci.Parallel(t)

	// Create a node and gets it computed class
	n := testNode()
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == "" {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	old := n.ComputedClass

	// Modify a meta key and compute the class again.
	n.Meta["pci-dss"] = "false"
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == "" {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	if old == n.ComputedClass {
		t.Fatal("ComputeClass() ignored meta change")
	}
	old = n.ComputedClass

	// Add a unique meta key and compute the class again.
	n.Meta["unique.foo"] = "ignore"
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == "" {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	if old != n.ComputedClass {
		t.Fatal("ComputeClass() didn't ignore unique meta key")
	}
}

func TestNode_ComputedClass_NodePool(t *testing.T) {
	ci.Parallel(t)

	// Create a node and get its computed class.
	n := testNode()
	err := n.ComputeClass()
	must.NoError(t, err)
	must.NotEq(t, "", n.ComputedClass)
	old := n.ComputedClass

	// Modify node pool and expect computed class to change.
	n.NodePool = "prod"
	err = n.ComputeClass()
	must.NoError(t, err)
	must.NotEq(t, "", n.ComputedClass)
	must.NotEq(t, old, n.ComputedClass)
	old = n.ComputedClass
}

func TestNode_EscapedConstraints(t *testing.T) {
	ci.Parallel(t)

	// Non-escaped constraints
	ne1 := &Constraint{
		LTarget: "${attr.kernel.name}",
		RTarget: "linux",
		Operand: "=",
	}
	ne2 := &Constraint{
		LTarget: "${meta.key_foo}",
		RTarget: "linux",
		Operand: "<",
	}
	ne3 := &Constraint{
		LTarget: "${node.dc}",
		RTarget: "test",
		Operand: "!=",
	}

	// Escaped constraints
	e1 := &Constraint{
		LTarget: "${attr.unique.kernel.name}",
		RTarget: "linux",
		Operand: "=",
	}
	e2 := &Constraint{
		LTarget: "${meta.unique.key_foo}",
		RTarget: "linux",
		Operand: "<",
	}
	e3 := &Constraint{
		LTarget: "${unique.node.id}",
		RTarget: "test",
		Operand: "!=",
	}
	constraints := []*Constraint{ne1, ne2, ne3, e1, e2, e3}
	expected := []*Constraint{ne1, ne2, ne3}
	if act := EscapedConstraints(constraints); reflect.DeepEqual(act, expected) {
		t.Fatalf("EscapedConstraints(%v) returned %v; want %v", constraints, act, expected)
	}
}
