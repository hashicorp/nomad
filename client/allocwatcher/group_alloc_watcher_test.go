// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocwatcher

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// TestPrevAlloc_GroupPrevAllocWatcher_Block asserts that when there are
// prevAllocs is set a groupPrevAllocWatcher will block on them
func TestPrevAlloc_GroupPrevAllocWatcher_Block(t *testing.T) {
	ci.Parallel(t)
	conf, cleanup := newConfig(t)

	defer cleanup()

	conf.Alloc.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "500ms",
	}

	waiter, _ := NewAllocWatcher(conf)

	groupWaiter := &groupPrevAllocWatcher{prevAllocs: []config.PrevAllocWatcher{waiter}}

	// Wait in a goroutine with a context to make sure it exits at the right time
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		defer cancel()
		groupWaiter.Wait(ctx)
	}()

	// Assert watcher is waiting
	testutil.WaitForResult(func() (bool, error) {
		return groupWaiter.IsWaiting(), fmt.Errorf("expected watcher to be waiting")
	}, func(err error) {
		t.Fatalf("error: %v", err)
	})

	// Broadcast a non-terminal alloc update to assert only terminal
	// updates break out of waiting.
	update := conf.PreviousRunner.Alloc().Copy()
	update.DesiredStatus = structs.AllocDesiredStatusStop
	update.ModifyIndex++
	update.AllocModifyIndex++

	broadcaster := conf.PreviousRunner.(*fakeAllocRunner).Broadcaster
	err := broadcaster.Send(update)
	require.NoError(t, err)

	// Assert watcher is still waiting because alloc isn't terminal
	testutil.WaitForResult(func() (bool, error) {
		return groupWaiter.IsWaiting(), fmt.Errorf("expected watcher to be waiting")
	}, func(err error) {
		t.Fatalf("error: %v", err)
	})

	// Stop the previous alloc and assert watcher stops blocking
	update = update.Copy()
	update.DesiredStatus = structs.AllocDesiredStatusStop
	update.ClientStatus = structs.AllocClientStatusComplete
	update.ModifyIndex++
	update.AllocModifyIndex++

	err = broadcaster.Send(update)
	require.NoError(t, err)

	testutil.WaitForResult(func() (bool, error) {
		return !groupWaiter.IsWaiting(), fmt.Errorf("did not expect watcher to be waiting")
	}, func(err error) {
		t.Fatalf("error: %v", err)
	})
}

// TestPrevAlloc_GroupPrevAllocWatcher_BlockMulti asserts that when there are
// multiple prevAllocs is set a groupPrevAllocWatcher will block until all
// are complete
func TestPrevAlloc_GroupPrevAllocWatcher_BlockMulti(t *testing.T) {
	ci.Parallel(t)

	conf1, cleanup1 := newConfig(t)
	defer cleanup1()
	conf1.Alloc.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "500ms",
	}

	conf2, cleanup2 := newConfig(t)
	defer cleanup2()
	conf2.Alloc.Job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "500ms",
	}

	waiter1, _ := NewAllocWatcher(conf1)
	waiter2, _ := NewAllocWatcher(conf2)

	groupWaiter := &groupPrevAllocWatcher{
		prevAllocs: []config.PrevAllocWatcher{
			waiter1,
			waiter2,
		},
	}

	terminalBroadcastFn := func(cfg Config) {
		update := cfg.PreviousRunner.Alloc().Copy()
		update.DesiredStatus = structs.AllocDesiredStatusStop
		update.ClientStatus = structs.AllocClientStatusComplete
		update.ModifyIndex++
		update.AllocModifyIndex++

		broadcaster := cfg.PreviousRunner.(*fakeAllocRunner).Broadcaster
		err := broadcaster.Send(update)
		require.NoError(t, err)
	}

	// Wait in a goroutine with a context to make sure it exits at the right time
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		defer cancel()
		groupWaiter.Wait(ctx)
	}()

	// Assert watcher is waiting
	testutil.WaitForResult(func() (bool, error) {
		return groupWaiter.IsWaiting(), fmt.Errorf("expected watcher to be waiting")
	}, func(err error) {
		t.Fatalf("error: %v", err)
	})

	// Broadcast a terminal alloc update to the first watcher
	terminalBroadcastFn(conf1)

	// Assert watcher is still waiting because alloc isn't terminal
	testutil.WaitForResult(func() (bool, error) {
		return groupWaiter.IsWaiting(), fmt.Errorf("expected watcher to be waiting")
	}, func(err error) {
		t.Fatalf("error: %v", err)
	})

	// Broadcast a terminal alloc update to the second watcher
	terminalBroadcastFn(conf2)

	testutil.WaitForResult(func() (bool, error) {
		return !groupWaiter.IsWaiting(), fmt.Errorf("did not expect watcher to be waiting")
	}, func(err error) {
		t.Fatalf("error: %v", err)
	})
}
