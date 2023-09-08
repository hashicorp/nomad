// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"fmt"
	"testing"
	"time"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/drainer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

// allocClientStateSimulator simulates the updates in state from the
// client. service allocations that are new on the server get marked with
// healthy deployments, and service allocations that are DesiredStatus=stop on
// the server get updates with terminal client status.
func allocClientStateSimulator(t *testing.T, errCh chan<- error, ctx context.Context,
	srv *Server, nodeID string, logger log.Logger) {

	codec := rpcClient(t, srv)
	store := srv.State()

	nindex := uint64(1)
	for {
		allocs, index, err := getNodeAllocs(ctx, store, nodeID, nindex)
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

			switch alloc.DesiredStatus {
			case structs.AllocDesiredStatusRun:
				if alloc.DeploymentStatus.HasHealth() {
					continue // only update to healthy once
				}
				newAlloc := alloc.Copy()
				newAlloc.DeploymentStatus = &structs.AllocDeploymentStatus{
					Healthy:   pointer.Of(true),
					Timestamp: now,
				}
				updates = append(updates, newAlloc)
				logger.Trace("marking deployment health for alloc", "alloc_id", alloc.ID)

			case structs.AllocDesiredStatusStop, structs.AllocDesiredStatusEvict:
				if alloc.ClientStatus == structs.AllocClientStatusComplete {
					continue // only update to complete once
				}
				newAlloc := alloc.Copy()
				newAlloc.ClientStatus = structs.AllocClientStatusComplete
				updates = append(updates, newAlloc)
				logger.Trace("marking alloc complete", "alloc_id", alloc.ID)
			}

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
// chan like the one given to the allocClientStateSimulator goroutine.
func checkAllocPromoter(errCh chan error) error {
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func getNodeAllocs(ctx context.Context, store *state.StateStore, nodeID string, index uint64) ([]*structs.Allocation, uint64, error) {
	resp, index, err := store.BlockingQuery(getNodeAllocsImpl(nodeID), index, ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}

	return resp.([]*structs.Allocation), index, nil
}

func getNodeAllocsImpl(nodeID string) func(ws memdb.WatchSet, store *state.StateStore) (interface{}, uint64, error) {
	return func(ws memdb.WatchSet, store *state.StateStore) (interface{}, uint64, error) {
		// Capture all the allocations
		allocs, err := store.AllocsByNode(ws, nodeID)
		if err != nil {
			return nil, 0, err
		}

		// Use the last index that affected the jobs table
		index, err := store.Index("allocs")
		if err != nil {
			return nil, index, err
		}

		return allocs, index, nil
	}
}

func TestDrainer_Simple_ServiceOnly(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSrv := TestServer(t, nil)
	defer cleanupSrv()
	codec := rpcClient(t, srv)
	testutil.WaitForLeader(t, srv.RPC)
	store := srv.State()

	// Create a node
	n1 := mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Create a job that runs on that node
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp structs.JobRegisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.Positive(t, resp.Index)

	// Wait for the two allocations to be placed
	waitForPlacedAllocs(t, store, n1.ID, 2)

	// Create the second node
	n2 := mock.Node()
	nodeReg = &structs.NodeRegisterRequest{
		Node:         n2,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

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
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Setup client simulator
	errCh := make(chan error, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go allocClientStateSimulator(t, errCh, ctx, srv, n1.ID, srv.logger)
	go allocClientStateSimulator(t, errCh, ctx, srv, n2.ID, srv.logger)

	// Wait for the allocs to be replaced
	waitForAllocsStop(t, store, n1.ID, nil)
	waitForPlacedAllocs(t, store, n2.ID, 2)

	// Wait for the node drain to be marked complete with the events we expect
	waitForNodeDrainComplete(t, store, n1.ID, errCh, 3, "")
}

func TestDrainer_Simple_ServiceOnly_Deadline(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSrv := TestServer(t, nil)
	defer cleanupSrv()
	codec := rpcClient(t, srv)
	testutil.WaitForLeader(t, srv.RPC)
	store := srv.State()

	// Create a node
	n1 := mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Create a job that runs on it
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
	var resp structs.JobRegisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.Positive(t, resp.Index)

	// Wait for the two allocations to be placed
	waitForPlacedAllocs(t, store, n1.ID, 2)

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
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Wait for the allocs to be stopped (but not replaced)
	waitForAllocsStop(t, store, n1.ID, nil)

	// Wait for the node drain to be marked complete with the events we expect
	waitForNodeDrainComplete(t, store, n1.ID, nil, 3, drainer.NodeDrainEventDetailDeadlined)
}

func TestDrainer_DrainEmptyNode(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSrv := TestServer(t, nil)
	defer cleanupSrv()
	codec := rpcClient(t, srv)
	testutil.WaitForLeader(t, srv.RPC)
	store := srv.State()

	// Create an empty node
	n1 := mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

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
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Wait for the node drain to be marked complete with the events we expect
	waitForNodeDrainComplete(t, store, n1.ID, nil, 3, "")
}

func TestDrainer_AllTypes_Deadline(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSrv := TestServer(t, nil)
	defer cleanupSrv()
	codec := rpcClient(t, srv)
	testutil.WaitForLeader(t, srv.RPC)
	store := srv.State()

	// Create a node
	n1 := mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Create a service job that runs on it
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp structs.JobRegisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.Positive(t, resp.Index)

	// Create a system job
	sysjob := mock.SystemJob()
	req = &structs.JobRegisterRequest{
		Job: sysjob,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.Positive(t, resp.Index)

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
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.Positive(t, resp.Index)

	// Wait for all the allocations to be placed
	waitForPlacedAllocs(t, store, n1.ID, 5)

	// Create a second node
	n2 := mock.Node()
	nodeReg = &structs.NodeRegisterRequest{
		Node:         n2,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Drain the first node
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
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Setup client simulator
	errCh := make(chan error, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go allocClientStateSimulator(t, errCh, ctx, srv, n1.ID, srv.logger)
	go allocClientStateSimulator(t, errCh, ctx, srv, n2.ID, srv.logger)

	// Wait for allocs to be replaced
	finalAllocs := waitForAllocsStop(t, store, n1.ID, nil)
	waitForPlacedAllocs(t, store, n2.ID, 5)

	// Assert that the service finished before the batch and system
	var serviceMax, batchMax uint64 = 0, 0
	for _, alloc := range finalAllocs {
		if alloc.Job.Type == structs.JobTypeService && alloc.ModifyIndex > serviceMax {
			serviceMax = alloc.ModifyIndex
		} else if alloc.Job.Type == structs.JobTypeBatch && alloc.ModifyIndex > batchMax {
			batchMax = alloc.ModifyIndex
		}
	}
	must.Less(t, batchMax, serviceMax)

	// Wait for the node drain to be marked complete with the events we expect
	waitForNodeDrainComplete(t, store, n1.ID, nil, 3, drainer.NodeDrainEventDetailDeadlined)
}

// Test that drain is unset when batch jobs naturally finish
func TestDrainer_AllTypes_NoDeadline(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSrv := TestServer(t, nil)
	defer cleanupSrv()
	codec := rpcClient(t, srv)
	testutil.WaitForLeader(t, srv.RPC)
	store := srv.State()

	// Create two nodes, registering the second later
	n1 := mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Create a service job
	job := mock.Job()
	job.TaskGroups[0].Count = 2
	req := &structs.JobRegisterRequest{
		Job: job,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	var resp structs.JobRegisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.Positive(t, resp.Index)

	// Create a system job
	sysjob := mock.SystemJob()
	req = &structs.JobRegisterRequest{
		Job: sysjob,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			Namespace: job.Namespace,
		},
	}
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.Positive(t, resp.Index)

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
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.Positive(t, resp.Index)

	// Wait for all the allocations to be placed
	waitForPlacedAllocs(t, store, n1.ID, 5)

	// Create a second node
	n2 := mock.Node()
	nodeReg = &structs.NodeRegisterRequest{
		Node:         n2,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Drain the first node
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
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Setup client simulator
	errCh := make(chan error, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go allocClientStateSimulator(t, errCh, ctx, srv, n1.ID, srv.logger)
	go allocClientStateSimulator(t, errCh, ctx, srv, n2.ID, srv.logger)

	// Wait for the service allocs (only) to be stopped on the draining node
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		allocs, err := store.AllocsByJob(nil, job.Namespace, job.ID, false)
		must.NoError(t, err)
		for _, alloc := range allocs {
			if alloc.NodeID != n1.ID {
				continue
			}
			if alloc.DesiredStatus != structs.AllocDesiredStatusStop {
				return fmt.Errorf("got desired status %v", alloc.DesiredStatus)
			}
		}
		return checkAllocPromoter(errCh)
	}),
		wait.Timeout(10*time.Second),
		wait.Gap(100*time.Millisecond),
	))

	// Mark the batch allocations as finished
	allocs, err := store.AllocsByJob(nil, job.Namespace, bjob.ID, false)
	must.NoError(t, err)

	var updates []*structs.Allocation
	for _, alloc := range allocs {
		new := alloc.Copy()
		new.ClientStatus = structs.AllocClientStatusComplete
		updates = append(updates, new)
	}

	batchDoneReq := &structs.AllocUpdateRequest{
		Alloc:        updates,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	err = msgpackrpc.CallWithCodec(codec, "Node.UpdateAlloc", batchDoneReq, &resp)
	must.NoError(t, err)

	// Wait for the service allocations to be replaced
	waitForPlacedAllocs(t, store, n2.ID, 3)

	// Wait for the node drain to be marked complete with the events we expect
	waitForNodeDrainComplete(t, store, n1.ID, errCh, 3, "")
}

func TestDrainer_AllTypes_Deadline_GarbageCollectedNode(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSrv := TestServer(t, nil)
	defer cleanupSrv()
	codec := rpcClient(t, srv)
	testutil.WaitForLeader(t, srv.RPC)
	store := srv.State()

	// Create a node
	n1 := mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

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
	var resp structs.JobRegisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.Positive(t, resp.Index)
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
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.Positive(t, resp.Index)
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
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
	must.Positive(t, resp.Index)
	bjob.CreateIndex = resp.JobModifyIndex

	// Wait for the allocations to be placed
	waitForPlacedAllocs(t, store, n1.ID, 5)

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
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 1, badAllocs))

	// Create the second node
	n2 := mock.Node()
	nodeReg = &structs.NodeRegisterRequest{
		Node:         n2,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	// Drain the first node
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
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Setup client simulator
	errCh := make(chan error, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go allocClientStateSimulator(t, errCh, ctx, srv, n1.ID, srv.logger)
	go allocClientStateSimulator(t, errCh, ctx, srv, n2.ID, srv.logger)

	// Wait for the allocs to be replaced
	waitForAllocsStop(t, store, n1.ID, errCh)
	waitForPlacedAllocs(t, store, n2.ID, 5)

	// Wait for the node drain to be marked complete with the events we expect
	waitForNodeDrainComplete(t, store, n1.ID, errCh, 3, drainer.NodeDrainEventDetailDeadlined)
}

// TestDrainer_MultipleNSes_ServiceOnly asserts that all jobs on an alloc, even
// when they belong to different namespaces and share the same ID
func TestDrainer_MultipleNSes_ServiceOnly(t *testing.T) {
	ci.Parallel(t)

	srv, cleanupSrv := TestServer(t, nil)
	defer cleanupSrv()
	codec := rpcClient(t, srv)
	testutil.WaitForLeader(t, srv.RPC)
	store := srv.State()

	// Create a node
	n1 := mock.Node()
	nodeReg := &structs.NodeRegisterRequest{
		Node:         n1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nodeResp structs.NodeUpdateResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

	nsrv, ns2 := mock.Namespace(), mock.Namespace()
	nses := []*structs.Namespace{nsrv, ns2}
	nsReg := &structs.NamespaceUpsertRequest{
		Namespaces:   nses,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var nsResp structs.GenericResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Namespace.UpsertNamespaces", nsReg, &nsResp))

	for _, ns := range nses {
		// Create a job for each namespace
		job := mock.Job()
		job.ID = "example"
		job.Name = "example"
		job.Namespace = ns.Name
		job.TaskGroups[0].Count = 1
		req := &structs.JobRegisterRequest{
			Job: job,
			WriteRequest: structs.WriteRequest{
				Region:    "global",
				Namespace: job.Namespace,
			},
		}

		// Fetch the response
		var resp structs.JobRegisterResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
		must.Positive(t, resp.Index)
	}

	// Wait for the two allocations to be placed
	waitForPlacedAllocs(t, store, n1.ID, 2)

	// Create the second node
	n2 := mock.Node()
	nodeReg = &structs.NodeRegisterRequest{
		Node:         n2,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

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
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

	// Setup client simulator
	errCh := make(chan error, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go allocClientStateSimulator(t, errCh, ctx, srv, n1.ID, srv.logger)
	go allocClientStateSimulator(t, errCh, ctx, srv, n2.ID, srv.logger)

	// Wait for the allocs to be replaced
	waitForAllocsStop(t, store, n1.ID, errCh)
	waitForPlacedAllocs(t, store, n2.ID, 2)

	// Wait for the node drain to be marked complete with the events we expect
	waitForNodeDrainComplete(t, store, n1.ID, errCh, 3, "")
}

// Test that transitions to force drain work.
func TestDrainer_Batch_TransitionToForce(t *testing.T) {
	ci.Parallel(t)

	for _, inf := range []bool{true, false} {
		name := "Infinite"
		if !inf {
			name = "Deadline"
		}
		t.Run(name, func(t *testing.T) {
			srv, cleanupSrv := TestServer(t, nil)
			defer cleanupSrv()
			codec := rpcClient(t, srv)
			testutil.WaitForLeader(t, srv.RPC)
			store := srv.State()

			// Create a node
			n1 := mock.Node()
			nodeReg := &structs.NodeRegisterRequest{
				Node:         n1,
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			var nodeResp structs.NodeUpdateResponse
			must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.Register", nodeReg, &nodeResp))

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
			must.NoError(t, msgpackrpc.CallWithCodec(codec, "Job.Register", req, &resp))
			must.Positive(t, resp.Index)

			// Wait for the allocations to be placed
			waitForPlacedAllocs(t, store, n1.ID, 2)

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
			must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", drainReq, &drainResp))

			// Setup client simulator
			errCh := make(chan error, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go allocClientStateSimulator(t, errCh, ctx, srv, n1.ID, srv.logger)

			// Make sure the batch job isn't affected
			must.Wait(t, wait.ContinualSuccess(wait.ErrorFunc(func() error {
				if err := checkAllocPromoter(errCh); err != nil {
					return fmt.Errorf("check alloc promoter error: %v", err)
				}

				allocs, err := store.AllocsByNode(nil, n1.ID)
				must.NoError(t, err)
				for _, alloc := range allocs {
					if alloc.DesiredStatus != structs.AllocDesiredStatusRun {
						return fmt.Errorf("got status %v", alloc.DesiredStatus)
					}
				}
				if len(allocs) != 2 {
					return fmt.Errorf("expected 2 allocs but got %d", len(allocs))
				}
				return nil
			}),
				wait.Timeout(500*time.Millisecond),
				wait.Gap(50*time.Millisecond),
			))

			// Force drain the node
			drainReq = &structs.NodeUpdateDrainRequest{
				NodeID: n1.ID,
				DrainStrategy: &structs.DrainStrategy{
					DrainSpec: structs.DrainSpec{
						Deadline: -1 * time.Second, // Infinite
					},
				},
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			must.NoError(t, msgpackrpc.CallWithCodec(
				codec, "Node.UpdateDrain", drainReq, &drainResp))

			// Make sure the batch job is migrated
			waitForAllocsStop(t, store, n1.ID, errCh)

			// Wait for the node drain to be marked complete with the events we expect
			waitForNodeDrainComplete(t, store, n1.ID, errCh, 4,
				drainer.NodeDrainEventDetailDeadlined)

		})
	}
}

// waitForNodeDrainComplete is a test helper that verifies the node drain has
// been removed and that the expected Node events have been written
func waitForNodeDrainComplete(t *testing.T, store *state.StateStore, nodeID string,
	errCh chan error, expectEvents int, expectDetail string) {
	t.Helper()

	var node *structs.Node

	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		if err := checkAllocPromoter(errCh); err != nil {
			return err
		}
		node, _ = store.NodeByID(nil, nodeID)
		if node.DrainStrategy != nil {
			return fmt.Errorf("has drain strategy still set")
		}
		// sometimes test gets a duplicate node drain complete event
		if len(node.Events) < expectEvents {
			return fmt.Errorf(
				"did not get enough events (expected %d): %v", expectEvents, node.Events)
		}
		return nil
	}),
		wait.Timeout(10*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	must.Eq(t, drainer.NodeDrainEventComplete, node.Events[expectEvents-1].Message)
	if expectDetail != "" {
		must.MapContainsKey(t, node.Events[expectEvents-1].Details, expectDetail,
			must.Sprintf("%#v", node.Events[expectEvents-1].Details),
		)
	}
}

func waitForPlacedAllocs(t *testing.T, store *state.StateStore, nodeID string, count int) {
	t.Helper()
	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			allocs, err := store.AllocsByNode(nil, nodeID)
			must.NoError(t, err)
			return len(allocs) == count
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(50*time.Millisecond),
	))
}

// waitForAllocsStop waits for all allocs on the node to be stopped
func waitForAllocsStop(t *testing.T, store *state.StateStore, nodeID string, errCh chan error) []*structs.Allocation {
	t.Helper()
	var finalAllocs []*structs.Allocation
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(func() error {
			if err := checkAllocPromoter(errCh); err != nil {
				return err
			}

			var err error
			finalAllocs, err = store.AllocsByNode(nil, nodeID)
			must.NoError(t, err)
			for _, alloc := range finalAllocs {
				if alloc.DesiredStatus != structs.AllocDesiredStatusStop {
					return fmt.Errorf("expected stop but got %s", alloc.DesiredStatus)
				}
			}
			return nil
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(50*time.Millisecond),
	))

	return finalAllocs
}
