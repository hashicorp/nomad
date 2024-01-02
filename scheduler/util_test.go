// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

func BenchmarkTasksUpdated(b *testing.B) {
	jobA := mock.BigBenchmarkJob()
	jobB := jobA.Copy()
	for n := 0; n < b.N; n++ {
		if c := tasksUpdated(jobA, jobB, jobA.TaskGroups[0].Name); c.modified {
			b.Errorf("tasks should be the same")
		}
	}
}

func newNode(name string) *structs.Node {
	n := mock.Node()
	n.Name = name
	return n
}

func TestReadyNodesInDCsAndPool(t *testing.T) {
	ci.Parallel(t)

	state := state.TestStateStore(t)
	node1 := mock.Node()
	node2 := mock.Node()
	node2.Datacenter = "dc2"
	node3 := mock.Node()
	node3.Datacenter = "dc2"
	node3.Status = structs.NodeStatusDown
	node4 := mock.DrainNode()
	node5 := mock.Node()
	node5.Datacenter = "not-this-dc"
	node6 := mock.Node()
	node6.Datacenter = "dc1"
	node6.NodePool = "other"
	node7 := mock.Node()
	node7.Datacenter = "dc2"
	node7.NodePool = "other"
	node8 := mock.Node()
	node8.Datacenter = "dc1"
	node8.NodePool = "other"
	node8.Status = structs.NodeStatusDown
	node9 := mock.DrainNode()
	node9.Datacenter = "dc2"
	node9.NodePool = "other"

	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1000, node1)) // dc1 ready
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1001, node2)) // dc2 ready
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1002, node3)) // dc2 not ready
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1003, node4)) // dc2 not ready
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1004, node5)) // ready never match
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1005, node6)) // dc1 other pool
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1006, node7)) // dc2 other pool
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1007, node8)) // dc1 other not ready
	must.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1008, node9)) // dc2 other not ready

	testCases := []struct {
		name           string
		datacenters    []string
		pool           string
		expectReady    []*structs.Node
		expectNotReady map[string]struct{}
		expectIndex    map[string]int
	}{
		{
			name:        "no wildcards in all pool",
			datacenters: []string{"dc1", "dc2"},
			pool:        structs.NodePoolAll,
			expectReady: []*structs.Node{node1, node2, node6, node7},
			expectNotReady: map[string]struct{}{
				node3.ID: {}, node4.ID: {}, node8.ID: {}, node9.ID: {}},
			expectIndex: map[string]int{"dc1": 2, "dc2": 2},
		},
		{
			name:        "with wildcard in all pool",
			datacenters: []string{"dc*"},
			pool:        structs.NodePoolAll,
			expectReady: []*structs.Node{node1, node2, node6, node7},
			expectNotReady: map[string]struct{}{
				node3.ID: {}, node4.ID: {}, node8.ID: {}, node9.ID: {}},
			expectIndex: map[string]int{"dc1": 2, "dc2": 2},
		},
		{
			name:           "no wildcards in default pool",
			datacenters:    []string{"dc1", "dc2"},
			pool:           structs.NodePoolDefault,
			expectReady:    []*structs.Node{node1, node2},
			expectNotReady: map[string]struct{}{node3.ID: {}, node4.ID: {}},
			expectIndex:    map[string]int{"dc1": 1, "dc2": 1},
		},
		{
			name:           "with wildcard in default pool",
			datacenters:    []string{"dc*"},
			pool:           structs.NodePoolDefault,
			expectReady:    []*structs.Node{node1, node2},
			expectNotReady: map[string]struct{}{node3.ID: {}, node4.ID: {}},
			expectIndex:    map[string]int{"dc1": 1, "dc2": 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ready, notReady, dcIndex, err := readyNodesInDCsAndPool(state, tc.datacenters, tc.pool)
			must.NoError(t, err)
			must.SliceContainsAll(t, tc.expectReady, ready, must.Sprint("expected ready to match"))
			must.Eq(t, tc.expectNotReady, notReady, must.Sprint("expected not-ready to match"))
			must.Eq(t, tc.expectIndex, dcIndex, must.Sprint("expected datacenter counts to match"))
		})
	}
}

func TestRetryMax(t *testing.T) {
	ci.Parallel(t)

	calls := 0
	bad := func() (bool, error) {
		calls += 1
		return false, nil
	}
	err := retryMax(3, bad, nil)
	require.Error(t, err)
	require.Equal(t, 3, calls, "mis match")

	calls = 0
	first := true
	reset := func() bool {
		if calls == 3 && first {
			first = false
			return true
		}
		return false
	}
	err = retryMax(3, bad, reset)
	require.Error(t, err)
	require.Equal(t, 6, calls, "mis match")

	calls = 0
	good := func() (bool, error) {
		calls += 1
		return true, nil
	}
	err = retryMax(3, good, nil)
	require.NoError(t, err)
	require.Equal(t, 1, calls, "mis match")
}

func TestTaintedNodes(t *testing.T) {
	ci.Parallel(t)

	state := state.TestStateStore(t)
	node1 := mock.Node()
	node2 := mock.Node()
	node2.Datacenter = "dc2"
	node3 := mock.Node()
	node3.Datacenter = "dc2"
	node3.Status = structs.NodeStatusDown
	node4 := mock.DrainNode()
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1000, node1))
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1001, node2))
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1002, node3))
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 1003, node4))

	allocs := []*structs.Allocation{
		{NodeID: node1.ID},
		{NodeID: node2.ID},
		{NodeID: node3.ID},
		{NodeID: node4.ID},
		{NodeID: "12345678-abcd-efab-cdef-123456789abc"},
	}
	tainted, err := taintedNodes(state, allocs)
	require.NoError(t, err)
	require.Equal(t, 3, len(tainted))
	require.NotContains(t, tainted, node1.ID)
	require.NotContains(t, tainted, node2.ID)

	require.Contains(t, tainted, node3.ID)
	require.NotNil(t, tainted[node3.ID])

	require.Contains(t, tainted, node4.ID)
	require.NotNil(t, tainted[node4.ID])

	require.Contains(t, tainted, "12345678-abcd-efab-cdef-123456789abc")
	require.Nil(t, tainted["12345678-abcd-efab-cdef-123456789abc"])
}

