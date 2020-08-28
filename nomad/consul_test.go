package nomad

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

var _ ConsulACLsAPI = (*consulACLsAPI)(nil)
var _ ConsulACLsAPI = (*mockConsulACLsAPI)(nil)
var _ ConsulConfigsAPI = (*consulConfigsAPI)(nil)

func TestConsulConfigsAPI_SetIngressGatewayConfigEntry(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, expErr error) {
		logger := testlog.HCLogger(t)
		configsAPI := consul.NewMockConfigsAPI(logger) // agent
		configsAPI.SetError(expErr)

		c := NewConsulConfigsAPI(configsAPI, logger)

		ctx := context.Background()
		err := c.SetIngressGatewayConfigEntry(ctx, "service1", &structs.ConsulIngressConfigEntry{
			TLS:       nil,
			Listeners: nil,
		})

		if expErr != nil {
			require.Equal(t, expErr, err)
		} else {
			require.NoError(t, err)
		}
	}

	t.Run("set ingress CE success", func(t *testing.T) {
		try(t, nil)
	})

	t.Run("set ingress CE failure", func(t *testing.T) {
		try(t, errors.New("consul broke"))
	})
}

type revokeRequest struct {
	accessorID string
	committed  bool
}

type mockConsulACLsAPI struct {
	lock           sync.Mutex
	revokeRequests []revokeRequest
	stopped        bool
}

func (m *mockConsulACLsAPI) CheckSIPolicy(_ context.Context, _, _ string) error {
	panic("not implemented yet")
}

func (m *mockConsulACLsAPI) CreateToken(_ context.Context, _ ServiceIdentityRequest) (*structs.SIToken, error) {
	panic("not implemented yet")
}

func (m *mockConsulACLsAPI) ListTokens() ([]string, error) {
	panic("not implemented yet")
}

func (m *mockConsulACLsAPI) Stop() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.stopped = true
}

type mockPurgingServer struct {
	purgedAccessorIDs []string
	failure           error
}

func (mps *mockPurgingServer) purgeFunc(accessors []*structs.SITokenAccessor) error {
	if mps.failure != nil {
		return mps.failure
	}

	for _, accessor := range accessors {
		mps.purgedAccessorIDs = append(mps.purgedAccessorIDs, accessor.AccessorID)
	}
	return nil
}

func (m *mockConsulACLsAPI) RevokeTokens(_ context.Context, accessors []*structs.SITokenAccessor, committed bool) bool {
	return m.storeForRevocation(accessors, committed)
}

func (m *mockConsulACLsAPI) MarkForRevocation(accessors []*structs.SITokenAccessor) {
	m.storeForRevocation(accessors, true)
}

func (m *mockConsulACLsAPI) storeForRevocation(accessors []*structs.SITokenAccessor, committed bool) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	for _, accessor := range accessors {
		m.revokeRequests = append(m.revokeRequests, revokeRequest{
			accessorID: accessor.AccessorID,
			committed:  committed,
		})
	}
	return false
}

