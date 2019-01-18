package api

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestNodes_List(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	var qm *QueryMeta
	var out []*NodeListStub
	var err error

	testutil.WaitForResult(func() (bool, error) {
		out, qm, err = nodes.List(nil)
		if err != nil {
			return false, err
		}
		if n := len(out); n != 1 {
			return false, fmt.Errorf("expected 1 node, got: %d", n)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Check that we got valid QueryMeta.
	assertQueryMeta(t, qm)
}

func TestNodes_PrefixList(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	var qm *QueryMeta
	var out []*NodeListStub
	var err error

	// Get the node ID
	var nodeID string
	testutil.WaitForResult(func() (bool, error) {
		out, _, err := nodes.List(nil)
		if err != nil {
			return false, err
		}
		if n := len(out); n != 1 {
			return false, fmt.Errorf("expected 1 node, got: %d", n)
		}
		nodeID = out[0].ID
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Find node based on four character prefix
	out, qm, err = nodes.PrefixList(nodeID[:4])
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n := len(out); n != 1 {
		t.Fatalf("expected 1 node, got: %d ", n)
	}

	// Check that we got valid QueryMeta.
	assertQueryMeta(t, qm)
}

func TestNodes_Info(t *testing.T) {
	t.Parallel()
	startTime := time.Now().Unix()
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Retrieving a nonexistent node returns error
	_, _, err := nodes.Info("12345678-abcd-efab-cdef-123456789abc", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %#v", err)
	}

	// Get the node ID
	var nodeID, dc string
	testutil.WaitForResult(func() (bool, error) {
		out, _, err := nodes.List(nil)
		if err != nil {
			return false, err
		}
		if n := len(out); n != 1 {
			return false, fmt.Errorf("expected 1 node, got: %d", n)
		}
		nodeID = out[0].ID
		dc = out[0].Datacenter
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Querying for existing nodes returns properly
	result, qm, err := nodes.Info(nodeID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	if result.ID != nodeID || result.Datacenter != dc {
		t.Fatalf("expected %s (%s), got: %s (%s)",
			nodeID, dc,
			result.ID, result.Datacenter)
	}

	// Check that the StatusUpdatedAt field is being populated correctly
	if result.StatusUpdatedAt < startTime {
		t.Fatalf("start time: %v, status updated: %v", startTime, result.StatusUpdatedAt)
	}

	if len(result.Events) < 1 {
		t.Fatalf("Expected at minimum the node register event to be populated: %+v", result)
	}
}

func TestNodes_ToggleDrain(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Wait for node registration and get the ID
	var nodeID string
	testutil.WaitForResult(func() (bool, error) {
		out, _, err := nodes.List(nil)
		if err != nil {
			return false, err
		}
		if n := len(out); n != 1 {
			return false, fmt.Errorf("expected 1 node, got: %d", n)
		}
		nodeID = out[0].ID
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Check for drain mode
	out, _, err := nodes.Info(nodeID, nil)
	require.Nil(err)
	if out.Drain {
		t.Fatalf("drain mode should be off")
	}

	// Toggle it on
	spec := &DrainSpec{
		Deadline: 10 * time.Second,
	}
	drainOut, err := nodes.UpdateDrain(nodeID, spec, false, nil)
	require.Nil(err)
	assertWriteMeta(t, &drainOut.WriteMeta)

	// Check again
	out, _, err = nodes.Info(nodeID, nil)
	require.Nil(err)
	if out.SchedulingEligibility != NodeSchedulingIneligible {
		t.Fatalf("bad eligibility: %v vs %v", out.SchedulingEligibility, NodeSchedulingIneligible)
	}

	// Toggle off again
	drainOut, err = nodes.UpdateDrain(nodeID, nil, true, nil)
	require.Nil(err)
	assertWriteMeta(t, &drainOut.WriteMeta)

	// Check again
	out, _, err = nodes.Info(nodeID, nil)
	require.Nil(err)
	if out.Drain {
		t.Fatalf("drain mode should be off")
	}
	if out.DrainStrategy != nil {
		t.Fatalf("drain strategy should be unset")
	}
	if out.SchedulingEligibility != NodeSchedulingEligible {
		t.Fatalf("should be eligible")
	}
}

func TestNodes_ToggleEligibility(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Wait for node registration and get the ID
	var nodeID string
	testutil.WaitForResult(func() (bool, error) {
		out, _, err := nodes.List(nil)
		if err != nil {
			return false, err
		}
		if n := len(out); n != 1 {
			return false, fmt.Errorf("expected 1 node, got: %d", n)
		}
		nodeID = out[0].ID
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Check for eligibility
	out, _, err := nodes.Info(nodeID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if out.SchedulingEligibility != NodeSchedulingEligible {
		t.Fatalf("node should be eligible")
	}

	// Toggle it off
	eligOut, err := nodes.ToggleEligibility(nodeID, false, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, &eligOut.WriteMeta)

	// Check again
	out, _, err = nodes.Info(nodeID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if out.SchedulingEligibility != NodeSchedulingIneligible {
		t.Fatalf("bad eligibility: %v vs %v", out.SchedulingEligibility, NodeSchedulingIneligible)
	}

	// Toggle on
	eligOut, err = nodes.ToggleEligibility(nodeID, true, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	assertWriteMeta(t, &eligOut.WriteMeta)

	// Check again
	out, _, err = nodes.Info(nodeID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if out.SchedulingEligibility != NodeSchedulingEligible {
		t.Fatalf("bad eligibility: %v vs %v", out.SchedulingEligibility, NodeSchedulingEligible)
	}
	if out.DrainStrategy != nil {
		t.Fatalf("drain strategy should be unset")
	}
}

func TestNodes_Allocations(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodes := c.Nodes()

	// Looking up by a nonexistent node returns nothing. We
	// don't check the index here because it's possible the node
	// has already registered, in which case we will get a non-
	// zero result anyways.
	allocs, _, err := nodes.Allocations("nope", nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n := len(allocs); n != 0 {
		t.Fatalf("expected 0 allocs, got: %d", n)
	}
}

func TestNodes_ForceEvaluate(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Force-eval on a nonexistent node fails
	_, _, err := nodes.ForceEvaluate("12345678-abcd-efab-cdef-123456789abc", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %#v", err)
	}

	// Wait for node registration and get the ID
	var nodeID string
	testutil.WaitForResult(func() (bool, error) {
		out, _, err := nodes.List(nil)
		if err != nil {
			return false, err
		}
		if n := len(out); n != 1 {
			return false, fmt.Errorf("expected 1 node, got: %d", n)
		}
		nodeID = out[0].ID
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	// Try force-eval again. We don't check the WriteMeta because
	// there are no allocations to process, so we would get an index
	// of zero. Same goes for the eval ID.
	_, _, err = nodes.ForceEvaluate(nodeID, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestNodes_Sort(t *testing.T) {
	t.Parallel()
	nodes := []*NodeListStub{
		{CreateIndex: 2},
		{CreateIndex: 1},
		{CreateIndex: 5},
	}
	sort.Sort(NodeIndexSort(nodes))

	expect := []*NodeListStub{
		{CreateIndex: 5},
		{CreateIndex: 2},
		{CreateIndex: 1},
	}
	if !reflect.DeepEqual(nodes, expect) {
		t.Fatalf("\n\n%#v\n\n%#v", nodes, expect)
	}
}

func TestNodes_GC(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodes := c.Nodes()

	err := nodes.GC(uuid.Generate(), nil)
	require.NotNil(err)
	require.True(structs.IsErrUnknownNode(err))
}

func TestNodes_GcAlloc(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	nodes := c.Nodes()

	err := nodes.GcAlloc(uuid.Generate(), nil)
	require.NotNil(err)
	require.True(structs.IsErrUnknownAllocation(err))
}

// Unittest monitorDrainMultiplex when an error occurs
func TestNodes_MonitorDrain_Multiplex_Bad(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctx := context.Background()
	multiplexCtx, cancel := context.WithCancel(ctx)

	// monitorDrainMultiplex doesn't require anything on *Nodes, so we
	// don't need to use a full Client
	var nodeClient *Nodes

	outCh := make(chan *MonitorMessage, 8)
	nodeCh := make(chan *MonitorMessage, 1)
	allocCh := make(chan *MonitorMessage, 8)
	exitedCh := make(chan struct{})
	go func() {
		defer close(exitedCh)
		nodeClient.monitorDrainMultiplex(ctx, cancel, outCh, nodeCh, allocCh)
	}()

	// Fake an alloc update
	msg := Messagef(0, "alloc update")
	allocCh <- msg
	require.Equal(msg, <-outCh)

	// Fake a node update
	msg = Messagef(0, "node update")
	nodeCh <- msg
	require.Equal(msg, <-outCh)

	// Fake an error that should shut everything down
	msg = Messagef(MonitorMsgLevelError, "fake error")
	nodeCh <- msg
	require.Equal(msg, <-outCh)

	_, ok := <-exitedCh
	require.False(ok)

	_, ok = <-outCh
	require.False(ok)

	// Exiting should also cancel the context that would be passed to the
	// node & alloc watchers
	select {
	case <-multiplexCtx.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("context wasn't canceled")
	}

}

// Unittest monitorDrainMultiplex when drain finishes
func TestNodes_MonitorDrain_Multiplex_Good(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	ctx := context.Background()
	multiplexCtx, cancel := context.WithCancel(ctx)

	// monitorDrainMultiplex doesn't require anything on *Nodes, so we
	// don't need to use a full Client
	var nodeClient *Nodes

	outCh := make(chan *MonitorMessage, 8)
	nodeCh := make(chan *MonitorMessage, 1)
	allocCh := make(chan *MonitorMessage, 8)
	exitedCh := make(chan struct{})
	go func() {
		defer close(exitedCh)
		nodeClient.monitorDrainMultiplex(ctx, cancel, outCh, nodeCh, allocCh)
	}()

	// Fake a node updating and finishing
	msg := Messagef(MonitorMsgLevelInfo, "node update")
	nodeCh <- msg
	close(nodeCh)
	require.Equal(msg, <-outCh)

	// Nothing else should have exited yet
	select {
	case msg, ok := <-outCh:
		if ok {
			t.Fatalf("unexpected output: %q", msg)
		}
		t.Fatalf("out channel closed unexpectedly")
	case <-exitedCh:
		t.Fatalf("multiplexer exited unexpectedly")
	case <-multiplexCtx.Done():
		t.Fatalf("multiplexer context canceled unexpectedly")
	case <-time.After(10 * time.Millisecond):
		t.Logf("multiplexer still running as expected")
	}

	// Fake an alloc update coming in after the node monitor has finished
	msg = Messagef(0, "alloc update")
	allocCh <- msg
	require.Equal(msg, <-outCh)

	// Closing the allocCh should cause everything to exit
	close(allocCh)

	_, ok := <-exitedCh
	require.False(ok)

	_, ok = <-outCh
	require.False(ok)

	// Exiting should also cancel the context that would be passed to the
	// node & alloc watchers
	select {
	case <-multiplexCtx.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("context wasn't canceled")
	}

}

func TestNodes_DrainStrategy_Equal(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// nil
	var d *DrainStrategy
	require.True(d.Equal(nil))

	o := &DrainStrategy{}
	require.False(d.Equal(o))
	require.False(o.Equal(d))

	d = &DrainStrategy{}
	require.True(d.Equal(o))

	// ForceDeadline
	d.ForceDeadline = time.Now()
	require.False(d.Equal(o))

	o.ForceDeadline = d.ForceDeadline
	require.True(d.Equal(o))

	// Deadline
	d.Deadline = 1
	require.False(d.Equal(o))

	o.Deadline = 1
	require.True(d.Equal(o))

	// IgnoreSystemJobs
	d.IgnoreSystemJobs = true
	require.False(d.Equal(o))

	o.IgnoreSystemJobs = true
	require.True(d.Equal(o))
}

func TestNodeStatValueFormatting(t *testing.T) {
	t.Parallel()

	cases := []struct {
		expected string
		value    StatValue
	}{
		{
			"true",
			StatValue{BoolVal: boolToPtr(true)},
		},
		{
			"false",
			StatValue{BoolVal: boolToPtr(false)},
		},
		{
			"myvalue",
			StatValue{StringVal: stringToPtr("myvalue")},
		},
		{
			"2.718",
			StatValue{
				FloatNumeratorVal: float64ToPtr(2.718),
			},
		},
		{
			"2.718 / 3.14",
			StatValue{
				FloatNumeratorVal:   float64ToPtr(2.718),
				FloatDenominatorVal: float64ToPtr(3.14),
			},
		},
		{
			"2.718 MHz",
			StatValue{
				FloatNumeratorVal: float64ToPtr(2.718),
				Unit:              "MHz",
			},
		},
		{
			"2.718 / 3.14 MHz",
			StatValue{
				FloatNumeratorVal:   float64ToPtr(2.718),
				FloatDenominatorVal: float64ToPtr(3.14),
				Unit:                "MHz",
			},
		},
		{
			"2",
			StatValue{
				IntNumeratorVal: int64ToPtr(2),
			},
		},
		{
			"2 / 3",
			StatValue{
				IntNumeratorVal:   int64ToPtr(2),
				IntDenominatorVal: int64ToPtr(3),
			},
		},
		{
			"2 MHz",
			StatValue{
				IntNumeratorVal: int64ToPtr(2),
				Unit:            "MHz",
			},
		},
		{
			"2 / 3 MHz",
			StatValue{
				IntNumeratorVal:   int64ToPtr(2),
				IntDenominatorVal: int64ToPtr(3),
				Unit:              "MHz",
			},
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d %v", i, c.expected), func(t *testing.T) {
			formatted := c.value.String()
			require.Equal(t, c.expected, formatted)
		})
	}
}
