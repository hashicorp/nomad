package nomad

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/stretchr/testify/require"
)

func TestConsulPolicy_ParseConsulPolicy(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, text string, expPolicy *ConsulPolicy, expErr string) {
		policy, err := ParseConsulPolicy(text)
		if expErr != "" {
			require.EqualError(t, err, expErr)
			require.True(t, policy.IsEmpty())
		} else {
			require.NoError(t, err)
			require.Equal(t, expPolicy, policy)
		}
	}

	t.Run("service", func(t *testing.T) {
		text := `service "web" { policy = "read" }`
		exp := &ConsulPolicy{
			Services:        []*ConsulServiceRule{{Name: "web", Policy: "read"}},
			ServicePrefixes: []*ConsulServiceRule(nil),
		}
		try(t, text, exp, "")
	})

	t.Run("service_prefix", func(t *testing.T) {
		text := `service_prefix "data" { policy = "write" }`
		exp := &ConsulPolicy{
			Services:        []*ConsulServiceRule(nil),
			ServicePrefixes: []*ConsulServiceRule{{Name: "data", Policy: "write"}},
		}
		try(t, text, exp, "")
	})

	t.Run("empty", func(t *testing.T) {
		text := ``
		expErr := "consul policy contains no service rules"
		try(t, text, nil, expErr)
	})

	t.Run("malformed", func(t *testing.T) {
		text := `this is not valid HCL!`
		expErr := "failed to parse ACL policy: At 1:22: illegal char"
		try(t, text, nil, expErr)
	})
}

func TestConsulPolicy_IsEmpty(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, cp *ConsulPolicy, exp bool) {
		result := cp.IsEmpty()
		require.Equal(t, exp, result)
	}

	t.Run("nil", func(t *testing.T) {
		cp := (*ConsulPolicy)(nil)
		try(t, cp, true)
	})

	t.Run("empty slices", func(t *testing.T) {
		cp := &ConsulPolicy{
			Services:        []*ConsulServiceRule(nil),
			ServicePrefixes: []*ConsulServiceRule(nil),
		}
		try(t, cp, true)
	})

	t.Run("services nonempty", func(t *testing.T) {
		cp := &ConsulPolicy{
			Services: []*ConsulServiceRule{{Name: "example", Policy: "write"}},
		}
		try(t, cp, false)
	})

	t.Run("service_prefixes nonempty", func(t *testing.T) {
		cp := &ConsulPolicy{
			ServicePrefixes: []*ConsulServiceRule{{Name: "pre", Policy: "read"}},
		}
		try(t, cp, false)
	})
}

func TestConsulACLsAPI_allowsServiceWrite(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, task string, cp *ConsulPolicy, exp bool) {
		result := cp.allowsServiceWrite(task)
		require.Equal(t, exp, result)
	}

	makeCP := func(services [][2]string, prefixes [][2]string) *ConsulPolicy {
		serviceRules := make([]*ConsulServiceRule, 0, len(services))
		for _, service := range services {
			serviceRules = append(serviceRules, &ConsulServiceRule{Name: service[0], Policy: service[1]})
		}
		prefixRules := make([]*ConsulServiceRule, 0, len(prefixes))
		for _, prefix := range prefixes {
			prefixRules = append(prefixRules, &ConsulServiceRule{Name: prefix[0], Policy: prefix[1]})
		}
		return &ConsulPolicy{Services: serviceRules, ServicePrefixes: prefixRules}
	}

	t.Run("matching service policy write", func(t *testing.T) {
		try(t, "task1", makeCP(
			[][2]string{{"task1", "write"}},
			nil,
		), true)
	})

	t.Run("matching service policy read", func(t *testing.T) {
		try(t, "task1", makeCP(
			[][2]string{{"task1", "read"}},
			nil,
		), false)
	})

	t.Run("wildcard service policy write", func(t *testing.T) {
		try(t, "task1", makeCP(
			[][2]string{{"*", "write"}},
			nil,
		), true)
	})

	t.Run("wrong service policy write", func(t *testing.T) {
		try(t, "other1", makeCP(
			[][2]string{{"task1", "write"}},
			nil,
		), false)
	})

	t.Run("matching prefix policy write", func(t *testing.T) {
		try(t, "task-one", makeCP(
			nil,
			[][2]string{{"task-", "write"}},
		), true)
	})

	t.Run("matching prefix policy read", func(t *testing.T) {
		try(t, "task-one", makeCP(
			nil,
			[][2]string{{"task-", "read"}},
		), false)
	})

	t.Run("empty prefix policy write", func(t *testing.T) {
		try(t, "task-one", makeCP(
			nil,
			[][2]string{{"", "write"}},
		), true)
	})

	t.Run("late matching service", func(t *testing.T) {
		try(t, "task1", makeCP(
			[][2]string{{"task0", "write"}, {"task1", "write"}},
			nil,
		), true)
	})

	t.Run("late matching prefix", func(t *testing.T) {
		try(t, "task-one", makeCP(
			nil,
			[][2]string{{"foo-", "write"}, {"task-", "write"}},
		), true)
	})
}

func TestConsulACLsAPI_hasSufficientPolicy(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, task string, token *api.ACLToken, exp bool) {
		logger := testlog.HCLogger(t)
		cAPI := &consulACLsAPI{
			aclClient: consul.NewMockACLsAPI(logger),
			logger:    logger,
		}
		result, err := cAPI.canWriteService(task, token)
		require.NoError(t, err)
		require.Equal(t, exp, result)
	}

	t.Run("no useful policy or role", func(t *testing.T) {
		try(t, "service1", consul.ExampleOperatorToken0, false)
	})

	t.Run("working policy only", func(t *testing.T) {
		try(t, "service1", consul.ExampleOperatorToken1, true)
	})

	t.Run("working role only", func(t *testing.T) {
		try(t, "service1", consul.ExampleOperatorToken4, true)
	})
}

func TestConsulPolicy_allowKeystoreRead(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		require.False(t, new(ConsulPolicy).allowsKeystoreRead())
	})

	t.Run("services only", func(t *testing.T) {
		require.False(t, (&ConsulPolicy{
			Services: []*ConsulServiceRule{{
				Name:   "service1",
				Policy: "write",
			}},
		}).allowsKeystoreRead())
	})

	t.Run("kv any read", func(t *testing.T) {
		require.True(t, (&ConsulPolicy{
			KeyPrefixes: []*ConsulKeyRule{{
				Name:   "",
				Policy: "read",
			}},
		}).allowsKeystoreRead())
	})

	t.Run("kv any write", func(t *testing.T) {
		require.True(t, (&ConsulPolicy{
			KeyPrefixes: []*ConsulKeyRule{{
				Name:   "",
				Policy: "write",
			}},
		}).allowsKeystoreRead())
	})

	t.Run("kv limited read", func(t *testing.T) {
		require.False(t, (&ConsulPolicy{
			KeyPrefixes: []*ConsulKeyRule{{
				Name:   "foo/bar",
				Policy: "read",
			}},
		}).allowsKeystoreRead())
	})

	t.Run("kv limited write", func(t *testing.T) {
		require.False(t, (&ConsulPolicy{
			KeyPrefixes: []*ConsulKeyRule{{
				Name:   "foo/bar",
				Policy: "write",
			}},
		}).allowsKeystoreRead())
	})
}
