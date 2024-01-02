// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package events

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

type EventsTest struct {
	framework.TC
	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Events",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(EventsTest),
		},
	})
}

func (tc *EventsTest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
}

func (tc *EventsTest) AfterEach(f *framework.F) {
	nomadClient := tc.Nomad()
	j := nomadClient.Jobs()

	for _, id := range tc.jobIDs {
		j.Deregister(id, true, nil)
	}
	_, err := e2eutil.Command("nomad", "system", "gc")
	f.NoError(err)
}

// TestDeploymentEvents registers a job then applies a change
// An event stream listening to Deployment Events asserts that
// a DeploymentPromotion event is emitted
func (tc *EventsTest) TestDeploymentEvents(f *framework.F) {
	t := f.T()

	nomadClient := tc.Nomad()
	events := nomadClient.EventStream()

	uuid := uuid.Generate()
	jobID := fmt.Sprintf("deployment-%s", uuid[0:8])
	tc.jobIDs = append(tc.jobIDs, jobID)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topics := map[api.Topic][]string{
		api.TopicDeployment: {jobID},
	}

	var deployEvents []api.Event
	streamCh, err := events.Stream(ctx, topics, 0, nil)
	require.NoError(t, err)

	// gather deployment events
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-streamCh:
				if event.IsHeartbeat() {
					continue
				}

				deployEvents = append(deployEvents, event.Events...)
			}
		}
	}()

	// register job
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "events/input/initial.nomad", jobID, "")

	// update job
	e2eutil.RegisterAndWaitForAllocs(t, nomadClient, "events/input/deploy.nomad", jobID, "")

	ds := e2eutil.DeploymentsForJob(t, nomadClient, jobID)
	require.Equal(t, 2, len(ds))
	deploy := ds[0]

	// wait for deployment to be running and ready for auto promote
	e2eutil.WaitForDeployment(t, nomadClient, deploy.ID, structs.DeploymentStatusRunning, structs.DeploymentStatusDescriptionRunningAutoPromotion)

	// ensure there is a deployment promotion event
	testutil.WaitForResult(func() (bool, error) {
		for _, e := range deployEvents {
			if e.Type == "DeploymentPromotion" {
				return true, nil
			}
		}
		var got []string
		for _, e := range deployEvents {
			got = append(got, e.Type)
		}
		return false, fmt.Errorf("expected to receive deployment promotion event, got: %#v", got)
	}, func(e error) {
		f.NoError(e)
	})
}

// TestBlockedEvalEvents applies a job with a large memory requirement. The
// event stream checks for a failed task group alloc
func (tc *EventsTest) TestBlockedEvalEvents(f *framework.F) {
	t := f.T()

	nomadClient := tc.Nomad()
	events := nomadClient.EventStream()

	uuid := uuid.Generate()
	jobID := fmt.Sprintf("blocked-deploy-%s", uuid[0:8])
	tc.jobIDs = append(tc.jobIDs, jobID)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topics := map[api.Topic][]string{
		api.TopicEvaluation: {"*"},
	}

	var evalEvents []api.Event
	streamCh, err := events.Stream(ctx, topics, 0, nil)
	require.NoError(t, err)

	// gather deployment events
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-streamCh:
				if event.IsHeartbeat() {
					continue
				}

				evalEvents = append(evalEvents, event.Events...)
			}
		}
	}()

	// register job
	e2eutil.Register(jobID, "events/input/large-job.nomad")

	// ensure there is a deployment promotion event
	testutil.WaitForResult(func() (bool, error) {
		for _, e := range evalEvents {
			eval, err := e.Evaluation()
			if err != nil {
				return false, fmt.Errorf("event was not an evaluation %w", err)
			}

			ftg := eval.FailedTGAllocs

			tg, ok := ftg["one"]
			if !ok {
				continue
			}

			mem := tg.DimensionExhausted["memory"]
			require.NotNil(t, mem, "memory dimension was nil")
			require.Greater(t, mem, 0, "memory dimension was zero")
			return true, nil

		}
		return false, fmt.Errorf("expected blocked eval with memory exhausted, got: %#v", evalEvents)
	}, func(e error) {
		require.NoError(t, e)
	})
}

// TestStartIndex applies a job, then connects to the stream with a start
// index to verify that the events from before the job are not included.
func (tc *EventsTest) TestStartIndex(f *framework.F) {
	t := f.T()

	nomadClient := tc.Nomad()
	events := nomadClient.EventStream()

	uuid := uuid.Short()
	noopID := fmt.Sprintf("noop-%s", uuid)
	jobID := fmt.Sprintf("deployment-%s", uuid)
	jobID2 := fmt.Sprintf("deployment2-%s", uuid)
	tc.jobIDs = append(tc.jobIDs, noopID, jobID, jobID2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// register job
	err := e2eutil.Register(jobID, "events/input/initial.nomad")
	require.NoError(t, err)

	// The stream request gets the event *closest* to the index, not
	// the exact match. Although events are written before raft
	// entries they're written asynchronously, so it's possible to
	// race and get a raft index from this query higher than the
	// current head of the event buffer. Ensure the job is running
	// before we try to get the index, so that we've given the event
	// enough time to land in the buffer.
	var job *api.Job
	f.Eventually(func() bool {
		job, _, err = nomadClient.Jobs().Info(jobID, nil)
		if err != nil {
			return false
		}
		return *job.Status == "running"
	}, 20*time.Second, 200*time.Millisecond, "job should be running")

	startIndex := *job.JobModifyIndex + 1

	topics := map[api.Topic][]string{
		api.TopicJob: {"*"},
	}

	// starting at Job.ModifyIndex + 1, the next (and only) JobRegistered event that we see
	// should be from a different job registration
	streamCh, err := events.Stream(ctx, topics, startIndex, nil)
	require.NoError(t, err)

	var jobEvents []api.Event
	// gather job register events
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-streamCh:
				if !ok {
					return
				}
				if event.IsHeartbeat() {
					continue
				}
				jobEvents = append(jobEvents, event.Events...)
			}
		}
	}()

	// new job (to make sure we get a JobRegistered event)
	err = e2eutil.Register(jobID2, "events/input/deploy.nomad")
	require.NoError(t, err)

	// ensure there is a deployment promotion event
	foundUnexpected := false
	testutil.WaitForResult(func() (bool, error) {
		for _, e := range jobEvents {
			if e.Type == "JobRegistered" {
				if e.Index < startIndex {
					foundUnexpected = true
				}
				if e.Index >= startIndex {
					return true, nil
				}
			}
		}
		return false, fmt.Errorf("expected to receive JobRegistered event for index at least %v", startIndex)
	}, func(e error) {
		f.NoError(e)
	})
	require.False(t, foundUnexpected, "found events from earlier-than-expected indices")
}
