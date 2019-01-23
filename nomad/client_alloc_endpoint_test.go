package nomad

import (
	"fmt"
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
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

func TestClientAllocations_GarbageCollectAll_Local(t *testing.T) {
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
	s := TestServer(t, nil)
	defer s.Shutdown()
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
	s := TestServer(t, nil)
	defer s.Shutdown()
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
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)
	codec := rpcClient(t, s2)

	c, cleanup := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s2.config.RPCAddr.String()}
		c.GCDiskUsageThreshold = 100.0
	})
	defer cleanup()

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
	s := TestServer(t, nil)
	defer s.Shutdown()
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
	s := TestServer(t, nil)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanup := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.config.RPCAddr.String()}
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
	require := require.New(t)

	// Start a server
	s, root := TestACLServer(t, nil)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	// Create a bad token
	policyBad := mock.NamespacePolicy("other", "", []string{acl.NamespaceCapabilityReadFS})
	tokenBad := mock.CreatePolicyAndToken(t, s.State(), 1005, "invalid", policyBad)

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilitySubmitJob})
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
			ExpectedError: structs.ErrUnknownAllocationPrefix,
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: structs.ErrUnknownAllocationPrefix,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request without having a node-id
			req := &structs.AllocSpecificRequest{
				AllocID: uuid.Generate(),
				QueryOptions: structs.QueryOptions{
					AuthToken: c.Token,
					Region:    "global",
					Namespace: structs.DefaultNamespace,
				},
			}

			// Fetch the response
			var resp structs.GenericResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.GarbageCollect", req, &resp)
			require.NotNil(err)
			require.Contains(err.Error(), c.ExpectedError)
		})
	}
}

func TestClientAllocations_GarbageCollect_Remote(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()
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
	s := TestServer(t, nil)
	defer s.Shutdown()
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
	s := TestServer(t, nil)
	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanup := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.config.RPCAddr.String()}
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
	require.Contains(err.Error(), "missing")

	// Fetch the response setting the node id
	req.AllocID = a.ID
	var resp2 cstructs.AllocStatsResponse
	err = msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp2)
	require.Nil(err)
	require.NotNil(resp2.Stats)
}

func TestClientAllocations_Stats_Local_ACL(t *testing.T) {
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

	policyGood := mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob})
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
			ExpectedError: structs.ErrUnknownAllocationPrefix,
		},
		{
			Name:          "root token",
			Token:         root.SecretID,
			ExpectedError: structs.ErrUnknownAllocationPrefix,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {

			// Make the request without having a node-id
			req := &structs.AllocSpecificRequest{
				AllocID: uuid.Generate(),
				QueryOptions: structs.QueryOptions{
					AuthToken: c.Token,
					Region:    "global",
					Namespace: structs.DefaultNamespace,
				},
			}

			// Fetch the response
			var resp cstructs.AllocStatsResponse
			err := msgpackrpc.CallWithCodec(codec, "ClientAllocations.Stats", req, &resp)
			require.NotNil(err)
			require.Contains(err.Error(), c.ExpectedError)
		})
	}
}

func TestClientAllocations_Stats_Remote(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	s2 := TestServer(t, func(c *Config) {
		c.DevDisableBootstrap = true
	})
	defer s2.Shutdown()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)
	codec := rpcClient(t, s2)

	c, cleanup := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s2.config.RPCAddr.String()}
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
