package scheduler

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestTaskGroupReconciler_BuildsCandidates_ByAllocName(t *testing.T) {
	logger := hclog.NewInterceptLogger(hclog.DefaultOptions)
	ready1 := mock.Node()
	ready1.Name = "ready1"
	ready1.Status = structs.NodeStatusReady

	ready2 := mock.Node()
	ready2.Name = "ready2"
	ready2.Status = structs.NodeStatusReady

	ready3 := mock.Node()
	ready3.Name = "ready3"
	ready3.Status = structs.NodeStatusReady

	disconnected := mock.Node()
	disconnected.Name = "disconnected"
	disconnected.Status = structs.NodeStatusDisconnected

	// nodes := map[string]*structs.Node{
	// 	ready1.ID: ready1,
	// 	ready2.ID: ready2,
	// 	ready3.ID: ready3,
	//  disconnected.ID: disconnected,
	// }

	running := structs.AllocClientStatusRunning
	complete := structs.AllocClientStatusComplete
	failed := structs.AllocClientStatusFailed
	unknown := structs.AllocClientStatusUnknown
	pending := structs.AllocClientStatusPending
	run := structs.AllocDesiredStatusRun
	stop := structs.AllocDesiredStatusStop

	job := mock.Job()
	job.TaskGroups[0].Count = 3
	updatedJob := job.Copy()
	updatedJob.Version = updatedJob.Version + 1

	// jobs := map[string]*structs.Job{
	// 	job.ID:        job,
	// 	updatedJob.ID: updatedJob,
	// }

	type testCase struct {
		name   string
		allocs []*structs.Allocation
	}

	testCases := []testCase{
		{
			name: "single-job-version",
			allocs: []*structs.Allocation{
				{Name: "my-job.web[0]", NodeID: ready1.ID, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2.ID, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3.ID, Job: job, ClientStatus: running, DesiredStatus: run},
			},
		},
		{
			name: "multiple-job-versions",
			allocs: []*structs.Allocation{
				{Name: "my-job.web[0]", NodeID: ready1.ID, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2.ID, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3.ID, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[0]", NodeID: ready1.ID, Job: updatedJob, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2.ID, Job: updatedJob, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3.ID, Job: updatedJob, ClientStatus: running, DesiredStatus: run},
			},
		},
		{
			name: "complete-and-failed",
			allocs: []*structs.Allocation{
				{Name: "my-job.web[0]", NodeID: ready1.ID, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2.ID, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3.ID, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[0]", NodeID: ready1.ID, Job: job, ClientStatus: complete, DesiredStatus: stop},
				{Name: "my-job.web[1]", NodeID: ready2.ID, Job: job, ClientStatus: complete, DesiredStatus: stop},
				{Name: "my-job.web[2]", NodeID: ready3.ID, Job: job, ClientStatus: complete, DesiredStatus: stop},
				{Name: "my-job.web[0]", NodeID: ready1.ID, Job: job, ClientStatus: failed, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2.ID, Job: job, ClientStatus: failed, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3.ID, Job: job, ClientStatus: failed, DesiredStatus: run},
			},
		},
		{
			name: "unknown-and-pending",
			allocs: []*structs.Allocation{
				{Name: "my-job.web[0]", NodeID: ready1.ID, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2.ID, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3.ID, Job: job, ClientStatus: running, DesiredStatus: run},
				{Name: "my-job.web[0]", NodeID: disconnected.ID, Job: job, ClientStatus: unknown, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: disconnected.ID, Job: job, ClientStatus: unknown, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: disconnected.ID, Job: job, ClientStatus: unknown, DesiredStatus: run},
				{Name: "my-job.web[0]", NodeID: ready1.ID, Job: job, ClientStatus: pending, DesiredStatus: run},
				{Name: "my-job.web[1]", NodeID: ready2.ID, Job: job, ClientStatus: pending, DesiredStatus: run},
				{Name: "my-job.web[2]", NodeID: ready3.ID, Job: job, ClientStatus: pending, DesiredStatus: run},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, alloc := range tc.allocs {
				alloc.ID = uuid.Generate()
				alloc.JobID = alloc.Job.ID
			}

			reconciler := NewTaskGroupReconciler(logger, allocUpdateFnDestructive, false, updatedJob.ID, updatedJob,
				structs.NewDeployment(updatedJob, 50), tc.allocs, nil, uuid.Generate(), 50, true)

			slot := reconciler.allocSlots[updatedJob.TaskGroups[0].Name]
			require.Len(t, slot, tc.)

			for name, _ := range slots.Candidates {
				allocCountByName := 0
				for _, alloc := range tc.allocs {
					if alloc.Name == name {
						allocCountByName++
					}
				}
				require.Len(t, slot.Candidates, allocCountByName)
			}
		})
	}
}
