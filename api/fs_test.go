package api

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	units "github.com/docker/go-units"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/testutil"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFS_Logs(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	rpcPort := 0
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		rpcPort = c.Ports.RPC
		c.Client = &testutil.ClientConfig{
			Enabled: true,
		}
	})
	defer s.Stop()

	//TODO There should be a way to connect the client to the servers in
	//makeClient above
	require.NoError(c.Agent().SetServers([]string{fmt.Sprintf("127.0.0.1:%d", rpcPort)}))

	index := uint64(0)
	testutil.WaitForResult(func() (bool, error) {
		nodes, qm, err := c.Nodes().List(&QueryOptions{WaitIndex: index})
		if err != nil {
			return false, err
		}
		index = qm.LastIndex
		if len(nodes) != 1 {
			return false, fmt.Errorf("expected 1 node but found: %s", pretty.Sprint(nodes))
		}
		if nodes[0].Status != "ready" {
			return false, fmt.Errorf("node not ready: %s", nodes[0].Status)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	var input strings.Builder
	input.Grow(units.MB)
	lines := 80 * units.KB
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&input, "%d\n", i)
	}

	job := &Job{
		ID:          helper.StringToPtr("TestFS_Logs"),
		Region:      helper.StringToPtr("global"),
		Datacenters: []string{"dc1"},
		Type:        helper.StringToPtr("batch"),
		TaskGroups: []*TaskGroup{
			{
				Name: helper.StringToPtr("TestFS_LogsGroup"),
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
	require.NoError(err)

	index = jobResp.EvalCreateIndex
	evals := c.Evaluations()
	testutil.WaitForResult(func() (bool, error) {
		evalResp, qm, err := evals.Info(jobResp.EvalID, &QueryOptions{WaitIndex: index})
		if err != nil {
			return false, err
		}
		if evalResp.BlockedEval != "" {
			t.Fatalf("Eval blocked: %s", pretty.Sprint(evalResp))
		}
		index = qm.LastIndex
		if evalResp.Status != "complete" {
			return false, fmt.Errorf("eval status: %v", evalResp.Status)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	allocID := ""
	testutil.WaitForResult(func() (bool, error) {
		allocs, _, err := jobs.Allocations(*job.ID, true, &QueryOptions{WaitIndex: index})
		if err != nil {
			return false, err
		}
		if len(allocs) != 1 {
			return false, fmt.Errorf("unexpected number of allocs: %d", len(allocs))
		}
		if allocs[0].ClientStatus != "complete" {
			return false, fmt.Errorf("alloc not complete: %s", allocs[0].ClientStatus)
		}
		allocID = allocs[0].ID
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	alloc, _, err := c.Allocations().Info(allocID, nil)
	require.NoError(err)

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
				// contain helpeful information.
				t.Errorf("Error: %v", err)
			}
		}

		// Check length
		assert.Equal(t, input.Len(), result.Len(), "file size mismatch")

		// Check complete ordering
		for i := 0; i < lines; i++ {
			line, err := result.ReadBytes('\n')
			require.NoErrorf(err, "unexpected error on line %d: %v", i, err)
			require.Equal(fmt.Sprintf("%d\n", i), string(line))
		}
	}
}

func TestFS_FrameReader(t *testing.T) {
	t.Parallel()
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
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if off := r.Offset(); off != n {
		t.Fatalf("unexpected read bytes: got %v; wanted %v", n, off)
	}

	off := n
	for {
		n, err = r.Read(p[off:])
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("Read failed: %v", err)
		}
		off += n
	}

	if !reflect.DeepEqual(p, expected) {
		t.Fatalf("read %q, wanted %q", string(p), string(expected))
	}

	if err := r.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}
	if _, ok := <-cancelCh; ok {
		t.Fatalf("Close() didn't close cancel channel")
	}
	if len(expected) != r.Offset() {
		t.Fatalf("offset %d, wanted %d", r.Offset(), len(expected))
	}
}

func TestFS_FrameReader_Unblock(t *testing.T) {
	t.Parallel()
	// Create a channel of the frames and a cancel channel
	framesCh := make(chan *StreamFrame, 3)
	errCh := make(chan error)
	cancelCh := make(chan struct{})

	r := NewFrameReader(framesCh, errCh, cancelCh)
	r.SetUnblockTime(10 * time.Millisecond)

	// Read a little
	p := make([]byte, 12)

	n, err := r.Read(p)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if n != 0 {
		t.Fatalf("should have unblocked")
	}

	// Unset the unblock
	r.SetUnblockTime(0)

	resultCh := make(chan struct{})
	go func() {
		r.Read(p)
		close(resultCh)
	}()

	select {
	case <-resultCh:
		t.Fatalf("shouldn't have unblocked")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestFS_FrameReader_Error(t *testing.T) {
	t.Parallel()
	// Create a channel of the frames and a cancel channel
	framesCh := make(chan *StreamFrame, 3)
	errCh := make(chan error, 1)
	cancelCh := make(chan struct{})

	r := NewFrameReader(framesCh, errCh, cancelCh)
	r.SetUnblockTime(10 * time.Millisecond)

	// Send an error
	expected := fmt.Errorf("test error")
	errCh <- expected

	// Read a little
	p := make([]byte, 12)

	_, err := r.Read(p)
	if err == nil || !strings.Contains(err.Error(), expected.Error()) {
		t.Fatalf("bad error: %v", err)
	}
}
