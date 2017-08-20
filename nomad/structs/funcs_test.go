package structs

import (
	"fmt"
	"regexp"
	"testing"

	lru "github.com/hashicorp/golang-lru"
	"github.com/stretchr/testify/assert"
)

func TestRemoveAllocs(t *testing.T) {
	l := []*Allocation{
		&Allocation{ID: "foo"},
		&Allocation{ID: "bar"},
		&Allocation{ID: "baz"},
		&Allocation{ID: "zip"},
	}

	out := RemoveAllocs(l, []*Allocation{l[1], l[3]})
	if len(out) != 2 {
		t.Fatalf("bad: %#v", out)
	}
	if out[0].ID != "foo" && out[1].ID != "baz" {
		t.Fatalf("bad: %#v", out)
	}
}

func TestFilterTerminalAllocs(t *testing.T) {
	l := []*Allocation{
		&Allocation{
			ID:            "bar",
			Name:          "myname1",
			DesiredStatus: AllocDesiredStatusEvict,
		},
		&Allocation{ID: "baz", DesiredStatus: AllocDesiredStatusStop},
		&Allocation{
			ID:            "foo",
			DesiredStatus: AllocDesiredStatusRun,
			ClientStatus:  AllocClientStatusPending,
		},
		&Allocation{
			ID:            "bam",
			Name:          "myname",
			DesiredStatus: AllocDesiredStatusRun,
			ClientStatus:  AllocClientStatusComplete,
			CreateIndex:   5,
		},
		&Allocation{
			ID:            "lol",
			Name:          "myname",
			DesiredStatus: AllocDesiredStatusRun,
			ClientStatus:  AllocClientStatusComplete,
			CreateIndex:   2,
		},
	}

	out, terminalAllocs := FilterTerminalAllocs(l)
	if len(out) != 1 {
		t.Fatalf("bad: %#v", out)
	}
	if out[0].ID != "foo" {
		t.Fatalf("bad: %#v", out)
	}

	if len(terminalAllocs) != 3 {
		for _, o := range terminalAllocs {
			fmt.Printf("%#v \n", o)
		}

		t.Fatalf("bad: %#v", terminalAllocs)
	}

	if terminalAllocs["myname"].ID != "bam" {
		t.Fatalf("bad: %#v", terminalAllocs["myname"])
	}
}