func TestShuffleNodes(t *testing.T) {
	ci.Parallel(t)

	// Use a large number of nodes to make the probability of shuffling to the
	// original order very low.
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	orig := make([]*structs.Node, len(nodes))
	copy(orig, nodes)
	eval := mock.Eval() // will have random EvalID
	plan := eval.MakePlan(mock.Job())
	shuffleNodes(plan, 1000, nodes)
	require.False(t, reflect.DeepEqual(nodes, orig))

	nodes2 := make([]*structs.Node, len(nodes))
	copy(nodes2, orig)
	shuffleNodes(plan, 1000, nodes2)

	require.True(t, reflect.DeepEqual(nodes, nodes2))

}

func TestTaskUpdatedAffinity(t *testing.T) {
	ci.Parallel(t)

	j1 := mock.Job()
	j2 := mock.Job()
	name := j1.TaskGroups[0].Name
	must.False(t, tasksUpdated(j1, j2, name).modified)

	// TaskGroup Affinity
	j2.TaskGroups[0].Affinities = []*structs.Affinity{
		{
			LTarget: "node.datacenter",
			RTarget: "dc1",
			Operand: "=",
			Weight:  100,
		},
	}
	must.True(t, tasksUpdated(j1, j2, name).modified)

	// TaskGroup Task Affinity
	j3 := mock.Job()
	j3.TaskGroups[0].Tasks[0].Affinities = []*structs.Affinity{
		{
			LTarget: "node.datacenter",
			RTarget: "dc1",
			Operand: "=",
			Weight:  100,
		},
	}
	must.True(t, tasksUpdated(j1, j3, name).modified)

	j4 := mock.Job()
	j4.TaskGroups[0].Tasks[0].Affinities = []*structs.Affinity{
		{
			LTarget: "node.datacenter",
			RTarget: "dc1",
			Operand: "=",
			Weight:  100,
		},
	}
	must.True(t, tasksUpdated(j1, j4, name).modified)

	// check different level of same affinity
	j5 := mock.Job()
	j5.Affinities = []*structs.Affinity{
		{
			LTarget: "node.datacenter",
			RTarget: "dc1",
			Operand: "=",
			Weight:  100,
		},
	}

	j6 := mock.Job()
	j6.Affinities = make([]*structs.Affinity, 0)
	j6.TaskGroups[0].Affinities = []*structs.Affinity{
		{
			LTarget: "node.datacenter",
			RTarget: "dc1",
			Operand: "=",
			Weight:  100,
		},
	}
	must.False(t, tasksUpdated(j5, j6, name).modified)
}

func TestTaskUpdatedSpread(t *testing.T) {
	ci.Parallel(t)

	j1 := mock.Job()
	j2 := mock.Job()
	name := j1.TaskGroups[0].Name

	must.False(t, tasksUpdated(j1, j2, name).modified)

	// TaskGroup Spread
	j2.TaskGroups[0].Spreads = []*structs.Spread{
		{
			Attribute: "node.datacenter",
			Weight:    100,
			SpreadTarget: []*structs.SpreadTarget{
				{
					Value:   "r1",
					Percent: 50,
				},
				{
					Value:   "r2",
					Percent: 50,
				},
			},
		},
	}
	must.True(t, tasksUpdated(j1, j2, name).modified)

	// check different level of same constraint
	j5 := mock.Job()
	j5.Spreads = []*structs.Spread{
		{
			Attribute: "node.datacenter",
			Weight:    100,
			SpreadTarget: []*structs.SpreadTarget{
				{
					Value:   "r1",
					Percent: 50,
				},
				{
					Value:   "r2",
					Percent: 50,
				},
			},
		},
	}

	j6 := mock.Job()
	j6.TaskGroups[0].Spreads = []*structs.Spread{
		{
			Attribute: "node.datacenter",
			Weight:    100,
			SpreadTarget: []*structs.SpreadTarget{
				{
					Value:   "r1",
					Percent: 50,
				},
				{
					Value:   "r2",
					Percent: 50,
				},
			},
		},
	}

	must.False(t, tasksUpdated(j5, j6, name).modified)
}

