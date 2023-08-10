// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

func TestConsulPolicy_ParseConsulPolicy(t *testing.T) {
	ci.Parallel(t)

	try := func(t *testing.T, text string, expPolicy *ConsulPolicy, expErr string) {
		policy, err := parseConsulPolicy(text)
		if expErr != "" {
			require.EqualError(t, err, expErr)
		} else {
			require.NoError(t, err)
			require.Equal(t, expPolicy, policy)
		}
	}

	t.Run("service", func(t *testing.T) {
		text := `service "web" { policy = "read" }`
		exp := &ConsulPolicy{
			Services: []*ConsulServiceRule{{Name: "web", Policy: "read"}},
		}
		try(t, text, exp, "")
	})

	t.Run("service_prefix", func(t *testing.T) {
		text := `service_prefix "data" { policy = "write" }`
		exp := &ConsulPolicy{
			ServicePrefixes: []*ConsulServiceRule{{Name: "data", Policy: "write"}},
		}
		try(t, text, exp, "")
	})

	t.Run("key_prefix", func(t *testing.T) {
		text := `key_prefix "keys" { policy = "read" }`
		exp := &ConsulPolicy{
			KeyPrefixes: []*ConsulKeyRule{{Name: "keys", Policy: "read"}},
		}
		try(t, text, exp, "")
	})

	t.Run("malformed", func(t *testing.T) {
		text := `this is not valid HCL!`
		expErr := "failed to parse ACL policy: At 1:22: illegal char"
		try(t, text, nil, expErr)
	})

	t.Run("multi-namespace", func(t *testing.T) {
		text := `
service_prefix "z" { policy = "write" }

namespace_prefix "b" {
  service_prefix "b" { policy = "write" }
  key_prefix "" { policy = "read" }
}

namespace_prefix "c" {
  service_prefix "c" { policy = "read" }
  key_prefix "" { policy = "read" }
}

namespace_prefix "" {
  key_prefix "shared/" { policy = "read" }
}

namespace "foo" {
  service "bar" { policy = "read" }
  service_prefix "foo-" { policy = "write" }
  key_prefix "" { policy = "read" }
}
`
		exp := &ConsulPolicy{
			ServicePrefixes: []*ConsulServiceRule{{Name: "z", Policy: "write"}},
			NamespacePrefixes: map[string]*ConsulPolicy{
				"b": {
					ServicePrefixes: []*ConsulServiceRule{{Name: "b", Policy: "write"}},
					KeyPrefixes:     []*ConsulKeyRule{{Name: "", Policy: "read"}},
				},
				"c": {
					ServicePrefixes: []*ConsulServiceRule{{Name: "c", Policy: "read"}},
					KeyPrefixes:     []*ConsulKeyRule{{Name: "", Policy: "read"}},
				},
				"": {
					KeyPrefixes: []*ConsulKeyRule{{Name: "shared/", Policy: "read"}},
				},
			},
			Namespaces: map[string]*ConsulPolicy{
				"foo": {
					Services:        []*ConsulServiceRule{{Name: "bar", Policy: "read"}},
					ServicePrefixes: []*ConsulServiceRule{{Name: "foo-", Policy: "write"}},
					KeyPrefixes:     []*ConsulKeyRule{{Name: "", Policy: "read"}},
				},
			},
		}
		try(t, text, exp, "")
	})
}

