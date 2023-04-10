// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-units"
	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestFS_Logs(t *testing.T) {
	testutil.RequireRoot(t)
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()

	node := oneNodeFromNodeList(t, c.Nodes())
	index := node.ModifyIndex

	var input strings.Builder
	input.Grow(units.MB)
	lines := 80 * units.KB
	for i := 0; i < lines; i++ {
		_, _ = fmt.Fprintf(&input, "%d\n", i)
	}

	job := &Job{
		ID:          pointerOf("TestFS_Logs"),
		Region:      pointerOf("global"),
		Datacenters: []string{"dc1"},
		Type:        pointerOf("batch"),
		TaskGroups: []*TaskGroup{
			{
				Name: pointerOf("TestFS_LogsGroup"),
				Tasks: []*Task{
					{
						Name:   "logger",
						Driver: "mock_driver",
						Config: map[string]interface{}{
							"stdout_string": input.String(),
						},
					},
				},
			},
		},
	}

	jobs := c.Jobs()
	jobResp, _, err := jobs.Register(job, nil)
	must.NoError(t, err)

	index = jobResp.EvalCreateIndex
	evaluations := c.Evaluations()

	f := func() error {
		resp, qm, err := evaluations.Info(jobResp.EvalID, &QueryOptions{WaitIndex: index})
		if err != nil {
			return fmt.Errorf("failed to get evaluation info: %w", err)
		}
		must.Eq(t, "", resp.BlockedEval)
		index = qm.LastIndex
		if resp.Status != "complete" {
			return fmt.Errorf("evaluation status is not complete, got: %s", resp.Status)
		}
		return nil
	}
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(10*time.Second),
		wait.Gap(1*time.Second),
	))

	allocID := ""
	g := func() error {
		allocs, _, err := jobs.Allocations(*job.ID, true, &QueryOptions{WaitIndex: index})
		if err != nil {
			return fmt.Errorf("failed to get allocations: %w", err)
		}
		if n := len(allocs); n != 1 {
			return fmt.Errorf("expected 1 allocation, got: %d", n)
		}
		if allocs[0].ClientStatus != "complete" {
			return fmt.Errorf("allocation not complete: %s", allocs[0].ClientStatus)
		}
		allocID = allocs[0].ID
		return nil
	}
	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(g),
		wait.Timeout(10*time.Second),
		wait.Gap(1*time.Second),
	))

	alloc, _, err := c.Allocations().Info(allocID, nil)
	must.NoError(t, err)

	for i := 0; i < 3; i++ {
		stopCh := make(chan struct{})
		defer close(stopCh)

		frames, errors := c.AllocFS().Logs(alloc, false, "logger", "stdout", "start", 0, stopCh, nil)

		var result bytes.Buffer
	READ_FRAMES:
		for {
			select {
			case f := <-frames:
				if f == nil {
					break READ_FRAMES
				}
				result.Write(f.Data)
			case err := <-errors:
				// Don't Fatal here as the other assertions may
				// contain helpful information.
				t.Errorf("Error: %v", err)
			}
		}

		// Check length
		must.Eq(t, input.Len(), result.Len())

		// Check complete ordering
		for i := 0; i < lines; i++ {
			line, readErr := result.ReadBytes('\n')
			must.NoError(t, readErr, must.Sprintf("unexpected error on line %d: %v", i, readErr))
			must.Eq(t, fmt.Sprintf("%d\n", i), string(line))
		}
	}
}

func TestFS_FrameReader(t *testing.T) {
	testutil.Parallel(t)

	// Create a channel of the frames and a cancel channel
	framesCh := make(chan *StreamFrame, 3)
	errCh := make(chan error)
	cancelCh := make(chan struct{})

	r := NewFrameReader(framesCh, errCh, cancelCh)

	// Create some frames and send them
	f1 := &StreamFrame{
		File:   "foo",
		Offset: 5,
		Data:   []byte("hello"),
	}
	f2 := &StreamFrame{
		File:   "foo",
		Offset: 10,
		Data:   []byte(", wor"),
	}
	f3 := &StreamFrame{
		File:   "foo",
		Offset: 12,
		Data:   []byte("ld"),
	}
	framesCh <- f1
	framesCh <- f2
	framesCh <- f3
	close(framesCh)

	expected := []byte("hello, world")

	// Read a little
	p := make([]byte, 12)

	n, err := r.Read(p[:5])
	must.NoError(t, err)
	must.Eq(t, n, r.Offset())

	off := n
	for {
		n, err = r.Read(p[off:])
		if err != nil {
			if err == io.EOF {
				break
			}
			must.NoError(t, err)
		}
		off += n
	}

	must.Eq(t, expected, p)
	must.NoError(t, r.Close())
	_, ok := <-cancelCh
	must.False(t, ok)
	must.Eq(t, len(expected), r.Offset())
}

func TestFS_FrameReader_Unblock(t *testing.T) {
	testutil.Parallel(t)
	// Create a channel of the frames and a cancel channel
	framesCh := make(chan *StreamFrame, 3)
	errCh := make(chan error)
	cancelCh := make(chan struct{})

	r := NewFrameReader(framesCh, errCh, cancelCh)
	r.SetUnblockTime(10 * time.Millisecond)

	// Read a little
	p := make([]byte, 12)

	n, err := r.Read(p)
	must.NoError(t, err)
	must.Zero(t, n)

	// Unset the unblock
	r.SetUnblockTime(0)

	resultCh := make(chan struct{})
	go func() {
		r.Read(p)
		close(resultCh)
	}()

	select {
	case <-resultCh:
		must.Unreachable(t, must.Sprint("must not have unblocked"))
	case <-time.After(300 * time.Millisecond):
	}
}

func TestFS_FrameReader_Error(t *testing.T) {
	testutil.Parallel(t)
	// Create a channel of the frames and a cancel channel
	framesCh := make(chan *StreamFrame, 3)
	errCh := make(chan error, 1)
	cancelCh := make(chan struct{})

	r := NewFrameReader(framesCh, errCh, cancelCh)
	r.SetUnblockTime(10 * time.Millisecond)

	// Send an error
	expected := errors.New("test error")
	errCh <- expected

	// Read a little
	p := make([]byte, 12)

	_, err := r.Read(p)
	must.ErrorIs(t, err, expected)
}
