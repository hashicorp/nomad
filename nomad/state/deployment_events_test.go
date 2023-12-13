// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestDeploymentEventFromChanges(t *testing.T) {
	ci.Parallel(t)
	s := TestStateStoreCfg(t, TestStateStorePublisher(t))
	defer s.StopEventBroker()

	// setup
	setupTx := s.db.WriteTxn(10)

	j := mock.Job()
	e := mock.Eval()
	e.JobID = j.ID

	d := mock.Deployment()
	d.JobID = j.ID

	require.NoError(t, s.upsertJobImpl(10, nil, j, false, setupTx))
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

	de := got.Payload.(*structs.DeploymentEvent)
	require.Equal(t, structs.DeploymentStatusPaused, de.Deployment.Status)
	require.Contains(t, got.FilterKeys, j.ID)

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
			require.Failf(t, "reached max attempts waiting for desired event count", "count %d", len(got))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func EventsForIndex(t *testing.T, s *StateStore, index uint64) []structs.Event {
	pub, err := s.EventBroker()
	require.NoError(t, err)

	sub, err := pub.Subscribe(&stream.SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": {"*"},
		},
		Namespace:           "default",
		Index:               index,
		StartExactlyAtIndex: true,
	})
	if err != nil {
		return []structs.Event{}
	}
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
