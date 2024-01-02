// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskevents

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/helper/uuid"
)

type TaskEventsTest struct {
	framework.TC
	jobIds []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "TaskEvents",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(TaskEventsTest),
		},
	})
}

func (tc *TaskEventsTest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *TaskEventsTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	jobs := nomadClient.Jobs()
	// Stop all jobs in test
	for _, id := range tc.jobIds {
		jobs.Deregister(id, true, nil)
	}
	// Garbage collect
	nomadClient.System().GarbageCollect()
}

func formatEvents(events []*api.TaskEvent) string {
	estrs := make([]string, len(events))
	for i, e := range events {
		estrs[i] = fmt.Sprintf("%2d %-20s fail=%t msg=> %s", i, e.Type, e.FailsTask, e.DisplayMessage)
	}
	return strings.Join(estrs, "\n")
}

// waitUntilEvents submits a job and then waits until the expected number of
// events exist.
//
// The job name is used to load the job file from "input/${job}.nomad", and
// events are only inspected for tasks named the same as the job. That task's
// state is returned as well as the last allocation received.
func (tc *TaskEventsTest) waitUntilEvents(f *framework.F, jobName string, numEvents int) (*api.Allocation, *api.TaskState) {
	t := f.T()
	nomadClient := tc.Nomad()
	uuid := uuid.Generate()
	uniqJobId := jobName + uuid[0:8]
	tc.jobIds = append(tc.jobIds, uniqJobId)

	jobFile := fmt.Sprintf("taskevents/input/%s.nomad", jobName)
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), nomadClient, jobFile, uniqJobId, "")

	require.Len(t, allocs, 1)
	allocID := allocs[0].ID
	qo := &api.QueryOptions{
		WaitTime: time.Second,
	}

	// Capture state outside of wait to ease assertions once expected
	// number of events have been received.
	var alloc *api.Allocation
	var taskState *api.TaskState

	testutil.WaitForResultRetries(10, func() (bool, error) {
		a, meta, err := nomadClient.Allocations().Info(allocID, qo)
		if err != nil {
			return false, err
		}

		qo.WaitIndex = meta.LastIndex

		// Capture alloc and task state
		alloc = a
		taskState = a.TaskStates[jobName]
		if taskState == nil {
			return false, fmt.Errorf("task state not found for %s", jobName)
		}

		// Assert expected number of task events; we can't check for the exact
		// count because of a race where Allocation Unhealthy events can be
		// emitted when a peer task dies, but the caller can assert the
		// specific events and their order up to that point
		if len(taskState.Events) < numEvents {
			return false, fmt.Errorf("expected %d task events but found %d\n%s",
				numEvents, len(taskState.Events), formatEvents(taskState.Events),
			)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err, "task events error")
	})

	return alloc, taskState
}

func (tc *TaskEventsTest) TestTaskEvents_SimpleBatch(f *framework.F) {
	t := f.T()
	_, taskState := tc.waitUntilEvents(f, "simple_batch", 4)
	events := taskState.Events

	// Assert task did not fail
	require.Falsef(t, taskState.Failed, "task unexpectedly failed after %d events\n%s",
		len(events), formatEvents(events),
	)

	// Assert the expected type of events were emitted in a specific order
	// (based on v0.8.6)
	require.Equal(t, api.TaskReceived, events[0].Type)
	require.Equal(t, api.TaskSetup, events[1].Type)
	require.Equal(t, api.TaskStarted, events[2].Type)
	require.Equal(t, api.TaskTerminated, events[3].Type)
}

func (tc *TaskEventsTest) TestTaskEvents_FailedBatch(f *framework.F) {
	t := f.T()
	_, taskState := tc.waitUntilEvents(f, "failed_batch", 4)
	events := taskState.Events

	// Assert task did fail
	require.Truef(t, taskState.Failed, "task unexpectedly succeeded after %d events\n%s",
		len(events), formatEvents(events),
	)

	// Assert the expected type of events were emitted in a specific order
	// (based on v0.8.6)
	require.Equal(t, api.TaskReceived, events[0].Type)
	require.Equal(t, api.TaskSetup, events[1].Type)
	require.Equal(t, api.TaskDriverFailure, events[2].Type)
	require.Equal(t, api.TaskNotRestarting, events[3].Type)
	require.True(t, events[3].FailsTask)
}

// TestTaskEvents_CompletedLeader asserts the proper events are emitted for a
// non-leader task when its leader task completes.
func (tc *TaskEventsTest) TestTaskEvents_CompletedLeader(f *framework.F) {
	t := f.T()
	_, taskState := tc.waitUntilEvents(f, "completed_leader", 7)
	events := taskState.Events

	// Assert task did not fail
	require.Falsef(t, taskState.Failed, "task unexpectedly failed after %d events\n%s",
		len(events), formatEvents(events),
	)

	// Assert the expected type of events were emitted in a specific order
	require.Equal(t, api.TaskReceived, events[0].Type)
	require.Equal(t, api.TaskSetup, events[1].Type)
	require.Equal(t, api.TaskStarted, events[2].Type)
	require.Equal(t, api.TaskLeaderDead, events[3].Type)
	require.Equal(t, api.TaskKilling, events[4].Type)
	require.Equal(t, api.TaskTerminated, events[5].Type)
	require.Equal(t, api.TaskKilled, events[6].Type)
}

// TestTaskEvents_FailedSibling asserts the proper events are emitted for a
// task when another task in its task group fails.
func (tc *TaskEventsTest) TestTaskEvents_FailedSibling(f *framework.F) {
	t := f.T()
	alloc, taskState := tc.waitUntilEvents(f, "failed_sibling", 7)
	events := taskState.Events

	// Just because a sibling failed doesn't mean this task fails. It
	// should exit cleanly. (same as in v0.8.6)
	require.Falsef(t, taskState.Failed, "task unexpectedly failed after %d events\n%s",
		len(events), formatEvents(events),
	)

	// The alloc should be faied
	require.Equal(t, "failed", alloc.ClientStatus)

	// Assert the expected type of events were emitted in a specific order
	require.Equal(t, api.TaskReceived, events[0].Type)
	require.Equal(t, api.TaskSetup, events[1].Type)
	require.Equal(t, api.TaskStarted, events[2].Type)
	require.Equal(t, api.TaskSiblingFailed, events[3].Type)
	require.Equal(t, api.TaskKilling, events[4].Type)
	require.Equal(t, api.TaskTerminated, events[5].Type)
	require.Equal(t, api.TaskKilled, events[6].Type)
}
