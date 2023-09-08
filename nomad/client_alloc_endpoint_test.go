// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

func TestClientAllocations_GarbageCollectAll_Local(t *testing.T) {
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
	req := &nstructs.NodeSpecificRequest{
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp nstructs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "missing")

	// Fetch the response setting the node id
	req.NodeID = c.NodeID()
	var resp2 nstructs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp2)
	require.Nil(err)
}

func TestClientAllocations_GarbageCollectAll_Local_ACL(t *testing.T) {
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
			ExpectedError: nstructs.ErrPermissionDenied.Error(),
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
			req := &nstructs.NodeSpecificRequest{
				NodeID: uuid.Generate(),
				QueryOptions: nstructs.QueryOptions{
					AuthToken: c.Token,
					Region:    "global",
				},
			}

			// Fetch the response
			var resp nstructs.GenericResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
			require.NotNil(err)
			require.Contains(err.Error(), c.ExpectedError)
		})
	}
}

func TestClientAllocations_GarbageCollectAll_NoNode(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Make the request without having a node-id
	req := &nstructs.NodeSpecificRequest{
		NodeID:       uuid.Generate(),
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp nstructs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Unknown node")
}

func TestClientAllocations_GarbageCollectAll_OldNode(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and fake an old client
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	state := s.State()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Test for an old version error
	node := mock.Node()
	node.Attributes["nomad.version"] = "0.7.1"
	require.Nil(state.UpsertNode(nstructs.MsgTypeTestSetup, 1005, node.Copy()))

	req := &nstructs.NodeSpecificRequest{
		NodeID:       node.ID,
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	var resp nstructs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
	require.True(nstructs.IsErrNodeLacksRpc(err))

	// Test for a missing version error
	delete(node.Attributes, "nomad.version")
	require.Nil(state.UpsertNode(nstructs.MsgTypeTestSetup, 1006, node))

	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
	require.True(nstructs.IsErrUnknownNomadVersion(err))
}

func TestClientAllocations_GarbageCollectAll_Remote(t *testing.T) {
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

	c, cleanupC := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s2.config.RPCAddr.String()}
		c.GCDiskUsageThreshold = 100.0
	})
	defer cleanupC()

	testutil.WaitForResult(func() (bool, error) {
		nodes := s2.connectedNodes()
		if len(nodes) != 1 {
			return false, fmt.Errorf("should have 1 client. found %d", len(nodes))
		}
		req := &nstructs.NodeSpecificRequest{
			NodeID:       c.NodeID(),
			QueryOptions: nstructs.QueryOptions{Region: "global"},
		}
		resp := nstructs.SingleNodeResponse{}
		if err := msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp); err != nil {
			return false, err
		}
		return resp.Node != nil && resp.Node.Status == nstructs.NodeStatusReady, fmt.Errorf(
			"expected ready but found %s", pretty.Sprint(resp.Node))
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// Force remove the connection locally in case it exists
	s1.nodeConnsLock.Lock()
	delete(s1.nodeConns, c.NodeID())
	s1.nodeConnsLock.Unlock()

	// Make the request
	req := &nstructs.NodeSpecificRequest{
		NodeID:       c.NodeID(),
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.ClientStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
	require.Nil(err)
}

func TestClientAllocations_GarbageCollect_OldNode(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and fake an old client
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	state := s.State()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Test for an old version error
	node := mock.Node()
	node.Attributes["nomad.version"] = "0.7.1"
	require.Nil(state.UpsertNode(nstructs.MsgTypeTestSetup, 1005, node))

	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	require.Nil(state.UpsertAllocs(nstructs.MsgTypeTestSetup, 1006, []*nstructs.Allocation{alloc}))

	req := &nstructs.AllocSpecificRequest{
		AllocID: alloc.ID,
		QueryOptions: nstructs.QueryOptions{
			Region:    "global",
			Namespace: nstructs.DefaultNamespace,
		},
	}

	var resp nstructs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp)
	require.True(nstructs.IsErrNodeLacksRpc(err), err.Error())

	// Test for a missing version error
	delete(node.Attributes, "nomad.version")
	require.Nil(state.UpsertNode(nstructs.MsgTypeTestSetup, 1007, node))

	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp)
	require.True(nstructs.IsErrUnknownNomadVersion(err), err.Error())
}

