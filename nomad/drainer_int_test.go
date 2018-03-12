package nomad

import (
	"context"
	"fmt"
	"log"
	"net/rpc"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func allocPromoter(t *testing.T, ctx context.Context,
	state *state.StateStore, codec rpc.ClientCodec, nodeID string,
	logger *log.Logger) {
	t.Helper()

	nindex := uint64(1)
	for {
		allocs, index, err := getNodeAllocs(ctx, state, nodeID, nindex)
		if err != nil {
			if err == context.Canceled {
				return
			}

			t.Fatalf("failed to get node allocs: %v", err)
		}
		nindex = index

		// For each alloc that doesn't have its deployment status set, set it
		var updates []*structs.Allocation
		for _, alloc := range allocs {
			if alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Healthy != nil {
				continue
			}

			newAlloc := alloc.Copy()
			newAlloc.DeploymentStatus = &structs.AllocDeploymentStatus{
				Healthy: helper.BoolToPtr(true),
			}
			updates = append(updates, newAlloc)
			logger.Printf("Marked deployment health for alloc %q", alloc.ID)
		}

		if len(updates) == 0 {
			continue
		}

		// Send the update
		req := &structs.AllocUpdateRequest{
			Alloc:        updates,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.NodeAllocsResponse
		require.Nil(t, msgpackrpc.CallWithCodec(codec, "Node.UpdateAlloc", req, &resp))
	}
}

func getNodeAllocs(ctx context.Context, state *state.StateStore, nodeID string, index uint64) ([]*structs.Allocation, uint64, error) {
	resp, index, err := state.BlockingQuery(getNodeAllocsImpl(nodeID), index, ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}

	return resp.([]*structs.Allocation), index, nil
}

func getNodeAllocsImpl(nodeID string) func(ws memdb.WatchSet, state *state.StateStore) (interface{}, uint64, error) {
	return func(ws memdb.WatchSet, state *state.StateStore) (interface{}, uint64, error) {
		// Capture all the allocations
		allocs, err := state.AllocsByNode(ws, nodeID)
		if err != nil {
			return nil, 0, err
		}

		// Use the last index that affected the jobs table
		index, err := state.Index("allocs")
		if err != nil {
			return nil, index, err
		}

		return allocs, index, nil
	}
}

func TestDrainer_Simple_ServiceOnly(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create two nodes
	n1, n2 := mock.Node(), mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Create a job that runs on just one
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)

	// Wait for the two allocations to be placed
	state := s1.State()
	testutil.WaitForResult(func() (bool, error) {
		allocs, err := state.AllocsByJob(nil, job.Namespace, job.ID, false)
		if err != nil {
			return false, err
		}
		return len(allocs) == 2, fmt.Errorf("got %d allocs", len(allocs))
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Create the second node
	nodeReg = &structs.NodeRegisterRequest{
		Node:         n2,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Drain the first node
	drainReq := &structs.NodeUpdateDrainRequest{
		NodeID: n1.ID,
		DrainStrategy: &structs.DrainStrategy{
			DrainSpec: structs.DrainSpec{
				Deadline: 10 * time.Minute,
			},
		},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var drainResp structs.NodeDrainUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Wait for the allocs to be replaced
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go allocPromoter(t, ctx, state, codec, n1.ID, s1.logger)
	go allocPromoter(t, ctx, state, codec, n2.ID, s1.logger)

	testutil.WaitForResult(func() (bool, error) {
		allocs, err := state.AllocsByNode(nil, n2.ID)
		if err != nil {
			return false, err
		}
		return len(allocs) == 2, fmt.Errorf("got %d allocs", len(allocs))
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Check that the node drain is removed
	testutil.WaitForResult(func() (bool, error) {
		node, err := state.NodeByID(nil, n1.ID)
		if err != nil {
			return false, err
		}
		return node.DrainStrategy == nil, fmt.Errorf("has drain strategy still set")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestDrainer_Simple_ServiceOnly_Deadline(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create a node
	n1 := mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Create a job that runs on just one
	job := mock.Job()
	job.Update = *structs.DefaultUpdateStrategy
	job.Update.Stagger = 30 * time.Second
	job.TaskGroups[0].Count = 2
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	var resp structs.JobRegisterResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)

	// Wait for the two allocations to be placed
	state := s1.State()
	testutil.WaitForResult(func() (bool, error) {
		allocs, err := state.AllocsByJob(nil, job.Namespace, job.ID, false)
		if err != nil {
			return false, err
		}
		return len(allocs) == 2, fmt.Errorf("got %d allocs", len(allocs))
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Drain the node
	drainReq := &structs.NodeUpdateDrainRequest{
		NodeID: n1.ID,
		DrainStrategy: &structs.DrainStrategy{
			DrainSpec: structs.DrainSpec{
				Deadline: 1 * time.Second,
			},
		},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var drainResp structs.NodeDrainUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Wait for the allocs to be stopped
	testutil.WaitForResult(func() (bool, error) {
		allocs, err := state.AllocsByNode(nil, n1.ID)
		if err != nil {
			return false, err
		}
		for _, alloc := range allocs {
			if alloc.DesiredStatus != structs.AllocDesiredStatusStop {
				return false, fmt.Errorf("got desired status %v", alloc.DesiredStatus)
			}
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Check that the node drain is removed
	testutil.WaitForResult(func() (bool, error) {
		node, err := state.NodeByID(nil, n1.ID)
		if err != nil {
			return false, err
		}
		return node.DrainStrategy == nil, fmt.Errorf("has drain strategy still set")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestDrainer_DrainEmptyNode(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	s1 := TestServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create a node
	n1 := mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Drain the node
	drainReq := &structs.NodeUpdateDrainRequest{
		NodeID: n1.ID,
		DrainStrategy: &structs.DrainStrategy{
			DrainSpec: structs.DrainSpec{
				Deadline: 10 * time.Minute,
			},
		},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var drainResp structs.NodeDrainUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Check that the node drain is removed
	state := s1.State()
	testutil.WaitForResult(func() (bool, error) {
		node, err := state.NodeByID(nil, n1.ID)
		if err != nil {
			return false, err
		}
		return node.DrainStrategy == nil, fmt.Errorf("has drain strategy still set")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}
