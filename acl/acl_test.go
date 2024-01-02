// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilitySet(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

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
	ci.Parallel(t)

	// Create management ACL
	acl, err := NewACL(true, nil)
	must.NoError(t, err)

	// Check default namespace rights
	must.True(t, acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	must.True(t, acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))
	must.True(t, acl.AllowNamespace("default"))

	// Check non-specified namespace
	must.True(t, acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))
	must.True(t, acl.AllowNamespace("foo"))

	// Check node pool rights.
	must.True(t, acl.AllowNodePoolOperation("my-pool", NodePoolCapabilityWrite))
	must.True(t, acl.AllowNodePool("my-pool"))

	// Check the other simpler operations
	must.True(t, acl.IsManagement())
	must.True(t, acl.AllowAgentRead())
	must.True(t, acl.AllowAgentWrite())
	must.True(t, acl.AllowNodeRead())
	must.True(t, acl.AllowNodeWrite())
	must.True(t, acl.AllowOperatorRead())
	must.True(t, acl.AllowOperatorWrite())
	must.True(t, acl.AllowQuotaRead())
	must.True(t, acl.AllowQuotaWrite())
}

func TestACLMerge(t *testing.T) {
	ci.Parallel(t)

	// Merge read + write policy
	p1, err := Parse(readAll)
	must.NoError(t, err)
	p2, err := Parse(writeAll)
	must.NoError(t, err)
	acl, err := NewACL(false, []*Policy{p1, p2})
	must.NoError(t, err)

	// Check default namespace rights
	must.True(t, acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	must.True(t, acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))
	must.True(t, acl.AllowNamespace("default"))

	// Check non-specified namespace
	must.False(t, acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))
	must.False(t, acl.AllowNamespace("foo"))

	// Check rights in the node pool specified in policies.
	must.True(t, acl.AllowNodePoolOperation("my-pool", NodePoolCapabilityRead))
	must.True(t, acl.AllowNodePoolOperation("my-pool", NodePoolCapabilityWrite))
	must.True(t, acl.AllowNodePool("my-pool"))

	// Check non-specified node pool policies.
	must.False(t, acl.AllowNodePoolOperation("other-pool", NodePoolCapabilityRead))
	must.False(t, acl.AllowNodePoolOperation("other-pool", NodePoolCapabilityWrite))
	must.False(t, acl.AllowNodePool("other-pool"))

	// Check the other simpler operations
	must.False(t, acl.IsManagement())
	must.True(t, acl.AllowAgentRead())
	must.True(t, acl.AllowAgentWrite())
	must.True(t, acl.AllowNodeRead())
	must.True(t, acl.AllowNodeWrite())
	must.True(t, acl.AllowOperatorRead())
	must.True(t, acl.AllowOperatorWrite())
	must.True(t, acl.AllowQuotaRead())
	must.True(t, acl.AllowQuotaWrite())

	// Merge read + blank
	p3, err := Parse("")
	must.NoError(t, err)
	acl, err = NewACL(false, []*Policy{p1, p3})
	must.NoError(t, err)

	// Check default namespace rights
	must.True(t, acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	must.False(t, acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))

	// Check non-specified namespace
	must.False(t, acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))

	// Check rights in the node pool specified in policies.
	must.True(t, acl.AllowNodePoolOperation("my-pool", NodePoolCapabilityRead))
	must.False(t, acl.AllowNodePoolOperation("my-pool", NodePoolCapabilityWrite))
	must.True(t, acl.AllowNodePool("my-pool"))

	// Check non-specified node pool policies.
	must.False(t, acl.AllowNodePoolOperation("other-pool", NodePoolCapabilityRead))
	must.False(t, acl.AllowNodePoolOperation("other-pool", NodePoolCapabilityWrite))
	must.False(t, acl.AllowNodePool("other-pool"))

	// Check the other simpler operations
	must.False(t, acl.IsManagement())
	must.True(t, acl.AllowAgentRead())
	must.False(t, acl.AllowAgentWrite())
	must.True(t, acl.AllowNodeRead())
	must.False(t, acl.AllowNodeWrite())
	must.True(t, acl.AllowOperatorRead())
	must.False(t, acl.AllowOperatorWrite())
	must.True(t, acl.AllowQuotaRead())
	must.False(t, acl.AllowQuotaWrite())

	// Merge read + deny
	p4, err := Parse(denyAll)
	must.NoError(t, err)
	acl, err = NewACL(false, []*Policy{p1, p4})
	must.NoError(t, err)

	// Check default namespace rights
	must.False(t, acl.AllowNamespaceOperation("default", NamespaceCapabilityListJobs))
	must.False(t, acl.AllowNamespaceOperation("default", NamespaceCapabilitySubmitJob))

	// Check non-specified namespace
	must.False(t, acl.AllowNamespaceOperation("foo", NamespaceCapabilityListJobs))

	// Check rights in the node pool specified in policies.
	must.False(t, acl.AllowNodePoolOperation("my-pool", NodePoolCapabilityRead))
	must.False(t, acl.AllowNodePoolOperation("my-pool", NodePoolCapabilityWrite))
	must.False(t, acl.AllowNodePool("my-pool"))

	// Check non-specified node pool policies.
	must.False(t, acl.AllowNodePoolOperation("other-pool", NodePoolCapabilityRead))
	must.False(t, acl.AllowNodePoolOperation("other-pool", NodePoolCapabilityWrite))
	must.False(t, acl.AllowNodePool("other-pool"))

	// Check the other simpler operations
	must.False(t, acl.IsManagement())
	must.False(t, acl.AllowAgentRead())
	must.False(t, acl.AllowAgentWrite())
	must.False(t, acl.AllowNodeRead())
	must.False(t, acl.AllowNodeWrite())
	must.False(t, acl.AllowOperatorRead())
	must.False(t, acl.AllowOperatorWrite())
	must.False(t, acl.AllowQuotaRead())
	must.False(t, acl.AllowQuotaWrite())
}