func TestClientAllocations_GarbageCollect_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.config.RPCAddr.String()}
		c.GCDiskUsageThreshold = 100.0
	})
	defer cleanupC()

	// Force an allocation onto the node
	a := mock.Alloc()
	a.Job.Type = nstructs.JobTypeBatch
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &nstructs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "2s",
		},
		LogConfig: nstructs.DefaultLogConfig(),
		Resources: &nstructs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
	}

	testutil.WaitForResult(func() (bool, error) {
		nodes := s.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// Upsert the allocation
	state := s.State()
	require.Nil(state.UpsertJob(nstructs.MsgTypeTestSetup, 999, nil, a.Job))
	require.Nil(state.UpsertAllocs(nstructs.MsgTypeTestSetup, 1003, []*nstructs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != nstructs.AllocClientStatusComplete {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not finished: %v", c.NodeID(), err)
	})

	// Make the request without having an alloc id
	req := &nstructs.AllocSpecificRequest{
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp nstructs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "missing")

	// Fetch the response setting the node id
	req.AllocID = a.ID
	var resp2 nstructs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp2)
	require.Nil(err)
}

func TestClientAllocations_GarbageCollect_Local_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start a server
	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(nstructs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob})
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	// Upsert the allocation
	state := s.State()
	alloc := mock.Alloc()
	require.NoError(t, state.UpsertJob(nstructs.MsgTypeTestSetup, 1010, nil, alloc.Job))
	require.NoError(t, state.UpsertAllocs(nstructs.MsgTypeTestSetup, 1011, []*nstructs.Allocation{alloc}))

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: nstructs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "good token",
			Token:         tokenGood.SecretID,
			ExpectedError: nstructs.ErrUnknownNodePrefix,
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: nstructs.ErrUnknownNodePrefix,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request without having a node-id
			req := &nstructs.AllocSpecificRequest{
				AllocID: alloc.ID,
				QueryOptions: nstructs.QueryOptions{
					AuthToken: c.Token,
					Region:    "global",
					Namespace: nstructs.DefaultNamespace,
				},
			}

			// Fetch the response
			var resp nstructs.GenericResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp)
			require.NotNil(t, err)
			require.Contains(t, err.Error(), c.ExpectedError)
		})
	}
}

func TestClientAllocations_GarbageCollect_Remote(t *testing.T) {
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
		c.GCDiskUsageThreshold = 100.0
	})
	defer cleanup()

	// Force an allocation onto the node
	a := mock.Alloc()
	a.Job.Type = nstructs.JobTypeBatch
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &nstructs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "2s",
		},
		LogConfig: nstructs.DefaultLogConfig(),
		Resources: &nstructs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
	}
	testutil.WaitForResult(func() (bool, error) {
		nodes := s2.connectedNodes()
		if len(nodes) != 1 {
			return false, fmt.Errorf("should have 1 client. found %d", len(nodes))
		}
		req := &nstructs.NodeSpecificRequest{
			NodeID:       c.NodeID(),
			QueryOptions: nstructs.QueryOptions{Region: "global"},
		}
		resp := nstructs.SingleNodeResponse{}
		if err := msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp); err != nil {
			return false, err
		}
		return resp.Node != nil && resp.Node.Status == nstructs.NodeStatusReady, fmt.Errorf(
			"expected ready but found %s", pretty.Sprint(resp.Node))
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// Upsert the allocation
	state1 := s1.State()
	state2 := s2.State()
	require.Nil(state1.UpsertJob(nstructs.MsgTypeTestSetup, 999, nil, a.Job))
	require.Nil(state1.UpsertAllocs(nstructs.MsgTypeTestSetup, 1003, []*nstructs.Allocation{a}))
	require.Nil(state2.UpsertJob(nstructs.MsgTypeTestSetup, 999, nil, a.Job))
	require.Nil(state2.UpsertAllocs(nstructs.MsgTypeTestSetup, 1003, []*nstructs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state2.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != nstructs.AllocClientStatusComplete {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not finished: %v", c.NodeID(), err)
	})

	// Force remove the connection locally in case it exists
	s1.nodeConnsLock.Lock()
	delete(s1.nodeConns, c.NodeID())
	s1.nodeConnsLock.Unlock()

	// Make the request
	req := &nstructs.AllocSpecificRequest{
		AllocID:      a.ID,
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.ClientStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp)
	require.Nil(err)
}

func TestClientAllocations_Stats_OldNode(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and fake an old client
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	state := s.State()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Test for an old version error
	node := mock.Node()
	node.Attributes["nomad.version"] = "0.7.1"
	require.Nil(state.UpsertNode(nstructs.MsgTypeTestSetup, 1005, node.Copy()))

	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	require.Nil(state.UpsertAllocs(nstructs.MsgTypeTestSetup, 1006, []*nstructs.Allocation{alloc}))

	req := &nstructs.AllocSpecificRequest{
		AllocID: alloc.ID,
		QueryOptions: nstructs.QueryOptions{
			Region: "global",
		},
	}

	var resp nstructs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp)
	require.True(nstructs.IsErrNodeLacksRpc(err), err.Error())

	// Test for a missing version error
	delete(node.Attributes, "nomad.version")
	require.Nil(state.UpsertNode(nstructs.MsgTypeTestSetup, 1007, node))

	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp)
	require.True(nstructs.IsErrUnknownNomadVersion(err), err.Error())
}

