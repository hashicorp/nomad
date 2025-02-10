// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func queryNodeList(t *testing.T, nodes *Nodes) ([]*NodeListStub, *QueryMeta) {
	t.Helper()
	var (
		nodeListStub []*NodeListStub
		queryMeta    *QueryMeta
		err          error
	)

	f := func() error {
		nodeListStub, queryMeta, err = nodes.List(nil)
		if err != nil {
			return fmt.Errorf("failed to list nodes: %w", err)
		}
		if len(nodeListStub) == 0 {
			return fmt.Errorf("no nodes yet")
		}
		return nil
	}

	must.Wait(t, wait.InitialSuccess(
		wait.ErrorFunc(f),
		wait.Timeout(10*time.Second),
		wait.Gap(1*time.Second),
	))

	return nodeListStub, queryMeta
}

func oneNodeFromNodeList(t *testing.T, nodes *Nodes) *NodeListStub {
	nodeListStub, _ := queryNodeList(t, nodes)
	must.Len(t, 1, nodeListStub, must.Sprint("expected 1 node"))
	return nodeListStub[0]
}

func TestNodes_List(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	nodeListStub, queryMeta := queryNodeList(t, nodes)
	must.Len(t, 1, nodeListStub)
	must.Eq(t, NodePoolDefault, nodeListStub[0].NodePool)

	// Check that we got valid QueryMeta.
	assertQueryMeta(t, queryMeta)
}

func TestNodes_PrefixList(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Get the node ID
	nodeID := oneNodeFromNodeList(t, nodes).ID

	// Find node based on four character prefix
	out, qm, err := nodes.PrefixList(nodeID[:4])
	must.NoError(t, err)
	must.Len(t, 1, out, must.Sprint("expected only 1 node"))

	// Check that we got valid QueryMeta.
	assertQueryMeta(t, qm)
}

// TestNodes_List_Resources asserts that ?resources=true includes allocated and
// reserved resources in the response.
func TestNodes_List_Resources(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	node := oneNodeFromNodeList(t, nodes)

	// By default resources should *not* be included
	must.Nil(t, node.NodeResources)
	must.Nil(t, node.ReservedResources)

	qo := &QueryOptions{
		Params: map[string]string{"resources": "true"},
	}

	out, _, err := nodes.List(qo)
	must.NoError(t, err)
	must.NotNil(t, out[0].NodeResources)
	must.NotNil(t, out[0].ReservedResources)
}

func TestNodes_Info(t *testing.T) {
	testutil.Parallel(t)

	startTime := time.Now().Unix()
	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Retrieving a nonexistent node returns error
	_, _, infoErr := nodes.Info("12345678-abcd-efab-cdef-123456789abc", nil)
	must.ErrorContains(t, infoErr, "not found")

	// Get the node ID and DC
	node := oneNodeFromNodeList(t, nodes)
	nodeID, dc := node.ID, node.Datacenter

	// Querying for existing nodes returns properly
	result, qm, err := nodes.Info(nodeID, nil)
	must.NoError(t, err)

	assertQueryMeta(t, qm)

	// Check that the result is what we expect
	must.Eq(t, nodeID, result.ID)
	must.Eq(t, dc, result.Datacenter)
	must.Eq(t, NodePoolDefault, result.NodePool)

	must.Eq(t, 20000, result.NodeResources.MinDynamicPort)
	must.Eq(t, 32000, result.NodeResources.MaxDynamicPort)

	// Check that the StatusUpdatedAt field is being populated correctly
	must.Less(t, result.StatusUpdatedAt, startTime)

	// check we have at least one event
	must.GreaterEq(t, 1, len(result.Events))
}

