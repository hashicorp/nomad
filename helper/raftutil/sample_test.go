// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/require"
)

// TestSampleInvariant illustrates how to find offending log entry for an invariant
func TestSampleInvariant(t *testing.T) {
	t.Skip("not a real test")

	path := "/tmp/nomad-datadir/server/raft"
	ns := "default"
	parentID := "myjob"

	fsm, err := NewFSM(path)
	require.NoError(t, err)

	state := fsm.State()
	for {
		idx, _, err := fsm.ApplyNext()
		if err == ErrNoMoreLogs {
			break
		}
		require.NoError(t, err)

		// Test invariant for each entry

		// For example, test job summary numbers against running jobs
		summary, err := state.JobSummaryByID(nil, ns, parentID)
		require.NoError(t, err)

		if summary == nil {
			// job hasn't been created yet
			continue
		}

		summaryCount := summary.Children.Running + summary.Children.Pending + summary.Children.Dead
		jobCountByParent := 0

		iter, err := state.Jobs(nil)
		require.NoError(t, err)
		for {
			rawJob := iter.Next()
			if rawJob == nil {
				break
			}
			job := rawJob.(*structs.Job)
			if job.Namespace == ns && job.ParentID == parentID {
				jobCountByParent++
			}
		}

		require.Equalf(t, summaryCount, jobCountByParent, "job summary at idx=%v", idx)

	}

	// any post-assertion follow
}

// TestSchedulerLogic illustrates how to test how to test the scheduler
// logic for handling an eval
func TestSchedulerLogic(t *testing.T) {
	t.Skip("not a real test")

	path := "/tmp/nomad-datadir/server/raft"
	ns := "default"
	jobID := "myjob"
	testIdx := uint64(3234)

	fsm, err := NewFSM(path)
	require.NoError(t, err)

	_, _, err = fsm.ApplyUntil(testIdx)
	require.NoError(t, err)

	state := fsm.State()

	job, err := state.JobByID(nil, ns, jobID)
	require.NoError(t, err)

	// Create an eval and schedule it!
	// Create a mock evaluation to register the job
	eval := &structs.Evaluation{
		Namespace:   ns,
		ID:          uuid.Generate(),
		Priority:    job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       job.ID,
		Status:      structs.EvalStatusPending,
	}

	// Process the evaluation
	h := scheduler.NewHarnessWithState(t, state)
	err = h.Process(scheduler.NewServiceScheduler, eval)
	require.NoError(t, err)

	require.Len(t, h.Plans, 1)
	pretty.Println(h.Plans[0])

}
