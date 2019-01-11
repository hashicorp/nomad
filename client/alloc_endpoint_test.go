package client

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/mock"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestAllocations_GarbageCollectAll(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client, cleanup := TestClient(t, nil)
	defer cleanup()

	req := &nstructs.NodeSpecificRequest{}
	var resp nstructs.GenericResponse
	require.Nil(client.ClientRPC("Allocations.GarbageCollectAll", &req, &resp))
}

func TestAllocations_GarbageCollectAll_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	server, addr, root := testACLServer(t, nil)
	defer server.Shutdown()

	client, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.ACLEnabled = true
	})
	defer cleanup()

	// Try request without a token and expect failure
	{
		req := &nstructs.NodeSpecificRequest{}
		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollectAll", &req, &resp)
		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = token.SecretID

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollectAll", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1007, "valid", mock.NodePolicy(acl.PolicyWrite))
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = token.SecretID
		var resp nstructs.GenericResponse
		require.Nil(client.ClientRPC("Allocations.GarbageCollectAll", &req, &resp))
	}

	// Try request with a management token
	{
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = root.SecretID
		var resp nstructs.GenericResponse
		require.Nil(client.ClientRPC("Allocations.GarbageCollectAll", &req, &resp))
	}
}

func TestAllocations_GarbageCollect(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client, cleanup := TestClient(t, func(c *config.Config) {
		c.GCDiskUsageThreshold = 100.0
	})
	defer cleanup()

	a := mock.Alloc()
	a.Job.TaskGroups[0].Tasks[0].Driver = "mock_driver"
	a.Job.TaskGroups[0].RestartPolicy = &nstructs.RestartPolicy{
		Attempts: 0,
		Mode:     nstructs.RestartPolicyModeFail,
	}
	a.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10ms",
	}
	require.Nil(client.addAlloc(a, ""))

	// Try with bad alloc
	req := &nstructs.AllocSpecificRequest{}
	var resp nstructs.GenericResponse
	err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp)
	require.NotNil(err)

	// Try with good alloc
	req.AllocID = a.ID
	testutil.WaitForResult(func() (bool, error) {
		// Check if has been removed first
		if ar, ok := client.allocs[a.ID]; !ok || ar.IsDestroyed() {
			return true, nil
		}

		var resp2 nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp2)
		return err == nil, err
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocations_GarbageCollect_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	server, addr, root := testACLServer(t, nil)
	defer server.Shutdown()

	client, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.ACLEnabled = true
	})
	defer cleanup()

	// Try request without a token and expect failure
	{
		req := &nstructs.AllocSpecificRequest{}
		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp)
		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
		req := &nstructs.AllocSpecificRequest{}
		req.AuthToken = token.SecretID

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "test-valid",
			mock.NamespacePolicy(nstructs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob}))
		req := &nstructs.AllocSpecificRequest{}
		req.AuthToken = token.SecretID
		req.Namespace = nstructs.DefaultNamespace

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp)
		require.True(nstructs.IsErrUnknownAllocation(err))
	}

	// Try request with a management token
	{
		req := &nstructs.AllocSpecificRequest{}
		req.AuthToken = root.SecretID

		var resp nstructs.GenericResponse
		err := client.ClientRPC("Allocations.GarbageCollect", &req, &resp)
		require.True(nstructs.IsErrUnknownAllocation(err))
	}
}

func TestAllocations_Stats(t *testing.T) {
	t.Skip("missing exec driver plugin implementation")
	t.Parallel()
	require := require.New(t)
	client, cleanup := TestClient(t, nil)
	defer cleanup()

	a := mock.Alloc()
	require.Nil(client.addAlloc(a, ""))

	// Try with bad alloc
	req := &cstructs.AllocStatsRequest{}
	var resp cstructs.AllocStatsResponse
	err := client.ClientRPC("Allocations.Stats", &req, &resp)
	require.NotNil(err)

	// Try with good alloc
	req.AllocID = a.ID
	testutil.WaitForResult(func() (bool, error) {
		var resp2 cstructs.AllocStatsResponse
		err := client.ClientRPC("Allocations.Stats", &req, &resp2)
		if err != nil {
			return false, err
		}
		if resp2.Stats == nil {
			return false, fmt.Errorf("invalid stats object")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestAllocations_Stats_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	server, addr, root := testACLServer(t, nil)
	defer server.Shutdown()

	client, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.ACLEnabled = true
	})
	defer cleanup()

	// Try request without a token and expect failure
	{
		req := &cstructs.AllocStatsRequest{}
		var resp cstructs.AllocStatsResponse
		err := client.ClientRPC("Allocations.Stats", &req, &resp)
		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
		req := &cstructs.AllocStatsRequest{}
		req.AuthToken = token.SecretID

		var resp cstructs.AllocStatsResponse
		err := client.ClientRPC("Allocations.Stats", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "test-valid",
			mock.NamespacePolicy(nstructs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
		req := &cstructs.AllocStatsRequest{}
		req.AuthToken = token.SecretID
		req.Namespace = nstructs.DefaultNamespace

		var resp cstructs.AllocStatsResponse
		err := client.ClientRPC("Allocations.Stats", &req, &resp)
		require.True(nstructs.IsErrUnknownAllocation(err))
	}

	// Try request with a management token
	{
		req := &cstructs.AllocStatsRequest{}
		req.AuthToken = root.SecretID

		var resp cstructs.AllocStatsResponse
		err := client.ClientRPC("Allocations.Stats", &req, &resp)
		require.True(nstructs.IsErrUnknownAllocation(err))
	}
}
