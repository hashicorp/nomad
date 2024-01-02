// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package drainer

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestDeadlineHeap_Interface(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	h := NewDeadlineHeap(context.Background(), 1*time.Second)
	require.Implements((*DrainDeadlineNotifier)(nil), h)
}

func TestDeadlineHeap_WatchAndGet(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	h := NewDeadlineHeap(context.Background(), 1*time.Second)

	now := time.Now()
	nodeID := "1"
	wait := 10 * time.Millisecond
	deadline := now.Add(wait)
	h.Watch(nodeID, deadline)

	var batch []string
	select {
	case batch = <-h.NextBatch():
	case <-time.After(testutil.Timeout(3 * wait)):
		t.Fatal("timeout")
	}

	require.Len(batch, 1)
	require.Equal(nodeID, batch[0])
}

func TestDeadlineHeap_WatchThenUpdateAndGet(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	h := NewDeadlineHeap(context.Background(), 1*time.Second)

	now := time.Now()
	nodeID := "1"
	wait := 10 * time.Millisecond
	deadline := now.Add(wait)

	// Initially watch way in the future
	h.Watch(nodeID, now.Add(24*time.Hour))

	// Rewatch
	h.Watch(nodeID, deadline)

	var batch []string
	select {
	case batch = <-h.NextBatch():
	case <-time.After(testutil.Timeout(2 * wait)):
		t.Fatal("timeout")
	}

	require.Len(batch, 1)
	require.Equal(nodeID, batch[0])
}

func TestDeadlineHeap_MultiwatchAndDelete(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	h := NewDeadlineHeap(context.Background(), 1*time.Second)

	now := time.Now()
	wait := 50 * time.Millisecond
	deadline := now.Add(wait)

	nodeID1 := "1"
	nodeID2 := "2"
	h.Watch(nodeID1, deadline)
	h.Watch(nodeID2, deadline)

	time.Sleep(1 * time.Millisecond)
	h.Remove(nodeID2)

	var batch []string
	select {
	case batch = <-h.NextBatch():
	case <-time.After(testutil.Timeout(2 * wait)):
		t.Fatal("timeout")
	}

	require.Len(batch, 1)
	require.Equal(nodeID1, batch[0])
}

func TestDeadlineHeap_WatchCoalesce(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	h := NewDeadlineHeap(context.Background(), 100*time.Millisecond)

	now := time.Now()

	group1 := map[string]time.Time{
		"1": now.Add(5 * time.Millisecond),
		"2": now.Add(10 * time.Millisecond),
		"3": now.Add(20 * time.Millisecond),
		"4": now.Add(100 * time.Millisecond),
	}

	group2 := map[string]time.Time{
		"10": now.Add(350 * time.Millisecond),
		"11": now.Add(360 * time.Millisecond),
	}

	for _, g := range []map[string]time.Time{group1, group2} {
		for n, d := range g {
			h.Watch(n, d)
		}
	}

	var batch []string
	select {
	case batch = <-h.NextBatch():
	case <-time.After(testutil.Timeout(time.Second)):
		t.Fatal("timeout")
	}

	require.Len(batch, len(group1))
	for nodeID := range group1 {
		require.Contains(batch, nodeID)
	}
	batch = nil

	select {
	case batch = <-h.NextBatch():
	case <-time.After(testutil.Timeout(2 * time.Second)):
		t.Fatal("timeout")
	}

	require.Len(batch, len(group2))
	for nodeID := range group2 {
		require.Contains(batch, nodeID)
	}

	select {
	case <-h.NextBatch():
		t.Fatal("unexpected batch")
	case <-time.After(testutil.Timeout(100 * time.Millisecond)):
	}
}

func TestDeadlineHeap_MultipleForce(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	h := NewDeadlineHeap(context.Background(), 1*time.Second)

	nodeID := "1"
	deadline := time.Time{}
	h.Watch(nodeID, deadline)

	var batch []string
	select {
	case batch = <-h.NextBatch():
	case <-time.After(testutil.Timeout(10 * time.Millisecond)):
		t.Fatal("timeout")
	}

	require.Len(batch, 1)
	require.Equal(nodeID, batch[0])

	nodeID = "2"
	h.Watch(nodeID, deadline)
	select {
	case batch = <-h.NextBatch():
	case <-time.After(testutil.Timeout(10 * time.Millisecond)):
		t.Fatal("timeout")
	}

	require.Len(batch, 1)
	require.Equal(nodeID, batch[0])
}