func TestConsulACLsAPI_allowsServiceWrite(t *testing.T) {
	ci.Parallel(t)

	try := func(t *testing.T, matches bool, namespace, task string, cp *ConsulPolicy, exp bool) {
		// If matches is false, the implication is that the consul acl token is in
		// the default namespace, otherwise prior validation would stop the request
		// before getting to policy checks. Only consul acl tokens in the default
		// namespace are allowed to have namespace_prefix blocks.
		result := cp.allowsServiceWrite(matches, namespace, task)
		require.Equal(t, exp, result)
	}

	// create a consul policy backed by service and/or service_prefix rules
	//
	// if namespace == "_", use the top level service/service_prefix rules, otherwise
	// set the rules as a namespace_prefix ruleset
	makeCP := func(namespace string, services [][2]string, prefixes [][2]string) *ConsulPolicy {
		serviceRules := make([]*ConsulServiceRule, 0, len(services))
		for _, service := range services {
			serviceRules = append(serviceRules, &ConsulServiceRule{Name: service[0], Policy: service[1]})
		}
		prefixRules := make([]*ConsulServiceRule, 0, len(prefixes))
		for _, prefix := range prefixes {
			prefixRules = append(prefixRules, &ConsulServiceRule{Name: prefix[0], Policy: prefix[1]})
		}

		if namespace == "_" {
			return &ConsulPolicy{Services: serviceRules, ServicePrefixes: prefixRules}
		}

		return &ConsulPolicy{
			Namespaces: map[string]*ConsulPolicy{
				namespace: {
					Services:        serviceRules,
					ServicePrefixes: prefixRules,
				},
			},
			NamespacePrefixes: map[string]*ConsulPolicy{
				namespace: {
					Services:        serviceRules,
					ServicePrefixes: prefixRules,
				},
			}}
	}

	t.Run("matching service policy write", func(t *testing.T) {
		rule := [][2]string{{"task1", "write"}}
		const task = "task1"
		t.Run("namespaces match", func(t *testing.T) {
			const matches = true
			try(t, matches, "default", task, makeCP("_", rule, nil), true)
			try(t, matches, "default", task, makeCP("default", rule, nil), true)
			try(t, matches, "apple", task, makeCP("_", rule, nil), true)
			try(t, matches, "apple", task, makeCP("apple", rule, nil), true)
			try(t, matches, "apple", task, makeCP("app", rule, nil), true)
			try(t, matches, "other", task, makeCP("", rule, nil), true)
			try(t, matches, "other", task, makeCP("apple", rule, nil), false)
		})
		t.Run("namespaces do not match", func(t *testing.T) {
			const matches = false
			try(t, matches, "apple", task, makeCP("_", rule, nil), false)
			try(t, matches, "apple", task, makeCP("apple", rule, nil), true)
			try(t, matches, "apple", task, makeCP("app", rule, nil), true)
			try(t, matches, "other", task, makeCP("", rule, nil), true)
			try(t, matches, "other", task, makeCP("apple", rule, nil), false)
		})
	})

	t.Run("matching service policy read", func(t *testing.T) {
		rule := [][2]string{{"task1", "read"}}
		const task = "task1"
		t.Run("namespaces match", func(t *testing.T) {
			const matches = true
			try(t, matches, "default", task, makeCP("_", rule, nil), false)
			try(t, matches, "default", task, makeCP("default", rule, nil), false)
			try(t, matches, "apple", task, makeCP("_", rule, nil), false)
			try(t, matches, "apple", task, makeCP("apple", rule, nil), false)
			try(t, matches, "apple", task, makeCP("app", rule, nil), false)
			try(t, matches, "other", task, makeCP("", rule, nil), false)
			try(t, matches, "other", task, makeCP("apple", rule, nil), false)
		})
		t.Run("namespaces do not match", func(t *testing.T) {
			const matches = false
			try(t, matches, "apple", task, makeCP("_", rule, nil), false)
			try(t, matches, "apple", task, makeCP("apple", rule, nil), false)
			try(t, matches, "apple", task, makeCP("app", rule, nil), false)
			try(t, matches, "other", task, makeCP("", rule, nil), false)
			try(t, matches, "other", task, makeCP("apple", rule, nil), false)
		})
	})

	t.Run("wildcard service policy write", func(t *testing.T) {
		rule := [][2]string{{"*", "write"}}
		const task = "task1"
		t.Run("namespaces match", func(t *testing.T) {
			const matches = true
			try(t, matches, "default", task, makeCP("_", rule, nil), true)
			try(t, matches, "default", task, makeCP("default", rule, nil), true)
			try(t, matches, "apple", task, makeCP("_", rule, nil), true)
			try(t, matches, "apple", task, makeCP("app", rule, nil), true)
			try(t, matches, "other", task, makeCP("", rule, nil), true)
			try(t, matches, "other", task, makeCP("apple", rule, nil), false)
		})
		t.Run("namespaces do not match", func(t *testing.T) {
			const matches = false
			try(t, matches, "apple", task, makeCP("_", rule, nil), false)
			try(t, matches, "apple", task, makeCP("app", rule, nil), true)
			try(t, matches, "other", task, makeCP("", rule, nil), true)
			try(t, matches, "other", task, makeCP("apple", rule, nil), false)
		})
	})

	t.Run("wrong service policy write", func(t *testing.T) {
		rule := [][2]string{{"task1", "write"}}
		const task = "other1"
		t.Run("namespaces match", func(t *testing.T) {
			const matches = true
			try(t, matches, "default", task, makeCP("_", rule, nil), false)
			try(t, matches, "default", task, makeCP("default", rule, nil), false)
			try(t, matches, "apple", task, makeCP("_", rule, nil), false)
			try(t, matches, "apple", task, makeCP("app", rule, nil), false)
			try(t, matches, "other", task, makeCP("", rule, nil), false)
			try(t, matches, "other", task, makeCP("apple", rule, nil), false)
		})
		t.Run("namespaces do not match", func(t *testing.T) {
			const matches = true
			try(t, matches, "apple", task, makeCP("_", rule, nil), false)
			try(t, matches, "apple", task, makeCP("app", rule, nil), false)
			try(t, matches, "other", task, makeCP("", rule, nil), false)
			try(t, matches, "other", task, makeCP("apple", rule, nil), false)
		})
	})

	t.Run("matching prefix policy write", func(t *testing.T) {
		rule := [][2]string{{"task-", "write"}}
		const task = "task-one"
		t.Run("namespaces match", func(t *testing.T) {
			const matches = true
			try(t, matches, "default", task, makeCP("_", nil, rule), true)
			try(t, matches, "default", task, makeCP("default", nil, rule), true)
			try(t, matches, "apple", task, makeCP("_", nil, rule), true)
			try(t, matches, "apple", task, makeCP("app", nil, rule), true)
			try(t, matches, "other", task, makeCP("", nil, rule), true)
			try(t, matches, "other", task, makeCP("apple", nil, rule), false)
		})
		t.Run("namespaces do not match", func(t *testing.T) {
			const matches = false
			try(t, matches, "apple", task, makeCP("_", nil, rule), false)
			try(t, matches, "apple", task, makeCP("app", nil, rule), true)
			try(t, matches, "other", task, makeCP("", nil, rule), true)
			try(t, matches, "other", task, makeCP("apple", nil, rule), false)
		})
	})

	t.Run("matching prefix policy read", func(t *testing.T) {
		rule := [][2]string{{"task-", "read"}}
		const task = "task-one"
		t.Run("namespaces match", func(t *testing.T) {
			const matches = true
			try(t, matches, "default", task, makeCP("_", nil, rule), false)
			try(t, matches, "default", task, makeCP("default", nil, rule), false)
			try(t, matches, "apple", task, makeCP("_", nil, rule), false)
			try(t, matches, "apple", task, makeCP("app", nil, rule), false)
			try(t, matches, "other", task, makeCP("", nil, rule), false)
			try(t, matches, "other", task, makeCP("apple", nil, rule), false)
		})
		t.Run("namespaces do not match", func(t *testing.T) {
			const matches = false
			try(t, matches, "apple", task, makeCP("_", nil, rule), false)
			try(t, matches, "apple", task, makeCP("app", nil, rule), false)
			try(t, matches, "other", task, makeCP("", nil, rule), false)
			try(t, matches, "other", task, makeCP("apple", nil, rule), false)
		})
	})

	t.Run("empty prefix policy write", func(t *testing.T) {
		rule := [][2]string{{"", "write"}}
		const task = "task-one"
		t.Run("namespaces match", func(t *testing.T) {
			const matches = true
			try(t, matches, "default", task, makeCP("_", nil, rule), true)
			try(t, matches, "default", task, makeCP("default", nil, rule), true)
			try(t, matches, "apple", task, makeCP("_", nil, rule), true)
			try(t, matches, "apple", task, makeCP("app", nil, rule), true)
			try(t, matches, "other", task, makeCP("", nil, rule), true)
			try(t, matches, "other", task, makeCP("apple", nil, rule), false)
		})
		t.Run("namespaces do not match", func(t *testing.T) {
			const matches = false
			try(t, matches, "apple", task, makeCP("_", nil, rule), false)
			try(t, matches, "apple", task, makeCP("app", nil, rule), true)
			try(t, matches, "other", task, makeCP("", nil, rule), true)
			try(t, matches, "other", task, makeCP("apple", nil, rule), false)
		})
	})

	t.Run("late matching service", func(t *testing.T) {
		rule := [][2]string{{"task0", "write"}, {"task1", "write"}}
		const task = "task1"
		t.Run("namespaces match", func(t *testing.T) {
			const matches = true
			try(t, matches, "default", task, makeCP("_", rule, nil), true)
			try(t, matches, "default", task, makeCP("default", rule, nil), true)
			try(t, matches, "apple", task, makeCP("_", rule, nil), true)
			try(t, matches, "apple", task, makeCP("app", rule, nil), true)
			try(t, matches, "other", task, makeCP("", rule, nil), true)
			try(t, matches, "other", task, makeCP("apple", rule, nil), false)
		})
		t.Run("namespaces do not match", func(t *testing.T) {
			const matches = false
			try(t, matches, "apple", task, makeCP("_", rule, nil), false)
			try(t, matches, "apple", task, makeCP("app", rule, nil), true)
			try(t, matches, "other", task, makeCP("", rule, nil), true)
			try(t, matches, "other", task, makeCP("apple", rule, nil), false)
		})
	})

	t.Run("late matching prefix", func(t *testing.T) {
		rule := [][2]string{{"foo-", "write"}, {"task-", "write"}}
		const task = "task-one"
		t.Run("namespaces match", func(t *testing.T) {
			const matches = true
			try(t, matches, "default", task, makeCP("_", nil, rule), true)
			try(t, matches, "default", task, makeCP("default", nil, rule), true)
			try(t, matches, "apple", task, makeCP("_", nil, rule), true)
			try(t, matches, "apple", task, makeCP("app", nil, rule), true)
			try(t, matches, "other", task, makeCP("", nil, rule), true)
			try(t, matches, "other", task, makeCP("apple", nil, rule), false)
		})
		t.Run("namespaces do not match", func(t *testing.T) {
			const matches = false
			try(t, matches, "apple", task, makeCP("_", nil, rule), false)
			try(t, matches, "apple", task, makeCP("app", nil, rule), true)
			try(t, matches, "other", task, makeCP("", nil, rule), true)
			try(t, matches, "other", task, makeCP("apple", nil, rule), false)
		})
	})
}