func TestTasksUpdated(t *testing.T) {
	ci.Parallel(t)

	j1 := mock.Job()
	j2 := mock.Job()
	name := j1.TaskGroups[0].Name
	must.False(t, tasksUpdated(j1, j2, name).modified)

	j2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	must.True(t, tasksUpdated(j1, j2, name).modified)

	j3 := mock.Job()
	j3.TaskGroups[0].Tasks[0].Name = "foo"
	must.True(t, tasksUpdated(j1, j3, name).modified)

	j4 := mock.Job()
	j4.TaskGroups[0].Tasks[0].Driver = "foo"
	must.True(t, tasksUpdated(j1, j4, name).modified)

	j5 := mock.Job()
	j5.TaskGroups[0].Tasks = append(j5.TaskGroups[0].Tasks,
		j5.TaskGroups[0].Tasks[0])
	must.True(t, tasksUpdated(j1, j5, name).modified)

	j6 := mock.Job()
	j6.TaskGroups[0].Networks[0].DynamicPorts = []structs.Port{
		{Label: "http", Value: 0},
		{Label: "https", Value: 0},
		{Label: "admin", Value: 0},
	}
	must.True(t, tasksUpdated(j1, j6, name).modified)

	j7 := mock.Job()
	j7.TaskGroups[0].Tasks[0].Env["NEW_ENV"] = "NEW_VALUE"
	must.True(t, tasksUpdated(j1, j7, name).modified)

	j8 := mock.Job()
	j8.TaskGroups[0].Tasks[0].User = "foo"
	must.True(t, tasksUpdated(j1, j8, name).modified)

	j9 := mock.Job()
	j9.TaskGroups[0].Tasks[0].Artifacts = []*structs.TaskArtifact{
		{
			GetterSource: "http://foo.com/bar",
		},
	}
	must.True(t, tasksUpdated(j1, j9, name).modified)

	j10 := mock.Job()
	j10.TaskGroups[0].Tasks[0].Meta["baz"] = "boom"
	must.True(t, tasksUpdated(j1, j10, name).modified)

	j11 := mock.Job()
	j11.TaskGroups[0].Tasks[0].Resources.CPU = 1337
	must.True(t, tasksUpdated(j1, j11, name).modified)

	j11d1 := mock.Job()
	j11d1.TaskGroups[0].Tasks[0].Resources.Devices = structs.ResourceDevices{
		&structs.RequestedDevice{
			Name:  "gpu",
			Count: 1,
		},
	}
	j11d2 := mock.Job()
	j11d2.TaskGroups[0].Tasks[0].Resources.Devices = structs.ResourceDevices{
		&structs.RequestedDevice{
			Name:  "gpu",
			Count: 2,
		},
	}
	must.True(t, tasksUpdated(j11d1, j11d2, name).modified)

	j13 := mock.Job()
	j13.TaskGroups[0].Networks[0].DynamicPorts[0].Label = "foobar"
	must.True(t, tasksUpdated(j1, j13, name).modified)

	j14 := mock.Job()
	j14.TaskGroups[0].Networks[0].ReservedPorts = []structs.Port{{Label: "foo", Value: 1312}}
	must.True(t, tasksUpdated(j1, j14, name).modified)

	j15 := mock.Job()
	j15.TaskGroups[0].Tasks[0].Vault = &structs.Vault{Policies: []string{"foo"}}
	must.True(t, tasksUpdated(j1, j15, name).modified)

	j16 := mock.Job()
	j16.TaskGroups[0].EphemeralDisk.Sticky = true
	must.True(t, tasksUpdated(j1, j16, name).modified)

	// Change group meta
	j17 := mock.Job()
	j17.TaskGroups[0].Meta["j17_test"] = "roll_baby_roll"
	must.True(t, tasksUpdated(j1, j17, name).modified)

	// Change job meta
	j18 := mock.Job()
	j18.Meta["j18_test"] = "roll_baby_roll"
	must.True(t, tasksUpdated(j1, j18, name).modified)

	// Change network mode
	j19 := mock.Job()
	j19.TaskGroups[0].Networks[0].Mode = "bridge"
	must.True(t, tasksUpdated(j1, j19, name).modified)

	// Change cores resource
	j20 := mock.Job()
	j20.TaskGroups[0].Tasks[0].Resources.CPU = 0
	j20.TaskGroups[0].Tasks[0].Resources.Cores = 2
	j21 := mock.Job()
	j21.TaskGroups[0].Tasks[0].Resources.CPU = 0
	j21.TaskGroups[0].Tasks[0].Resources.Cores = 4
	must.True(t, tasksUpdated(j20, j21, name).modified)

	// Compare identical Template wait configs
	j22 := mock.Job()
	j22.TaskGroups[0].Tasks[0].Templates = []*structs.Template{
		{
			Wait: &structs.WaitConfig{
				Min: pointer.Of(5 * time.Second),
				Max: pointer.Of(5 * time.Second),
			},
		},
	}
	j23 := mock.Job()
	j23.TaskGroups[0].Tasks[0].Templates = []*structs.Template{
		{
			Wait: &structs.WaitConfig{
				Min: pointer.Of(5 * time.Second),
				Max: pointer.Of(5 * time.Second),
			},
		},
	}
	must.False(t, tasksUpdated(j22, j23, name).modified)
	// Compare changed Template wait configs
	j23.TaskGroups[0].Tasks[0].Templates[0].Wait.Max = pointer.Of(10 * time.Second)
	must.True(t, tasksUpdated(j22, j23, name).modified)

	// Add a volume
	j24 := mock.Job()
	j25 := j24.Copy()
	j25.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
		"myvolume": {
			Name:   "myvolume",
			Type:   "csi",
			Source: "test-volume[0]",
		}}
	must.True(t, tasksUpdated(j24, j25, name).modified)

	// Alter a volume
	j26 := j25.Copy()
	j26.TaskGroups[0].Volumes["myvolume"].ReadOnly = true
	must.True(t, tasksUpdated(j25, j26, name).modified)

	// Alter a CSI plugin
	j27 := mock.Job()
	j27.TaskGroups[0].Tasks[0].CSIPluginConfig = &structs.TaskCSIPluginConfig{
		ID:   "myplugin",
		Type: "node",
	}
	j28 := j27.Copy()
	j28.TaskGroups[0].Tasks[0].CSIPluginConfig.Type = "monolith"
	must.True(t, tasksUpdated(j27, j28, name).modified)

	// Compare identical Template ErrMissingKey
	j29 := mock.Job()
	j29.TaskGroups[0].Tasks[0].Templates = []*structs.Template{
		{
			ErrMissingKey: false,
		},
	}
	j30 := mock.Job()
	j30.TaskGroups[0].Tasks[0].Templates = []*structs.Template{
		{
			ErrMissingKey: false,
		},
	}
	must.False(t, tasksUpdated(j29, j30, name).modified)

	// Compare changed Template ErrMissingKey
	j30.TaskGroups[0].Tasks[0].Templates[0].ErrMissingKey = true
	must.True(t, tasksUpdated(j29, j30, name).modified)
}