func TestConsulACLsAPI_CreateToken(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, expErr error) {
		logger := testlog.HCLogger(t)
		aclAPI := consul.NewMockACLsAPI(logger)
		aclAPI.SetError(expErr)

		c := NewConsulACLsAPI(aclAPI, logger, nil)

		ctx := context.Background()
		sii := ServiceIdentityRequest{
			AllocID:   uuid.Generate(),
			ClusterID: uuid.Generate(),
			TaskName:  "my-task1-sidecar-proxy",
			TaskKind:  structs.NewTaskKind(structs.ConnectProxyPrefix, "my-service"),
		}

		token, err := c.CreateToken(ctx, sii)

		if expErr != nil {
			require.Equal(t, expErr, err)
			require.Nil(t, token)
		} else {
			require.NoError(t, err)
			require.Equal(t, "my-task1-sidecar-proxy", token.TaskName)
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

		c := NewConsulACLsAPI(aclAPI, logger, nil)

		ctx := context.Background()
		generated, err := c.CreateToken(ctx, ServiceIdentityRequest{
			ClusterID: uuid.Generate(),
			AllocID:   uuid.Generate(),
			TaskName:  "task1-sidecar-proxy",
			TaskKind:  structs.NewTaskKind(structs.ConnectProxyPrefix, "service1"),
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
		retryLater := c.RevokeTokens(ctx, accessors(token.AccessorID), false)
		require.False(t, retryLater)
	})

	t.Run("revoke token non-existent", func(t *testing.T) {
		ctx, c, _ := setup(t, nil)
		retryLater := c.RevokeTokens(ctx, accessors(uuid.Generate()), false)
		require.False(t, retryLater)
	})

	t.Run("revoke token error", func(t *testing.T) {
		exp := errors.New("consul broke")
		ctx, c, token := setup(t, exp)
		retryLater := c.RevokeTokens(ctx, accessors(token.AccessorID), false)
		require.True(t, retryLater)
	})
}

func TestConsulACLsAPI_MarkForRevocation(t *testing.T) {
	t.Parallel()

	logger := testlog.HCLogger(t)
	aclAPI := consul.NewMockACLsAPI(logger)

	c := NewConsulACLsAPI(aclAPI, logger, nil)

	generated, err := c.CreateToken(context.Background(), ServiceIdentityRequest{
		ClusterID: uuid.Generate(),
		AllocID:   uuid.Generate(),
		TaskName:  "task1-sidecar-proxy",
		TaskKind:  structs.NewTaskKind(structs.ConnectProxyPrefix, "service1"),
	})
	require.NoError(t, err)

	// set the mock error after calling CreateToken for setting up
	aclAPI.SetError(nil)

	accessors := []*structs.SITokenAccessor{{AccessorID: generated.AccessorID}}
	c.MarkForRevocation(accessors)
	require.Len(t, c.bgRetryRevocation, 1)
	require.Contains(t, c.bgRetryRevocation, accessors[0])
}

func TestConsulACLsAPI_bgRetryRevoke(t *testing.T) {
	t.Parallel()

	// manually create so the bg daemon does not run, letting us explicitly
	// call and test bgRetryRevoke
	setup := func(t *testing.T) (*consulACLsAPI, *mockPurgingServer) {
		logger := testlog.HCLogger(t)
		aclAPI := consul.NewMockACLsAPI(logger)
		server := new(mockPurgingServer)
		shortWait := rate.Limit(1 * time.Millisecond)

		return &consulACLsAPI{
			aclClient: aclAPI,
			purgeFunc: server.purgeFunc,
			limiter:   rate.NewLimiter(shortWait, int(shortWait)),
			stopC:     make(chan struct{}),
			logger:    logger,
		}, server
	}

	t.Run("retry revoke no items", func(t *testing.T) {
		c, server := setup(t)
		c.bgRetryRevoke()
		require.Empty(t, server)
	})

	t.Run("retry revoke success", func(t *testing.T) {
		c, server := setup(t)
		accessorID := uuid.Generate()
		c.bgRetryRevocation = append(c.bgRetryRevocation, &structs.SITokenAccessor{
			NodeID:     uuid.Generate(),
			AllocID:    uuid.Generate(),
			AccessorID: accessorID,
			TaskName:   "task1",
		})
		require.Empty(t, server.purgedAccessorIDs)
		c.bgRetryRevoke()
		require.Equal(t, 1, len(server.purgedAccessorIDs))
		require.Equal(t, accessorID, server.purgedAccessorIDs[0])
		require.Empty(t, c.bgRetryRevocation) // should be empty now
	})

	t.Run("retry revoke failure", func(t *testing.T) {
		c, server := setup(t)
		server.failure = errors.New("revocation fail")
		accessorID := uuid.Generate()
		c.bgRetryRevocation = append(c.bgRetryRevocation, &structs.SITokenAccessor{
			NodeID:     uuid.Generate(),
			AllocID:    uuid.Generate(),
			AccessorID: accessorID,
			TaskName:   "task1",
		})
		require.Empty(t, server.purgedAccessorIDs)
		c.bgRetryRevoke()
		require.Equal(t, 1, len(c.bgRetryRevocation)) // non-empty because purge failed
		require.Equal(t, accessorID, c.bgRetryRevocation[0].AccessorID)
	})
}

func TestConsulACLsAPI_Stop(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) *consulACLsAPI {
		logger := testlog.HCLogger(t)
		return NewConsulACLsAPI(nil, logger, nil)
	}

	c := setup(t)
	c.Stop()
	_, err := c.CreateToken(context.Background(), ServiceIdentityRequest{
		ClusterID: "",
		AllocID:   "",
		TaskName:  "",
	})
	require.Error(t, err)
}

func TestConsulACLsAPI_CheckSIPolicy(t *testing.T) {
	t.Parallel()

	try := func(t *testing.T, service, token string, expErr string) {
		logger := testlog.HCLogger(t)
		aclAPI := consul.NewMockACLsAPI(logger)
		cAPI := NewConsulACLsAPI(aclAPI, logger, nil)

		err := cAPI.CheckSIPolicy(context.Background(), service, token)
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