func TestConsulPolicy_isManagementToken(t *testing.T) {
	ci.Parallel(t)

	aclsAPI := new(consulACLsAPI)

	t.Run("nil", func(t *testing.T) {
		token := (*api.ACLToken)(nil)
		result := aclsAPI.isManagementToken(token)
		require.False(t, result)
	})

	t.Run("no policies", func(t *testing.T) {
		token := &api.ACLToken{
			Policies: []*api.ACLTokenPolicyLink{},
		}
		result := aclsAPI.isManagementToken(token)
		require.False(t, result)
	})

	t.Run("management policy", func(t *testing.T) {
		token := &api.ACLToken{
			Policies: []*api.ACLTokenPolicyLink{{
				ID: consulGlobalManagementPolicyID,
			}},
		}
		result := aclsAPI.isManagementToken(token)
		require.True(t, result)
	})

	t.Run("other policy", func(t *testing.T) {
		token := &api.ACLToken{
			Policies: []*api.ACLTokenPolicyLink{{
				ID: uuid.Generate(),
			}},
		}
		result := aclsAPI.isManagementToken(token)
		require.False(t, result)
	})

	t.Run("mixed policies", func(t *testing.T) {
		token := &api.ACLToken{
			Policies: []*api.ACLTokenPolicyLink{{
				ID: uuid.Generate(),
			}, {
				ID: consulGlobalManagementPolicyID,
			}, {
				ID: uuid.Generate(),
			}},
		}
		result := aclsAPI.isManagementToken(token)
		require.True(t, result)
	})
}