func TestClientAllocations_Stats_Local(t *testing.T) {
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

	// Force an allocation onto the node
	a := mock.Alloc()
	a.Job.Type = nstructs.JobTypeBatch
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &nstructs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "2s",
		},
		LogConfig: nstructs.DefaultLogConfig(),
		Resources: &nstructs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
	}

	testutil.WaitForResult(func() (bool, error) {
		nodes := s.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// Upsert the allocation
	state := s.State()
	require.Nil(state.UpsertJob(nstructs.MsgTypeTestSetup, 999, nil, a.Job))
	require.Nil(state.UpsertAllocs(nstructs.MsgTypeTestSetup, 1003, []*nstructs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != nstructs.AllocClientStatusComplete {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not finished: %v", c.NodeID(), err)
	})

	// Make the request without having an alloc id
	req := &nstructs.AllocSpecificRequest{
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.AllocStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp)
	require.NotNil(err)
	require.EqualError(err, nstructs.ErrMissingAllocID.Error(), "(%T) %v")

	// Fetch the response setting the node id
	req.AllocID = a.ID
	var resp2 cstructs.AllocStatsResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp2)
	require.Nil(err)
	require.NotNil(resp2.Stats)
}

func TestClientAllocations_Stats_Local_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start a server
	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(nstructs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob})
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	// Upsert the allocation
	state := s.State()
	alloc := mock.Alloc()
	require.NoError(t, state.UpsertJob(nstructs.MsgTypeTestSetup, 1010, nil, alloc.Job))
	require.NoError(t, state.UpsertAllocs(nstructs.MsgTypeTestSetup, 1011, []*nstructs.Allocation{alloc}))

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: nstructs.ErrPermissionDenied.Error(),
		},
		{
			Name:          "good token",
			Token:         tokenGood.SecretID,
			ExpectedError: nstructs.ErrUnknownNodePrefix,
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: nstructs.ErrUnknownNodePrefix,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request without having a node-id
			req := &nstructs.AllocSpecificRequest{
				AllocID: alloc.ID,
				QueryOptions: nstructs.QueryOptions{
					AuthToken: c.Token,
					Region:    "global",
					Namespace: nstructs.DefaultNamespace,
				},
			}

			// Fetch the response
			var resp cstructs.AllocStatsResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp)
			require.NotNil(t, err)
			require.Contains(t, err.Error(), c.ExpectedError)
		})
	}
}

func TestClientAllocations_Stats_Remote(t *testing.T) {
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

	c, cleanupC := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s2.config.RPCAddr.String()}
	})
	defer cleanupC()

	// Force an allocation onto the node
	a := mock.Alloc()
	a.Job.Type = nstructs.JobTypeBatch
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &nstructs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "2s",
		},
		LogConfig: nstructs.DefaultLogConfig(),
		Resources: &nstructs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
	}
	testutil.WaitForResult(func() (bool, error) {
		nodes := s2.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// Upsert the allocation
	state1 := s1.State()
	state2 := s2.State()
	require.Nil(state1.UpsertJob(nstructs.MsgTypeTestSetup, 999, nil, a.Job))
	require.Nil(state1.UpsertAllocs(nstructs.MsgTypeTestSetup, 1003, []*nstructs.Allocation{a}))
	require.Nil(state2.UpsertJob(nstructs.MsgTypeTestSetup, 999, nil, a.Job))
	require.Nil(state2.UpsertAllocs(nstructs.MsgTypeTestSetup, 1003, []*nstructs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state2.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != nstructs.AllocClientStatusComplete {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not finished: %v", c.NodeID(), err)
	})

	// Force remove the connection locally in case it exists
	s1.nodeConnsLock.Lock()
	delete(s1.nodeConns, c.NodeID())
	s1.nodeConnsLock.Unlock()

	// Make the request
	req := &nstructs.AllocSpecificRequest{
		AllocID:      a.ID,
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.AllocStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp)
	require.Nil(err)
	require.NotNil(resp.Stats)
}

func TestClientAllocations_Restart_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	// Start a server and client
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.config.RPCAddr.String()}
		c.GCDiskUsageThreshold = 100.0
	})
	defer cleanupC()

	// Force an allocation onto the node
	a := mock.Alloc()
	a.Job.Type = nstructs.JobTypeService
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &nstructs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "10s",
		},
		LogConfig: nstructs.DefaultLogConfig(),
		Resources: &nstructs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
	}

	testutil.WaitForResult(func() (bool, error) {
		nodes := s.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a client")
	})

	// Upsert the allocation
	state := s.State()
	require.Nil(state.UpsertJob(nstructs.MsgTypeTestSetup, 999, nil, a.Job))
	require.Nil(state.UpsertAllocs(nstructs.MsgTypeTestSetup, 1003, []*nstructs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != nstructs.AllocClientStatusRunning {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not running: %v", c.NodeID(), err)
	})

	// Make the request without having an alloc id
	req := &nstructs.AllocRestartRequest{
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp nstructs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Restart", req, &resp)
	require.NotNil(err)
	require.EqualError(err, nstructs.ErrMissingAllocID.Error(), "(%T) %v")

	// Fetch the response setting the alloc id - This should not error because the
	// alloc is running.
	req.AllocID = a.ID
	var resp2 nstructs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.Restart", req, &resp2)
	require.Nil(err)

	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}

		taskState := alloc.TaskStates["web"]
		if taskState == nil {
			return false, fmt.Errorf("could not find task state")
		}

		if taskState.Restarts != 1 {
			return false, fmt.Errorf("expected task 'web' to have 1 restart, got: %d", taskState.Restarts)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not running: %v", c.NodeID(), err)
	})
}

