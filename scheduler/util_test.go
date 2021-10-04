package scheduler

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestMaterializeTaskGroups(t *testing.T) {
	job := mock.Job()
	index := materializeTaskGroups(job)
	require.Equal(t, 10, len(index))

	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("my-job.web[%d]", i)
		require.Contains(t, index, name)
		require.Equal(t, job.TaskGroups[0], index[name])
	}
}

func newNode(name string) *structs.Node {
	n := mock.Node()
	n.Name = name
	return n
}

func TestDiffSystemAllocsForNode_Sysbatch_terminal(t *testing.T) {
	// For a sysbatch job, the scheduler should not re-place an allocation
	// that has become terminal, unless the job has been updated.

	job := mock.SystemBatchJob()
	required := materializeTaskGroups(job)

	eligible := map[string]*structs.Node{
		"node1": newNode("node1"),
	}

	var live []*structs.Allocation // empty

	tainted := map[string]*structs.Node(nil)

	t.Run("current job", func(t *testing.T) {
		terminal := structs.TerminalByNodeByName{
			"node1": map[string]*structs.Allocation{
				"my-sysbatch.pinger[0]": {
					ID:           uuid.Generate(),
					NodeID:       "node1",
					Name:         "my-sysbatch.pinger[0]",
					Job:          job,
					ClientStatus: structs.AllocClientStatusComplete,
				},
			},
		}

		diff := diffSystemAllocsForNode(job, "node1", eligible, tainted, required, live, terminal)
		require.Empty(t, diff.place)
		require.Empty(t, diff.update)
		require.Empty(t, diff.stop)
		require.Empty(t, diff.migrate)
		require.Empty(t, diff.lost)
		require.True(t, len(diff.ignore) == 1 && diff.ignore[0].Alloc == terminal["node1"]["my-sysbatch.pinger[0]"])
	})

	t.Run("outdated job", func(t *testing.T) {
		previousJob := job.Copy()
		previousJob.JobModifyIndex -= 1
		terminal := structs.TerminalByNodeByName{
			"node1": map[string]*structs.Allocation{
				"my-sysbatch.pinger[0]": {
					ID:     uuid.Generate(),
					NodeID: "node1",
					Name:   "my-sysbatch.pinger[0]",
					Job:    previousJob,
				},
			},
		}

		expAlloc := terminal["node1"]["my-sysbatch.pinger[0]"]
		expAlloc.NodeID = "node1"

		diff := diffSystemAllocsForNode(job, "node1", eligible, tainted, required, live, terminal)
		require.Empty(t, diff.place)
		require.Equal(t, 1, len(diff.update))
		require.Empty(t, diff.stop)
		require.Empty(t, diff.migrate)
		require.Empty(t, diff.lost)
		require.Empty(t, diff.ignore)
	})
}