func TestNode_Stats(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodesAPI := c.Nodes()
	nodeID := oneNodeFromNodeList(t, nodesAPI).ID

	stats, err := nodesAPI.Stats(nodeID, nil)
	must.NoError(t, err)

	// there isn't much we can reliably check here except that the values are
	// populated
	must.NotNil(t, stats.Memory)
	must.NonZero(t, stats.Memory.Available)
	must.NotNil(t, stats.AllocDirStats)
	must.NonZero(t, stats.AllocDirStats.Size)
}

func TestNodes_NoSecretID(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Get the node ID
	nodeID := oneNodeFromNodeList(t, nodes).ID

	// perform a raw http call and make sure that:
	// - "ID" to make sure that raw decoding is working correctly
	// - "SecretID" to make sure it's not present
	resp := make(map[string]interface{})
	_, err := c.query("/v1/node/"+nodeID, &resp, nil)
	must.NoError(t, err)
	must.Eq(t, nodeID, resp["ID"].(string))
	must.Eq(t, "", resp["SecretID"])
}

func TestNodes_ToggleDrain(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Wait for node registration and get the ID
	nodeID := oneNodeFromNodeList(t, nodes).ID

	// Check for drain mode
	out, _, err := nodes.Info(nodeID, nil)
	must.NoError(t, err)
	must.False(t, out.Drain)
	must.Nil(t, out.LastDrain)

	// Toggle it on
	timeBeforeDrain := time.Now().Add(-1 * time.Second)
	spec := &DrainSpec{
		Deadline: 10 * time.Second,
	}
	drainMeta := map[string]string{
		"reason": "this node needs to go",
	}
	drainOut, err := nodes.UpdateDrainOpts(nodeID, &DrainOptions{
		DrainSpec:    spec,
		MarkEligible: false,
		Meta:         drainMeta,
	}, nil)
	must.NoError(t, err)
	assertWriteMeta(t, &drainOut.WriteMeta)

	// Drain may have completed before we can check, use event stream
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamCh, err := c.EventStream().Stream(ctx, map[Topic][]string{
		TopicNode: {nodeID},
	}, 0, nil)
	must.NoError(t, err)

	// we expect to see the node change to Drain:true and then back to Drain:false+ineligible
	var sawDraining, sawDrainComplete uint64
	for sawDrainComplete == 0 {
		select {
		case events := <-streamCh:
			must.NoError(t, events.Err)
			for _, e := range events.Events {
				node, err := e.Node()
				must.NoError(t, err)
				must.Eq(t, node.DrainStrategy != nil, node.Drain)
				must.True(t, !node.Drain || node.SchedulingEligibility == NodeSchedulingIneligible) // node.Drain => "ineligible"
				if node.Drain && node.SchedulingEligibility == NodeSchedulingIneligible {
					must.NotNil(t, node.LastDrain)
					must.Eq(t, DrainStatusDraining, node.LastDrain.Status)
					now := time.Now()
					must.False(t, node.LastDrain.StartedAt.Before(timeBeforeDrain))
					must.False(t, node.LastDrain.StartedAt.After(now))
					must.Eq(t, drainMeta, node.LastDrain.Meta)
					sawDraining = node.ModifyIndex
				} else if sawDraining != 0 && !node.Drain && node.SchedulingEligibility == NodeSchedulingIneligible {
					must.NotNil(t, node.LastDrain)
					must.Eq(t, DrainStatusComplete, node.LastDrain.Status)
					must.True(t, !node.LastDrain.UpdatedAt.Before(node.LastDrain.StartedAt))
					must.Eq(t, drainMeta, node.LastDrain.Meta)
					sawDrainComplete = node.ModifyIndex
				}
			}
		case <-time.After(5 * time.Second):
			must.Unreachable(t, must.Sprint("waiting on stream event that never happened"))
		}
	}

	// Toggle off again
	drainOut, err = nodes.UpdateDrain(nodeID, nil, true, nil)
	must.NoError(t, err)
	assertWriteMeta(t, &drainOut.WriteMeta)

	// Check again
	out, _, err = nodes.Info(nodeID, nil)
	must.NoError(t, err)
	must.False(t, out.Drain)
	must.Nil(t, out.DrainStrategy)
	must.Eq(t, NodeSchedulingEligible, out.SchedulingEligibility)
}