func TestTasksUpdated_connectServiceUpdated(t *testing.T) {
	ci.Parallel(t)

	servicesA := []*structs.Service{{
		Name:      "service1",
		PortLabel: "1111",
		Connect: &structs.ConsulConnect{
			SidecarService: &structs.ConsulSidecarService{
				Tags: []string{"a"},
			},
		},
	}}

	t.Run("service not updated", func(t *testing.T) {
		servicesB := []*structs.Service{{
			Name: "service0",
		}, {
			Name:      "service1",
			PortLabel: "1111",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Tags: []string{"a"},
				},
			},
		}, {
			Name: "service2",
		}}
		updated := connectServiceUpdated(servicesA, servicesB).modified
		must.False(t, updated)
	})

	t.Run("service connect tags updated", func(t *testing.T) {
		servicesB := []*structs.Service{{
			Name: "service0",
		}, {
			Name:      "service1",
			PortLabel: "1111",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Tags: []string{"b"}, // in-place update
				},
			},
		}}
		updated := connectServiceUpdated(servicesA, servicesB).modified
		must.False(t, updated)
	})

	t.Run("service connect port updated", func(t *testing.T) {
		servicesB := []*structs.Service{{
			Name: "service0",
		}, {
			Name:      "service1",
			PortLabel: "1111",
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Tags: []string{"a"},
					Port: "2222", // destructive update
				},
			},
		}}
		updated := connectServiceUpdated(servicesA, servicesB).modified
		must.True(t, updated)
	})

	t.Run("service port label updated", func(t *testing.T) {
		servicesB := []*structs.Service{{
			Name: "service0",
		}, {
			Name:      "service1",
			PortLabel: "1112", // destructive update
			Connect: &structs.ConsulConnect{
				SidecarService: &structs.ConsulSidecarService{
					Tags: []string{"1"},
				},
			},
		}}
		updated := connectServiceUpdated(servicesA, servicesB).modified
		must.True(t, updated)
	})
}

func TestNetworkUpdated(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name    string
		a       []*structs.NetworkResource
		b       []*structs.NetworkResource
		updated bool
	}{
		{
			name: "mode updated",
			a: []*structs.NetworkResource{
				{Mode: "host"},
			},
			b: []*structs.NetworkResource{
				{Mode: "bridge"},
			},
			updated: true,
		},
		{
			name: "host_network updated",
			a: []*structs.NetworkResource{
				{DynamicPorts: []structs.Port{
					{Label: "http", To: 8080},
				}},
			},
			b: []*structs.NetworkResource{
				{DynamicPorts: []structs.Port{
					{Label: "http", To: 8080, HostNetwork: "public"},
				}},
			},
			updated: true,
		},
		{
			name: "port.To updated",
			a: []*structs.NetworkResource{
				{DynamicPorts: []structs.Port{
					{Label: "http", To: 8080},
				}},
			},
			b: []*structs.NetworkResource{
				{DynamicPorts: []structs.Port{
					{Label: "http", To: 8088},
				}},
			},
			updated: true,
		},
		{
			name: "hostname updated",
			a: []*structs.NetworkResource{
				{Hostname: "foo"},
			},
			b: []*structs.NetworkResource{
				{Hostname: "bar"},
			},
			updated: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.updated, networkUpdated(tc.a, tc.b).modified)
		})
	}
}

func TestSetStatus(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)
	logger := testlog.HCLogger(t)
	eval := mock.Eval()
	status := "a"
	desc := "b"
	require.NoError(t, setStatus(logger, h, eval, nil, nil, nil, status, desc, nil, ""))
	require.Equal(t, 1, len(h.Evals), "setStatus() didn't update plan: %v", h.Evals)

	newEval := h.Evals[0]
	require.True(t, newEval.ID == eval.ID && newEval.Status == status && newEval.StatusDescription == desc,
		"setStatus() submited invalid eval: %v", newEval)

	// Test next evals
	h = NewHarness(t)
	next := mock.Eval()
	require.NoError(t, setStatus(logger, h, eval, next, nil, nil, status, desc, nil, ""))
	require.Equal(t, 1, len(h.Evals), "setStatus() didn't update plan: %v", h.Evals)

	newEval = h.Evals[0]
	require.Equal(t, next.ID, newEval.NextEval, "setStatus() didn't set nextEval correctly: %v", newEval)

	// Test blocked evals
	h = NewHarness(t)
	blocked := mock.Eval()
	require.NoError(t, setStatus(logger, h, eval, nil, blocked, nil, status, desc, nil, ""))
	require.Equal(t, 1, len(h.Evals), "setStatus() didn't update plan: %v", h.Evals)

	newEval = h.Evals[0]
	require.Equal(t, blocked.ID, newEval.BlockedEval, "setStatus() didn't set BlockedEval correctly: %v", newEval)

	// Test metrics
	h = NewHarness(t)
	metrics := map[string]*structs.AllocMetric{"foo": nil}
	require.NoError(t, setStatus(logger, h, eval, nil, nil, metrics, status, desc, nil, ""))
	require.Equal(t, 1, len(h.Evals), "setStatus() didn't update plan: %v", h.Evals)

	newEval = h.Evals[0]
	require.True(t, reflect.DeepEqual(newEval.FailedTGAllocs, metrics),
		"setStatus() didn't set failed task group metrics correctly: %v", newEval)

	// Test queued allocations
	h = NewHarness(t)
	queuedAllocs := map[string]int{"web": 1}

	require.NoError(t, setStatus(logger, h, eval, nil, nil, metrics, status, desc, queuedAllocs, ""))
	require.Equal(t, 1, len(h.Evals), "setStatus() didn't update plan: %v", h.Evals)

	newEval = h.Evals[0]
	require.True(t, reflect.DeepEqual(newEval.QueuedAllocations, queuedAllocs), "setStatus() didn't set failed task group metrics correctly: %v", newEval)

	h = NewHarness(t)
	dID := uuid.Generate()
	require.NoError(t, setStatus(logger, h, eval, nil, nil, metrics, status, desc, queuedAllocs, dID))
	require.Equal(t, 1, len(h.Evals), "setStatus() didn't update plan: %v", h.Evals)

	newEval = h.Evals[0]
	require.Equal(t, dID, newEval.DeploymentID, "setStatus() didn't set deployment id correctly: %v", newEval)
}

