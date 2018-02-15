package nomad

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// TestNodeDrainer_SimpleDrain asserts that draining when there are two nodes
// moves allocs from the draining node to the other node.
func TestNodeDrainer_SimpleDrain(t *testing.T) {
	require := require.New(t)
	server := testServer(t, nil)
	defer server.Shutdown()

	state := server.fsm.State()

	// Setup 2 Nodes: A & B; A has allocs and is draining
	nodeA := mock.Node()
	nodeA.Name = "NodeA"
	nodeA.Drain = true
	nodeA.DrainStrategy = &structs.DrainStrategy{
		StartTime: time.Now().UnixNano(),
		Deadline:  time.Second,
	}

	nodeB := mock.Node()
	nodeB.Name = "NodeB"

	serviceJob := mock.Job()
	serviceJob.Type = structs.JobTypeService
	serviceAllocs := make([]*structs.Allocation, serviceJob.TaskGroups[0].Count)
	for i := range serviceAllocs {
		a := mock.Alloc()
		a.JobID = serviceJob.ID
		a.NodeID = nodeA.ID
		serviceAllocs[i] = a
	}
	require.Nil(state.UpsertJob(1002, serviceJob))
	require.Nil(state.UpsertAllocs(1003, serviceAllocs))

	systemJob := mock.SystemJob()
	systemJob.Type = structs.JobTypeSystem
	systemAlloc := mock.Alloc()
	systemAlloc.JobID = systemJob.ID
	systemAlloc.NodeID = nodeA.ID
	require.Nil(state.UpsertJob(1004, systemJob))
	require.Nil(state.UpsertAllocs(1005, []*structs.Allocation{systemAlloc}))

	batchJob := mock.Job()
	batchJob.Type = structs.JobTypeBatch
	batchAllocs := make([]*structs.Allocation, batchJob.TaskGroups[0].Count)
	for i := range batchAllocs {
		a := mock.Alloc()
		a.JobID = batchJob.ID
		a.NodeID = nodeA.ID
		batchAllocs[i] = a
	}
	require.Nil(state.UpsertJob(1006, batchJob))
	require.Nil(state.UpsertAllocs(1007, batchAllocs))

	// Upsert Nodes last to trigger draining
	require.Nil(state.UpsertNode(9001, nodeB))
	require.Nil(state.UpsertNode(9000, nodeA))

	//TODO watch for allocs to go DesiredStatus=stop and transition them to done
	serviceDone, batchDone := 0, 0
	testutil.WaitForResult(func() (bool, error) {
		iter, err := state.Allocs(nil)
		require.Nil(err)

		deadlineReached := time.Now().After(nodeA.DrainStrategy.DeadlineTime())

		for {
			raw := iter.Next()
			if raw == nil {
				break
			}

			alloc := raw.(*structs.Allocation)
			switch alloc.JobID {
			case systemJob.ID:
				if alloc.DesiredStatus != structs.AllocDesiredStatusRun {
					t.Fatalf("system alloc %q told to stop: %q", alloc.ID, alloc.DesiredStatus)
				}
			case batchJob.ID:
				if deadlineReached && alloc.DesiredStatus == structs.AllocDesiredStatusStop && alloc.DeploymentStatus == nil {
					batchDone++
					t.Logf("batch alloc %q drained", alloc.ID)
					alloc.ClientStatus = structs.AllocClientStatusComplete
					alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
						Healthy: helper.BoolToPtr(true),
					}
					//FIXME valid index?!
					require.Nil(state.UpsertAllocs(9999, []*structs.Allocation{alloc}))
				}
			case serviceJob.ID:
				if alloc.DesiredStatus == structs.AllocDesiredStatusStop && alloc.DeploymentStatus == nil {
					serviceDone++
					t.Logf("service alloc %q drained", alloc.ID)
					alloc.ClientStatus = structs.AllocClientStatusComplete
					alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
						Healthy: helper.BoolToPtr(true),
					}
					//FIXME valid index?!
					require.Nil(state.UpsertAllocs(9999, []*structs.Allocation{alloc}))
				}
			default:
				t.Fatalf("unknown alloc job id: %q", alloc.JobID)
			}
		}

		// success means all jobs have been migrated
		success := serviceDone == serviceJob.TaskGroups[0].Count && batchDone == batchJob.TaskGroups[0].Count
		if success {
			return success, nil
		}
		return success, fmt.Errorf("services drained: %d/%d  --  batch drained: %d/%d",
			serviceDone, serviceJob.TaskGroups[0].Count, batchDone, batchJob.TaskGroups[0].Count)
	}, func(err error) {
		t.Fatalf("error waiting for all non-system allocs to be drained: %v", err)
	})

	//TODO watch for nodeA to be done draining

	time.Sleep(time.Second)
}
