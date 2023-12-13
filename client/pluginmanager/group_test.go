// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pluginmanager

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/stretchr/testify/require"
)

func TestPluginGroup_RegisterAndRun(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	var hasRun bool
	var wg sync.WaitGroup
	wg.Add(1)
	manager := &MockPluginManager{RunF: func() {
		hasRun = true
		wg.Done()
	}}

	group := New(testlog.HCLogger(t))
	require.NoError(group.RegisterAndRun(manager))
	wg.Wait()
	require.True(hasRun)
}

func TestPluginGroup_Shutdown(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	var stack []int
	var stackMu sync.Mutex
	var runWg sync.WaitGroup
	var shutdownWg sync.WaitGroup
	group := New(testlog.HCLogger(t))
	for i := 1; i < 4; i++ {
		i := i
		runWg.Add(1)
		shutdownWg.Add(1)
		manager := &MockPluginManager{RunF: func() {
			stackMu.Lock()
			defer stackMu.Unlock()
			defer runWg.Done()
			stack = append(stack, i)
		}, ShutdownF: func() {
			stackMu.Lock()
			defer stackMu.Unlock()
			defer shutdownWg.Done()
			idx := len(stack) - 1
			val := stack[idx]
			require.Equal(i, val)
			stack = stack[:idx]
		}}
		require.NoError(group.RegisterAndRun(manager))
		runWg.Wait()
	}
	group.Shutdown()
	shutdownWg.Wait()
	require.Empty(stack)

	require.Error(group.RegisterAndRun(&MockPluginManager{}))
}

func TestPluginGroup_WaitForFirstFingerprint(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	managerCh := make(chan struct{})
	manager := &MockPluginManager{
		RunF:                      func() {},
		WaitForFirstFingerprintCh: managerCh,
	}

	// close immediately to beat the context timeout
	close(managerCh)

	group := New(testlog.HCLogger(t))
	require.NoError(group.RegisterAndRun(manager))

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	groupCh, err := group.WaitForFirstFingerprint(ctx)
	require.NoError(err)

	select {
	case <-groupCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected groupCh to be closed")
	}
}

func TestPluginGroup_WaitForFirstFingerprint_Timeout(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	managerCh := make(chan struct{})
	manager := &MockPluginManager{
		RunF:                      func() {},
		WaitForFirstFingerprintCh: managerCh,
	}

	group := New(testlog.HCLogger(t))
	require.NoError(group.RegisterAndRun(manager))

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	groupCh, err := group.WaitForFirstFingerprint(ctx)

	select {
	case <-groupCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected groupCh to be closed due to context timeout")
	}
	require.NoError(err)
}
