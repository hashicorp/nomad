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

func TestClientMetadata_Metadata(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client, cleanup := TestClient(t, nil)
	defer cleanup()

	req := &nstructs.NodeSpecificRequest{}
	var resp structs.ClientMetadataResponse
	require.Nil(client.ClientRPC("ClientMetadata.Metadata", &req, &resp))
	require.NotNil(resp.Metadata)
}

func TestClientMetadata_Metadata_ACL(t *testing.T) {
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
		var resp structs.ClientMetadataResponse
		err := client.ClientRPC("ClientMetadata.Metadata", &req, &resp)
		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = token.SecretID

		var resp structs.ClientMetadataResponse
		err := client.ClientRPC("ClientMetadata.Metadata", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1007, "valid", mock.NodePolicy(acl.PolicyRead))
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = token.SecretID

		var resp structs.ClientMetadataResponse
		err := client.ClientRPC("ClientMetadata.Metadata", &req, &resp)

		require.Nil(err)
		require.NotNil(resp.Metadata)
	}

	// Try request with a management token
	{
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = root.SecretID

		var resp structs.ClientMetadataResponse
		err := client.ClientRPC("ClientMetadata.Metadata", &req, &resp)

		require.Nil(err)
		require.NotNil(resp.Metadata)
	}
}

func TestClientMetadata_UpdateMetadata(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client, cleanup := TestClient(t, nil)
	defer cleanup()

	req := &structs.ClientMetadataUpdateRequest{
		Updates: map[string]string{
			"SomeKey": "Value",
		},
	}
	var resp structs.ClientMetadataUpdateResponse
	require.Nil(client.ClientRPC("ClientMetadata.UpdateMetadata", &req, &resp))
	require.True(resp.Updated)
	require.Equal(client.configCopy.Node.Meta["SomeKey"], "Value")
}

func TestClientMetadata_UpdateMetadata_ACL(t *testing.T) {
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
		req := &structs.ClientMetadataUpdateRequest{}
		var resp structs.ClientMetadataUpdateResponse
		err := client.ClientRPC("ClientMetadata.UpdateMetadata", &req, &resp)
		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyRead))
		req := &structs.ClientMetadataUpdateRequest{}
		req.AuthToken = token.SecretID

		var resp structs.ClientMetadataUpdateResponse
		err := client.ClientRPC("ClientMetadata.UpdateMetadata", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid write token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1007, "valid", mock.NodePolicy(acl.PolicyWrite))
		req := &structs.ClientMetadataUpdateRequest{}
		req.AuthToken = token.SecretID

		var resp structs.ClientMetadataUpdateResponse
		err := client.ClientRPC("ClientMetadata.UpdateMetadata", &req, &resp)

		require.Nil(err)
	}

	// Try request with a management token
	{
		req := &structs.ClientMetadataUpdateRequest{}
		req.AuthToken = root.SecretID

		var resp structs.ClientMetadataUpdateResponse
		err := client.ClientRPC("ClientMetadata.UpdateMetadata", &req, &resp)

		require.Nil(err)
	}
}

func TestClientMetadata_ReplaceMetadata(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	client, cleanup := TestClient(t, nil)
	defer cleanup()

	meta := map[string]string{"foo": "bar", "boo": "baz"}

	req := &structs.ClientMetadataReplaceRequest{
		Metadata: meta,
	}
	var resp structs.ClientMetadataUpdateResponse
	require.Nil(client.ClientRPC("ClientMetadata.ReplaceMetadata", &req, &resp))
	require.True(resp.Updated)
	require.Equal(client.configCopy.Node.Meta, meta)
}

func TestClientMetadata_ReplaceMetadata_ACL(t *testing.T) {
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
		req := &structs.ClientMetadataReplaceRequest{}
		var resp structs.ClientMetadataUpdateResponse
		err := client.ClientRPC("ClientMetadata.ReplaceMetadata", &req, &resp)
		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyRead))
		req := &structs.ClientMetadataReplaceRequest{}
		req.AuthToken = token.SecretID

		var resp structs.ClientMetadataUpdateResponse
		err := client.ClientRPC("ClientMetadata.ReplaceMetadata", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid write token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1007, "valid", mock.NodePolicy(acl.PolicyWrite))
		req := &structs.ClientMetadataReplaceRequest{}
		req.AuthToken = token.SecretID

		var resp structs.ClientMetadataUpdateResponse
		err := client.ClientRPC("ClientMetadata.ReplaceMetadata", &req, &resp)

		require.Nil(err)
	}

	// Try request with a management token
	{
		req := &structs.ClientMetadataReplaceRequest{}
		req.AuthToken = root.SecretID

		var resp structs.ClientMetadataUpdateResponse
		err := client.ClientRPC("ClientMetadata.ReplaceMetadata", &req, &resp)

		require.Nil(err)
	}
}
