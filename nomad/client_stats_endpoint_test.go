// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestClientStats_Stats_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.config.RPCAddr.String()}
	})
	defer cleanupC()

	testutil.WaitForResult(func() (bool, error) {
		nodes := s.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// Make the request without having a node-id
	req := &structs.NodeSpecificRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.ClientStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientStats.Stats", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "missing")

	// Fetch the response setting the node id
	req.NodeID = c.NodeID()
	var resp2 cstructs.ClientStatsResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientStats.Stats", req, &resp2)
	require.Nil(err)
	require.NotNil(resp2.HostStats)
}

func TestClientStats_Stats_Local_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server
	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()
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
			var resp cstructs.ClientStatsResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientStats.Stats", req, &resp)
			require.NotNil(err)
			require.Contains(err.Error(), c.ExpectedError)
		})
	}
}

func TestClientStats_Stats_NoNode(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Make the request with a nonexistent node-id
	req := &structs.NodeSpecificRequest{
		NodeID:       uuid.Generate(),
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.ClientStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientStats.Stats", req, &resp)
	require.Nil(resp.HostStats)
	require.NotNil(err)
	require.Contains(err.Error(), "Unknown node")
}

func TestClientStats_Stats_OldNode(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	state := s.State()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Test for an old version error
	node := mock.Node()
	node.Attributes["nomad.version"] = "0.7.1"
	require.Nil(state.UpsertNode(structs.MsgTypeTestSetup, 1005, node))

	req := &structs.NodeSpecificRequest{
		NodeID:       node.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.ClientStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientStats.Stats", req, &resp)
	require.True(structs.IsErrNodeLacksRpc(err), err.Error())
}

func TestClientStats_Stats_Remote(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
	})
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
	})
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)
	codec := rpcClient(t, s2)

	c, cleanup := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s2.config.RPCAddr.String()}
	})
	defer cleanup()

	// Wait for client initialization
	select {
	case <-c.Ready():
	case <-time.After(10 * time.Second):
		require.Fail("client timedout on initialize")
	}

	testutil.WaitForResult(func() (bool, error) {
		nodes := s2.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// Force remove the connection locally in case it exists
	s1.nodeConnsLock.Lock()
	delete(s1.nodeConns, c.NodeID())
	s1.nodeConnsLock.Unlock()

	// Make the request without having a node-id
	req := &structs.NodeSpecificRequest{
		NodeID:       uuid.Generate(),
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	req.NodeID = c.NodeID()
	var resp cstructs.ClientStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientStats.Stats", req, &resp)
	require.Nil(err)
	require.NotNil(resp.HostStats)
}