func TestInplaceUpdate_ChangedTaskGroup(t *testing.T) {
	ci.Parallel(t)

	state, ctx := testContext(t)
	eval := mock.Eval()
	job := mock.Job()

	node := mock.Node()
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 900, node))

	// Register an alloc
	alloc := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        uuid.Generate(),
		EvalID:    eval.ID,
		NodeID:    node.ID,
		JobID:     job.ID,
		Job:       job,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		TaskGroup:     "web",
	}
	alloc.TaskResources = map[string]*structs.Resources{"web": alloc.Resources}
	require.NoError(t, state.UpsertJobSummary(1000, mock.JobSummary(alloc.JobID)))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))

	// Create a new task group that prevents in-place updates.
	tg := &structs.TaskGroup{}
	*tg = *job.TaskGroups[0]
	task := &structs.Task{
		Name:      "FOO",
		Resources: &structs.Resources{},
	}
	tg.Tasks = nil
	tg.Tasks = append(tg.Tasks, task)

	updates := []allocTuple{{Alloc: alloc, TaskGroup: tg}}
	stack := NewGenericStack(false, ctx)

	// Do the inplace update.
	unplaced, inplace := inplaceUpdate(ctx, eval, job, stack, updates)

	require.True(t, len(unplaced) == 1 && len(inplace) == 0, "inplaceUpdate incorrectly did an inplace update")
	require.Empty(t, ctx.plan.NodeAllocation, "inplaceUpdate incorrectly did an inplace update")
}

func TestInplaceUpdate_AllocatedResources(t *testing.T) {
	ci.Parallel(t)

	state, ctx := testContext(t)
	eval := mock.Eval()
	job := mock.Job()

	node := mock.Node()
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 900, node))

	// Register an alloc
	alloc := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        uuid.Generate(),
		EvalID:    eval.ID,
		NodeID:    node.ID,
		JobID:     job.ID,
		Job:       job,
		AllocatedResources: &structs.AllocatedResources{
			Shared: structs.AllocatedSharedResources{
				Ports: structs.AllocatedPorts{
					{
						Label: "api-port",
						Value: 19910,
						To:    8080,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		TaskGroup:     "web",
	}
	alloc.TaskResources = map[string]*structs.Resources{"web": alloc.Resources}
	require.NoError(t, state.UpsertJobSummary(1000, mock.JobSummary(alloc.JobID)))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))

	// Update TG to add a new service (inplace)
	tg := job.TaskGroups[0]
	tg.Services = []*structs.Service{
		{
			Name: "tg-service",
		},
	}

	updates := []allocTuple{{Alloc: alloc, TaskGroup: tg}}
	stack := NewGenericStack(false, ctx)

	// Do the inplace update.
	unplaced, inplace := inplaceUpdate(ctx, eval, job, stack, updates)

	require.True(t, len(unplaced) == 0 && len(inplace) == 1, "inplaceUpdate incorrectly did not perform an inplace update")
	require.NotEmpty(t, ctx.plan.NodeAllocation, "inplaceUpdate incorrectly did an inplace update")
	require.NotEmpty(t, ctx.plan.NodeAllocation[node.ID][0].AllocatedResources.Shared.Ports)

	port, ok := ctx.plan.NodeAllocation[node.ID][0].AllocatedResources.Shared.Ports.Get("api-port")
	require.True(t, ok)
	require.Equal(t, 19910, port.Value)
}

func TestInplaceUpdate_NoMatch(t *testing.T) {
	ci.Parallel(t)

	state, ctx := testContext(t)
	eval := mock.Eval()
	job := mock.Job()

	node := mock.Node()
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 900, node))

	// Register an alloc
	alloc := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        uuid.Generate(),
		EvalID:    eval.ID,
		NodeID:    node.ID,
		JobID:     job.ID,
		Job:       job,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
		TaskGroup:     "web",
	}
	alloc.TaskResources = map[string]*structs.Resources{"web": alloc.Resources}
	require.NoError(t, state.UpsertJobSummary(1000, mock.JobSummary(alloc.JobID)))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))

	// Create a new task group that requires too much resources.
	tg := &structs.TaskGroup{}
	*tg = *job.TaskGroups[0]
	resource := &structs.Resources{CPU: 9999}
	tg.Tasks[0].Resources = resource

	updates := []allocTuple{{Alloc: alloc, TaskGroup: tg}}
	stack := NewGenericStack(false, ctx)

	// Do the inplace update.
	unplaced, inplace := inplaceUpdate(ctx, eval, job, stack, updates)

	require.True(t, len(unplaced) == 1 && len(inplace) == 0, "inplaceUpdate incorrectly did an inplace update")
	require.Empty(t, ctx.plan.NodeAllocation, "inplaceUpdate incorrectly did an inplace update")
}

