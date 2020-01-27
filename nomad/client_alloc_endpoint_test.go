package nomad

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
	"github.com/ugorji/go/codec"
)

func TestClientAllocations_GarbageCollectAll_Local(t *testing.T) {
	t.Parallel()
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
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "missing")

	// Fetch the response setting the node id
	req.NodeID = c.NodeID()
	var resp2 structs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp2)
	require.Nil(err)
}

func TestClientAllocations_GarbageCollectAll_Local_ACL(t *testing.T) {
	t.Parallel()
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
			err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
			require.NotNil(err)
			require.Contains(err.Error(), c.ExpectedError)
		})
	}
}

func TestClientAllocations_GarbageCollectAll_NoNode(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Make the request without having a node-id
	req := &structs.NodeSpecificRequest{
		NodeID:       uuid.Generate(),
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "Unknown node")
}

func TestClientAllocations_GarbageCollectAll_OldNode(t *testing.T) {
	t.Parallel()
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
	require.Nil(state.UpsertNode(1005, node))

	req := &structs.NodeSpecificRequest{
		NodeID:       node.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
	require.True(structs.IsErrNodeLacksRpc(err))

	// Test for a missing version error
	delete(node.Attributes, "nomad.version")
	require.Nil(state.UpsertNode(1006, node))

	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
	require.True(structs.IsErrUnknownNomadVersion(err))
}

func TestClientAllocations_GarbageCollectAll_Remote(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
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
		req := &structs.NodeSpecificRequest{
			NodeID:       c.NodeID(),
			QueryOptions: structs.QueryOptions{Region: "global"},
		}
		resp := structs.SingleNodeResponse{}
		if err := msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp); err != nil {
			return false, err
		}
		return resp.Node != nil && resp.Node.Status == structs.NodeStatusReady, fmt.Errorf(
			"expected ready but found %s", pretty.Sprint(resp.Node))
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// Force remove the connection locally in case it exists
	s1.nodeConnsLock.Lock()
	delete(s1.nodeConns, c.NodeID())
	s1.nodeConnsLock.Unlock()

	// Make the request
	req := &structs.NodeSpecificRequest{
		NodeID:       c.NodeID(),
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.ClientStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollectAll", req, &resp)
	require.Nil(err)
}

func TestClientAllocations_GarbageCollect_OldNode(t *testing.T) {
	t.Parallel()
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
	require.Nil(state.UpsertNode(1005, node))

	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	require.Nil(state.UpsertAllocs(1006, []*structs.Allocation{alloc}))

	req := &structs.AllocSpecificRequest{
		AllocID: alloc.ID,
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			Namespace: structs.DefaultNamespace,
		},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp)
	require.True(structs.IsErrNodeLacksRpc(err), err.Error())

	// Test for a missing version error
	delete(node.Attributes, "nomad.version")
	require.Nil(state.UpsertNode(1007, node))

	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp)
	require.True(structs.IsErrUnknownNomadVersion(err), err.Error())
}

func TestClientAllocations_GarbageCollect_Local(t *testing.T) {
	t.Parallel()
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
	a.Job.Type = structs.JobTypeBatch
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &structs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "2s",
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &structs.Resources{
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
	require.Nil(state.UpsertJob(999, a.Job))
	require.Nil(state.UpsertAllocs(1003, []*structs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not finished: %v", c.NodeID(), err)
	})

	// Make the request without having an alloc id
	req := &structs.AllocSpecificRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "missing")

	// Fetch the response setting the node id
	req.AllocID = a.ID
	var resp2 structs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp2)
	require.Nil(err)
}

func TestClientAllocations_GarbageCollect_Local_ACL(t *testing.T) {
	t.Parallel()

	// Start a server
	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob})
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	// Upsert the allocation
	state := s.State()
	alloc := mock.Alloc()
	require.NoError(t, state.UpsertJob(1010, alloc.Job))
	require.NoError(t, state.UpsertAllocs(1011, []*structs.Allocation{alloc}))

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
			ExpectedError: structs.ErrUnknownNodePrefix,
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: structs.ErrUnknownNodePrefix,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request without having a node-id
			req := &structs.AllocSpecificRequest{
				AllocID: alloc.ID,
				QueryOptions: structs.QueryOptions{
					AuthToken: c.Token,
					Region:    "global",
					Namespace: structs.DefaultNamespace,
				},
			}

			// Fetch the response
			var resp structs.GenericResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp)
			require.NotNil(t, err)
			require.Contains(t, err.Error(), c.ExpectedError)
		})
	}
}