func TestNodes_ToggleEligibility(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Get node ID
	nodeID := oneNodeFromNodeList(t, nodes).ID

	// Check for eligibility
	out, _, err := nodes.Info(nodeID, nil)
	must.NoError(t, err)
	must.Eq(t, NodeSchedulingEligible, out.SchedulingEligibility)

	// Toggle it off
	eligOut, err := nodes.ToggleEligibility(nodeID, false, nil)
	must.NoError(t, err)
	assertWriteMeta(t, &eligOut.WriteMeta)

	// Check again
	out, _, err = nodes.Info(nodeID, nil)
	must.NoError(t, err)
	must.Eq(t, NodeSchedulingIneligible, out.SchedulingEligibility)

	// Toggle on
	eligOut, err = nodes.ToggleEligibility(nodeID, true, nil)
	must.NoError(t, err)
	assertWriteMeta(t, &eligOut.WriteMeta)

	// Check again
	out, _, err = nodes.Info(nodeID, nil)
	must.NoError(t, err)
	must.Eq(t, NodeSchedulingEligible, out.SchedulingEligibility)
	must.Nil(t, out.DrainStrategy)
}

func TestNodes_Allocations(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Looking up by a nonexistent node returns nothing. We
	// don't check the index here because it's possible the node
	// has already registered, in which case we will get a non-
	// zero result anyways.
	allocations, _, err := nodes.Allocations("nope", nil)
	must.NoError(t, err)
	must.Len(t, 0, allocations)
}

func TestNodes_ForceEvaluate(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Force-eval on a nonexistent node fails
	_, _, err := nodes.ForceEvaluate("12345678-abcd-efab-cdef-123456789abc", nil)
	must.ErrorContains(t, err, "not found")

	// Wait for node registration and get the ID
	nodeID := oneNodeFromNodeList(t, nodes).ID

	// Try force-eval again. We don't check the WriteMeta because
	// there are no allocations to process, so we would get an index
	// of zero. Same goes for the eval ID.
	_, _, err = nodes.ForceEvaluate(nodeID, nil)
	must.NoError(t, err)
}

func TestNodes_Sort(t *testing.T) {
	testutil.Parallel(t)

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
	must.Eq(t, expect, nodes)
}

// Unittest monitorDrainMultiplex when an error occurs
func TestNodes_MonitorDrain_Multiplex_Bad(t *testing.T) {
	testutil.Parallel(t)

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
	must.Eq(t, msg, <-outCh)

	// Fake a node update
	msg = Messagef(0, "node update")
	nodeCh <- msg
	must.Eq(t, msg, <-outCh)

	// Fake an error that should shut everything down
	msg = Messagef(MonitorMsgLevelError, "fake error")
	nodeCh <- msg
	must.Eq(t, msg, <-outCh)

	_, ok := <-exitedCh
	must.False(t, ok)

	_, ok = <-outCh
	must.False(t, ok)

	// Exiting should also cancel the context that would be passed to the
	// node & alloc watchers
	select {
	case <-multiplexCtx.Done():
	case <-time.After(100 * time.Millisecond):
		must.Unreachable(t, must.Sprint("multiplex context was not cancelled"))
	}
}