func TestInplaceUpdate_Success(t *testing.T) {
	ci.Parallel(t)

	state, ctx := testContext(t)
	eval := mock.Eval()
	job := mock.Job()

	node := mock.Node()
	require.NoError(t, state.UpsertNode(structs.MsgTypeTestSetup, 900, node))

	// Register an alloc
	alloc := &structs.Allocation{
		Namespace: structs.DefaultNamespace,
		ID:        uuid.Generate(),
		EvalID:    eval.ID,
		NodeID:    node.ID,
		JobID:     job.ID,
		Job:       job,
		TaskGroup: job.TaskGroups[0].Name,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				"web": {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 2048,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 2048,
					},
				},
			},
		},
		DesiredStatus: structs.AllocDesiredStatusRun,
	}
	alloc.TaskResources = map[string]*structs.Resources{"web": alloc.Resources}
	require.NoError(t, state.UpsertJobSummary(999, mock.JobSummary(alloc.JobID)))
	require.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))

	// Create a new task group that updates the resources.
	tg := &structs.TaskGroup{}
	*tg = *job.TaskGroups[0]
	resource := &structs.Resources{CPU: 737}
	tg.Tasks[0].Resources = resource
	newServices := []*structs.Service{
		{
			Name:      "dummy-service",
			PortLabel: "http",
		},
		{
			Name:      "dummy-service2",
			PortLabel: "http",
		},
	}

	// Delete service 2
	tg.Tasks[0].Services = tg.Tasks[0].Services[:1]

	// Add the new services
	tg.Tasks[0].Services = append(tg.Tasks[0].Services, newServices...)

	updates := []allocTuple{{Alloc: alloc, TaskGroup: tg}}
	stack := NewGenericStack(false, ctx)
	stack.SetJob(job)

	// Do the inplace update.
	unplaced, inplace := inplaceUpdate(ctx, eval, job, stack, updates)

	require.True(t, len(unplaced) == 0 && len(inplace) == 1, "inplaceUpdate did not do an inplace update")
	require.Equal(t, 1, len(ctx.plan.NodeAllocation), "inplaceUpdate did not do an inplace update")
	require.Equal(t, alloc.ID, inplace[0].Alloc.ID, "inplaceUpdate returned the wrong, inplace updated alloc: %#v", inplace)

	// Get the alloc we inserted.
	a := inplace[0].Alloc // TODO(sean@): Verify this is correct vs: ctx.plan.NodeAllocation[alloc.NodeID][0]
	require.NotNil(t, a.Job)
	require.Equal(t, 1, len(a.Job.TaskGroups))
	require.Equal(t, 1, len(a.Job.TaskGroups[0].Tasks))
	require.Equal(t, 3, len(a.Job.TaskGroups[0].Tasks[0].Services),
		"Expected number of services: %v, Actual: %v", 3, len(a.Job.TaskGroups[0].Tasks[0].Services))

	serviceNames := make(map[string]struct{}, 3)
	for _, consulService := range a.Job.TaskGroups[0].Tasks[0].Services {
		serviceNames[consulService.Name] = struct{}{}
	}
	require.Equal(t, 3, len(serviceNames))

	for _, name := range []string{"dummy-service", "dummy-service2", "web-frontend"} {
		if _, found := serviceNames[name]; !found {
			t.Errorf("Expected consul service name missing: %v", name)
		}
	}
}

func TestInplaceUpdate_WildcardDatacenters(t *testing.T) {
	ci.Parallel(t)

	store, ctx := testContext(t)
	eval := mock.Eval()
	job := mock.Job()
	job.Datacenters = []string{"*"}

	node := mock.Node()
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 900, node))

	// Register an alloc
	alloc := mock.AllocForNode(node)
	alloc.Job = job
	alloc.JobID = job.ID
	must.NoError(t, store.UpsertJobSummary(1000, mock.JobSummary(alloc.JobID)))
	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 1001, []*structs.Allocation{alloc}))

	updates := []allocTuple{{Alloc: alloc, TaskGroup: job.TaskGroups[0]}}
	stack := NewGenericStack(false, ctx)
	unplaced, inplace := inplaceUpdate(ctx, eval, job, stack, updates)

	must.Len(t, 1, inplace,
		must.Sprintf("inplaceUpdate should have an inplace update"))
	must.Len(t, 0, unplaced)
	must.MapNotEmpty(t, ctx.plan.NodeAllocation,
		must.Sprintf("inplaceUpdate should have an inplace update"))
}

func TestInplaceUpdate_NodePools(t *testing.T) {
	ci.Parallel(t)

	store, ctx := testContext(t)
	eval := mock.Eval()
	job := mock.Job()
	job.Datacenters = []string{"*"}

	node1 := mock.Node()
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1000, node1))

	node2 := mock.Node()
	node2.NodePool = "other"
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 1001, node2))

	// Register an alloc
	alloc1 := mock.AllocForNode(node1)
	alloc1.Job = job
	alloc1.JobID = job.ID
	must.NoError(t, store.UpsertJobSummary(1002, mock.JobSummary(alloc1.JobID)))

	alloc2 := mock.AllocForNode(node2)
	alloc2.Job = job
	alloc2.JobID = job.ID
	must.NoError(t, store.UpsertJobSummary(1003, mock.JobSummary(alloc2.JobID)))

	t.Logf("alloc1=%s alloc2=%s", alloc1.ID, alloc2.ID)

	must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, 1004,
		[]*structs.Allocation{alloc1, alloc2}))

	updates := []allocTuple{
		{Alloc: alloc1, TaskGroup: job.TaskGroups[0]},
		{Alloc: alloc2, TaskGroup: job.TaskGroups[0]},
	}
	stack := NewGenericStack(false, ctx)
	destructive, inplace := inplaceUpdate(ctx, eval, job, stack, updates)

	must.Len(t, 1, inplace, must.Sprint("should have an inplace update"))
	must.Eq(t, alloc1.ID, inplace[0].Alloc.ID)
	must.Len(t, 1, ctx.plan.NodeAllocation[node1.ID],
		must.Sprint("NodeAllocation should have an inplace update for node1"))

	// note that NodeUpdate with the new alloc won't be populated here yet
	must.Len(t, 1, destructive, must.Sprint("should have a destructive update"))
	must.Eq(t, alloc2.ID, destructive[0].Alloc.ID)
}

