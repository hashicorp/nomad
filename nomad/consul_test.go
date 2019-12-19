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
		try(t, "service1", consul.ExampleOperatorTokenID1, "")
	})

	t.Run("operator has service_prefix write", func(t *testing.T) {
		try(t, "foo-service1", consul.ExampleOperatorTokenID2, "")
	})

	t.Run("operator permissions insufficient", func(t *testing.T) {
		try(t, "service1", consul.ExampleOperatorTokenID3,
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
