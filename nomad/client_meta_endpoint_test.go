package nomad

import (
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestClientMeta_Get(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s := TestServer(t, nil)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanup := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.config.RPCAddr.String()}
	})
	defer cleanup()

	testutil.WaitForResult(func() (bool, error) {
		nodes := s.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		require.Fail("should have a client")
	})

	// Make the request without having a node-id
	req := &structs.NodeSpecificRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientMeta.Get", req, &resp)
	require.NotNil(err)
	require.Equal(err.Error(), "missing NodeID")

	// Fetch the response setting the node id
	req.NodeID = c.NodeID()
	var resp2 cstructs.ClientMetadataResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientMeta.Get", req, &resp2)
	require.Nil(err)
	require.NotNil(resp2.Metadata)
}

func TestClientMeta_Get_ACL(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server
	s, root := TestACLServer(t, nil)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NodePolicy(acl.PolicyRead)
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: structs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "good token",
			Token:         tokenGood.SecretID,
			ExpectedError: "Unknown node",
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: "Unknown node",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request without having a node-id
			req := &structs.NodeSpecificRequest{
				NodeID: uuid.Generate(),
				QueryOptions: structs.QueryOptions{
					AuthToken: c.Token,
					Region:    "global",
				},
			}

			// Fetch the response
			var resp structs.GenericResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientMeta.Get", req, &resp)
			require.NotNil(err)
			require.Contains(err.Error(), c.ExpectedError)
		})
	}
}

func TestClientMeta_Put(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s := TestServer(t, nil)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanup := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.config.RPCAddr.String()}
	})
	defer cleanup()

	testutil.WaitForResult(func() (bool, error) {
		nodes := s.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		require.Fail("should have a client")
	})

	// Make the request without having a node-id
	req := &cstructs.ClientMetadataReplaceRequest{
		Metadata: map[string]string{
			"Some": "Meta",
		},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.ClientMetadataUpdateResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientMeta.Put", req, &resp)
	require.NotNil(err)
	require.Equal(err.Error(), "missing NodeID")

	// Fetch the response setting the node id
	req.NodeID = c.NodeID()
	var resp2 cstructs.ClientMetadataUpdateResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientMeta.Put", req, &resp2)
	require.Nil(err)
	require.True(resp2.Updated)
}

func TestClientMeta_Put_ACL(t *testing.T) {
	require := require.New(t)

	// Start a server
	s, root := TestACLServer(t, nil)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NodePolicy(acl.PolicyWrite)
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: structs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "good token",
			Token:         tokenGood.SecretID,
			ExpectedError: "Unknown node",
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: "Unknown node",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request without having a node-id
			req := &cstructs.ClientMetadataReplaceRequest{
				Metadata: map[string]string{"foo": "bar"},
				NodeID:   uuid.Generate(),
				WriteRequest: structs.WriteRequest{
					AuthToken: c.Token,
					Region:    "global",
				},
			}

			// Fetch the response
			var resp cstructs.ClientMetadataUpdateResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientMeta.Put", req, &resp)
			require.NotNil(err)
			require.Contains(err.Error(), c.ExpectedError)
		})
	}
}

func TestClientMeta_Patch(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s := TestServer(t, nil)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanup := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.config.RPCAddr.String()}
	})
	defer cleanup()

	testutil.WaitForResult(func() (bool, error) {
		nodes := s.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		require.Fail("should have a client")
	})

	// Make the request without having a node-id
	req := &cstructs.ClientMetadataUpdateRequest{
		Updates: map[string]string{
			"Some": "Meta",
		},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.ClientMetadataUpdateResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientMeta.Patch", req, &resp)
	require.NotNil(err)
	require.Equal(err.Error(), "missing NodeID")

	// Fetch the response setting the node id
	req.NodeID = c.NodeID()
	var resp2 cstructs.ClientMetadataUpdateResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientMeta.Patch", req, &resp2)
	require.Nil(err)
	require.True(resp2.Updated)
}

func TestClientMeta_Patch_ACL(t *testing.T) {
	require := require.New(t)

	// Start a server
	s, root := TestACLServer(t, nil)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NodePolicy(acl.PolicyWrite)
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: structs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "good token",
			Token:         tokenGood.SecretID,
			ExpectedError: "Unknown node",
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: "Unknown node",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request without having a node-id
			req := &cstructs.ClientMetadataUpdateRequest{
				Updates: map[string]string{"foo": "bar"},
				NodeID:  uuid.Generate(),
				WriteRequest: structs.WriteRequest{
					AuthToken: c.Token,
					Region:    "global",
				},
			}

			// Fetch the response
			var resp cstructs.ClientMetadataUpdateResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientMeta.Patch", req, &resp)
			require.NotNil(err)
			require.Contains(err.Error(), c.ExpectedError)
		})
	}
}
