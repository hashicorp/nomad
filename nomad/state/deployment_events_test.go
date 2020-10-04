package state

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestDeploymentEventFromChanges(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventPublisher()

	// setup
	setupTx := s.db.WriteTxn(10)

	j := mock.Job()
	e := mock.Eval()
	e.JobID = j.ID

	d := mock.Deployment()
	d.JobID = j.ID

	require.NoError(t, s.upsertJobImpl(10, j, false, setupTx))
	require.NoError(t, s.upsertDeploymentImpl(10, d, setupTx))

	setupTx.Txn.Commit()

	msgType := structs.DeploymentStatusUpdateRequestType

	req := &structs.DeploymentStatusUpdateRequest{
		DeploymentUpdate: &structs.DeploymentStatusUpdate{
			DeploymentID:      d.ID,
			Status:            structs.DeploymentStatusPaused,
			StatusDescription: structs.DeploymentStatusDescriptionPaused,
		},
		Eval: e,
		// Exlude Job and assert its added
	}

	require.NoError(t, s.UpdateDeploymentStatus(msgType, 100, req))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 2)

	got := events[0]
	require.Equal(t, uint64(100), got.Index)
	require.Equal(t, d.ID, got.Key)

	de := got.Payload.(*DeploymentEvent)
	require.Equal(t, structs.DeploymentStatusPaused, de.Deployment.Status)
	require.Contains(t, got.FilterKeys, j.ID)

}

func TestDeploymentEventFromChanges_Promotion(t *testing.T) {
	t.Parallel()
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventPublisher()

	// setup
	setupTx := s.db.WriteTxn(10)

	j := mock.Job()
	tg1 := j.TaskGroups[0]
	tg2 := tg1.Copy()
	tg2.Name = "foo"
	j.TaskGroups = append(j.TaskGroups, tg2)
	require.NoError(t, s.upsertJobImpl(10, j, false, setupTx))

	d := mock.Deployment()
	d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	d.JobID = j.ID
	d.TaskGroups = map[string]*structs.DeploymentState{
		"web": {
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
		"foo": {
			DesiredTotal:    10,
			DesiredCanaries: 1,
		},
	}
	require.NoError(t, s.upsertDeploymentImpl(10, d, setupTx))

	// create set of allocs
	c1 := mock.Alloc()
	c1.JobID = j.ID
	c1.DeploymentID = d.ID
	d.TaskGroups[c1.TaskGroup].PlacedCanaries = append(d.TaskGroups[c1.TaskGroup].PlacedCanaries, c1.ID)
	c1.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}
	c2 := mock.Alloc()
	c2.JobID = j.ID
	c2.DeploymentID = d.ID
	d.TaskGroups[c2.TaskGroup].PlacedCanaries = append(d.TaskGroups[c2.TaskGroup].PlacedCanaries, c2.ID)
	c2.TaskGroup = tg2.Name
	c2.DeploymentStatus = &structs.AllocDeploymentStatus{
		Healthy: helper.BoolToPtr(true),
	}

	require.NoError(t, s.upsertAllocsImpl(10, []*structs.Allocation{c1, c2}, setupTx))

	// commit setup transaction
	setupTx.Txn.Commit()

	e := mock.Eval()
	// Request to promote canaries
	msgType := structs.DeploymentPromoteRequestType
	req := &structs.ApplyDeploymentPromoteRequest{
		DeploymentPromoteRequest: structs.DeploymentPromoteRequest{
			DeploymentID: d.ID,
			All:          true,
		},
		Eval: e,
	}

	require.NoError(t, s.UpdateDeploymentPromotion(msgType, 100, req))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	require.Len(t, events, 2)

	got := events[0]
	require.Equal(t, uint64(100), got.Index)
	require.Equal(t, d.ID, got.Key)

	de := got.Payload.(*DeploymentEvent)
	require.Equal(t, structs.DeploymentStatusRunning, de.Deployment.Status)
}

func WaitForEvents(t *testing.T, s *StateStore, index uint64, minEvents int, timeout time.Duration) []structs.Event {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(timeout):
			require.Fail(t, "timeout waiting for events")
		}
	}()

	maxAttempts := 10
	for {
		got := EventsForIndex(t, s, index)
		if len(got) >= minEvents {
			return got
		}
		maxAttempts--
		if maxAttempts == 0 {
			require.Fail(t, "reached max attempts waiting for desired event count")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func EventsForIndex(t *testing.T, s *StateStore, index uint64) []structs.Event {
	pub, err := s.EventPublisher()
	require.NoError(t, err)

	sub, err := pub.Subscribe(&stream.SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": []string{"*"},
		},
		Index: index,
	})
	defer sub.Unsubscribe()

	require.NoError(t, err)

	var events []structs.Event
	for {
		e, err := sub.NextNoBlock()
		require.NoError(t, err)
		if e == nil {
			break
		}
		events = append(events, e...)
	}
	return events
}