func TestDiffSystemAllocsForNode(t *testing.T) {
	job := mock.Job()
	required := materializeTaskGroups(job)

	// The "old" job has a previous modify index
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	eligibleNode := mock.Node()
	eligibleNode.ID = "zip"

	drainNode := mock.DrainNode()

	deadNode := mock.Node()
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		"dead":      deadNode,
		"drainNode": drainNode,
	}

	eligible := map[string]*structs.Node{
		eligibleNode.ID: eligibleNode,
	}

	allocs := []*structs.Allocation{
		// Update the 1st
		{
			ID:     uuid.Generate(),
			NodeID: "zip",
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},

		// Ignore the 2rd
		{
			ID:     uuid.Generate(),
			NodeID: "zip",
			Name:   "my-job.web[1]",
			Job:    job,
		},

		// Evict 11th
		{
			ID:     uuid.Generate(),
			NodeID: "zip",
			Name:   "my-job.web[10]",
			Job:    oldJob,
		},

		// Migrate the 3rd
		{
			ID:     uuid.Generate(),
			NodeID: "drainNode",
			Name:   "my-job.web[2]",
			Job:    oldJob,
			DesiredTransition: structs.DesiredTransition{
				Migrate: helper.BoolToPtr(true),
			},
		},
		// Mark the 4th lost
		{
			ID:     uuid.Generate(),
			NodeID: "dead",
			Name:   "my-job.web[3]",
			Job:    oldJob,
		},
	}

	// Have three terminal allocs
	terminal := structs.TerminalByNodeByName{
		"zip": map[string]*structs.Allocation{
			"my-job.web[4]": {
				ID:     uuid.Generate(),
				NodeID: "zip",
				Name:   "my-job.web[4]",
				Job:    job,
			},
			"my-job.web[5]": {
				ID:     uuid.Generate(),
				NodeID: "zip",
				Name:   "my-job.web[5]",
				Job:    job,
			},
			"my-job.web[6]": {
				ID:     uuid.Generate(),
				NodeID: "zip",
				Name:   "my-job.web[6]",
				Job:    job,
			},
		},
	}

	diff := diffSystemAllocsForNode(job, "zip", eligible, tainted, required, allocs, terminal)
	place := diff.place
	update := diff.update
	migrate := diff.migrate
	stop := diff.stop
	ignore := diff.ignore
	lost := diff.lost

	// We should update the first alloc
	require.True(t, len(update) == 1 && update[0].Alloc == allocs[0])

	// We should ignore the second alloc
	require.True(t, len(ignore) == 1 && ignore[0].Alloc == allocs[1])

	// We should stop the 3rd alloc
	require.True(t, len(stop) == 1 && stop[0].Alloc == allocs[2])

	// We should migrate the 4rd alloc
	require.True(t, len(migrate) == 1 && migrate[0].Alloc == allocs[3])

	// We should mark the 5th alloc as lost
	require.True(t, len(lost) == 1 && lost[0].Alloc == allocs[4])

	// We should place 6
	require.Equal(t, 6, len(place))

	// Ensure that the allocations which are replacements of terminal allocs are
	// annotated.
	for _, m := range terminal {
		for _, alloc := range m {
			for _, tuple := range diff.place {
				if alloc.Name == tuple.Name {
					require.True(t, reflect.DeepEqual(alloc, tuple.Alloc),
						"expected: %#v, actual: %#v", alloc, tuple.Alloc)
				}
			}
		}
	}
}

// Test the desired diff for an updated system job running on a
// ineligible node
func TestDiffSystemAllocsForNode_ExistingAllocIneligibleNode(t *testing.T) {
	job := mock.Job()
	job.TaskGroups[0].Count = 1
	required := materializeTaskGroups(job)

	// The "old" job has a previous modify index
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	eligibleNode := mock.Node()
	ineligibleNode := mock.Node()
	ineligibleNode.SchedulingEligibility = structs.NodeSchedulingIneligible

	tainted := map[string]*structs.Node{}

	eligible := map[string]*structs.Node{
		eligibleNode.ID: eligibleNode,
	}

	allocs := []*structs.Allocation{
		// Update the TG alloc running on eligible node
		{
			ID:     uuid.Generate(),
			NodeID: eligibleNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},

		// Ignore the TG alloc running on ineligible node
		{
			ID:     uuid.Generate(),
			NodeID: ineligibleNode.ID,
			Name:   "my-job.web[0]",
			Job:    job,
		},
	}

	// No terminal allocs
	terminal := make(structs.TerminalByNodeByName)

	diff := diffSystemAllocsForNode(job, eligibleNode.ID, eligible, tainted, required, allocs, terminal)
	place := diff.place
	update := diff.update
	migrate := diff.migrate
	stop := diff.stop
	ignore := diff.ignore
	lost := diff.lost

	require.Len(t, place, 0)
	require.Len(t, update, 1)
	require.Len(t, migrate, 0)
	require.Len(t, stop, 0)
	require.Len(t, ignore, 1)
	require.Len(t, lost, 0)
}