var readAll = `
namespace "default" {
	policy = "read"
}
node_pool "my-pool" {
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
node_pool "my-pool" {
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
node_pool "my-pool" {
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
	ci.Parallel(t)

	tests := []struct {
		name      string
		policy    string
		allow     bool
		namespace string
	}{
		{
			name:      "foo namespace - no capabilities",
			policy:    `namespace "foo" {}`,
			allow:     false,
			namespace: "foo",
		},
		{
			name:      "foo namespace - deny policy",
			policy:    `namespace "foo" { policy = "deny" }`,
			allow:     false,
			namespace: "foo",
		},
		{
			name:      "foo namespace - deny capability",
			policy:    `namespace "foo" { capabilities = ["deny"] }`,
			allow:     false,
			namespace: "foo",
		},
		{
			name:      "foo namespace - with capability",
			policy:    `namespace "foo" { capabilities = ["list-jobs"] }`,
			allow:     true,
			namespace: "foo",
		},
		{
			name:      "foo namespace - with policy",
			policy:    `namespace "foo" { policy = "read" }`,
			allow:     true,
			namespace: "foo",
		},
		{
			name:      "wildcard namespace - no capabilities",
			policy:    `namespace "foo" {}`,
			allow:     false,
			namespace: "*",
		},
		{
			name:      "wildcard namespace - deny policy",
			policy:    `namespace "foo" { policy = "deny" }`,
			allow:     false,
			namespace: "*",
		},
		{
			name:      "wildcard namespace - deny capability",
			policy:    `namespace "foo" { capabilities = ["deny"] }`,
			allow:     false,
			namespace: "*",
		},
		{
			name:      "wildcard namespace - with capability",
			policy:    `namespace "foo" { capabilities = ["list-jobs"] }`,
			allow:     true,
			namespace: "*",
		},
		{
			name:      "wildcard namespace - with policy",
			policy:    `namespace "foo" { policy = "read" }`,
			allow:     true,
			namespace: "*",
		},
		{
			name:      "wildcard namespace - no namespace rule",
			policy:    `agent { policy = "read" }`,
			allow:     false,
			namespace: "*",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := Parse(tc.policy)
			require.NoError(t, err)

			acl, err := NewACL(false, []*Policy{policy})
			require.NoError(t, err)

			got := acl.AllowNamespace(tc.namespace)
			require.Equal(t, tc.allow, got)
		})
	}
}

func TestWildcardNamespaceMatching(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name      string
		policy    string
		allow     bool
		namespace string
	}{
		{
			name:      "wildcard matches",
			policy:    `namespace "prod-api-*" { policy = "write" }`,
			allow:     true,
			namespace: "prod-api-services",
		},
		{
			name:      "non globbed namespaces are not wildcards",
			policy:    `namespace "prod-api" { policy = "write" }`,
			allow:     false,
			namespace: "prod-api-services",
		},
		{
			name: "concrete matches take precedence",
			policy: `namespace "prod-api-services" { policy = "deny" }
			         namespace "prod-api-*" { policy = "write" }`,
			allow:     false,
			namespace: "prod-api-services",
		},
		{
			name: "glob match",
			policy: `namespace "prod-api-*" { policy = "deny" }
			         namespace "prod-api-services" { policy = "write" }`,
			allow:     true,
			namespace: "prod-api-services",
		},
		{
			name: "closest character match wins - suffix",
			policy: `namespace "*-api-services" { policy = "deny" }
			         namespace "prod-api-*" { policy = "write" }`, // 4 vs 8 chars
			allow:     false,
			namespace: "prod-api-services",
		},
		{
			name: "closest character match wins - prefix",
			policy: `namespace "prod-api-*" { policy = "write" }
               namespace "*-api-services" { policy = "deny" }`, // 4 vs 8 chars
			allow:     false,
			namespace: "prod-api-services",
		},
		{
			name: "wildcard namespace with glob match",
			policy: `namespace "prod-api-*" { policy = "deny" }
			         namespace "prod-api-services" { policy = "write" }`,
			allow:     true,
			namespace: "*",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := Parse(tc.policy)
			require.NoError(t, err)
			require.NotNil(t, policy.Namespaces)

			acl, err := NewACL(false, []*Policy{policy})
			require.NoError(t, err)

			got := acl.AllowNamespace(tc.namespace)
			require.Equal(t, tc.allow, got)
		})
	}
}

func TestNodePool(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name     string
		policy   string
		pool     string
		allowOps []string
		denyOps  []string
		allow    bool
	}{
		{
			name: "policy read",
			policy: `
