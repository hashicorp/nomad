package structs

import (
	"fmt"
	"testing"
)

func testNode() *Node {
	return &Node{
		ID:         GenerateUUID(),
		Datacenter: "dc1",
		Name:       "foobar",
		Attributes: map[string]string{
			"kernel.name": "linux",
			"arch":        "x86",
			"version":     "0.1.0",
			"driver.exec": "1",
		},
		Resources: &Resources{
			CPU:      4000,
			MemoryMB: 8192,
			DiskMB:   100 * 1024,
			IOPS:     150,
			Networks: []*NetworkResource{
				&NetworkResource{
					Device: "eth0",
					CIDR:   "192.168.0.100/32",
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
		Status:    NodeStatusReady,
	}
}

func TestNode_ComputedClass(t *testing.T) {
	// Create a node and gets it computed class
	n := testNode()
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	old := n.ComputedClass

	// Compute again to ensure determinism
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if old != n.ComputedClass {
		t.Fatalf("ComputeClass() should have returned same class; got %v; want %v", n.ComputedClass, old)
	}

	// Modify a field and compute the class again.
	n.Datacenter = "New DC"
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}

	if old == n.ComputedClass {
		t.Fatal("ComputeClass() returned same computed class")
	}
}

func TestNode_ComputedClass_Ignore(t *testing.T) {
	// Create a node and gets it computed class
	n := testNode()
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	old := n.ComputedClass

	// Modify an ignored field and compute the class again.
	n.ID = "New ID"
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}

	if old != n.ComputedClass {
		t.Fatal("ComputeClass() should have ignored field")
	}
}

func TestNode_ComputedClass_Attr(t *testing.T) {
	// Create a node and gets it computed class
	n := testNode()
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	old := n.ComputedClass

	// Add a unique addr and compute the class again
	n.Attributes[fmt.Sprintf("%s%s", "foo", NodeUniqueSuffix)] = "bar"
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
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	if old == n.ComputedClass {
		t.Fatal("ComputeClass() ignored attribute change")
	}
	old = n.ComputedClass

	// Add an ignored attribute and compute the class again.
	n.Attributes["storage.bytes-foo"] = "hello world"
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	if old != n.ComputedClass {
		t.Fatal("ComputeClass() didn't ignore unique attribute")
	}
}

func TestNode_ComputedClass_Meta(t *testing.T) {
	// Create a node and gets it computed class
	n := testNode()
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	old := n.ComputedClass

	// Modify a meta key and compute the class again.
	n.Meta["pci-dss"] = "false"
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	if old == n.ComputedClass {
		t.Fatal("ComputeClass() ignored meta change")
	}
	old = n.ComputedClass

	// Add a unique meta key and compute the class again.
	key := "test_unique"
	n.Meta[key] = "ignore"
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	if old != n.ComputedClass {
		t.Fatal("ComputeClass() didn't ignore unique meta key")
	}
}
