package structs

import (
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
		UniqueAttributes: make(map[string]struct{}),
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
		Reserved: &Resources{
			CPU:      100,
			MemoryMB: 256,
			DiskMB:   4 * 1024,
			Networks: []*NetworkResource{
				&NetworkResource{
					Device:        "eth0",
					IP:            "192.168.0.100",
					ReservedPorts: []Port{{Label: "main", Value: 22}},
					MBits:         1,
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

func TestNode_ComputedClass_NetworkResources(t *testing.T) {
	// Create a node with a few network resources and gets it computed class
	nr1 := &NetworkResource{
		Device: "eth0",
		CIDR:   "192.168.0.100/32",
		MBits:  1000,
	}
	nr2 := &NetworkResource{
		Device: "eth1",
		CIDR:   "192.168.0.100/32",
		MBits:  500,
	}
	n := &Node{
		Resources: &Resources{
			Networks: []*NetworkResource{nr1, nr2},
		},
	}
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}
	old := n.ComputedClass

	// Change the order of the network resources and compute the class again.
	n.Resources.Networks = []*NetworkResource{nr2, nr1}
	if err := n.ComputeClass(); err != nil {
		t.Fatalf("ComputeClass() failed: %v", err)
	}
	if n.ComputedClass == 0 {
		t.Fatal("ComputeClass() didn't set computed class")
	}

	if old != n.ComputedClass {
		t.Fatal("ComputeClass() didn't ignore NetworkResource order")
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
	key := "ignore"
	n.Attributes[key] = "hello world"
	n.UniqueAttributes[key] = struct{}{}
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