func TestConsulPolicy_namespaceCheck(t *testing.T) {
	ci.Parallel(t)

	withoutNS := &api.ACLToken{Namespace: ""}
	withDefault := &api.ACLToken{Namespace: "default"}
	withOther := &api.ACLToken{Namespace: "other"}

	// ACLs not enabled

	t.Run("acl:disable ns:unset", func(t *testing.T) {
		err := namespaceCheck("", withoutNS)
		require.NoError(t, err)
	})

	t.Run("acl:disable ns:default", func(t *testing.T) {
		err := namespaceCheck("default", withoutNS)
		require.EqualError(t, err, `consul ACL token cannot use namespace "default"`)
	})

	t.Run("acl:disable ns:other", func(t *testing.T) {
		err := namespaceCheck("other", withoutNS)
		require.EqualError(t, err, `consul ACL token cannot use namespace "other"`)
	})

	// ACLs with "default" token

	t.Run("acl:enable token:default ns:unset", func(t *testing.T) {
		// the bypass case where a legacy job (with no namespace set) should work
		// with the a token in the "default" consul namespace
		err := namespaceCheck("", withDefault)
		require.NoError(t, err)
	})

	t.Run("acl:enable token:default ns:default", func(t *testing.T) {
		err := namespaceCheck("default", withDefault)
		require.NoError(t, err)
	})

	t.Run("acl:enable token:default ns:other", func(t *testing.T) {
		// the bypass case where a default token could have namespace_prefix
		// blocks
		err := namespaceCheck("other", withDefault)
		require.NoError(t, err)
	})

	// ACLs with non-"default" token

	t.Run("acl:enable token:other ns:unset", func(t *testing.T) {
		err := namespaceCheck("", withOther)
		require.EqualError(t, err, `consul ACL token requires using namespace "other"`)
	})

	t.Run("acl:enable token:other ns:default", func(t *testing.T) {
		err := namespaceCheck("default", withOther)
		require.EqualError(t, err, `consul ACL token cannot use namespace "default"`)
	})

	t.Run("acl:enable token:other ns:other", func(t *testing.T) {
		err := namespaceCheck("other", withOther)
		require.NoError(t, err)
	})
}