func TestDiffSystemAllocs(t *testing.T) {
	job := mock.SystemJob()

	drainNode := mock.DrainNode()

	deadNode := mock.Node()
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		deadNode.ID:  deadNode,
		drainNode.ID: drainNode,
	}

	// Create three alive nodes.
	nodes := []*structs.Node{{ID: "foo"}, {ID: "bar"}, {ID: "baz"},
		{ID: "pipe"}, {ID: drainNode.ID}, {ID: deadNode.ID}}

	// The "old" job has a previous modify index
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	allocs := []*structs.Allocation{
		// Update allocation on baz
		{
			ID:     uuid.Generate(),
			NodeID: "baz",
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},

		// Ignore allocation on bar
		{
			ID:     uuid.Generate(),
			NodeID: "bar",
			Name:   "my-job.web[0]",
			Job:    job,
		},

		// Stop allocation on draining node.
		{
			ID:     uuid.Generate(),
			NodeID: drainNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
			DesiredTransition: structs.DesiredTransition{
				Migrate: helper.BoolToPtr(true),
			},
		},
		// Mark as lost on a dead node
		{
			ID:     uuid.Generate(),
			NodeID: deadNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},
	}

	// Have three (?) terminal allocs
	terminal := structs.TerminalByNodeByName{
		"pipe": map[string]*structs.Allocation{
			"my-job.web[0]": {
				ID:     uuid.Generate(),
				NodeID: "pipe",
				Name:   "my-job.web[0]",
				Job:    job,
			},
		},
	}

	diff := diffSystemAllocs(job, nodes, tainted, allocs, terminal)
	place := diff.place
	update := diff.update
	migrate := diff.migrate
	stop := diff.stop
	ignore := diff.ignore
	lost := diff.lost

	// We should update the first alloc
	require.True(t, len(update) == 1 && update[0].Alloc == allocs[0])

	// We should ignore the second alloc
	require.True(t, len(ignore) == 1 && ignore[0].Alloc == allocs[1])

	// We should stop the third alloc
	require.Empty(t, stop)

	// There should be no migrates.
	require.True(t, len(migrate) == 1 && migrate[0].Alloc == allocs[2])

	// We should mark the 5th alloc as lost
	require.True(t, len(lost) == 1 && lost[0].Alloc == allocs[3])

	// We should place 1
	require.Equal(t, 2, len(place))

	// Ensure that the allocations which are replacements of terminal allocs are
	// annotated.
	for _, m := range terminal {
		for _, alloc := range m {
			for _, tuple := range diff.place {
				if alloc.NodeID == tuple.Alloc.NodeID {
					require.True(t, reflect.DeepEqual(alloc, tuple.Alloc),
						"expected: %#v, actual: %#v", alloc, tuple.Alloc)
				}
			}
		}
	}
}

func TestReadyNodesInDCs(t *testing.T) {
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

	nodes, dc, err := readyNodesInDCs(state, []string{"dc1", "dc2"})
	require.NoError(t, err)
	require.Equal(t, 2, len(nodes))
	require.True(t, nodes[0].ID != node3.ID && nodes[1].ID != node3.ID)

	require.Contains(t, dc, "dc1")
	require.Equal(t, 1, dc["dc1"])
	require.Contains(t, dc, "dc2")
	require.Equal(t, 1, dc["dc2"])
}

func TestRetryMax(t *testing.T) {
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
	shuffleNodes(nodes)
	require.False(t, reflect.DeepEqual(nodes, orig))
}

func TestTaskUpdatedAffinity(t *testing.T) {
	j1 := mock.Job()
	j2 := mock.Job()
	name := j1.TaskGroups[0].Name

	require.False(t, tasksUpdated(j1, j2, name))

	// TaskGroup Affinity
	j2.TaskGroups[0].Affinities = []*structs.Affinity{
		{
			LTarget: "node.datacenter",
			RTarget: "dc1",
			Operand: "=",
			Weight:  100,
		},
	}
	require.True(t, tasksUpdated(j1, j2, name))

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

	require.True(t, tasksUpdated(j1, j3, name))

	j4 := mock.Job()
	j4.TaskGroups[0].Tasks[0].Affinities = []*structs.Affinity{
		{
			LTarget: "node.datacenter",
			RTarget: "dc1",
			Operand: "=",
			Weight:  100,
		},
	}

	require.True(t, tasksUpdated(j1, j4, name))

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

	require.False(t, tasksUpdated(j5, j6, name))
}

