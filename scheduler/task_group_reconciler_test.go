package scheduler

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// TODO: Create mock resultMediator.
type mockResultMediator struct{}

var (
	util = &tgrTestUtil{}

	running  = structs.AllocClientStatusRunning
	complete = structs.AllocClientStatusComplete
	failed   = structs.AllocClientStatusFailed
	unknown  = structs.AllocClientStatusUnknown
	pending  = structs.AllocClientStatusPending
	run      = structs.AllocDesiredStatusRun
	stop     = structs.AllocDesiredStatusStop

	ready1        = "ready1"
	ready2        = "ready2"
	ready3        = "ready3"
	disconnected1 = "disconnected1"
)

// TODO: Add helper methods for generating well-known and ad-hoc allocs
type tgrTestUtil struct{}

func (ttu *tgrTestUtil) Job() *structs.Job {
	job := mock.Job()
	job.ID = "my-job"
	job.Name = job.ID
	return job
}

func (ttu *tgrTestUtil) Nodes(nodeCount int, disconnectedCount int) map[string]*structs.Node {
	nodes := map[string]*structs.Node{}

	for i := 1; i <= nodeCount; i++ {
		ready := mock.Node()
		ready.Name = fmt.Sprintf("ready%d", i)
		ready.ID = ready.Name
		ready.Status = structs.NodeStatusReady
		nodes[ready.Name] = ready
	}

	if disconnectedCount > 0 {
		for i := disconnectedCount; i >= nodeCount-disconnectedCount; i-- {
			delete(nodes, fmt.Sprintf("ready%d", i))
			disconnected := mock.Node()
			disconnected.Name = fmt.Sprintf("disconnected%d", i)
			disconnected.ID = disconnected.Name
			disconnected.Status = structs.NodeStatusDisconnected
		}

	}

	return nodes
}

func (ttu *tgrTestUtil) MakeAllocSet(allocs []*structs.Allocation) allocSet {
	set := allocSet{}
	for _, alloc := range allocs {
		set[alloc.ID] = alloc
	}

	return set
}

func (ttu *tgrTestUtil) SetIDs(allocs []*structs.Allocation) {
	for _, alloc := range allocs {
		alloc.ID = uuid.Generate()
		alloc.JobID = alloc.Job.ID
	}
}

func (ttu *tgrTestUtil) SetTaskGroup(taskGroupName string, allocs []*structs.Allocation) {
	for _, alloc := range allocs {
		alloc.TaskGroup = taskGroupName
	}
}