node_pool "my-pool" {
	policy = "read"
}
`,
			pool:     "my-pool",
			allowOps: []string{NodePoolCapabilityRead},
			denyOps: []string{
				NodePoolCapabilityDelete,
				NodePoolCapabilityWrite,
			},
			allow: true,
		},
		{
			name: "policy write",
			policy: `
node_pool "my-pool" {
	policy = "write"
}
`,
			pool: "my-pool",
			allowOps: []string{
				NodePoolCapabilityDelete,
				NodePoolCapabilityRead,
				NodePoolCapabilityWrite,
			},
			denyOps: []string{},
			allow:   true,
		},
		{
			name: "capability write",
			policy: `
node_pool "my-pool" {
	capabilities = ["write"]
}
`,
			pool: "my-pool",
			allowOps: []string{
				NodePoolCapabilityWrite,
			},
			denyOps: []string{
				NodePoolCapabilityDelete,
				NodePoolCapabilityRead,
			},
			allow: true,
		},
		{
			name: "multiple capabilities",
			policy: `
node_pool "my-pool" {
	capabilities = ["read", "delete"]
}
`,
			pool: "my-pool",
			allowOps: []string{
				NodePoolCapabilityRead,
				NodePoolCapabilityDelete,
			},
			denyOps: []string{
				NodePoolCapabilityWrite,
			},
			allow: true,
		},
		{
			name: "policy deny takes precedence",
			policy: `
node_pool "my-pool" {
	policy = "deny"
	capabilities = ["write", "delete"]
}
`,
			pool:     "my-pool",
			allowOps: []string{},
			denyOps: []string{
				NodePoolCapabilityDelete,
				NodePoolCapabilityRead,
				NodePoolCapabilityWrite,
			},
			allow: false,
		},
		{
			name: "capability deny takes precedence",
			policy: `
node_pool "my-pool" {
	capabilities = ["write", "delete", "deny"]
}
`,
			pool:     "my-pool",
			allowOps: []string{},
			denyOps: []string{
				NodePoolCapabilityDelete,
				NodePoolCapabilityRead,
				NodePoolCapabilityWrite,
			},
			allow: false,
		},
		{
			name: "wildcard matches all",
			policy: `
node_pool "*" {
	policy = "read"
}
`,
			pool:     "my-pool",
			allowOps: []string{NodePoolCapabilityRead},
			denyOps: []string{
				NodePoolCapabilityDelete,
				NodePoolCapabilityWrite,
			},
			allow: true,
		},
		{
			name: "wildcard matches subset",
			policy: `
