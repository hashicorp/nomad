package nomad

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
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
	now := time.Now()
	nodeA := mock.Node()
	nodeA.Name = "NodeA"
	nodeA.Drain = true
	nodeA.DrainStrategy = &structs.DrainStrategy{
		StartTime: now.UnixNano(),
		Deadline:  5 * time.Second,
	}
	nodeA.SchedulingEligibility = structs.NodeSchedulingIneligible

	nodeB := mock.Node()
	nodeB.Name = "NodeB"

	serviceJob := mock.Job()
	serviceJob.Name = "service-job"
	serviceJob.Type = structs.JobTypeService
	serviceJob.TaskGroups[0].Tasks[0].Resources.CPU = 20
	serviceJob.TaskGroups[0].Tasks[0].Resources.MemoryMB = 20
	serviceJob.TaskGroups[0].Tasks[0].Resources.Networks[0].MBits = 1
	serviceAllocs := make([]*structs.Allocation, serviceJob.TaskGroups[0].Count)
	serviceAllocsMap := make(map[string]*structs.Allocation, len(serviceAllocs))
	for i := range serviceAllocs {
		a := mock.Alloc()
		a.JobID = serviceJob.ID
		a.NodeID = nodeA.ID
		serviceAllocs[i] = a

		// index by ID as well
		serviceAllocsMap[a.ID] = a
	}
	require.Nil(state.UpsertJob(1002, serviceJob))
	require.Nil(state.UpsertAllocs(1003, serviceAllocs))

	systemJob := mock.SystemJob()
	systemJob.Name = "system-job"
	systemJob.Type = structs.JobTypeSystem
	systemAlloc := mock.Alloc()
	systemAlloc.JobID = systemJob.ID
	systemAlloc.NodeID = nodeA.ID
	require.Nil(state.UpsertJob(1004, systemJob))
	require.Nil(state.UpsertAllocs(1005, []*structs.Allocation{systemAlloc}))

	batchJob := mock.Job()
	batchJob.Name = "batch-job"
	batchJob.Type = structs.JobTypeBatch
	batchJob.TaskGroups[0].Tasks[0].Resources.CPU = 20
	batchJob.TaskGroups[0].Tasks[0].Resources.MemoryMB = 20
	batchJob.TaskGroups[0].Tasks[0].Resources.Networks[0].MBits = 1
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

	// Wait for allocs to go DesiredStatus=stop and transition them to done (fake being a well-behaved client)
	serviceDone, batchDone := 0, 0
	testutil.WaitForResult(func() (bool, error) {
		snap, _ := state.Snapshot()
		index, err := state.Index("allocs")
		require.Nil(err)
		iter, err := snap.Allocs(nil)
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
				if deadlineReached && alloc.DesiredStatus == structs.AllocDesiredStatusStop && alloc.ClientStatus != structs.AllocClientStatusComplete {
					batchDone++
					t.Logf("batch alloc %q completed", alloc.ID)
					alloc.ClientStatus = structs.AllocClientStatusComplete
					//FIXME valid index?!
					require.Nil(snap.UpsertAllocs(index+1, []*structs.Allocation{alloc}))
				}
			case serviceJob.ID:
				if alloc.DesiredStatus == structs.AllocDesiredStatusStop && alloc.ClientStatus != structs.AllocClientStatusComplete {
					// Alloc is being drained so fake a replacement.
					serviceDone++
					alloc.ClientStatus = structs.AllocClientStatusComplete

					//FIXME valid index?!
					require.Nil(snap.UpsertAllocs(index+1, []*structs.Allocation{alloc}))
					t.Logf("service alloc %q drained", alloc.ID)
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
		return success, fmt.Errorf("services draining: %d/%d  --  batch draining: %d/%d",
			serviceDone, serviceJob.TaskGroups[0].Count, batchDone, batchJob.TaskGroups[0].Count)
	}, func(err error) {
		t.Errorf("error waiting for all non-system allocs to be drained: %v", err)
	})

	// Wait for all service allocs to be replaced
	allocs := make([]*structs.Allocation, 0, 100)
	testutil.WaitForResult(func() (bool, error) {
		iter, err := state.Allocs(nil)
		if err != nil {
			t.Fatalf("error iterating over allocs: %v", err)
		}

		allocs = allocs[:0]
		replacements := make([]*structs.Allocation, 0, serviceJob.TaskGroups[0].Count)
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}

			alloc := raw.(*structs.Allocation)
			allocs = append(allocs, alloc)
			if _, ok := serviceAllocsMap[alloc.PreviousAllocation]; ok {
				replacements = append(replacements, alloc)
			}
		}

		success := len(replacements) == serviceJob.TaskGroups[0].Count
		if success {
			return success, nil
		}
		return success, fmt.Errorf("replaced %d/%d allocs", len(replacements), serviceJob.TaskGroups[0].Count)
	}, func(err error) {
		t.Errorf("error waiting for replacements: %v", err)
	})

	for _, alloc := range allocs {
		t.Logf("job: %s alloc: %s desired: %s actual: %s replaces: %s", alloc.Job.Name, alloc.ID, alloc.DesiredStatus, alloc.ClientStatus, alloc.PreviousAllocation)
	}

	iter, err := state.Evals(nil)
	require.Nil(err)

	evals := map[string]*structs.Evaluation{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		eval := raw.(*structs.Evaluation)
		evals[eval.ID] = eval
	}

	for _, eval := range evals {
		if eval.Status == structs.EvalStatusBlocked {
			blocked := evals[eval.PreviousEval]
			t.Logf("Blocked evaluation: %q - %v\n%s\n--blocked %q - %v\n%s", eval.ID, eval.StatusDescription, pretty.Sprint(eval), blocked.ID, blocked.StatusDescription, pretty.Sprint(blocked))
		}
	}
}