func TestClientAllocations_GarbageCollect_Remote(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
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
	a.Job.Type = structs.JobTypeBatch
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &structs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "2s",
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &structs.Resources{
			CPU:      500,
			MemoryMB: 256,
		},
	}
	testutil.WaitForResult(func() (bool, error) {
		nodes := s2.connectedNodes()
		if len(nodes) != 1 {
			return false, fmt.Errorf("should have 1 client. found %d", len(nodes))
		}
		req := &structs.NodeSpecificRequest{
			NodeID:       c.NodeID(),
			QueryOptions: structs.QueryOptions{Region: "global"},
		}
		resp := structs.SingleNodeResponse{}
		if err := msgpackrpc.CallWithCodec(codec, "Node.GetNode", req, &resp); err != nil {
			return false, err
		}
		return resp.Node != nil && resp.Node.Status == structs.NodeStatusReady, fmt.Errorf(
			"expected ready but found %s", pretty.Sprint(resp.Node))
	}, func(err error) {
		t.Fatalf("should have a clients")
	})

	// Upsert the allocation
	state1 := s1.State()
	state2 := s2.State()
	require.Nil(state1.UpsertJob(999, a.Job))
	require.Nil(state1.UpsertAllocs(1003, []*structs.Allocation{a}))
	require.Nil(state2.UpsertJob(999, a.Job))
	require.Nil(state2.UpsertAllocs(1003, []*structs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state2.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != structs.AllocClientStatusComplete {
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
	req := &structs.AllocSpecificRequest{
		AllocID:      a.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.ClientStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp)
	require.Nil(err)
}

func TestClientAllocations_Stats_OldNode(t *testing.T) {
	t.Parallel()
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
	require.Nil(state.UpsertNode(1005, node))

	alloc := mock.Alloc()
	alloc.NodeID = node.ID
	require.Nil(state.UpsertAllocs(1006, []*structs.Allocation{alloc}))

	req := &structs.AllocSpecificRequest{
		AllocID: alloc.ID,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp)
	require.True(structs.IsErrNodeLacksRpc(err), err.Error())

	// Test for a missing version error
	delete(node.Attributes, "nomad.version")
	require.Nil(state.UpsertNode(1007, node))

	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp)
	require.True(structs.IsErrUnknownNomadVersion(err), err.Error())
}

func TestClientAllocations_Stats_Local(t *testing.T) {
	t.Parallel()
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
	a.Job.Type = structs.JobTypeBatch
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &structs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "2s",
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &structs.Resources{
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
	require.Nil(state.UpsertJob(999, a.Job))
	require.Nil(state.UpsertAllocs(1003, []*structs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != structs.AllocClientStatusComplete {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not finished: %v", c.NodeID(), err)
	})

	// Make the request without having an alloc id
	req := &structs.AllocSpecificRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.AllocStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp)
	require.NotNil(err)
	require.EqualError(err, structs.ErrMissingAllocID.Error(), "(%T) %v")

	// Fetch the response setting the node id
	req.AllocID = a.ID
	var resp2 cstructs.AllocStatsResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp2)
	require.Nil(err)
	require.NotNil(resp2.Stats)
}

func TestClientAllocations_Stats_Local_ACL(t *testing.T) {
	t.Parallel()

	// Start a server
	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob})
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	// Upsert the allocation
	state := s.State()
	alloc := mock.Alloc()
	require.NoError(t, state.UpsertJob(1010, alloc.Job))
	require.NoError(t, state.UpsertAllocs(1011, []*structs.Allocation{alloc}))

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
			ExpectedError: structs.ErrUnknownNodePrefix,
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: structs.ErrUnknownNodePrefix,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request without having a node-id
			req := &structs.AllocSpecificRequest{
				AllocID: alloc.ID,
				QueryOptions: structs.QueryOptions{
					AuthToken: c.Token,
					Region:    "global",
					Namespace: structs.DefaultNamespace,
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
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
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
	a.Job.Type = structs.JobTypeBatch
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &structs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "2s",
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &structs.Resources{
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
	require.Nil(state1.UpsertJob(999, a.Job))
	require.Nil(state1.UpsertAllocs(1003, []*structs.Allocation{a}))
	require.Nil(state2.UpsertJob(999, a.Job))
	require.Nil(state2.UpsertAllocs(1003, []*structs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state2.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != structs.AllocClientStatusComplete {
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
	req := &structs.AllocSpecificRequest{
		AllocID:      a.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp cstructs.AllocStatsResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp)
	require.Nil(err)
	require.NotNil(resp.Stats)
}

func TestClientAllocations_Restart_Local(t *testing.T) {
	t.Parallel()
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
	a.Job.Type = structs.JobTypeService
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &structs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "10s",
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &structs.Resources{
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
	require.Nil(state.UpsertJob(999, a.Job))
	require.Nil(state.UpsertAllocs(1003, []*structs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not running: %v", c.NodeID(), err)
	})

	// Make the request without having an alloc id
	req := &structs.AllocRestartRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Restart", req, &resp)
	require.NotNil(err)
	require.EqualError(err, structs.ErrMissingAllocID.Error(), "(%T) %v")

	// Fetch the response setting the alloc id - This should not error because the
	// alloc is running.
	req.AllocID = a.ID
	var resp2 structs.GenericResponse
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
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
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
	a.Job.Type = structs.JobTypeService
	a.NodeID = c.NodeID()
	a.Job.TaskGroups[0].Count = 1
	a.Job.TaskGroups[0].Tasks[0] = &structs.Task{
		Name:   "web",
		Driver: "mock_driver",
		Config: map[string]interface{}{
			"run_for": "10s",
		},
		LogConfig: structs.DefaultLogConfig(),
		Resources: &structs.Resources{
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
	require.Nil(state1.UpsertJob(999, a.Job))
	require.Nil(state1.UpsertAllocs(1003, []*structs.Allocation{a}))
	require.Nil(state2.UpsertJob(999, a.Job))
	require.Nil(state2.UpsertAllocs(1003, []*structs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := state2.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("Alloc on node %q not running: %v", c.NodeID(), err)
	})

	// Make the request without having an alloc id
	req := &structs.AllocRestartRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}

	// Fetch the response
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Restart", req, &resp)
	require.NotNil(err)
	require.EqualError(err, structs.ErrMissingAllocID.Error(), "(%T) %v")

	// Fetch the response setting the alloc id - This should succeed because the
	// alloc is running
	req.AllocID = a.ID
	var resp2 structs.GenericResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.Restart", req, &resp2)
	require.NoError(err)
}

func TestClientAllocations_Restart_ACL(t *testing.T) {
	// Start a server
	s, root, cleanupS := TestACLServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, acl.PolicyWrite, nil)
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "valid2", policyGood)

	// Upsert the allocation
	state := s.State()
	alloc := mock.Alloc()
	require.NoError(t, state.UpsertJob(1010, alloc.Job))
	require.NoError(t, state.UpsertAllocs(1011, []*structs.Allocation{alloc}))

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
			req := &structs.AllocRestartRequest{
				AllocID: alloc.ID,
				QueryOptions: structs.QueryOptions{
					Namespace: structs.DefaultNamespace,
					AuthToken: c.Token,
					Region:    "global",
				},
			}

			// Fetch the response
			var resp structs.GenericResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Restart", req, &resp)
			require.NotNil(t, err)
			require.Contains(t, err.Error(), c.ExpectedError)
		})
	}
}

// TestAlloc_ExecStreaming asserts that exec task requests are forwarded
// to appropriate server or remote regions
func TestAlloc_ExecStreaming(t *testing.T) {
	t.Parallel()

	////// Nomad clusters topology - not specific to test
	localServer, cleanupLS := TestServer(t, nil)
	defer cleanupLS()

	remoteServer, cleanupRS := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
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
	a.Job.Type = structs.JobTypeBatch
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
	require.Nil(t, localState.UpsertJob(999, a.Job))
	require.Nil(t, localState.UpsertAllocs(1003, []*structs.Allocation{a}))
	remoteState := remoteServer.State()
	require.Nil(t, remoteState.UpsertJob(999, a.Job))
	require.Nil(t, remoteState.UpsertAllocs(1003, []*structs.Allocation{a}))

	// Wait for the client to run the allocation
	testutil.WaitForResult(func() (bool, error) {
		alloc, err := localState.AllocByID(nil, a.ID)
		if err != nil {
			return false, err
		}
		if alloc == nil {
			return false, fmt.Errorf("unknown alloc")
		}
		if alloc.ClientStatus != structs.AllocClientStatusRunning {
			return false, fmt.Errorf("alloc client status: %v", alloc.ClientStatus)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err, "task didn't start yet")
	})

	/////////  Actually run query now
	cases := []struct {
		name string
		rpc  func(string) (structs.StreamingRpcHandler, error)
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