func TestTaskUpdatedSpread(t *testing.T) {
	j1 := mock.Job()
	j2 := mock.Job()
	name := j1.TaskGroups[0].Name

	require.False(t, tasksUpdated(j1, j2, name))

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
	require.True(t, tasksUpdated(j1, j2, name))

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

	require.False(t, tasksUpdated(j5, j6, name))
}
func TestTasksUpdated(t *testing.T) {
	j1 := mock.Job()
	j2 := mock.Job()
	name := j1.TaskGroups[0].Name
	require.False(t, tasksUpdated(j1, j2, name))

	j2.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
	require.True(t, tasksUpdated(j1, j2, name))

	j3 := mock.Job()
	j3.TaskGroups[0].Tasks[0].Name = "foo"
	require.True(t, tasksUpdated(j1, j3, name))

	j4 := mock.Job()
	j4.TaskGroups[0].Tasks[0].Driver = "foo"
	require.True(t, tasksUpdated(j1, j4, name))

	j5 := mock.Job()
	j5.TaskGroups[0].Tasks = append(j5.TaskGroups[0].Tasks,
		j5.TaskGroups[0].Tasks[0])
	require.True(t, tasksUpdated(j1, j5, name))

	j6 := mock.Job()
	j6.TaskGroups[0].Networks[0].DynamicPorts = []structs.Port{
		{Label: "http", Value: 0},
		{Label: "https", Value: 0},
		{Label: "admin", Value: 0},
	}
	require.True(t, tasksUpdated(j1, j6, name))

	j7 := mock.Job()
	j7.TaskGroups[0].Tasks[0].Env["NEW_ENV"] = "NEW_VALUE"
	require.True(t, tasksUpdated(j1, j7, name))

	j8 := mock.Job()
	j8.TaskGroups[0].Tasks[0].User = "foo"
	require.True(t, tasksUpdated(j1, j8, name))

	j9 := mock.Job()
	j9.TaskGroups[0].Tasks[0].Artifacts = []*structs.TaskArtifact{
		{
			GetterSource: "http://foo.com/bar",
		},
	}
	require.True(t, tasksUpdated(j1, j9, name))

	j10 := mock.Job()
	j10.TaskGroups[0].Tasks[0].Meta["baz"] = "boom"
	require.True(t, tasksUpdated(j1, j10, name))

	j11 := mock.Job()
	j11.TaskGroups[0].Tasks[0].Resources.CPU = 1337
	require.True(t, tasksUpdated(j1, j11, name))

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
	require.True(t, tasksUpdated(j11d1, j11d2, name))

	j13 := mock.Job()
	j13.TaskGroups[0].Networks[0].DynamicPorts[0].Label = "foobar"
	require.True(t, tasksUpdated(j1, j13, name))

	j14 := mock.Job()
	j14.TaskGroups[0].Networks[0].ReservedPorts = []structs.Port{{Label: "foo", Value: 1312}}
	require.True(t, tasksUpdated(j1, j14, name))

	j15 := mock.Job()
	j15.TaskGroups[0].Tasks[0].Vault = &structs.Vault{Policies: []string{"foo"}}
	require.True(t, tasksUpdated(j1, j15, name))

	j16 := mock.Job()
	j16.TaskGroups[0].EphemeralDisk.Sticky = true
	require.True(t, tasksUpdated(j1, j16, name))

	// Change group meta
	j17 := mock.Job()
	j17.TaskGroups[0].Meta["j17_test"] = "roll_baby_roll"
	require.True(t, tasksUpdated(j1, j17, name))

	// Change job meta
	j18 := mock.Job()
	j18.Meta["j18_test"] = "roll_baby_roll"
	require.True(t, tasksUpdated(j1, j18, name))

	// Change network mode
	j19 := mock.Job()
	j19.TaskGroups[0].Networks[0].Mode = "bridge"
	require.True(t, tasksUpdated(j1, j19, name))

	// Change cores resource
	j20 := mock.Job()
	j20.TaskGroups[0].Tasks[0].Resources.CPU = 0
	j20.TaskGroups[0].Tasks[0].Resources.Cores = 2
	j21 := mock.Job()
	j21.TaskGroups[0].Tasks[0].Resources.CPU = 0
	j21.TaskGroups[0].Tasks[0].Resources.Cores = 4
	require.True(t, tasksUpdated(j20, j21, name))

}

func TestTasksUpdated_connectServiceUpdated(t *testing.T) {
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
		updated := connectServiceUpdated(servicesA, servicesB)
		require.False(t, updated)
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
		updated := connectServiceUpdated(servicesA, servicesB)
		require.False(t, updated)
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
		updated := connectServiceUpdated(servicesA, servicesB)
		require.True(t, updated)
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
		updated := connectServiceUpdated(servicesA, servicesB)
		require.True(t, updated)
	})
}

func TestNetworkUpdated(t *testing.T) {
	t.Parallel()
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

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(tc *testing.T) {
			tc.Parallel()
			require.Equal(tc, c.updated, networkUpdated(c.a, c.b), "unexpected network updated result")
		})
	}
}