node_pool "my-pool-*" {
	policy = "read"
}
`,
			pool:     "my-pool-1",
			allowOps: []string{NodePoolCapabilityRead},
			denyOps: []string{
				NodePoolCapabilityDelete,
				NodePoolCapabilityWrite,
			},
			allow: true,
		},
		{
			name: "wildcard doesn't match subset",
			policy: `
node_pool "my-pool-*" {
	policy = "read"
}
`,
			pool:     "your-pool-1",
			allowOps: []string{},
			denyOps: []string{
				NodePoolCapabilityDelete,
				NodePoolCapabilityRead,
				NodePoolCapabilityWrite,
			},
			allow: false,
		},
		{
			name: "wildcard matches closest",
			policy: `
node_pool "my-pool-dev-*" {
	policy = "read"
}

node_pool "my-pool-*" {
	policy = "write"
}

node_pool "*" {
	policy = "deny"
}
`,
			pool:     "my-pool-dev-1",
			allowOps: []string{NodePoolCapabilityRead},
			denyOps: []string{
				NodePoolCapabilityDelete,
				NodePoolCapabilityWrite,
			},
			allow: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := Parse(tc.policy)
			must.NoError(t, err)
			must.NotNil(t, policy.NodePools)

			acl, err := NewACL(false, []*Policy{policy})
			must.NoError(t, err)

			for _, op := range tc.allowOps {
				got := acl.AllowNodePoolOperation(tc.pool, op)
				assert.True(t, got, must.Sprintf("expected operation %q to be allowed", op))
			}

			for _, op := range tc.denyOps {
				got := acl.AllowNodePoolOperation(tc.pool, op)
				assert.False(t, got, must.Sprintf("expected operation %q to be denied", op))
			}

			if tc.allow {
				must.True(t, acl.AllowNodePool(tc.pool), must.Sprint("expected node pool to be allowed"))
			} else {
				must.False(t, acl.AllowNodePool(tc.pool), must.Sprint("expected node pool to be denied"))
			}
		})
	}
}

func TestWildcardHostVolumeMatching(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		Policy string
		Allow  bool
	}{
		{ // Wildcard matches
			Policy: `host_volume "prod-api-*" { policy = "write" }`,
			Allow:  true,
		},
		{ // Non globbed volumes are not wildcards
			Policy: `host_volume "prod-api" { policy = "write" }`,
			Allow:  false,
		},
		{ // Concrete matches take precedence
			Policy: `host_volume "prod-api-services" { policy = "deny" }
			         host_volume "prod-api-*" { policy = "write" }`,
			Allow: false,
		},
		{
			Policy: `host_volume "prod-api-*" { policy = "deny" }
			         host_volume "prod-api-services" { policy = "write" }`,
			Allow: true,
		},
		{ // The closest character match wins
			Policy: `host_volume "*-api-services" { policy = "deny" }
			         host_volume "prod-api-*" { policy = "write" }`, // 4 vs 8 chars
			Allow: false,
		},
		{
			Policy: `host_volume "prod-api-*" { policy = "write" }
               host_volume "*-api-services" { policy = "deny" }`, // 4 vs 8 chars
			Allow: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.Policy, func(t *testing.T) {
			assert := assert.New(t)

			policy, err := Parse(tc.Policy)
			assert.NoError(err)
			assert.NotNil(policy.HostVolumes)

			acl, err := NewACL(false, []*Policy{policy})
			assert.Nil(err)

			assert.Equal(tc.Allow, acl.AllowHostVolume("prod-api-services"))
		})
	}
}

func TestVariablesMatching(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		name   string
		policy string
		ns     string
		path   string
		op     string
		claim  *ACLClaim
		allow  bool
	}{
		{
			name: "concrete namespace with concrete path matches",
			policy: `namespace "ns" {
					variables { path "foo/bar" { capabilities = ["read"] }}}`,
			ns:    "ns",
			path:  "foo/bar",
			op:    "read",
			allow: true,
		},
		{
			name: "concrete namespace with concrete path matches for expanded caps",
			policy: `namespace "ns" {
					variables { path "foo/bar" { capabilities = ["read"] }}}`,
			ns:    "ns",
			path:  "foo/bar",
			op:    "list",
			allow: true,
		},
		{
			name: "concrete namespace with wildcard path matches",
			policy: `namespace "ns" {
					variables { path "foo/*" { capabilities = ["read"] }}}`,
			ns:    "ns",
			path:  "foo/bar",
			op:    "read",
			allow: true,
		},
		{
			name: "concrete namespace with non-prefix wildcard path matches",
			policy: `namespace "ns" {
					variables { path "*/bar" { capabilities = ["read"] }}}`,
			ns:    "ns",
			path:  "foo/bar",
			op:    "read",
			allow: true,
		},
		{
			name: "concrete namespace with overlapping wildcard path prefix over suffix matches",
			policy: `namespace "ns" {
					variables {
						path "*/bar" { capabilities = ["list"] }
						path "foo/*" { capabilities = ["write"] }
					}}`,
			ns:    "ns",
			path:  "foo/bar",
			op:    "write",
			allow: true,
		},
		{
			name: "concrete namespace with overlapping wildcard path prefix over suffix denied",
			policy: `namespace "ns" {
					variables {
						path "*/bar" { capabilities = ["list"] }
						path "foo/*" { capabilities = ["write"] }
					}}`,
			ns:    "ns",
			path:  "foo/bar",
			op:    "list",
			allow: false,
		},
		{
			name: "concrete namespace with wildcard path matches most specific only",
			policy: `namespace "ns" {
					variables {
						path "*" { capabilities = ["read"] }
						path "foo/*" { capabilities = ["read"] }
						path "foo/bar" { capabilities = ["list"] }
					}}`,
			ns:    "ns",
			path:  "foo/bar",
			op:    "read",
			allow: false,
		},
		{
			name: "concrete namespace with invalid concrete path fails",
			policy: `namespace "ns" {
					variables { path "bar" { capabilities = ["read"] }}}`,
			ns:    "ns",
			path:  "foo/bar",
			op:    "read",
			allow: false,
		},
		{
			name: "concrete namespace with invalid wildcard path fails",
			policy: `namespace "ns" {
					variables { path "*/foo" { capabilities = ["read"] }}}`,
			ns:    "ns",
			path:  "foo/bar",
			op:    "read",
			allow: false,
		},
		{
			name: "wildcard namespace with concrete path matches",
			policy: `namespace "*" {
					variables { path "foo/bar" { capabilities = ["read"] }}}`,
			ns:    "ns",
			path:  "foo/bar",
			op:    "read",
			allow: true,
		},
		{
			name: "wildcard namespace with invalid concrete path fails",
			policy: `namespace "*" {
					variables { path "bar" { capabilities = ["read"] }}}`,
			ns:    "ns",
			path:  "foo/bar",
			op:    "read",
			allow: false,
		},
		{
			name: "wildcard in user provided path fails",
			policy: `namespace "ns" {
					variables { path "foo/bar" { capabilities = ["read"] }}}`,
			ns:    "ns",
			path:  "*",
			op:    "read",
			allow: false,
		},
		{
			name: "wildcard attempt to bypass delimiter null byte fails",
			policy: `namespace "ns" {
					variables { path "foo/bar" { capabilities = ["read"] }}}`,
			ns:    "ns*",
			path:  "bar",
			op:    "read",
			allow: false,
		},
		{
			name: "wildcard with more specific denied path",
			policy: `namespace "ns" {
					variables {
					path "*" { capabilities = ["list"] }
					path "system/*" { capabilities = ["deny"] }}}`,
			ns:    "ns",
			path:  "system/not-allowed",
			op:    "list",
			allow: false,
		},
		{
			name: "multiple namespace with overlapping paths",
			policy: `namespace "ns" {
						variables {
  						path "*" { capabilities = ["list"] }
						path "system/*" { capabilities = ["deny"] }}}
					namespace "prod" {
						variables {
						path "*" { capabilities = ["list"]}}}`,
			ns:    "prod",
			path:  "system/is-allowed",
			op:    "list",
			allow: true,
		},
		{
			name: "claim with more specific policy",
			policy: `namespace "ns" {
					variables { path "nomad/jobs/example" { capabilities = ["deny"] }}}`,
			ns:    "ns",
			path:  "nomad/jobs/example",
			op:    "read",
			claim: &ACLClaim{Namespace: "ns", Job: "example", Group: "foo", Task: "bar"},
			allow: false,
		},
		{
			name: "claim with less specific policy",
			policy: `namespace "ns" {
					variables { path "nomad/jobs" { capabilities = ["deny"] }}}`,
			ns:    "ns",
			path:  "nomad/jobs/example",
			op:    "read",
			claim: &ACLClaim{Namespace: "ns", Job: "example", Group: "foo", Task: "bar"},
			allow: true,
		},
		{
			name: "claim with less specific wildcard policy",
			policy: `namespace "ns" {
					variables { path "nomad/jobs/*" { capabilities = ["deny"] }}}`,
			ns:    "ns",
			path:  "nomad/jobs/example",
			op:    "read",
			claim: &ACLClaim{Namespace: "ns", Job: "example", Group: "foo", Task: "bar"},
			allow: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			policy, err := Parse(tc.policy)
			require.NoError(t, err)
			require.NotNil(t, policy.Namespaces[0].Variables)

			acl, err := NewACL(false, []*Policy{policy})
			require.NoError(t, err)
			allowed := acl.AllowVariableOperation(tc.ns, tc.path, tc.op, tc.claim)
			require.Equal(t, tc.allow, allowed)
		})
	}

	t.Run("search over namespace", func(t *testing.T) {
		policy, err := Parse(`namespace "ns" {
					variables { path "foo/bar" { capabilities = ["read"] }}}`)
		require.NoError(t, err)
		require.NotNil(t, policy.Namespaces[0].Variables)

		acl, err := NewACL(false, []*Policy{policy})
		require.NoError(t, err)
		require.True(t, acl.AllowVariableSearch("ns"))
		require.False(t, acl.AllowVariableSearch("no-access"))
	})

}

func TestACL_matchingCapabilitySet_returnsAllMatches(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		Policy        string
		NS            string
		MatchingGlobs []string
	}{
		{
			Policy:        `namespace "production-*" { policy = "write" }`,
			NS:            "production-api",
			MatchingGlobs: []string{"production-*"},
		},
		{
			Policy:        `namespace "prod-*" { policy = "write" }`,
			NS:            "production-api",
			MatchingGlobs: nil,
		},
		{
			Policy: `namespace "production-*" { policy = "write" }
							 namespace "production-*-api" { policy = "deny" }`,

			NS:            "production-admin-api",
			MatchingGlobs: []string{"production-*", "production-*-api"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Policy, func(t *testing.T) {
			assert := assert.New(t)

			policy, err := Parse(tc.Policy)
			assert.NoError(err)
			assert.NotNil(policy.Namespaces)

			acl, err := NewACL(false, []*Policy{policy})
			assert.Nil(err)

			var namespaces []string
			for _, cs := range findAllMatchingWildcards(acl.wildcardNamespaces, tc.NS) {
				namespaces = append(namespaces, cs.name)
			}

			assert.Equal(tc.MatchingGlobs, namespaces)
		})
	}
}

func TestACL_matchingCapabilitySet_difference(t *testing.T) {
	ci.Parallel(t)

	tests := []struct {
		Policy     string
		NS         string
		Difference int
	}{
		{
			Policy:     `namespace "production-*" { policy = "write" }`,
			NS:         "production-api",
			Difference: 3,
		},
		{
			Policy:     `namespace "production-*" { policy = "write" }`,
			NS:         "production-admin-api",
			Difference: 9,
		},
		{
			Policy:     `namespace "production-**" { policy = "write" }`,
			NS:         "production-admin-api",
			Difference: 9,
		},
		{
			Policy:     `namespace "*" { policy = "write" }`,
			NS:         "production-admin-api",
			Difference: 20,
		},
		{
			Policy:     `namespace "*admin*" { policy = "write" }`,
			NS:         "production-admin-api",
			Difference: 15,
		},
	}

	for _, tc := range tests {
		t.Run(tc.Policy, func(t *testing.T) {
			assert := assert.New(t)

			policy, err := Parse(tc.Policy)
			assert.NoError(err)
			assert.NotNil(policy.Namespaces)

			acl, err := NewACL(false, []*Policy{policy})
			assert.Nil(err)

			matches := findAllMatchingWildcards(acl.wildcardNamespaces, tc.NS)
			assert.Equal(tc.Difference, matches[0].difference)
		})
	}

}
