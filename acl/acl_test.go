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
	assert := assert.New(t)

	// Create management ACL
	acl, err := NewACL(true, nil)
	assert.Nil(err)

	// Check default namespace rights
	assert.True(acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	assert.True(acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))
	assert.True(acl.AllowNamespace("default"))

	// Check non-specified namespace
	assert.True(acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))
	assert.True(acl.AllowNamespace("foo"))

	// Check the other simpler operations
	assert.True(acl.IsManagement())
	assert.True(acl.AllowAgentRead())
	assert.True(acl.AllowAgentWrite())
	assert.True(acl.AllowNodeRead())
	assert.True(acl.AllowNodeWrite())
	assert.True(acl.AllowOperatorRead())
	assert.True(acl.AllowOperatorWrite())
	assert.True(acl.AllowQuotaRead())
	assert.True(acl.AllowQuotaWrite())
}

func TestACLMerge(t *testing.T) {
	assert := assert.New(t)

	// Merge read + write policy
	p1, err := Parse(readAll)
	assert.Nil(err)
	p2, err := Parse(writeAll)
	assert.Nil(err)
	acl, err := NewACL(false, []*Policy{p1, p2})
	assert.Nil(err)

	// Check default namespace rights
	assert.True(acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	assert.True(acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))
	assert.True(acl.AllowNamespace("default"))

	// Check non-specified namespace
	assert.False(acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))
	assert.False(acl.AllowNamespace("foo"))

	// Check the other simpler operations
	assert.False(acl.IsManagement())
	assert.True(acl.AllowAgentRead())
	assert.True(acl.AllowAgentWrite())
	assert.True(acl.AllowNodeRead())
	assert.True(acl.AllowNodeWrite())
	assert.True(acl.AllowOperatorRead())
	assert.True(acl.AllowOperatorWrite())
	assert.True(acl.AllowQuotaRead())
	assert.True(acl.AllowQuotaWrite())

	// Merge read + blank
	p3, err := Parse("")
	assert.Nil(err)
	acl, err = NewACL(false, []*Policy{p1, p3})
	assert.Nil(err)

	// Check default namespace rights
	assert.True(acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	assert.False(acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))

	// Check non-specified namespace
	assert.False(acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))

	// Check the other simpler operations
	assert.False(acl.IsManagement())
	assert.True(acl.AllowAgentRead())
	assert.False(acl.AllowAgentWrite())
	assert.True(acl.AllowNodeRead())
	assert.False(acl.AllowNodeWrite())
	assert.True(acl.AllowOperatorRead())
	assert.False(acl.AllowOperatorWrite())
	assert.True(acl.AllowQuotaRead())
	assert.False(acl.AllowQuotaWrite())

	// Merge read + deny
	p4, err := Parse(denyAll)
	assert.Nil(err)
	acl, err = NewACL(false, []*Policy{p1, p4})
	assert.Nil(err)

	// Check default namespace rights
	assert.False(acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	assert.False(acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))

	// Check non-specified namespace
	assert.False(acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))

	// Check the other simpler operations
	assert.False(acl.IsManagement())
	assert.False(acl.AllowAgentRead())
	assert.False(acl.AllowAgentWrite())
	assert.False(acl.AllowNodeRead())
	assert.False(acl.AllowNodeWrite())
	assert.False(acl.AllowOperatorRead())
	assert.False(acl.AllowOperatorWrite())
	assert.False(acl.AllowQuotaRead())
	assert.False(acl.AllowQuotaWrite())
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
quota {
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
quota {
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
quota {
	policy = "deny"
}
`

func TestAllowNamespace(t *testing.T) {
	tests := []struct {
		Policy string
		Allow  bool
	}{
		{
			Policy: `namespace "foo" {}`,
			Allow:  false,
		},
		{
			Policy: `namespace "foo" { policy = "deny" }`,
			Allow:  false,
		},
		{
			Policy: `namespace "foo" { capabilities = ["deny"] }`,
			Allow:  false,
		},
		{
			Policy: `namespace "foo" { capabilities = ["list-jobs"] }`,
			Allow:  true,
		},
		{
			Policy: `namespace "foo" { policy = "read" }`,
			Allow:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.Policy, func(t *testing.T) {
			assert := assert.New(t)

			policy, err := Parse(tc.Policy)
			assert.Nil(err)

			acl, err := NewACL(false, []*Policy{policy})
			assert.Nil(err)

			assert.Equal(tc.Allow, acl.AllowNamespace("foo"))
		})
	}
}