func TestUtil_connectUpdated(t *testing.T) {
	ci.Parallel(t)

	t.Run("both nil", func(t *testing.T) {
		must.False(t, connectUpdated(nil, nil).modified)
	})

	t.Run("one nil", func(t *testing.T) {
		must.True(t, connectUpdated(nil, new(structs.ConsulConnect)).modified)
	})

	t.Run("native differ", func(t *testing.T) {
		a := &structs.ConsulConnect{Native: true}
		b := &structs.ConsulConnect{Native: false}
		must.True(t, connectUpdated(a, b).modified)
	})

	t.Run("gateway differ", func(t *testing.T) {
		a := &structs.ConsulConnect{Gateway: &structs.ConsulGateway{
			Ingress: new(structs.ConsulIngressConfigEntry),
		}}
		b := &structs.ConsulConnect{Gateway: &structs.ConsulGateway{
			Terminating: new(structs.ConsulTerminatingConfigEntry),
		}}
		must.True(t, connectUpdated(a, b).modified)
	})

	t.Run("sidecar task differ", func(t *testing.T) {
		a := &structs.ConsulConnect{SidecarTask: &structs.SidecarTask{
			Driver: "exec",
		}}
		b := &structs.ConsulConnect{SidecarTask: &structs.SidecarTask{
			Driver: "docker",
		}}
		must.True(t, connectUpdated(a, b).modified)
	})

	t.Run("sidecar service differ", func(t *testing.T) {
		a := &structs.ConsulConnect{SidecarService: &structs.ConsulSidecarService{
			Port: "1111",
		}}
		b := &structs.ConsulConnect{SidecarService: &structs.ConsulSidecarService{
			Port: "2222",
		}}
		must.True(t, connectUpdated(a, b).modified)
	})

	t.Run("same", func(t *testing.T) {
		a := new(structs.ConsulConnect)
		b := new(structs.ConsulConnect)
		must.False(t, connectUpdated(a, b).modified)
	})
}

func TestUtil_connectSidecarServiceUpdated(t *testing.T) {
	ci.Parallel(t)

	t.Run("both nil", func(t *testing.T) {
		require.False(t, connectSidecarServiceUpdated(nil, nil).modified)
	})

	t.Run("one nil", func(t *testing.T) {
		require.True(t, connectSidecarServiceUpdated(nil, new(structs.ConsulSidecarService)).modified)
	})

	t.Run("ports differ", func(t *testing.T) {
		a := &structs.ConsulSidecarService{Port: "1111"}
		b := &structs.ConsulSidecarService{Port: "2222"}
		require.True(t, connectSidecarServiceUpdated(a, b).modified)
	})

	t.Run("same", func(t *testing.T) {
		a := &structs.ConsulSidecarService{Port: "1111"}
		b := &structs.ConsulSidecarService{Port: "1111"}
		require.False(t, connectSidecarServiceUpdated(a, b).modified)
	})
}

func TestTasksUpdated_Identity(t *testing.T) {
	ci.Parallel(t)

	j1 := mock.Job()
	name := j1.TaskGroups[0].Name
	j1.TaskGroups[0].Tasks[0].Identity = nil

	j2 := j1.Copy()

	must.False(t, tasksUpdated(j1, j2, name).modified)

	// Set identity on j1 and assert update
	j1.TaskGroups[0].Tasks[0].Identity = &structs.WorkloadIdentity{}

	must.True(t, tasksUpdated(j1, j2, name).modified)
}

func TestTaskGroupConstraints(t *testing.T) {
	ci.Parallel(t)

	constr := &structs.Constraint{RTarget: "bar"}
	constr2 := &structs.Constraint{LTarget: "foo"}
	constr3 := &structs.Constraint{Operand: "<"}

	tg := &structs.TaskGroup{
		Name:          "web",
		Count:         10,
		Constraints:   []*structs.Constraint{constr},
		EphemeralDisk: &structs.EphemeralDisk{},
		Tasks: []*structs.Task{
			{
				Driver: "exec",
				Resources: &structs.Resources{
					CPU:      500,
					MemoryMB: 256,
				},
				Constraints: []*structs.Constraint{constr2},
			},
			{
				Driver: "docker",
				Resources: &structs.Resources{
					CPU:      500,
					MemoryMB: 256,
				},
				Constraints: []*structs.Constraint{constr3},
			},
		},
	}

	// Build the expected values.
	expConstr := []*structs.Constraint{constr, constr2, constr3}
	expDrivers := map[string]struct{}{"exec": {}, "docker": {}}

	actConstrains := taskGroupConstraints(tg)
	require.True(t, reflect.DeepEqual(actConstrains.constraints, expConstr),
		"taskGroupConstraints(%v) returned %v; want %v", tg, actConstrains.constraints, expConstr)
	require.True(t, reflect.DeepEqual(actConstrains.drivers, expDrivers),
		"taskGroupConstraints(%v) returned %v; want %v", tg, actConstrains.drivers, expDrivers)
}

func TestProgressMade(t *testing.T) {
	ci.Parallel(t)

	noopPlan := &structs.PlanResult{}
	require.False(t, progressMade(nil) || progressMade(noopPlan), "no progress plan marked as making progress")

	m := map[string][]*structs.Allocation{
		"foo": {mock.Alloc()},
	}
	both := &structs.PlanResult{
		NodeAllocation: m,
		NodeUpdate:     m,
	}
	update := &structs.PlanResult{NodeUpdate: m}
	alloc := &structs.PlanResult{NodeAllocation: m}
	deployment := &structs.PlanResult{Deployment: mock.Deployment()}
	deploymentUpdates := &structs.PlanResult{
		DeploymentUpdates: []*structs.DeploymentStatusUpdate{
			{DeploymentID: uuid.Generate()},
		},
	}

	require.True(t, progressMade(both) && progressMade(update) && progressMade(alloc) &&
		progressMade(deployment) && progressMade(deploymentUpdates))
}

