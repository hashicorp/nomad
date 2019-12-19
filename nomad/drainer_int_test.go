package nomad

import (
	"context"
	"fmt"
	"net/rpc"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/drainer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func allocPromoter(errCh chan<- error, ctx context.Context,
	state *state.StateStore, codec rpc.ClientCodec, nodeID string,
	logger log.Logger) {

	nindex := uint64(1)
	for {
		allocs, index, err := getNodeAllocs(ctx, state, nodeID, nindex)
		if err != nil {
			if err == context.Canceled {
				return
			}

			errCh <- fmt.Errorf("failed to get node allocs: %v", err)
			return
		}
		nindex = index

		// For each alloc that doesn't have its deployment status set, set it
		var updates []*structs.Allocation
		now := time.Now()
		for _, alloc := range allocs {
			if alloc.Job.Type != structs.JobTypeService {
				continue
			}

			if alloc.DeploymentStatus.HasHealth() {
				continue
			}
			newAlloc := alloc.Copy()
			newAlloc.DeploymentStatus = &structs.AllocDeploymentStatus{
				Healthy:   helper.BoolToPtr(true),
				Timestamp: now,
			}
			updates = append(updates, newAlloc)
			logger.Trace("marked deployment health for alloc", "alloc_id", alloc.ID)
		}

		if len(updates) == 0 {
			continue
		}

		// Send the update
		req := &structs.AllocUpdateRequest{
			Alloc:        updates,
			WriteRequest: structs.WriteRequest{Region: "global"},
		}
		var resp structs.GenericResponse
		if err := msgpackrpc.CallWithCodec(codec, "Node.UpdateAlloc", req, &resp); err != nil {
			if ctx.Err() == context.Canceled {
				return
			} else if err != nil {
				errCh <- err
			}
		}
	}
}

// checkAllocPromoter is a small helper to return an error or nil from an error
// chan like the one given to the allocPromoter goroutine.
func checkAllocPromoter(errCh chan error) error {
	select {
	case err := <-errCh:
		return err
	default:
		return nil
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

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
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
	errCh := make(chan error, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go allocPromoter(errCh, ctx, state, codec, n1.ID, s1.logger)
	go allocPromoter(errCh, ctx, state, codec, n2.ID, s1.logger)

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
		if err := checkAllocPromoter(errCh); err != nil {
			return false, err
		}
		node, err := state.NodeByID(nil, n1.ID)
		if err != nil {
			return false, err
		}
		return node.DrainStrategy == nil, fmt.Errorf("has drain strategy still set")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Check we got the right events
	node, err := state.NodeByID(nil, n1.ID)
	require.NoError(err)
	require.Len(node.Events, 3)
	require.Equal(drainer.NodeDrainEventComplete, node.Events[2].Message)
}

func TestDrainer_Simple_ServiceOnly_Deadline(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
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

	// Check we got the right events
	node, err := state.NodeByID(nil, n1.ID)
	require.NoError(err)
	require.Len(node.Events, 3)
	require.Equal(drainer.NodeDrainEventComplete, node.Events[2].Message)
	require.Contains(node.Events[2].Details, drainer.NodeDrainEventDetailDeadlined)
}

func TestDrainer_DrainEmptyNode(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
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

	// Check we got the right events
	node, err := state.NodeByID(nil, n1.ID)
	require.NoError(err)
	require.Len(node.Events, 3)
	require.Equal(drainer.NodeDrainEventComplete, node.Events[2].Message)
}

func TestDrainer_AllTypes_Deadline(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create two nodes, registering the second later
	n1, n2 := mock.Node(), mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Create a service job that runs on just one
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

	// Create a system job
	sysjob := mock.SystemJob()
	req = &structs.JobRegisterRequest{
		Job: sysjob,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)

	// Create a batch job
	bjob := mock.BatchJob()
	bjob.TaskGroups[0].Count = 2
	req = &structs.JobRegisterRequest{
		Job: bjob,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)

	// Wait for the allocations to be placed
	state := s1.State()
	testutil.WaitForResult(func() (bool, error) {
		allocs, err := state.AllocsByNode(nil, n1.ID)
		if err != nil {
			return false, err
		}
		return len(allocs) == 5, fmt.Errorf("got %d allocs", len(allocs))
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Create the second node
	nodeReg = &structs.NodeRegisterRequest{
		Node:         n2,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Drain the node
	drainReq := &structs.NodeUpdateDrainRequest{
		NodeID: n1.ID,
		DrainStrategy: &structs.DrainStrategy{
			DrainSpec: structs.DrainSpec{
				Deadline: 2 * time.Second,
			},
		},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var drainResp structs.NodeDrainUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Wait for the allocs to be replaced
	errCh := make(chan error, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go allocPromoter(errCh, ctx, state, codec, n1.ID, s1.logger)
	go allocPromoter(errCh, ctx, state, codec, n2.ID, s1.logger)

	// Wait for the allocs to be stopped
	var finalAllocs []*structs.Allocation
	testutil.WaitForResult(func() (bool, error) {
		if err := checkAllocPromoter(errCh); err != nil {
			return false, err
		}

		var err error
		finalAllocs, err = state.AllocsByNode(nil, n1.ID)
		if err != nil {
			return false, err
		}
		for _, alloc := range finalAllocs {
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

	// Wait for the allocations to be placed on the other node
	testutil.WaitForResult(func() (bool, error) {
		allocs, err := state.AllocsByNode(nil, n2.ID)
		if err != nil {
			return false, err
		}
		return len(allocs) == 5, fmt.Errorf("got %d allocs", len(allocs))
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Assert that the service finished before the batch and system
	var serviceMax, batchMax uint64 = 0, 0
	for _, alloc := range finalAllocs {
		if alloc.Job.Type == structs.JobTypeService && alloc.ModifyIndex > serviceMax {
			serviceMax = alloc.ModifyIndex
		} else if alloc.Job.Type == structs.JobTypeBatch && alloc.ModifyIndex > batchMax {
			batchMax = alloc.ModifyIndex
		}
	}
	require.True(serviceMax < batchMax)

	// Check we got the right events
	node, err := state.NodeByID(nil, n1.ID)
	require.NoError(err)
	require.Len(node.Events, 3)
	require.Equal(drainer.NodeDrainEventComplete, node.Events[2].Message)
	require.Contains(node.Events[2].Details, drainer.NodeDrainEventDetailDeadlined)
}

// Test that drain is unset when batch jobs naturally finish
func TestDrainer_AllTypes_NoDeadline(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create two nodes, registering the second later
	n1, n2 := mock.Node(), mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Create a service job that runs on just one
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

	// Create a system job
	sysjob := mock.SystemJob()
	req = &structs.JobRegisterRequest{
		Job: sysjob,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)

	// Create a batch job
	bjob := mock.BatchJob()
	bjob.TaskGroups[0].Count = 2
	req = &structs.JobRegisterRequest{
		Job: bjob,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)

	// Wait for the allocations to be placed
	state := s1.State()
	testutil.WaitForResult(func() (bool, error) {
		allocs, err := state.AllocsByNode(nil, n1.ID)
		if err != nil {
			return false, err
		}
		return len(allocs) == 5, fmt.Errorf("got %d allocs", len(allocs))
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Create the second node
	nodeReg = &structs.NodeRegisterRequest{
		Node:         n2,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Drain the node
	drainReq := &structs.NodeUpdateDrainRequest{
		NodeID: n1.ID,
		DrainStrategy: &structs.DrainStrategy{
			DrainSpec: structs.DrainSpec{
				Deadline: 0 * time.Second, // Infinite
			},
		},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var drainResp structs.NodeDrainUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Wait for the allocs to be replaced
	errCh := make(chan error, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go allocPromoter(errCh, ctx, state, codec, n1.ID, s1.logger)
	go allocPromoter(errCh, ctx, state, codec, n2.ID, s1.logger)

	// Wait for the service allocs to be stopped on the draining node
	testutil.WaitForResult(func() (bool, error) {
		allocs, err := state.AllocsByJob(nil, job.Namespace, job.ID, false)
		if err != nil {
			return false, err
		}
		for _, alloc := range allocs {
			if alloc.NodeID != n1.ID {
				continue
			}
			if alloc.DesiredStatus != structs.AllocDesiredStatusStop {
				return false, fmt.Errorf("got desired status %v", alloc.DesiredStatus)
			}
		}
		if err := checkAllocPromoter(errCh); err != nil {
			return false, err
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Mark the batch allocations as finished
	allocs, err := state.AllocsByJob(nil, job.Namespace, bjob.ID, false)
	require.Nil(err)

	var updates []*structs.Allocation
	for _, alloc := range allocs {
		new := alloc.Copy()
		new.ClientStatus = structs.AllocClientStatusComplete
		updates = append(updates, new)
	}
	require.Nil(state.UpdateAllocsFromClient(1000, updates))

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

	// Wait for the service allocations to be placed on the other node
	testutil.WaitForResult(func() (bool, error) {
		allocs, err := state.AllocsByNode(nil, n2.ID)
		if err != nil {
			return false, err
		}
		return len(allocs) == 3, fmt.Errorf("got %d allocs", len(allocs))
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Check we got the right events
	node, err := state.NodeByID(nil, n1.ID)
	require.NoError(err)
	require.Len(node.Events, 3)
	require.Equal(drainer.NodeDrainEventComplete, node.Events[2].Message)
}

func TestDrainer_AllTypes_Deadline_GarbageCollectedNode(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Create two nodes, registering the second later
	n1, n2 := mock.Node(), mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Create a service job that runs on just one
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
	job.CreateIndex = resp.JobModifyIndex

	// Create a system job
	sysjob := mock.SystemJob()
	req = &structs.JobRegisterRequest{
		Job: sysjob,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)
	sysjob.CreateIndex = resp.JobModifyIndex

	// Create a batch job
	bjob := mock.BatchJob()
	bjob.TaskGroups[0].Count = 2
	req = &structs.JobRegisterRequest{
		Job: bjob,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}

	// Fetch the response
	require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	require.NotZero(resp.Index)
	bjob.CreateIndex = resp.JobModifyIndex

	// Wait for the allocations to be placed
	state := s1.State()
	testutil.WaitForResult(func() (bool, error) {
		allocs, err := state.AllocsByNode(nil, n1.ID)
		if err != nil {
			return false, err
		}
		return len(allocs) == 5, fmt.Errorf("got %d allocs", len(allocs))
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Create some old terminal allocs for each job that point at a non-existent
	// node to simulate it being on a GC'd node.
	var badAllocs []*structs.Allocation
	for _, job := range []*structs.Job{job, sysjob, bjob} {
		alloc := mock.Alloc()
		alloc.Namespace = job.Namespace
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.TaskGroup = job.TaskGroups[0].Name
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		alloc.ClientStatus = structs.AllocClientStatusComplete
		badAllocs = append(badAllocs, alloc)
	}
	require.NoError(state.UpsertAllocs(1, badAllocs))

	// Create the second node
	nodeReg = &structs.NodeRegisterRequest{
		Node:         n2,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Drain the node
	drainReq := &structs.NodeUpdateDrainRequest{
		NodeID: n1.ID,
		DrainStrategy: &structs.DrainStrategy{
			DrainSpec: structs.DrainSpec{
				Deadline: 2 * time.Second,
			},
		},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var drainResp structs.NodeDrainUpdateResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Wait for the allocs to be replaced
	errCh := make(chan error, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go allocPromoter(errCh, ctx, state, codec, n1.ID, s1.logger)
	go allocPromoter(errCh, ctx, state, codec, n2.ID, s1.logger)

	// Wait for the allocs to be stopped
	var finalAllocs []*structs.Allocation
	testutil.WaitForResult(func() (bool, error) {
		if err := checkAllocPromoter(errCh); err != nil {
			return false, err
		}

		var err error
		finalAllocs, err = state.AllocsByNode(nil, n1.ID)
		if err != nil {
			return false, err
		}
		for _, alloc := range finalAllocs {
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

	// Wait for the allocations to be placed on the other node
	testutil.WaitForResult(func() (bool, error) {
		allocs, err := state.AllocsByNode(nil, n2.ID)
		if err != nil {
			return false, err
		}
		return len(allocs) == 5, fmt.Errorf("got %d allocs", len(allocs))
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Check we got the right events
	node, err := state.NodeByID(nil, n1.ID)
	require.NoError(err)
	require.Len(node.Events, 3)
	require.Equal(drainer.NodeDrainEventComplete, node.Events[2].Message)
	require.Contains(node.Events[2].Details, drainer.NodeDrainEventDetailDeadlined)
}

// Test that transitions to force drain work.
func TestDrainer_Batch_TransitionToForce(t *testing.T) {
	t.Parallel()

	for _, inf := range []bool{true, false} {
		name := "Infinite"
		if !inf {
			name = "Deadline"
		}
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			s1, cleanupS1 := TestServer(t, nil)
			defer cleanupS1()
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

			// Create a batch job
			bjob := mock.BatchJob()
			bjob.TaskGroups[0].Count = 2
			req := &structs.JobRegisterRequest{
				Job: bjob,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					Namespace: bjob.Namespace,
				},
			}

			// Fetch the response
			var resp structs.JobRegisterResponse
			require.Nil(msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
			require.NotZero(resp.Index)

			// Wait for the allocations to be placed
			state := s1.State()
			testutil.WaitForResult(func() (bool, error) {
				allocs, err := state.AllocsByNode(nil, n1.ID)
				if err != nil {
					return false, err
				}
				return len(allocs) == 2, fmt.Errorf("got %d allocs", len(allocs))
			}, func(err error) {
				t.Fatalf("err: %v", err)
			})

			// Pick the deadline
			deadline := 0 * time.Second
			if !inf {
				deadline = 10 * time.Second
			}

			// Drain the node
			drainReq := &structs.NodeUpdateDrainRequest{
				NodeID: n1.ID,
				DrainStrategy: &structs.DrainStrategy{
					DrainSpec: structs.DrainSpec{
						Deadline: deadline,
					},
				},
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			var drainResp structs.NodeDrainUpdateResponse
			require.Nil(msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

			// Wait for the allocs to be replaced
			errCh := make(chan error, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go allocPromoter(errCh, ctx, state, codec, n1.ID, s1.logger)

			// Make sure the batch job isn't affected
			testutil.AssertUntil(500*time.Millisecond, func() (bool, error) {
				if err := checkAllocPromoter(errCh); err != nil {
					return false, fmt.Errorf("check alloc promoter error: %v", err)
				}

				allocs, err := state.AllocsByNode(nil, n1.ID)
				if err != nil {
					return false, fmt.Errorf("AllocsByNode error: %v", err)
				}
				for _, alloc := range allocs {
					if alloc.DesiredStatus != structs.AllocDesiredStatusRun {
						return false, fmt.Errorf("got status %v", alloc.DesiredStatus)
					}
				}
				return len(allocs) == 2, fmt.Errorf("got %d allocs", len(allocs))
			}, func(err error) {
				t.Fatalf("err: %v", err)
			})

			// Foce drain the node
			drainReq = &structs.NodeUpdateDrainRequest{
				NodeID: n1.ID,
				DrainStrategy: &structs.DrainStrategy{
					DrainSpec: structs.DrainSpec{
						Deadline: -1 * time.Second, // Infinite
					},
				},
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			require.Nil(msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

			// Make sure the batch job is migrated
			testutil.WaitForResult(func() (bool, error) {
				allocs, err := state.AllocsByNode(nil, n1.ID)
				if err != nil {
					return false, err
				}
				for _, alloc := range allocs {
					if alloc.DesiredStatus != structs.AllocDesiredStatusStop {
						return false, fmt.Errorf("got status %v", alloc.DesiredStatus)
					}
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

			// Check we got the right events
			node, err := state.NodeByID(nil, n1.ID)
			require.NoError(err)
			require.Len(node.Events, 4)
			require.Equal(drainer.NodeDrainEventComplete, node.Events[3].Message)
			require.Contains(node.Events[3].Details, drainer.NodeDrainEventDetailDeadlined)
		})
	}
}