func TestAllocsFit_PortsOvercommitted(t *testing.T) {
	n := &Node{
		Resources: &Resources{
			Networks: []*NetworkResource{
				&NetworkResource{
					Device: "eth0",
					CIDR:   "10.0.0.0/8",
					MBits:  100,
				},
			},
		},
	}

	a1 := &Allocation{
		Job: &Job{
			TaskGroups: []*TaskGroup{
				{
					Name:          "web",
					EphemeralDisk: DefaultEphemeralDisk(),
				},
			},
		},
		TaskResources: map[string]*Resources{
			"web": &Resources{
				Networks: []*NetworkResource{
					&NetworkResource{
						Device:        "eth0",
						IP:            "10.0.0.1",
						MBits:         50,
						ReservedPorts: []Port{{"main", 8000}},
					},
				},
			},
		},
	}

	// Should fit one allocation
	fit, dim, _, err := AllocsFit(n, []*Allocation{a1}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !fit {
		t.Fatalf("Bad: %s", dim)
	}

	// Should not fit second allocation
	fit, _, _, err = AllocsFit(n, []*Allocation{a1, a1}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("Bad")
	}
}

func TestAllocsFit(t *testing.T) {
	n := &Node{
		Resources: &Resources{
			CPU:      2000,
			MemoryMB: 2048,
			DiskMB:   10000,
			IOPS:     100,
			Networks: []*NetworkResource{
				&NetworkResource{
					Device: "eth0",
					CIDR:   "10.0.0.0/8",
					MBits:  100,
				},
			},
		},
		Reserved: &Resources{
			CPU:      1000,
			MemoryMB: 1024,
			DiskMB:   5000,
			IOPS:     50,
			Networks: []*NetworkResource{
				&NetworkResource{
					Device:        "eth0",
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"main", 80}},
				},
			},
		},
	}

	a1 := &Allocation{
		Resources: &Resources{
			CPU:      1000,
			MemoryMB: 1024,
			DiskMB:   5000,
			IOPS:     50,
			Networks: []*NetworkResource{
				&NetworkResource{
					Device:        "eth0",
					IP:            "10.0.0.1",
					MBits:         50,
					ReservedPorts: []Port{{"main", 8000}},
				},
			},
		},
	}

	// Should fit one allocation
	fit, _, used, err := AllocsFit(n, []*Allocation{a1}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !fit {
		t.Fatalf("Bad")
	}

	// Sanity check the used resources
	if used.CPU != 2000 {
		t.Fatalf("bad: %#v", used)
	}
	if used.MemoryMB != 2048 {
		t.Fatalf("bad: %#v", used)
	}

	// Should not fit second allocation
	fit, _, used, err = AllocsFit(n, []*Allocation{a1, a1}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fit {
		t.Fatalf("Bad")
	}

	// Sanity check the used resources
	if used.CPU != 3000 {
		t.Fatalf("bad: %#v", used)
	}
	if used.MemoryMB != 3072 {
		t.Fatalf("bad: %#v", used)
	}

}

func TestScoreFit(t *testing.T) {
	node := &Node{}
	node.Resources = &Resources{
		CPU:      4096,
		MemoryMB: 8192,
	}
	node.Reserved = &Resources{
		CPU:      2048,
		MemoryMB: 4096,
	}

	// Test a perfect fit
	util := &Resources{
		CPU:      2048,
		MemoryMB: 4096,
	}
	score := ScoreFit(node, util)
	if score != 18.0 {
		t.Fatalf("bad: %v", score)
	}

	// Test the worst fit
	util = &Resources{
		CPU:      0,
		MemoryMB: 0,
	}
	score = ScoreFit(node, util)
	if score != 0.0 {
		t.Fatalf("bad: %v", score)
	}

	// Test a mid-case scenario
	util = &Resources{
		CPU:      1024,
		MemoryMB: 2048,
	}
	score = ScoreFit(node, util)
	if score < 10.0 || score > 16.0 {
		t.Fatalf("bad: %v", score)
	}
}

func TestGenerateUUID(t *testing.T) {
	prev := GenerateUUID()
	for i := 0; i < 100; i++ {
		id := GenerateUUID()
		if prev == id {
			t.Fatalf("Should get a new ID!")
		}

		matched, err := regexp.MatchString(
			"[\\da-f]{8}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{12}", id)
		if !matched || err != nil {
			t.Fatalf("expected match %s %v %s", id, matched, err)
		}
	}
}

func TestACLPolicyListHash(t *testing.T) {
	h1 := ACLPolicyListHash(nil)
	assert.NotEqual(t, "", h1)

	p1 := &ACLPolicy{
		Name:        fmt.Sprintf("policy-%s", GenerateUUID()),
		Description: "Super cool policy!",
		Rules: `
		namespace "default" {
			policy = "write"
		}
		node {
			policy = "read"
		}
		agent {
			policy = "read"
		}
		`,
		CreateIndex: 10,
		ModifyIndex: 20,
	}

	h2 := ACLPolicyListHash([]*ACLPolicy{p1})
	assert.NotEqual(t, "", h2)
	assert.NotEqual(t, h1, h2)

	// Create P2 as copy of P1 with new name
	p2 := &ACLPolicy{}
	*p2 = *p1
	p2.Name = fmt.Sprintf("policy-%s", GenerateUUID())

	h3 := ACLPolicyListHash([]*ACLPolicy{p1, p2})
	assert.NotEqual(t, "", h3)
	assert.NotEqual(t, h2, h3)

	h4 := ACLPolicyListHash([]*ACLPolicy{p2})
	assert.NotEqual(t, "", h4)
	assert.NotEqual(t, h3, h4)

	// ModifyIndex should change the hash
	p2.ModifyIndex++
	h5 := ACLPolicyListHash([]*ACLPolicy{p2})
	assert.NotEqual(t, "", h5)
	assert.NotEqual(t, h4, h5)
}

func TestCompileACLObject(t *testing.T) {
	p1 := &ACLPolicy{
		Name:        fmt.Sprintf("policy-%s", GenerateUUID()),
		Description: "Super cool policy!",
		Rules: `
		namespace "default" {
			policy = "write"
		}
		node {
			policy = "read"
		}
		agent {
			policy = "read"
		}
		`,
		CreateIndex: 10,
		ModifyIndex: 20,
	}

	// Create P2 as copy of P1 with new name
	p2 := &ACLPolicy{}
	*p2 = *p1
	p2.Name = fmt.Sprintf("policy-%s", GenerateUUID())

	// Create a small cache
	cache, err := lru.New2Q(16)
	assert.Nil(t, err)

	// Test compilation
	aclObj, err := CompileACLObject(cache, []*ACLPolicy{p1})
	assert.Nil(t, err)
	assert.NotNil(t, aclObj)

	// Should get the same object
	aclObj2, err := CompileACLObject(cache, []*ACLPolicy{p1})
	assert.Nil(t, err)
	if aclObj != aclObj2 {
		t.Fatalf("expected the same object")
	}

	// Should get another object
	aclObj3, err := CompileACLObject(cache, []*ACLPolicy{p1, p2})
	assert.Nil(t, err)
	assert.NotNil(t, aclObj3)
	if aclObj == aclObj3 {
		t.Fatalf("unexpected same object")
	}

	// Should be order independent
	aclObj4, err := CompileACLObject(cache, []*ACLPolicy{p2, p1})
	assert.Nil(t, err)
	assert.NotNil(t, aclObj4)
	if aclObj3 != aclObj4 {
		t.Fatalf("expected same object")
	}
}
