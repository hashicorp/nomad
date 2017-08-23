package acl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCapabilitySet(t *testing.T) {
	var cs capabilitySet = make(map[string]struct{})

	// Check no capabilities by default
	if cs.Check(PolicyDeny) {
		t.Fatalf("unexpected check")
	}

	// Do a set and check
	cs.Set(PolicyDeny)
	if !cs.Check(PolicyDeny) {
		t.Fatalf("missing check")
	}

	// Clear and check
	cs.Clear()
	if cs.Check(PolicyDeny) {
		t.Fatalf("unexpected check")
	}
}

func TestMaxPrivilege(t *testing.T) {
	type tcase struct {
		Privilege      string
		PrecedenceOver []string
	}
	tcases := []tcase{
		{
			PolicyDeny,
			[]string{PolicyDeny, PolicyWrite, PolicyRead, ""},
		},
		{
			PolicyWrite,
			[]string{PolicyWrite, PolicyRead, ""},
		},
		{
			PolicyRead,
			[]string{PolicyRead, ""},
		},
	}

	for idx1, tc := range tcases {
		for idx2, po := range tc.PrecedenceOver {
			if maxPrivilege(tc.Privilege, po) != tc.Privilege {
				t.Fatalf("failed %d %d", idx1, idx2)
			}
			if maxPrivilege(po, tc.Privilege) != tc.Privilege {
				t.Fatalf("failed %d %d", idx1, idx2)
			}
		}
	}
}

func TestACLManagement(t *testing.T) {
	// Create management ACL
	acl, err := NewACL(true, nil)
	assert.Nil(t, err)

	// Check default namespace rights
	assert.Equal(t, true, acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	assert.Equal(t, true, acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))

	// Check non-specified namespace
	assert.Equal(t, true, acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))

	// Check the other simpler operations
	assert.Equal(t, true, acl.IsManagement())
	assert.Equal(t, true, acl.AllowAgentRead())
	assert.Equal(t, true, acl.AllowAgentWrite())
	assert.Equal(t, true, acl.AllowNodeRead())
	assert.Equal(t, true, acl.AllowNodeWrite())
	assert.Equal(t, true, acl.AllowOperatorRead())
	assert.Equal(t, true, acl.AllowOperatorWrite())
}

func TestACLMerge(t *testing.T) {
	// Merge read + write policy
	p1, err := Parse(readAll)
	assert.Nil(t, err)
	p2, err := Parse(writeAll)
	assert.Nil(t, err)
	acl, err := NewACL(false, []*Policy{p1, p2})
	assert.Nil(t, err)

	// Check default namespace rights
	assert.Equal(t, true, acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	assert.Equal(t, true, acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))

	// Check non-specified namespace
	assert.Equal(t, false, acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))

	// Check the other simpler operations
	assert.Equal(t, false, acl.IsManagement())
	assert.Equal(t, true, acl.AllowAgentRead())
	assert.Equal(t, true, acl.AllowAgentWrite())
	assert.Equal(t, true, acl.AllowNodeRead())
	assert.Equal(t, true, acl.AllowNodeWrite())
	assert.Equal(t, true, acl.AllowOperatorRead())
	assert.Equal(t, true, acl.AllowOperatorWrite())

	// Merge read + blank
	p3, err := Parse("")
	assert.Nil(t, err)
	acl, err = NewACL(false, []*Policy{p1, p3})
	assert.Nil(t, err)

	// Check default namespace rights
	assert.Equal(t, true, acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	assert.Equal(t, false, acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))

	// Check non-specified namespace
	assert.Equal(t, false, acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))

	// Check the other simpler operations
	assert.Equal(t, false, acl.IsManagement())
	assert.Equal(t, true, acl.AllowAgentRead())
	assert.Equal(t, false, acl.AllowAgentWrite())
	assert.Equal(t, true, acl.AllowNodeRead())
	assert.Equal(t, false, acl.AllowNodeWrite())
	assert.Equal(t, true, acl.AllowOperatorRead())
	assert.Equal(t, false, acl.AllowOperatorWrite())

	// Merge read + deny
	p4, err := Parse(denyAll)
	assert.Nil(t, err)
	acl, err = NewACL(false, []*Policy{p1, p4})
	assert.Nil(t, err)

	// Check default namespace rights
	assert.Equal(t, false, acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	assert.Equal(t, false, acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))

	// Check non-specified namespace
	assert.Equal(t, false, acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))

	// Check the other simpler operations
	assert.Equal(t, false, acl.IsManagement())
	assert.Equal(t, false, acl.AllowAgentRead())
	assert.Equal(t, false, acl.AllowAgentWrite())
	assert.Equal(t, false, acl.AllowNodeRead())
	assert.Equal(t, false, acl.AllowNodeWrite())
	assert.Equal(t, false, acl.AllowOperatorRead())
	assert.Equal(t, false, acl.AllowOperatorWrite())
}

var readAll = `
namespace "default" {
	policy = "read"
}
agent {
	policy = "read"
}
node {
	policy = "read"
}
operator {
	policy = "read"
}
`

var writeAll = `
namespace "default" {
	policy = "write"
}
agent {
	policy = "write"
}
node {
	policy = "write"
}
operator {
	policy = "write"
}
`

var denyAll = `
namespace "default" {
	policy = "deny"
}
agent {
	policy = "deny"
}
node {
	policy = "deny"
}
operator {
	policy = "deny"
}
`