func TestConsulPolicy_allowKeystoreRead(t *testing.T) {
	ci.Parallel(t)

	t.Run("empty", func(t *testing.T) {
		require.False(t, new(ConsulPolicy).allowsKeystoreRead(true, "default"))
	})

	t.Run("services only", func(t *testing.T) {
		policy := &ConsulPolicy{
			Services: []*ConsulServiceRule{{
				Name:   "service1",
				Policy: "write",
			}},
		}
		require.False(t, policy.allowsKeystoreRead(true, "default"))
		require.False(t, policy.allowsKeystoreRead(false, "apple"))
	})

	// using top-level key_prefix block

	t.Run("kv any read", func(t *testing.T) {
		policy := &ConsulPolicy{
			KeyPrefixes: []*ConsulKeyRule{{
				Name:   "",
				Policy: "read",
			}},
		}
		require.True(t, policy.allowsKeystoreRead(true, "default"))
		require.False(t, policy.allowsKeystoreRead(false, "apple"))
	})

	t.Run("kv any write", func(t *testing.T) {
		policy := &ConsulPolicy{
			KeyPrefixes: []*ConsulKeyRule{{
				Name:   "",
				Policy: "write",
			}},
		}
		require.True(t, policy.allowsKeystoreRead(true, "default"))
		require.False(t, policy.allowsKeystoreRead(false, "apple"))
	})

	t.Run("kv limited read", func(t *testing.T) {
		policy := &ConsulPolicy{
			KeyPrefixes: []*ConsulKeyRule{{
				Name:   "foo/bar",
				Policy: "read",
			}},
		}
		require.False(t, policy.allowsKeystoreRead(true, "default"))
		require.False(t, policy.allowsKeystoreRead(false, "apple"))
	})

	t.Run("kv limited write", func(t *testing.T) {
		policy := &ConsulPolicy{
			KeyPrefixes: []*ConsulKeyRule{{
				Name:   "foo/bar",
				Policy: "write",
			}},
		}
		require.False(t, policy.allowsKeystoreRead(true, "default"))
		require.False(t, policy.allowsKeystoreRead(false, "apple"))
	})

	// using namespace_prefix block

	t.Run("kv wild namespace prefix any read", func(t *testing.T) {
		policy := &ConsulPolicy{
			NamespacePrefixes: map[string]*ConsulPolicy{
				"": {
					KeyPrefixes: []*ConsulKeyRule{{
						Name:   "",
						Policy: "read",
					}},
				},
			},
		}
		require.True(t, policy.allowsKeystoreRead(true, "default"))
		require.True(t, policy.allowsKeystoreRead(false, "apple"))
	})

	t.Run("kv apple namespace prefix any read", func(t *testing.T) {
		policy := &ConsulPolicy{
			NamespacePrefixes: map[string]*ConsulPolicy{
				"apple": {
					KeyPrefixes: []*ConsulKeyRule{{
						Name:   "",
						Policy: "read",
					}},
				},
			},
		}
		require.False(t, policy.allowsKeystoreRead(true, "default"))
		require.True(t, policy.allowsKeystoreRead(false, "apple"))
	})

	t.Run("kv matching namespace prefix any read", func(t *testing.T) {
		policy := &ConsulPolicy{
			NamespacePrefixes: map[string]*ConsulPolicy{
				"app": {
					KeyPrefixes: []*ConsulKeyRule{{
						Name:   "",
						Policy: "read",
					}},
				},
			},
		}
		require.False(t, policy.allowsKeystoreRead(true, "default"))
		require.True(t, policy.allowsKeystoreRead(false, "apple"))
	})

	t.Run("kv other namespace prefix any read", func(t *testing.T) {
		policy := &ConsulPolicy{
			NamespacePrefixes: map[string]*ConsulPolicy{
				"other": {
					KeyPrefixes: []*ConsulKeyRule{{
						Name:   "",
						Policy: "read",
					}},
				},
			},
		}
		require.False(t, policy.allowsKeystoreRead(true, "default"))
		require.False(t, policy.allowsKeystoreRead(false, "apple"))
	})

	// using namespace block

	t.Run("kv match namespace any read", func(t *testing.T) {
		policy := &ConsulPolicy{
			Namespaces: map[string]*ConsulPolicy{
				"apple": {
					KeyPrefixes: []*ConsulKeyRule{{
						Name:   "",
						Policy: "read",
					}},
				},
			},
		}
		require.False(t, policy.allowsKeystoreRead(true, "default"))
		require.True(t, policy.allowsKeystoreRead(true, "apple"))
	})

	t.Run("kv mismatch namespace any read", func(t *testing.T) {
		policy := &ConsulPolicy{
			Namespaces: map[string]*ConsulPolicy{
				"other": {
					KeyPrefixes: []*ConsulKeyRule{{
						Name:   "",
						Policy: "read",
					}},
				},
			},
		}
		require.False(t, policy.allowsKeystoreRead(true, "default"))
		require.False(t, policy.allowsKeystoreRead(true, "apple"))
	})

	t.Run("kv matching namespace prefix any read", func(t *testing.T) {
		policy := &ConsulPolicy{
			Namespaces: map[string]*ConsulPolicy{
				"apple": {
					KeyPrefixes: []*ConsulKeyRule{{
						Name:   "",
						Policy: "read",
					}},
				},
			},
		}
		require.False(t, policy.allowsKeystoreRead(false, "default"))
		require.True(t, policy.allowsKeystoreRead(false, "apple"))
	})

	t.Run("kv mismatch namespace prefix any read", func(t *testing.T) {
		policy := &ConsulPolicy{
			Namespaces: map[string]*ConsulPolicy{
				"other": {
					KeyPrefixes: []*ConsulKeyRule{{
						Name:   "",
						Policy: "read",
					}},
				},
			},
		}
		require.False(t, policy.allowsKeystoreRead(true, "default"))
		require.False(t, policy.allowsKeystoreRead(true, "apple"))
	})
}
