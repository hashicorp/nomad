// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package broker

import (
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/stretchr/testify/require"
)

func TestGenericNotifier(t *testing.T) {
	ci.Parallel(t)

	// Create the new notifier.
	stopChan := make(chan struct{})
	defer close(stopChan)

	notifier := NewGenericNotifier()
	go notifier.Run(stopChan)

	// Ensure we have buffered channels.
	require.Equal(t, 1, cap(notifier.publishCh))
	require.Equal(t, 1, cap(notifier.subscribeCh))
	require.Equal(t, 1, cap(notifier.unsubscribeCh))

	// Test that the timeout works.
	var timeoutWG sync.WaitGroup

	for i := 0; i < 6; i++ {
		go func(wg *sync.WaitGroup) {
			wg.Add(1)
			msg := notifier.WaitForChange(100 * time.Millisecond)
			require.Equal(t, "wait timed out after 100ms", msg)
			wg.Done()
		}(&timeoutWG)
	}
	timeoutWG.Wait()

	// Test that all subscribers receive an update when a single notification
	// is sent.
	var notifiedWG sync.WaitGroup

	for i := 0; i < 6; i++ {
		go func(wg *sync.WaitGroup) {
			wg.Add(1)
			msg := notifier.WaitForChange(3 * time.Second)
			require.Equal(t, "we got an update and not a timeout", msg)
			wg.Done()
		}(&notifiedWG)
	}

	// Ensure the routines have had time to start before sending the notify
	// signal, otherwise the test is a flake.
	time.Sleep(500 * time.Millisecond)

	notifier.Notify("we got an update and not a timeout")
	notifiedWG.Wait()
}
