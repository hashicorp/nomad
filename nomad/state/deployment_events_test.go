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
	"github.com/shoenig/test/must"
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

	must.NoError(t, s.upsertJobImpl(10, nil, j, false, setupTx))
	must.NoError(t, s.upsertDeploymentImpl(10, d, setupTx))

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

	must.NoError(t, s.UpdateDeploymentStatus(msgType, 100, req))

	events := WaitForEvents(t, s, 100, 1, 1*time.Second)
	must.Len(t, 2, events)

	got := events[0]
	must.Eq(t, uint64(100), got.Index)
	must.Eq(t, d.ID, got.Key)

	de := got.Payload.(*structs.DeploymentEvent)
	must.Eq(t, structs.DeploymentStatusPaused, de.Deployment.Status)
	must.SliceContains(t, got.FilterKeys, j.ID)
}

func WaitForEvents(t *testing.T, s *StateStore, index uint64, minEvents int, timeout time.Duration) []structs.Event {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(timeout):
			t.Fatal("timeout waiting for events")
		}
	}()

	maxAttempts := 10
	for {
		got := EventsForIndex(t, s, index)
		if len(got) >= minEvents {
			return got
		}
		maxAttempts--
		must.NotEq(t, 0, maxAttempts, must.Sprintf(
			"reached max attempts waiting for desired event count: count %d got: %+v",
			len(got), got))
		time.Sleep(10 * time.Millisecond)
	}
}

func EventsForIndex(t *testing.T, s *StateStore, index uint64) []structs.Event {
	pub, err := s.EventBroker()
	must.NoError(t, err)

	sub, err := pub.Subscribe(&stream.SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": {"*"},
		},
		Namespaces:          []string{"default"},
		Index:               index,
		StartExactlyAtIndex: true,
	})
	if err != nil {
		return []structs.Event{}
	}
	defer sub.Unsubscribe()

	must.NoError(t, err)

	var events []structs.Event
	for {
		e, err := sub.NextNoBlock()
		must.NoError(t, err)
		if e == nil {
			break
		}
		events = append(events, e...)
	}
	return events
}