func TestDesiredUpdates(t *testing.T) {
	ci.Parallel(t)

	tg1 := &structs.TaskGroup{Name: "foo"}
	tg2 := &structs.TaskGroup{Name: "bar"}
	a2 := &structs.Allocation{TaskGroup: "bar"}

	place := []allocTuple{
		{TaskGroup: tg1},
		{TaskGroup: tg1},
		{TaskGroup: tg1},
		{TaskGroup: tg2},
	}
	stop := []allocTuple{
		{TaskGroup: tg2, Alloc: a2},
		{TaskGroup: tg2, Alloc: a2},
	}
	ignore := []allocTuple{
		{TaskGroup: tg1},
	}
	migrate := []allocTuple{
		{TaskGroup: tg2},
	}
	inplace := []allocTuple{
		{TaskGroup: tg1},
		{TaskGroup: tg1},
	}
	destructive := []allocTuple{
		{TaskGroup: tg1},
		{TaskGroup: tg2},
		{TaskGroup: tg2},
	}
	diff := &diffResult{
		place:   place,
		stop:    stop,
		ignore:  ignore,
		migrate: migrate,
	}

	expected := map[string]*structs.DesiredUpdates{
		"foo": {
			Place:             3,
			Ignore:            1,
			InPlaceUpdate:     2,
			DestructiveUpdate: 1,
		},
		"bar": {
			Place:             1,
			Stop:              2,
			Migrate:           1,
			DestructiveUpdate: 2,
		},
	}

	desired := desiredUpdates(diff, inplace, destructive)
	require.True(t, reflect.DeepEqual(desired, expected), "desiredUpdates() returned %#v; want %#v", desired, expected)
}

func TestUtil_AdjustQueuedAllocations(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	alloc1 := mock.Alloc()
	alloc2 := mock.Alloc()
	alloc2.CreateIndex = 4
	alloc2.ModifyIndex = 4
	alloc3 := mock.Alloc()
	alloc3.CreateIndex = 3
	alloc3.ModifyIndex = 5
	alloc4 := mock.Alloc()
	alloc4.CreateIndex = 6
	alloc4.ModifyIndex = 8

	planResult := structs.PlanResult{
		NodeUpdate: map[string][]*structs.Allocation{
			"node-1": {alloc1},
		},
		NodeAllocation: map[string][]*structs.Allocation{
			"node-1": {
				alloc2,
			},
			"node-2": {
				alloc3, alloc4,
			},
		},
		RefreshIndex: 3,
		AllocIndex:   16, // Should not be considered
	}

	queuedAllocs := map[string]int{"web": 2}
	adjustQueuedAllocations(logger, &planResult, queuedAllocs)

	require.Equal(t, 1, queuedAllocs["web"])
}

func TestUtil_UpdateNonTerminalAllocsToLost(t *testing.T) {
	ci.Parallel(t)

	node := mock.Node()
	node.Status = structs.NodeStatusDown
	alloc1 := mock.Alloc()
	alloc1.NodeID = node.ID
	alloc1.DesiredStatus = structs.AllocDesiredStatusStop

	alloc2 := mock.Alloc()
	alloc2.NodeID = node.ID
	alloc2.DesiredStatus = structs.AllocDesiredStatusStop
	alloc2.ClientStatus = structs.AllocClientStatusRunning

	alloc3 := mock.Alloc()
	alloc3.NodeID = node.ID
	alloc3.DesiredStatus = structs.AllocDesiredStatusStop
	alloc3.ClientStatus = structs.AllocClientStatusComplete

	alloc4 := mock.Alloc()
	alloc4.NodeID = node.ID
	alloc4.DesiredStatus = structs.AllocDesiredStatusStop
	alloc4.ClientStatus = structs.AllocClientStatusFailed

	allocs := []*structs.Allocation{alloc1, alloc2, alloc3, alloc4}
	plan := structs.Plan{
		NodeUpdate: make(map[string][]*structs.Allocation),
	}
	tainted := map[string]*structs.Node{node.ID: node}

	updateNonTerminalAllocsToLost(&plan, tainted, allocs)

	allocsLost := make([]string, 0, 2)
	for _, alloc := range plan.NodeUpdate[node.ID] {
		allocsLost = append(allocsLost, alloc.ID)
	}
	expected := []string{alloc1.ID, alloc2.ID}
	require.True(t, reflect.DeepEqual(allocsLost, expected), "actual: %v, expected: %v", allocsLost, expected)

	// Update the node status to ready and try again
	plan = structs.Plan{
		NodeUpdate: make(map[string][]*structs.Allocation),
	}
	node.Status = structs.NodeStatusReady
	updateNonTerminalAllocsToLost(&plan, tainted, allocs)

	allocsLost = make([]string, 0, 2)
	for _, alloc := range plan.NodeUpdate[node.ID] {
		allocsLost = append(allocsLost, alloc.ID)
	}
	expected = []string{}
	require.True(t, reflect.DeepEqual(allocsLost, expected), "actual: %v, expected: %v", allocsLost, expected)
}

func TestTaskGroupUpdated_Restart(t *testing.T) {
	ci.Parallel(t)

	j1 := mock.Job()
	name := j1.TaskGroups[0].Name
	j2 := j1.Copy()
	j3 := j1.Copy()

	must.False(t, tasksUpdated(j1, j2, name).modified)
	j2.TaskGroups[0].RestartPolicy.RenderTemplates = true
	must.True(t, tasksUpdated(j1, j2, name).modified)

	j3.TaskGroups[0].Tasks[0].RestartPolicy.RenderTemplates = true
	must.True(t, tasksUpdated(j1, j3, name).modified)
}
