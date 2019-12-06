package nomad

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
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

var _ ConsulACLsAPI = (*consulACLsAPI)(nil)
var _ ConsulACLsAPI = (*mockConsulACLsAPI)(nil)

type mockConsulACLsAPI struct {
	revokeRequests []string
}

func (m *mockConsulACLsAPI) CheckSIPolicy(_ context.Context, _, _ string) error {
	panic("not implemented yet")
}

func (m *mockConsulACLsAPI) CreateToken(_ context.Context, _ ServiceIdentityIndex) (*structs.SIToken, error) {
	panic("not implemented yet")
}

func (m *mockConsulACLsAPI) ListTokens() ([]string, error) {
	panic("not implemented yet")
}

func (m *mockConsulACLsAPI) RevokeTokens(_ context.Context, accessors []*structs.SITokenAccessor) error {
	for _, accessor := range accessors {
		m.revokeRequests = append(m.revokeRequests, accessor.AccessorID)
	}
	return nil
}

func TestConsulACLsAPI_CreateToken(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, expErr error) {
		logger := testlog.HCLogger(t)
		aclAPI := consul.NewMockACLsAPI(logger)
		aclAPI.SetError(expErr)

		c, err := NewConsulACLsAPI(aclAPI, logger)
		require.NoError(t, err)

		ctx := context.Background()
		sii := ServiceIdentityIndex{
			AllocID:   uuid.Generate(),
			ClusterID: uuid.Generate(),
			TaskName:  "my-task1",
		}

		token, err := c.CreateToken(ctx, sii)

		if expErr != nil {
			require.Equal(t, expErr, err)
			require.Nil(t, token)
		} else {
			require.NoError(t, err)
			require.Equal(t, "my-task1", token.TaskName)
			require.True(t, helper.IsUUID(token.AccessorID))
			require.True(t, helper.IsUUID(token.SecretID))
		}
	}

	t.Run("create token success", func(t *testing.T) {
		try(t, nil)
	})

	t.Run("create token error", func(t *testing.T) {
		try(t, errors.New("consul broke"))
	})
}

func TestConsulACLsAPI_RevokeTokens(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T, exp error) (context.Context, ConsulACLsAPI, *structs.SIToken) {
		logger := testlog.HCLogger(t)
		aclAPI := consul.NewMockACLsAPI(logger)

		c, err := NewConsulACLsAPI(aclAPI, logger)
		require.NoError(t, err)

		ctx := context.Background()
		generated, err := c.CreateToken(ctx, ServiceIdentityIndex{
			ClusterID: uuid.Generate(),
			AllocID:   uuid.Generate(),
			TaskName:  "task1",
		})
		require.NoError(t, err)

		// set the mock error after calling CreateToken for setting up
		aclAPI.SetError(exp)

		return context.Background(), c, generated
	}

	accessors := func(ids ...string) (result []*structs.SITokenAccessor) {
		for _, id := range ids {
			result = append(result, &structs.SITokenAccessor{AccessorID: id})
		}
		return
	}

	t.Run("revoke token success", func(t *testing.T) {
		ctx, c, token := setup(t, nil)
		err := c.RevokeTokens(ctx, accessors(token.AccessorID))
		require.NoError(t, err)
	})

	t.Run("revoke token non-existent", func(t *testing.T) {
		ctx, c, _ := setup(t, nil)
		err := c.RevokeTokens(ctx, accessors(uuid.Generate()))
		require.EqualError(t, err, "token does not exist")
	})

	t.Run("revoke token error", func(t *testing.T) {
		exp := errors.New("consul broke")
		ctx, c, token := setup(t, exp)
		err := c.RevokeTokens(ctx, accessors(token.AccessorID))
		require.EqualError(t, err, exp.Error())
	})
}

func TestConsulACLsAPI_CheckSIPolicy(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, service, token string, expErr string) {
		logger := testlog.HCLogger(t)
		aclAPI := consul.NewMockACLsAPI(logger)
		cAPI, err := NewConsulACLsAPI(aclAPI, logger)
		require.NoError(t, err)

		err = cAPI.CheckSIPolicy(context.Background(), service, token)
		if expErr != "" {
			require.EqualError(t, err, expErr)
		} else {
			require.NoError(t, err)
		}
	}

	t.Run("operator has service write", func(t *testing.T) {
		try(t, "service1", consul.ExampleOperatorToken1, "")
	})

	t.Run("operator has service_prefix write", func(t *testing.T) {
		try(t, "foo-service1", consul.ExampleOperatorToken2, "")
	})

	t.Run("operator permissions insufficient", func(t *testing.T) {
		try(t, "service1", consul.ExampleOperatorToken3,
			"permission denied for \"service1\"",
		)
	})

	t.Run("no token provided", func(t *testing.T) {
		try(t, "service1", "", "missing consul token")
	})

	t.Run("nonsense token provided", func(t *testing.T) {
		try(t, "service1", "f1682bde-1e71-90b1-9204-85d35467ba61",
			"unable to validate operator consul token: no such token",
		)
	})
}

func TestConsulACLsAPI_allowsServiceWrite(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, task string, cp *ConsulPolicy, exp bool) {
		cAPI := new(consulACLsAPI)
		result := cAPI.allowsServiceWrite(task, cp)
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