func TestClientAllocations_Restart_Remote(t *testing.T) {
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

	c, cleanupC := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s2.config.RPCAddr.String()}
	})
	defer cleanupC()

	// Force an allocation onto the node
	a := mock.Alloc()
	a.Job.Type = nstructs.JobTypeService
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &nstructs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "10s",
		},
		LogConfig: nstructs.DefaultLogConfig(),
		Resources: &nstructs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
	}

	testutil.WaitForResult(func() (bool, error) {
		nodes := s2.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		t.Fatalf("should have a client")
	})

	// Upsert the allocation
	state1 := s1.State()
	state2 := s2.State()
	require.Nil(state1.UpsertJob(nstructs.MsgTypeTestSetup, 999, nil, a.Job))
	require.Nil(state1.UpsertAllocs(nstructs.MsgTypeTestSetup, 1003, []*nstructs.Allocation{a}))
	require.Nil(state2.UpsertJob(nstructs.MsgTypeTestSetup, 999, nil, a.Job))
	require.Nil(state2.UpsertAllocs(nstructs.MsgTypeTestSetup, 1003, []*nstructs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state2.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != nstructs.AllocClientStatusRunning {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not running: %v", c.NodeID(), err)
	})

	// Make the request without having an alloc id
	req := &nstructs.AllocRestartRequest{
		QueryOptions: nstructs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp nstructs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Restart", req, &resp)
	require.NotNil(err)
	require.EqualError(err, nstructs.ErrMissingAllocID.Error(), "(%T) %v")

	// Fetch the response setting the alloc id - This should succeed because the
	// alloc is running
	req.AllocID = a.ID
	var resp2 nstructs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.Restart", req, &resp2)
	require.NoError(err)
}

func TestClientAllocations_Restart_ACL(t *testing.T) {
	ci.Parallel(t)

	// Start a server
	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(nstructs.DefaultNamespace, acl.PolicyWrite, nil)
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	// Upsert the allocation
	state := s.State()
	alloc := mock.Alloc()
	require.NoError(t, state.UpsertJob(nstructs.MsgTypeTestSetup, 1010, nil, alloc.Job))
	require.NoError(t, state.UpsertAllocs(nstructs.MsgTypeTestSetup, 1011, []*nstructs.Allocation{alloc}))

	cases := []struct {
		Name          string
		Token         string
		ExpectedError string
	}{
		{
			Name:          "bad token",
			Token:         tokenBad.SecretID,
			ExpectedError: nstructs.ErrPermissionDenied.Error(),
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
			req := &nstructs.AllocRestartRequest{
				AllocID: alloc.ID,
				QueryOptions: nstructs.QueryOptions{
					Namespace: nstructs.DefaultNamespace,
					AuthToken: c.Token,
					Region:    "global",
				},
			}

			// Fetch the response
			var resp nstructs.GenericResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Restart", req, &resp)
			require.NotNil(t, err)
			require.Contains(t, err.Error(), c.ExpectedError)
		})
	}
}