func TestTaskGroupReconciler_BuildsCandidates_ByAllocName(t *testing.T) {
	logger := hclog.NewInterceptLogger(hclog.DefaultOptions)

	job := util.Job()
	job.TaskGroups[0].Count = 3
	updatedJob := job.Copy()
	updatedJob.Version = updatedJob.Version + 1

	type testCase struct {
		name   string
		nodes  map[string]*structs.Node
		allocs []*structs.Allocation
	}

	// TODO: refactor to testutil helper methods for creating well-known alloc sets.
	testCases := []testCase{
		{
			name:  "single-job-version",
			nodes: util.Nodes(3, 0),
			allocs: []*structs.Allocation{
				{Name: "my-job.web[0]", NodeID: ready1, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3, Job: job, ClientStatus: running, DesiredStatus: run},
			},
		},
		{
			name:  "multiple-job-versions",
			nodes: util.Nodes(3, 0),
			allocs: []*structs.Allocation{
				{Name: "my-job.web[0]", NodeID: ready1, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[0]", NodeID: ready1, Job: updatedJob, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2, Job: updatedJob, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3, Job: updatedJob, ClientStatus: running, DesiredStatus: run},
			},
		},
		{
			name:  "complete-and-failed",
			nodes: util.Nodes(3, 0),
			allocs: []*structs.Allocation{
				{Name: "my-job.web[0]", NodeID: ready1, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[0]", NodeID: ready1, Job: job, ClientStatus: complete, DesiredStatus: stop},
				{Name: "my-job.web[1]", NodeID: ready2, Job: job, ClientStatus: complete, DesiredStatus: stop},
				{Name: "my-job.web[2]", NodeID: ready3, Job: job, ClientStatus: complete, DesiredStatus: stop},
				{Name: "my-job.web[0]", NodeID: ready1, Job: job, ClientStatus: failed, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2, Job: job, ClientStatus: failed, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3, Job: job, ClientStatus: failed, DesiredStatus: run},
			},
		},
		{
			name:  "unknown-and-pending",
			nodes: util.Nodes(3, 1),
			allocs: []*structs.Allocation{
				{Name: "my-job.web[0]", NodeID: ready1, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[0]", NodeID: disconnected1, Job: job, ClientStatus: unknown, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: disconnected1, Job: job, ClientStatus: unknown, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: disconnected1, Job: job, ClientStatus: unknown, DesiredStatus: run},
				{Name: "my-job.web[0]", NodeID: ready1, Job: job, ClientStatus: pending, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2, Job: job, ClientStatus: pending, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3, Job: job, ClientStatus: pending, DesiredStatus: run},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			util.SetIDs(tc.allocs)
			util.SetTaskGroup("web", tc.allocs)

			allocs := allocSet{}
			allocNames := map[string]int{}
			for _, alloc := range tc.allocs {
				allocs[alloc.ID] = alloc
				if _, ok := allocNames[alloc.Name]; ok {
					allocNames[alloc.Name]++
				} else {
					allocNames[alloc.Name] = 1
				}
			}

			reconciler := newTaskGroupReconciler(
				"web",
				logger,
				allocUpdateFnDestructive,
				false,
				updatedJob.ID,
				updatedJob,
				structs.NewDeployment(updatedJob, 50),
				allocs,
				nil,
				uuid.Generate(),
				50,
				&reconcileResults{},
				true)

			require.Len(t, reconciler.allocSlots, len(allocNames))

			for _, slot := range reconciler.allocSlots {
				require.Len(t, slot.candidates, allocNames[slot.name])
			}
		})
	}
}

// TestTaskGroupReconciler_Stops_Out_Of_Bounds_Allocs assures that running allocations
// that target a slot with an index higher than the current TaskGroup.Count -1
// are marked for stop.
func TestTaskGroupReconciler_Stops_Out_Of_Bounds_Allocs(t *testing.T) {
	logger := hclog.NewInterceptLogger(hclog.DefaultOptions)

	job := util.Job()
	job.TaskGroups[0].Count = 2

	type testCase struct {
		name         string
		expectedStop int
		allocs       []*structs.Allocation
	}

	// TODO: refactor to testutil helper methods for creating well-known alloc sets.
	testCases := []testCase{
		{
			name:         "stops-slot-2",
			expectedStop: 1,
			allocs: []*structs.Allocation{
				{Name: "my-job.web[0]", NodeID: ready1, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3, Job: job, ClientStatus: running, DesiredStatus: run},
			},
		},
		{
			name:         "stops-slot-2-and-3",
			expectedStop: 2,
			allocs: []*structs.Allocation{
				{Name: "my-job.web[0]", NodeID: ready1, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[3]", NodeID: ready3, Job: job, ClientStatus: running, DesiredStatus: run},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			util.SetIDs(tc.allocs)
			util.SetTaskGroup("web", tc.allocs)
			allocs := util.MakeAllocSet(tc.allocs)

			reconciler := newTaskGroupReconciler(
				"web",
				logger,
				allocUpdateFnDestructive,
				false,
				job.ID,
				job,
				structs.NewDeployment(job, 50),
				allocs,
				nil,
				uuid.Generate(),
				50,
				&reconcileResults{},
				true)

			require.Len(t, reconciler.result.stop, tc.expectedStop)
			require.Equal(t, uint64(tc.expectedStop), reconciler.desiredUpdates.Stop)
		})
	}
}

// TODO: Unit test all domain methods