// Unittest monitorDrainMultiplex when drain finishes
func TestNodes_MonitorDrain_Multiplex_Good(t *testing.T) {
	testutil.Parallel(t)

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
	must.Eq(t, msg, <-outCh)

	// Nothing else should have exited yet
	select {
	case badMsg, ok := <-outCh:
		must.False(t, ok, must.Sprintf("unexpected output %v", badMsg))
		must.Unreachable(t, must.Sprint("out channel closed unexpectedly"))
	case <-exitedCh:
		must.Unreachable(t, must.Sprint("multiplexer exited unexpectedly"))
	case <-multiplexCtx.Done():
		must.Unreachable(t, must.Sprint("multiplexer context canceled unexpectedly"))
	case <-time.After(10 * time.Millisecond):
		t.Logf("multiplexer still running as expected")
	}

	// Fake an alloc update coming in after the node monitor has finished
	msg = Messagef(0, "alloc update")
	allocCh <- msg
	must.Eq(t, msg, <-outCh)

	// Closing the allocCh should cause everything to exit
	close(allocCh)

	_, ok := <-exitedCh
	must.False(t, ok)

	_, ok = <-outCh
	must.False(t, ok)

	// Exiting should also cancel the context that would be passed to the
	// node & alloc watchers
	select {
	case <-multiplexCtx.Done():
	case <-time.After(100 * time.Millisecond):
		must.Unreachable(t, must.Sprint("context was not cancelled"))
	}
}

func TestNodes_DrainStrategy_Equal(t *testing.T) {
	testutil.Parallel(t)

	// nil
	var d *DrainStrategy
	must.Equal(t, nil, d)

	o := &DrainStrategy{}
	must.NotEqual(t, d, o)
	must.NotEqual(t, o, d)

	d = &DrainStrategy{}
	must.Equal(t, d, o)
	must.Equal(t, o, d)

	// ForceDeadline
	d.ForceDeadline = time.Now()
	must.NotEqual(t, d, o)

	o.ForceDeadline = d.ForceDeadline
	must.Equal(t, d, o)

	// Deadline
	d.Deadline = 1
	must.NotEqual(t, d, o)

	o.Deadline = 1
	must.Equal(t, d, o)

	// IgnoreSystemJobs
	d.IgnoreSystemJobs = true
	must.NotEqual(t, d, o)

	o.IgnoreSystemJobs = true
	must.Equal(t, d, o)
}

func TestNodes_Purge(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, func(c *testutil.TestServerConfig) {
		c.DevMode = true
	})
	defer s.Stop()
	nodes := c.Nodes()

	// Purge on a nonexistent node fails.
	_, _, err := c.Nodes().Purge("12345678-abcd-efab-cdef-123456789abc", nil)
	must.ErrorContains(t, err, "not found")

	// Wait for nodeID
	nodeID := oneNodeFromNodeList(t, nodes).ID

	// Perform the node purge and check the response objects.
	out, meta, err := c.Nodes().Purge(nodeID, nil)
	must.NoError(t, err)
	must.NotNil(t, out)

	// We can't use assertQueryMeta here, as the RPC response does not populate
	// the known leader field.
	must.Positive(t, meta.LastIndex)
}

func TestNodeStatValueFormatting(t *testing.T) {
	testutil.Parallel(t)

	cases := []struct {
		expected string
		value    StatValue
	}{
		{
			"true",
			StatValue{BoolVal: pointerOf(true)},
		},
		{
			"false",
			StatValue{BoolVal: pointerOf(false)},
		},
		{
			"myvalue",
			StatValue{StringVal: pointerOf("myvalue")},
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
				IntNumeratorVal: pointerOf(int64(2)),
			},
		},
		{
			"2 / 3",
			StatValue{
				IntNumeratorVal:   pointerOf(int64(2)),
				IntDenominatorVal: pointerOf(int64(3)),
			},
		},
		{
			"2 MHz",
			StatValue{
				IntNumeratorVal: pointerOf(int64(2)),
				Unit:            "MHz",
			},
		},
		{
			"2 / 3 MHz",
			StatValue{
				IntNumeratorVal:   pointerOf(int64(2)),
				IntDenominatorVal: pointerOf(int64(3)),
				Unit:              "MHz",
			},
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d %v", i, c.expected), func(t *testing.T) {
			formatted := c.value.String()
			must.Eq(t, c.expected, formatted)
		})
	}
}
