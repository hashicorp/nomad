// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
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

func TestConsulConfigsAPI_SetCE(t *testing.T) {
	ci.Parallel(t)

	try := func(t *testing.T, expect error, f func(ConsulConfigsAPI) error) {
		logger := testlog.HCLogger(t)
		configsAPI := consul.NewMockConfigsAPI(logger)
		configsAPI.SetError(expect)

		c := NewConsulConfigsAPI(configsAPI, logger)
		err := f(c) // set the config entry

		switch expect {
		case nil:
			require.NoError(t, err)
		default:
			require.Equal(t, expect, err)
		}
	}

	ctx := context.Background()

	// existing behavior is no set namespace
	consulNamespace := ""

	ingressCE := new(structs.ConsulIngressConfigEntry)
	t.Run("ingress ok", func(t *testing.T) {
		try(t, nil, func(c ConsulConfigsAPI) error {
			return c.SetIngressCE(ctx, consulNamespace, "ig", ingressCE)
		})
	})

	t.Run("ingress fail", func(t *testing.T) {
		try(t, errors.New("consul broke"), func(c ConsulConfigsAPI) error {
			return c.SetIngressCE(ctx, consulNamespace, "ig", ingressCE)
		})
	})

	terminatingCE := new(structs.ConsulTerminatingConfigEntry)
	t.Run("terminating ok", func(t *testing.T) {
		try(t, nil, func(c ConsulConfigsAPI) error {
			return c.SetTerminatingCE(ctx, consulNamespace, "tg", terminatingCE)
		})
	})

	t.Run("terminating fail", func(t *testing.T) {
		try(t, errors.New("consul broke"), func(c ConsulConfigsAPI) error {
			return c.SetTerminatingCE(ctx, consulNamespace, "tg", terminatingCE)
		})
	})

	// also mesh
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

func (m *mockConsulACLsAPI) CheckPermissions(context.Context, string, *structs.ConsulUsage, string) error {
	panic("not implemented yet")
}

func (m *mockConsulACLsAPI) CreateToken(context.Context, ServiceIdentityRequest) (*structs.SIToken, error) {
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
	ci.Parallel(t)

	try := func(t *testing.T, expErr error) {
		logger := testlog.HCLogger(t)
		aclAPI := consul.NewMockACLsAPI(logger)
		aclAPI.SetError(expErr)

		c := NewConsulACLsAPI(aclAPI, logger, nil)

		ctx := context.Background()
		sii := ServiceIdentityRequest{
			ConsulNamespace: "foo-namespace",
			AllocID:         uuid.Generate(),
			ClusterID:       uuid.Generate(),
			TaskName:        "my-task1-sidecar-proxy",
			TaskKind:        structs.NewTaskKind(structs.ConnectProxyPrefix, "my-service"),
		}

		token, err := c.CreateToken(ctx, sii)

		if expErr != nil {
			require.Equal(t, expErr, err)
			require.Nil(t, token)
		} else {
			require.NoError(t, err)
			require.Equal(t, "foo-namespace", token.ConsulNamespace)
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
	ci.Parallel(t)

	setup := func(t *testing.T, exp error) (context.Context, ConsulACLsAPI, *structs.SIToken) {
		logger := testlog.HCLogger(t)
		aclAPI := consul.NewMockACLsAPI(logger)

		c := NewConsulACLsAPI(aclAPI, logger, nil)

		ctx := context.Background()
		generated, err := c.CreateToken(ctx, ServiceIdentityRequest{
			ConsulNamespace: "foo-namespace",
			ClusterID:       uuid.Generate(),
			AllocID:         uuid.Generate(),
			TaskName:        "task1-sidecar-proxy",
			TaskKind:        structs.NewTaskKind(structs.ConnectProxyPrefix, "service1"),
		})
		require.NoError(t, err)

		// set the mock error after calling CreateToken for setting up
		aclAPI.SetError(exp)

		return context.Background(), c, generated
	}

	accessors := func(ids ...string) (result []*structs.SITokenAccessor) {
		for _, id := range ids {
			result = append(result, &structs.SITokenAccessor{
				AccessorID:      id,
				ConsulNamespace: "foo-namespace",
			})
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
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	aclAPI := consul.NewMockACLsAPI(logger)

	c := NewConsulACLsAPI(aclAPI, logger, nil)

	generated, err := c.CreateToken(context.Background(), ServiceIdentityRequest{
		ConsulNamespace: "foo-namespace",
		ClusterID:       uuid.Generate(),
		AllocID:         uuid.Generate(),
		TaskName:        "task1-sidecar-proxy",
		TaskKind:        structs.NewTaskKind(structs.ConnectProxyPrefix, "service1"),
	})
	require.NoError(t, err)

	// set the mock error after calling CreateToken for setting up
	aclAPI.SetError(nil)

	accessors := []*structs.SITokenAccessor{{
		ConsulNamespace: "foo-namespace",
		AccessorID:      generated.AccessorID,
	}}
	c.MarkForRevocation(accessors)
	require.Len(t, c.bgRetryRevocation, 1)
	require.Contains(t, c.bgRetryRevocation, accessors[0])
}

func TestConsulACLsAPI_bgRetryRevoke(t *testing.T) {
	ci.Parallel(t)

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
			ConsulNamespace: "foo-namespace",
			NodeID:          uuid.Generate(),
			AllocID:         uuid.Generate(),
			AccessorID:      accessorID,
			TaskName:        "task1",
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
			ConsulNamespace: "foo-namespace",
			NodeID:          uuid.Generate(),
			AllocID:         uuid.Generate(),
			AccessorID:      accessorID,
			TaskName:        "task1",
		})
		require.Empty(t, server.purgedAccessorIDs)
		c.bgRetryRevoke()
		require.Equal(t, 1, len(c.bgRetryRevocation)) // non-empty because purge failed
		require.Equal(t, accessorID, c.bgRetryRevocation[0].AccessorID)
	})
}

func TestConsulACLsAPI_Stop(t *testing.T) {
	ci.Parallel(t)

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
