package events

import (
	"context"
	"fmt"

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
		"Deployment": {jobID},
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
	e2eutil.RegisterAllocs(t, nomadClient, "events/input/deploy.nomad", jobID, "")

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
		"Eval": {"*"},
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
			evalRaw, ok := e.Payload["Eval"].(map[string]interface{})
			if !ok {
				return false, fmt.Errorf("type assertion on eval")
			}

			ftg, ok := evalRaw["FailedTGAllocs"].(map[string]interface{})
			if !ok {
				continue
			}

			tg, ok := ftg["one"].(map[string]interface{})
			if !ok {
				continue
			}
			mem := tg["DimensionExhausted"].(map[string]interface{})["memory"]
			require.NotNil(t, mem, "memory dimension was nil")
			memInt := int(mem.(float64))
			require.Greater(t, memInt, 0, "memory dimension was zero")
			return true, nil

		}
		return false, fmt.Errorf("expected blocked eval with memory exhausted, got: %#v", evalEvents)
	}, func(e error) {
		require.NoError(t, e)
	})
}
