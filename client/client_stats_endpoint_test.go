package client

import (
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/mock"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestClientStats_Stats(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client := TestClient(t, nil)

	req := &nstructs.NodeSpecificRequest{}
	var resp structs.ClientStatsResponse
	require.Nil(client.ClientRPC("ClientStats.Stats", &req, &resp))
	require.NotNil(resp.HostStats)
	require.NotNil(resp.HostStats.AllocDirStats)
	require.NotZero(resp.HostStats.Uptime)
}

func TestClientStats_Stats_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	server, addr, root := testACLServer(t, nil)
	defer server.Shutdown()

	client := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.ACLEnabled = true
	})
	defer client.Shutdown()

	// Try request without a token and expect failure
	{
		req := &nstructs.NodeSpecificRequest{}
		var resp structs.ClientStatsResponse
		err := client.ClientRPC("ClientStats.Stats", &req, &resp)
		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = token.SecretID

		var resp structs.ClientStatsResponse
		err := client.ClientRPC("ClientStats.Stats", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1007, "valid", mock.NodePolicy(acl.PolicyRead))
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = token.SecretID

		var resp structs.ClientStatsResponse
		err := client.ClientRPC("ClientStats.Stats", &req, &resp)

		require.Nil(err)
		require.NotNil(resp.HostStats)
	}

	// Try request with a management token
	{
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = root.SecretID

		var resp structs.ClientStatsResponse
		err := client.ClientRPC("ClientStats.Stats", &req, &resp)

		require.Nil(err)
		require.NotNil(resp.HostStats)
	}
}
