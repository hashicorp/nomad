// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tasklifecycle

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper"
)

func TestGate(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name string
		test func(*testing.T, *Gate)
	}{
		{
			name: "starts blocked",
			test: func(t *testing.T, g *Gate) {
				requireChannelBlocking(t, g.WaitCh(), "wait")
			},
		},
		{
			name: "block",
			test: func(t *testing.T, g *Gate) {
				g.Close()
				requireChannelBlocking(t, g.WaitCh(), "wait")
			},
		},
		{
			name: "allow",
			test: func(t *testing.T, g *Gate) {
				g.Open()
				requireChannelPassing(t, g.WaitCh(), "wait")
			},
		},
		{
			name: "block twice",
			test: func(t *testing.T, g *Gate) {
				g.Close()
				g.Close()
				requireChannelBlocking(t, g.WaitCh(), "wait")
			},
		},
		{
			name: "allow twice",
			test: func(t *testing.T, g *Gate) {
				g.Open()
				g.Open()
				requireChannelPassing(t, g.WaitCh(), "wait")
			},
		},
		{
			name: "allow block allow",
			test: func(t *testing.T, g *Gate) {
				g.Open()
				requireChannelPassing(t, g.WaitCh(), "first allow")
				g.Close()
				requireChannelBlocking(t, g.WaitCh(), "block")
				g.Open()
				requireChannelPassing(t, g.WaitCh(), "second allow")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shutdownCh := make(chan struct{})
			defer close(shutdownCh)

			g := NewGate(shutdownCh)
			tc.test(t, g)
		})
	}
}

// TestGate_shutdown tests a gate with a closed shutdown channel.
func TestGate_shutdown(t *testing.T) {
	ci.Parallel(t)

	// Create a Gate with a closed shutdownCh.
	shutdownCh := make(chan struct{})
	close(shutdownCh)

	g := NewGate(shutdownCh)

	// Test that Open() and Close() doesn't block forever.
	openCh := make(chan struct{})
	closeCh := make(chan struct{})

	go func() {
		g.Open()
		close(openCh)
	}()
	go func() {
		g.Close()
		close(closeCh)
	}()

	timer, stop := helper.NewSafeTimer(time.Second)
	defer stop()

	select {
	case <-openCh:
	case <-timer.C:
		t.Fatalf("timeout waiting for gate operations")
	}

	select {
	case <-closeCh:
	case <-timer.C:
		t.Fatalf("timeout waiting for gate operations")
	}

	// A Gate with a shutdownCh should be closed.
	requireChannelBlocking(t, g.WaitCh(), "gate should be closed")
}