func TestEvictAndPlace_LimitLessThanAllocs(t *testing.T) {
	_, ctx := testContext(t)
	allocs := []allocTuple{
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
	}
	diff := &diffResult{}

	limit := 2
	require.True(t, evictAndPlace(ctx, diff, allocs, "", &limit), "evictAndReplace() should have returned true")
	require.Zero(t, limit, "evictAndReplace() should decremented limit; got %v; want 0", limit)
	require.Equal(t, 2, len(diff.place), "evictAndReplace() didn't insert into diffResult properly: %v", diff.place)
}

func TestEvictAndPlace_LimitEqualToAllocs(t *testing.T) {
	_, ctx := testContext(t)
	allocs := []allocTuple{
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
	}
	diff := &diffResult{}

	limit := 4
	require.False(t, evictAndPlace(ctx, diff, allocs, "", &limit), "evictAndReplace() should have returned false")
	require.Zero(t, limit, "evictAndReplace() should decremented limit; got %v; want 0", limit)
	require.Equal(t, 4, len(diff.place), "evictAndReplace() didn't insert into diffResult properly: %v", diff.place)
}

func TestSetStatus(t *testing.T) {
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

func TestEvictAndPlace_LimitGreaterThanAllocs(t *testing.T) {
	_, ctx := testContext(t)
	allocs := []allocTuple{
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
		{Alloc: &structs.Allocation{ID: uuid.Generate()}},
	}
	diff := &diffResult{}

	limit := 6
	require.False(t, evictAndPlace(ctx, diff, allocs, "", &limit))
	require.Equal(t, 2, limit, "evictAndReplace() should decremented limit")
	require.Equal(t, 4, len(diff.place), "evictAndReplace() didn't insert into diffResult properly: %v", diff.place)
}

func TestTaskGroupConstraints(t *testing.T) {
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

func TestUtil_connectUpdated(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		require.False(t, connectUpdated(nil, nil))
	})

	t.Run("one nil", func(t *testing.T) {
		require.True(t, connectUpdated(nil, new(structs.ConsulConnect)))
	})

	t.Run("native differ", func(t *testing.T) {
		a := &structs.ConsulConnect{Native: true}
		b := &structs.ConsulConnect{Native: false}
		require.True(t, connectUpdated(a, b))
	})

	t.Run("gateway differ", func(t *testing.T) {
		a := &structs.ConsulConnect{Gateway: &structs.ConsulGateway{
			Ingress: new(structs.ConsulIngressConfigEntry),
		}}
		b := &structs.ConsulConnect{Gateway: &structs.ConsulGateway{
			Terminating: new(structs.ConsulTerminatingConfigEntry),
		}}
		require.True(t, connectUpdated(a, b))
	})

	t.Run("sidecar task differ", func(t *testing.T) {
		a := &structs.ConsulConnect{SidecarTask: &structs.SidecarTask{
			Driver: "exec",
		}}
		b := &structs.ConsulConnect{SidecarTask: &structs.SidecarTask{
			Driver: "docker",
		}}
		require.True(t, connectUpdated(a, b))
	})

	t.Run("sidecar service differ", func(t *testing.T) {
		a := &structs.ConsulConnect{SidecarService: &structs.ConsulSidecarService{
			Port: "1111",
		}}
		b := &structs.ConsulConnect{SidecarService: &structs.ConsulSidecarService{
			Port: "2222",
		}}
		require.True(t, connectUpdated(a, b))
	})

	t.Run("same", func(t *testing.T) {
		a := new(structs.ConsulConnect)
		b := new(structs.ConsulConnect)
		require.False(t, connectUpdated(a, b))
	})
}

func TestUtil_connectSidecarServiceUpdated(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		require.False(t, connectSidecarServiceUpdated(nil, nil))
	})

	t.Run("one nil", func(t *testing.T) {
		require.True(t, connectSidecarServiceUpdated(nil, new(structs.ConsulSidecarService)))
	})

	t.Run("ports differ", func(t *testing.T) {
		a := &structs.ConsulSidecarService{Port: "1111"}
		b := &structs.ConsulSidecarService{Port: "2222"}
		require.True(t, connectSidecarServiceUpdated(a, b))
	})

	t.Run("same", func(t *testing.T) {
		a := &structs.ConsulSidecarService{Port: "1111"}
		b := &structs.ConsulSidecarService{Port: "1111"}
		require.False(t, connectSidecarServiceUpdated(a, b))
	})
}