// TestAlloc_ExecStreaming asserts that exec task requests are forwarded
// to appropriate server or remote regions
func TestAlloc_ExecStreaming(t *testing.T) {
	ci.Parallel(t)

	////// Nomad clusters topology - not specific to test
	localServer, cleanupLS := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
	})
	defer cleanupLS()

	remoteServer, cleanupRS := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 2
	})
	defer cleanupRS()

	remoteRegionServer, cleanupRRS := TestServer(t, func(c *Config) {
		c.Region = "two"
	})
	defer cleanupRRS()

	TestJoin(t, localServer, remoteServer)
	TestJoin(t, localServer, remoteRegionServer)
	testutil.WaitForLeader(t, localServer.RPC)
	testutil.WaitForLeader(t, remoteServer.RPC)
	testutil.WaitForLeader(t, remoteRegionServer.RPC)

	c, cleanup := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{localServer.config.RPCAddr.String()}
	})
	defer cleanup()

	// Wait for the client to connect
	testutil.WaitForResult(func() (bool, error) {
		nodes := remoteServer.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		require.NoError(t, err, "failed to have a client")
	})

	// Force remove the connection locally in case it exists
	remoteServer.nodeConnsLock.Lock()
	delete(remoteServer.nodeConns, c.NodeID())
	remoteServer.nodeConnsLock.Unlock()

	///// Start task
	a := mock.BatchAlloc()
	a.NodeID = c.NodeID()
	a.Job.Type = nstructs.JobTypeBatch
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "20s",
		"exec_command": map[string]interface{}{
			"run_for":       "1ms",
			"stdout_string": "expected output",
			"exit_code":     3,
		},
	}

	// Upsert the allocation
	localState := localServer.State()
	require.Nil(t, localState.UpsertJob(nstructs.MsgTypeTestSetup, 999, nil, a.Job))
	require.Nil(t, localState.UpsertAllocs(nstructs.MsgTypeTestSetup, 1003, []*nstructs.Allocation{a}))
	remoteState := remoteServer.State()
	require.Nil(t, remoteState.UpsertJob(nstructs.MsgTypeTestSetup, 999, nil, a.Job))
	require.Nil(t, remoteState.UpsertAllocs(nstructs.MsgTypeTestSetup, 1003, []*nstructs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := localState.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != nstructs.AllocClientStatusRunning {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err, "task didn't start yet")
	})

	/////////  Actually run query now
	cases := []struct {
		name string
		rpc  func(string) (nstructs.StreamingRpcHandler, error)
	}{
		{"client", c.StreamingRpcHandler},
		{"local_server", localServer.StreamingRpcHandler},
		{"remote_server", remoteServer.StreamingRpcHandler},
		{"remote_region", remoteRegionServer.StreamingRpcHandler},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			// Make the request
			req := &cstructs.AllocExecRequest{
				AllocID:      a.ID,
				Task:         a.Job.TaskGroups[0].Tasks[0].Name,
				Tty:          true,
				Cmd:          []string{"placeholder command"},
				QueryOptions: nstructs.QueryOptions{Region: "global"},
			}

			// Get the handler
			handler, err := tc.rpc("Allocations.Exec")
			require.Nil(t, err)

			// Create a pipe
			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			errCh := make(chan error)
			frames := make(chan *drivers.ExecTaskStreamingResponseMsg)

			// Start the handler
			go handler(p2)
			go decodeFrames(t, p1, frames, errCh)

			// Send the request
			encoder := codec.NewEncoder(p1, nstructs.MsgpackHandle)
			require.Nil(t, encoder.Encode(req))

			timeout := time.After(3 * time.Second)

		OUTER:
			for {
				select {
				case <-timeout:
					require.FailNow(t, "timed out before getting exit code")
				case err := <-errCh:
					require.NoError(t, err)
				case f := <-frames:
					if f.Exited && f.Result != nil {
						code := int(f.Result.ExitCode)
						require.Equal(t, 3, code)
						break OUTER
					}
				}
			}
		})
	}
}

func decodeFrames(t *testing.T, p1 net.Conn, frames chan<- *drivers.ExecTaskStreamingResponseMsg, errCh chan<- error) {
	// Start the decoder
	decoder := codec.NewDecoder(p1, nstructs.MsgpackHandle)

	for {
		var msg cstructs.StreamErrWrapper
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "closed") {
				return
			}
			t.Logf("received error decoding: %#v", err)

			errCh <- fmt.Errorf("error decoding: %v", err)
			return
		}

		if msg.Error != nil {
			errCh <- msg.Error
			continue
		}

		var frame drivers.ExecTaskStreamingResponseMsg
		json.Unmarshal(msg.Payload, &frame)
		t.Logf("received message: %#v", msg)
		frames <- &frame
	}
}
