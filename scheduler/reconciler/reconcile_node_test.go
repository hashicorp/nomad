// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"fmt"
	"testing"
	"time"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

// diffResultCount is a test helper struct that makes it easier to specify an
// expected diff
type diffResultCount struct {
	place, update, migrate, stop, ignore, lost, disconnecting, reconnecting int
}

// assertDiffCount is a test helper that compares against a diffResult
func assertDiffCount(t *testing.T, expected diffResultCount, diff *NodeReconcileResult) {
	t.Helper()
	test.Len(t, expected.update, diff.Update, test.Sprintf("expected update"))
	test.Len(t, expected.ignore, diff.Ignore, test.Sprintf("expected ignore"))
	test.Len(t, expected.stop, diff.Stop, test.Sprintf("expected stop"))
	test.Len(t, expected.migrate, diff.Migrate, test.Sprintf("expected migrate"))
	test.Len(t, expected.lost, diff.Lost, test.Sprintf("expected lost"))
	test.Len(t, expected.place, diff.Place, test.Sprintf("expected place"))
}

func newNode(name string) *structs.Node {
	n := mock.Node()
	n.Name = name
	return n
}

func TestDiffSystemAllocsForNode_Sysbatch_terminal(t *testing.T) {
	ci.Parallel(t)

	// For a sysbatch job, the scheduler should not re-place an allocation
	// that has become terminal, unless the job has been updated.

	job := mock.SystemBatchJob()
	job.TaskGroups[0].Count = 2
	required := materializeSystemTaskGroups(job)

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

		nr := NewNodeReconciler(nil)
		diff, _ := nr.computeForNode(job, "node1", eligible, nil, tainted, nil, nil, required, live, terminal, true)

		assertDiffCount(t, diffResultCount{ignore: 1, place: 1}, diff)
		if len(diff.Ignore) > 0 {
			must.Eq(t, terminal["node1"]["my-sysbatch.pinger[0]"], diff.Ignore[0].Alloc)
		}
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

		nr := NewNodeReconciler(nil)
		diff, _ := nr.computeForNode(job, "node1", eligible, nil, tainted, nil, nil, required, live, terminal, true)
		assertDiffCount(t, diffResultCount{update: 1, place: 1}, diff)
	})

}

// TestDiffSystemAllocsForNode_Placements verifies we only place on nodes that
// need placements
func TestDiffSystemAllocsForNode_Placements(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	required := materializeSystemTaskGroups(job)

	goodNode := mock.Node()
	unusedNode := mock.Node()
	drainNode := mock.DrainNode()
	deadNode := mock.Node()
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		deadNode.ID:  deadNode,
		drainNode.ID: drainNode,
	}

	eligible := map[string]*structs.Node{
		goodNode.ID: goodNode,
	}

	terminal := structs.TerminalByNodeByName{}
	allocsForNode := []*structs.Allocation{}

	testCases := []struct {
		name     string
		nodeID   string
		expected diffResultCount
	}{
		{
			name:     "expect placement on good node",
			nodeID:   goodNode.ID,
			expected: diffResultCount{place: 1},
		},
		{ // "unused" here means outside of the eligible set
			name:     "expect no placement on unused node",
			nodeID:   unusedNode.ID,
			expected: diffResultCount{},
		},
		{
			name:     "expect no placement on dead node",
			nodeID:   deadNode.ID,
			expected: diffResultCount{},
		},
		{
			name:     "expect no placement on draining node",
			nodeID:   drainNode.ID,
			expected: diffResultCount{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nr := NewNodeReconciler(nil)
			diff, _ := nr.computeForNode(
				job, tc.nodeID, eligible, nil,
				tainted, nil, nil, required, allocsForNode, terminal, true)

			assertDiffCount(t, tc.expected, diff)
		})
	}
}

// TestDiffSystemAllocsForNodes_Stops verifies we stop allocs we no longer need
func TestDiffSystemAllocsForNode_Stops(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	required := materializeSystemTaskGroups(job)

	// The "old" job has a previous modify index but is otherwise unchanged, so
	// existing non-terminal allocs for this version should be updated in-place

	// TODO(tgross): *unless* there's another alloc for the same job already on
	// the node. See https://github.com/hashicorp/nomad/pull/16097
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	node := mock.Node()

	eligible := map[string]*structs.Node{
		node.ID: node,
	}

	allocs := []*structs.Allocation{
		{
			// extraneous alloc for old version of job should be updated
			// TODO(tgross): this should actually be stopped.
			// See https://github.com/hashicorp/nomad/pull/16097
			ID:     uuid.Generate(),
			NodeID: node.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},
		{ // most recent alloc for current version of job should be ignored
			ID:     uuid.Generate(),
			NodeID: node.ID,
			Name:   "my-job.web[0]",
			Job:    job,
		},
		{ // task group not required, should be stopped
			ID:     uuid.Generate(),
			NodeID: node.ID,
			Name:   "my-job.something-else[0]",
			Job:    job,
		},
	}

	tainted := map[string]*structs.Node{}
	terminal := structs.TerminalByNodeByName{}

	nr := NewNodeReconciler(nil)
	diff, _ := nr.computeForNode(
		job, node.ID, eligible, nil, tainted, nil, nil, required, allocs, terminal, true)

	assertDiffCount(t, diffResultCount{ignore: 1, stop: 1, update: 1}, diff)
	if len(diff.Update) > 0 {
		test.Eq(t, allocs[0], diff.Update[0].Alloc)
	}
	if len(diff.Ignore) > 0 {
		test.Eq(t, allocs[1], diff.Ignore[0].Alloc)
	}
	if len(diff.Stop) > 0 {
		test.Eq(t, allocs[2], diff.Stop[0].Alloc)
	}
}

// Test the desired diff for an updated system job running on a ineligible node
func TestDiffSystemAllocsForNode_IneligibleNode(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	required := materializeSystemTaskGroups(job)

	ineligibleNode := mock.Node()
	ineligibleNode.SchedulingEligibility = structs.NodeSchedulingIneligible
	ineligible := map[string]struct{}{
		ineligibleNode.ID: {},
	}

	eligible := map[string]*structs.Node{}
	tainted := map[string]*structs.Node{}

	terminal := structs.TerminalByNodeByName{
		ineligibleNode.ID: map[string]*structs.Allocation{
			"my-job.web[0]": { // terminal allocs should not appear in diff
				ID:           uuid.Generate(),
				NodeID:       ineligibleNode.ID,
				Name:         "my-job.web[0]",
				Job:          job,
				ClientStatus: structs.AllocClientStatusComplete,
			},
		},
	}

	testCases := []struct {
		name   string
		nodeID string
		expect diffResultCount
	}{
		{
			name:   "non-terminal alloc on ineligible node should be ignored",
			nodeID: ineligibleNode.ID,
			expect: diffResultCount{ignore: 1},
		},
		{
			name:   "non-terminal alloc on node not in eligible set should be stopped",
			nodeID: uuid.Generate(),
			expect: diffResultCount{stop: 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alloc := &structs.Allocation{
				ID:     uuid.Generate(),
				NodeID: tc.nodeID,
				Name:   "my-job.web[0]",
				Job:    job,
			}

			nr := NewNodeReconciler(nil)
			diff, _ := nr.computeForNode(
				job, tc.nodeID, eligible, ineligible, tainted, nil, nil,
				required, []*structs.Allocation{alloc}, terminal, true,
			)
			assertDiffCount(t, tc.expect, diff)
		})
	}
}

func TestDiffSystemAllocsForNode_DrainingNode(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	required := materializeSystemTaskGroups(job)

	// The "old" job has a previous modify index but is otherwise unchanged, so
	// existing non-terminal allocs for this version should be updated in-place
	oldJob := job.Copy()
	oldJob.JobModifyIndex -= 1

	drainNode := mock.DrainNode()
	tainted := map[string]*structs.Node{
		drainNode.ID: drainNode,
	}

	// Terminal allocs don't get touched
	terminal := structs.TerminalByNodeByName{
		drainNode.ID: map[string]*structs.Allocation{
			"my-job.web[0]": {
				ID:           uuid.Generate(),
				NodeID:       drainNode.ID,
				Name:         "my-job.web[0]",
				Job:          job,
				ClientStatus: structs.AllocClientStatusComplete,
			},
		},
	}

	allocs := []*structs.Allocation{
		{ // allocs for draining node should be migrated
			ID:     uuid.Generate(),
			NodeID: drainNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
			DesiredTransition: structs.DesiredTransition{
				Migrate: pointer.Of(true),
			},
		},
		{ // allocs not marked for drain should be ignored
			ID:     uuid.Generate(),
			NodeID: drainNode.ID,
			Name:   "my-job.web[0]",
			Job:    job,
		},
	}

	nr := NewNodeReconciler(nil)
	diff, _ := nr.computeForNode(
		job, drainNode.ID, map[string]*structs.Node{}, nil,
		tainted, nil, nil, required, allocs, terminal, true)

	assertDiffCount(t, diffResultCount{migrate: 1, ignore: 1}, diff)
	if len(diff.Migrate) > 0 {
		test.Eq(t, allocs[0], diff.Migrate[0].Alloc)
	}
}

func TestDiffSystemAllocsForNode_LostNode(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	required := materializeSystemTaskGroups(job)

	// The "old" job has a previous modify index but is otherwise unchanged, so
	// existing non-terminal allocs for this version should be updated in-place
	oldJob := new(structs.Job)
	*oldJob = *job
	oldJob.JobModifyIndex -= 1

	deadNode := mock.Node()
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		deadNode.ID: deadNode,
	}

	allocs := []*structs.Allocation{
		{ // current allocs on a lost node are lost, even if terminal
			ID:     uuid.Generate(),
			NodeID: deadNode.ID,
			Name:   "my-job.web[0]",
			Job:    job,
		},
		{ // old allocs on a lost node are also lost
			ID:     uuid.Generate(),
			NodeID: deadNode.ID,
			Name:   "my-job.web[0]",
			Job:    oldJob,
		},
	}

	// Terminal allocs don't get touched
	terminal := structs.TerminalByNodeByName{
		deadNode.ID: map[string]*structs.Allocation{
			"my-job.web[0]": allocs[0],
		},
	}

	nr := NewNodeReconciler(nil)
	diff, _ := nr.computeForNode(
		job, deadNode.ID, map[string]*structs.Node{}, nil,
		tainted, nil, nil, required, allocs, terminal, true)

	assertDiffCount(t, diffResultCount{lost: 2}, diff)
	if len(diff.Migrate) > 0 {
		test.Eq(t, allocs[0], diff.Migrate[0].Alloc)
	}
}

func TestDiffSystemAllocsForNode_DisconnectedNode(t *testing.T) {
	ci.Parallel(t)

	// Create job.
	job := mock.SystemJob()
	job.TaskGroups[0].Disconnect = &structs.DisconnectStrategy{
		LostAfter: time.Hour,
	}

	// Create nodes.
	readyNode := mock.Node()
	readyNode.Status = structs.NodeStatusReady

	disconnectedNode := mock.Node()
	disconnectedNode.Status = structs.NodeStatusDisconnected

	eligibleNodes := map[string]*structs.Node{
		readyNode.ID: readyNode,
	}

	taintedNodes := map[string]*structs.Node{
		disconnectedNode.ID: disconnectedNode,
	}

	// Create allocs.
	required := materializeSystemTaskGroups(job)
	terminal := make(structs.TerminalByNodeByName)

	testCases := []struct {
		name    string
		node    *structs.Node
		allocFn func(*structs.Allocation)
		expect  diffResultCount
	}{
		{
			name: "alloc in disconnected client is marked as unknown",
			node: disconnectedNode,
			allocFn: func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusRunning
			},
			expect: diffResultCount{disconnecting: 1},
		},
		{
			name: "disconnected alloc reconnects",
			node: readyNode,
			allocFn: func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.AllocStates = []*structs.AllocState{{
					Field: structs.AllocStateFieldClientStatus,
					Value: structs.AllocClientStatusUnknown,
					Time:  time.Now().Add(-time.Minute),
				}}
			},
			expect: diffResultCount{reconnecting: 1},
		},
		{
			name: "alloc not reconnecting after it reconnects",
			node: readyNode,
			allocFn: func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusRunning

				alloc.AllocStates = []*structs.AllocState{
					{
						Field: structs.AllocStateFieldClientStatus,
						Value: structs.AllocClientStatusUnknown,
						Time:  time.Now().Add(-time.Minute),
					},
					{
						Field: structs.AllocStateFieldClientStatus,
						Value: structs.AllocClientStatusRunning,
						Time:  time.Now(),
					},
				}
			},
			expect: diffResultCount{ignore: 1},
		},
		{
			name: "disconnected alloc is lost after it expires",
			node: disconnectedNode,
			allocFn: func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusUnknown
				alloc.AllocStates = []*structs.AllocState{{
					Field: structs.AllocStateFieldClientStatus,
					Value: structs.AllocClientStatusUnknown,
					Time:  time.Now().Add(-10 * time.Hour),
				}}
			},
			expect: diffResultCount{lost: 1},
		},
		{
			name: "disconnected allocs are ignored",
			node: disconnectedNode,
			allocFn: func(alloc *structs.Allocation) {
				alloc.ClientStatus = structs.AllocClientStatusUnknown
				alloc.AllocStates = []*structs.AllocState{{
					Field: structs.AllocStateFieldClientStatus,
					Value: structs.AllocClientStatusUnknown,
					Time:  time.Now(),
				}}
			},
			expect: diffResultCount{ignore: 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alloc := mock.AllocForNode(tc.node)
			alloc.JobID = job.ID
			alloc.Job = job
			alloc.Name = fmt.Sprintf("%s.%s[0]", job.Name, job.TaskGroups[0].Name)

			if tc.allocFn != nil {
				tc.allocFn(alloc)
			}

			nr := NewNodeReconciler(nil)
			got, _ := nr.computeForNode(
				job, tc.node.ID, eligibleNodes, nil, taintedNodes, nil, nil,
				required, []*structs.Allocation{alloc}, terminal, true,
			)
			assertDiffCount(t, tc.expect, got)
		})
	}
}

// TestDiffSystemAllocs is a higher-level test of interactions of diffs across
// multiple nodes.
func TestDiffSystemAllocs(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	tg := job.TaskGroups[0].Copy()
	tg.Name = "other"
	job.TaskGroups = append(job.TaskGroups, tg)

	drainNode := mock.DrainNode()
	drainNode.ID = "drain"

	deadNode := mock.Node()
	deadNode.ID = "dead"
	deadNode.Status = structs.NodeStatusDown

	tainted := map[string]*structs.Node{
		deadNode.ID:  deadNode,
		drainNode.ID: drainNode,
	}

	// Create four alive nodes.
	nodes := []*structs.Node{{ID: "foo"}, {ID: "bar"}, {ID: "baz"},
		{ID: "has-term"}, {ID: drainNode.ID}, {ID: deadNode.ID}}

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
				Migrate: pointer.Of(true),
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

	// Have one terminal allocs
	terminal := structs.TerminalByNodeByName{
		"has-term": map[string]*structs.Allocation{
			"my-job.web[0]": {
				ID:     uuid.Generate(),
				NodeID: "has-term",
				Name:   "my-job.web[0]",
				Job:    job,
			},
		},
	}

	nr := NewNodeReconciler(nil)
	diff := nr.Compute(job, nodes, nil, tainted, allocs, terminal, true)
	assertDiffCount(t, diffResultCount{
		update: 1, ignore: 1, migrate: 1, lost: 1, place: 6}, diff)

	if len(diff.Update) > 0 {
		must.Eq(t, allocs[0], diff.Update[0].Alloc) // first alloc should be updated
	}
	if len(diff.Ignore) > 0 {
		must.Eq(t, allocs[1], diff.Ignore[0].Alloc) // We should ignore the second alloc
	}
	if len(diff.Migrate) > 0 {
		must.Eq(t, allocs[2], diff.Migrate[0].Alloc)
	}
	if len(diff.Lost) > 0 {
		must.Eq(t, allocs[3], diff.Lost[0].Alloc) // We should mark the 5th alloc as lost
	}

	// Ensure that the allocations which are replacements of terminal allocs are
	// annotated.
	for _, m := range terminal {
		for _, alloc := range m {
			for _, tuple := range diff.Place {
				if alloc.NodeID == tuple.Alloc.NodeID && alloc.TaskGroup == "web" {
					must.Eq(t, alloc, tuple.Alloc)
				}
			}
		}
	}
}

// TestNodeDeployments tests various deployment-related scenarios for the node
// reconciler
func TestNodeDeployments(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	tg := job.TaskGroups[0].Copy()
	tg.Name = "other"
	tg.Update = structs.DefaultUpdateStrategy
	job.TaskGroups = append(job.TaskGroups, tg)

	// Create two alive nodes.
	nodes := []*structs.Node{{ID: "foo"}, {ID: "bar"}}

	// Stopped job to make sure we handle these correctly
	stoppedJob := job.Copy()
	stoppedJob.Stop = true

	allocs := []*structs.Allocation{}
	for _, n := range nodes {
		a := mock.Alloc()
		a.Job = job
		a.Name = "my-job.web[0]"
		a.NodeID = n.ID
		a.NodeName = n.Name

		allocs = append(allocs, a)
	}

	newJobWithNoAllocs := job.Copy()
	newJobWithNoAllocs.Name = "new-job"
	newJobWithNoAllocs.Version = 100
	newJobWithNoAllocs.CreateIndex = 1000

	testCases := []struct {
		name                                  string
		job                                   *structs.Job
		existingDeployment                    *structs.Deployment
		newDeployment                         bool
		expectedNewDeploymentStatus           string
		expectedDeploymenStatusUpdateContains string
	}{
		{
			"existing successful deployment for the current job version should not return a deployment",
			job,
			&structs.Deployment{
				JobCreateIndex: job.CreateIndex,
				JobVersion:     job.Version,
				Status:         structs.DeploymentStatusSuccessful,
			},
			false,
			"",
			"",
		},
		{
			"existing running deployment should remain untouched",
			job,
			&structs.Deployment{
				JobID:             job.ID,
				JobCreateIndex:    job.CreateIndex,
				JobVersion:        job.Version,
				Status:            structs.DeploymentStatusRunning,
				StatusDescription: structs.DeploymentStatusDescriptionRunning,
				TaskGroups: map[string]*structs.DeploymentState{
					job.TaskGroups[0].Name: {
						AutoRevert:       true,
						ProgressDeadline: time.Minute,
					},
					tg.Name: {
						AutoPromote: true,
					},
				},
			},
			false,
			structs.DeploymentStatusSuccessful,
			"",
		},
		{
			"existing running deployment for a stopped job should be cancelled",
			stoppedJob,
			&structs.Deployment{
				JobCreateIndex: job.CreateIndex,
				JobVersion:     job.Version,
				Status:         structs.DeploymentStatusRunning,
			},
			false,
			structs.DeploymentStatusCancelled,
			structs.DeploymentStatusDescriptionStoppedJob,
		},
		{
			"no existing deployment for a new job that needs one should result in a new deployment",
			newJobWithNoAllocs,
			nil,
			true,
			structs.DeploymentStatusRunning,
			structs.DeploymentStatusDescriptionRunning,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nr := NewNodeReconciler(tc.existingDeployment)
			nr.Compute(tc.job, nodes, nil, nil, allocs, nil, true)
			if tc.newDeployment {
				must.NotNil(t, nr.DeploymentCurrent, must.Sprintf("expected a non-nil deployment"))
				must.Eq(t, nr.DeploymentCurrent.Status, tc.expectedNewDeploymentStatus)
			}
			if tc.expectedDeploymenStatusUpdateContains != "" {
				must.SliceContainsFunc(t, nr.DeploymentUpdates, tc.expectedDeploymenStatusUpdateContains,
					func(a *structs.DeploymentStatusUpdate, status string) bool {
						return a.StatusDescription == status
					},
				)
			}
		})
	}
}

func Test_computeCanaryNodes(t *testing.T) {
	ci.Parallel(t)

	// generate an odd number of nodes
	fiveEligibleNodes := map[string]*structs.Node{}
	// name them so we can refer to their names while testing pre-existing
	// canary allocs
	fiveEligibleNodeNames := []string{"node1", "node2", "node3", "node4", "node5"}
	for _, name := range fiveEligibleNodeNames {
		node := mock.Node()
		node.ID = name
		fiveEligibleNodes[name] = node
	}

	// generate an even number of nodes
	fourEligibleNodes := map[string]*structs.Node{}
	for range 4 {
		nodeID := uuid.Generate()
		node := mock.Node()
		node.ID = nodeID
		fourEligibleNodes[nodeID] = node
	}

	testCases := []struct {
		name                 string
		nodes                map[string]*structs.Node
		liveAllocs           map[string][]*structs.Allocation
		terminalAllocs       structs.TerminalByNodeByName
		required             map[string]*structs.TaskGroup
		existingDeployment   *structs.Deployment
		expectedCanaryNodes  map[string]int    // number of nodes per tg
		expectedCanaryNodeID map[string]string // sometimes we want to make sure a particular node ID is a canary
	}{
		{
			name:                 "no required task groups",
			nodes:                fourEligibleNodes,
			liveAllocs:           nil,
			terminalAllocs:       nil,
			required:             nil,
			existingDeployment:   nil,
			expectedCanaryNodes:  map[string]int{},
			expectedCanaryNodeID: nil,
		},
		{
			name:           "one task group with no update strategy",
			nodes:          fourEligibleNodes,
			liveAllocs:     nil,
			terminalAllocs: nil,
			required: map[string]*structs.TaskGroup{
				"foo": {
					Name: "foo",
				}},
			existingDeployment:   nil,
			expectedCanaryNodes:  map[string]int{},
			expectedCanaryNodeID: nil,
		},
		{
			name:           "one task group with 33% canary deployment",
			nodes:          fourEligibleNodes,
			liveAllocs:     nil,
			terminalAllocs: nil,
			required: map[string]*structs.TaskGroup{
				"foo": {
					Name: "foo",
					Update: &structs.UpdateStrategy{
						Canary:      33,
						MaxParallel: 1, // otherwise the update strategy will be considered nil
					},
				},
			},
			existingDeployment: nil,
			expectedCanaryNodes: map[string]int{
				"foo": 2, // we always round up
			},
			expectedCanaryNodeID: nil,
		},
		{
			name:           "one task group with 100% canary deployment, four nodes",
			nodes:          fourEligibleNodes,
			liveAllocs:     nil,
			terminalAllocs: nil,
			required: map[string]*structs.TaskGroup{
				"foo": {
					Name: "foo",
					Update: &structs.UpdateStrategy{
						Canary:      100,
						MaxParallel: 1, // otherwise the update strategy will be considered nil
					},
				},
			},
			existingDeployment: nil,
			expectedCanaryNodes: map[string]int{
				"foo": 4,
			},
			expectedCanaryNodeID: nil,
		},
		{
			name:           "one task group with 50% canary deployment, even nodes",
			nodes:          fourEligibleNodes,
			liveAllocs:     nil,
			terminalAllocs: nil,
			required: map[string]*structs.TaskGroup{
				"foo": {
					Name: "foo",
					Update: &structs.UpdateStrategy{
						Canary:      50,
						MaxParallel: 1, // otherwise the update strategy will be considered nil
					},
				},
			},
			existingDeployment: nil,
			expectedCanaryNodes: map[string]int{
				"foo": 2,
			},
			expectedCanaryNodeID: nil,
		},
		{
			name:  "two task groups: one with 50% canary deploy, second one with 2% canary deploy, pre-existing canary alloc",
			nodes: fiveEligibleNodes,
			liveAllocs: map[string][]*structs.Allocation{
				"foo": {mock.Alloc()}, // should be disregarded since it's not one of our nodes
				fiveEligibleNodeNames[0]: {
					{DeploymentStatus: nil},
					{DeploymentStatus: &structs.AllocDeploymentStatus{Canary: false}},
					{DeploymentStatus: &structs.AllocDeploymentStatus{Canary: true}, TaskGroup: "foo"},
				},
				fiveEligibleNodeNames[1]: {
					{DeploymentStatus: &structs.AllocDeploymentStatus{Canary: true}, TaskGroup: "bar"},
				},
			},
			terminalAllocs: structs.TerminalByNodeByName{
				fiveEligibleNodeNames[2]: map[string]*structs.Allocation{
					"foo": {
						DeploymentStatus: &structs.AllocDeploymentStatus{
							Canary: true,
						},
						TaskGroup: "foo",
					},
				},
			},
			required: map[string]*structs.TaskGroup{
				"foo": {
					Name: "foo",
					Update: &structs.UpdateStrategy{
						Canary:      50,
						MaxParallel: 1, // otherwise the update strategy will be considered nil
					},
				},
				"bar": {
					Name: "bar",
					Update: &structs.UpdateStrategy{
						Canary:      2,
						MaxParallel: 1, // otherwise the update strategy will be considered nil
					},
				},
			},
			existingDeployment: structs.NewDeployment(mock.SystemJob(), 100, time.Now().Unix()),
			expectedCanaryNodes: map[string]int{
				"foo": 3, // we always round up
				"bar": 1, // we always round up
			},
			expectedCanaryNodeID: map[string]string{
				fiveEligibleNodeNames[0]: "foo",
				fiveEligibleNodeNames[1]: "bar",
				fiveEligibleNodeNames[2]: "foo",
			},
		},
		{
			name:           "task group with 100% canary deploy, 1 eligible node",
			nodes:          map[string]*structs.Node{"foo": mock.Node()},
			liveAllocs:     nil,
			terminalAllocs: nil,
			required: map[string]*structs.TaskGroup{
				"foo": {
					Name: "foo",
					Update: &structs.UpdateStrategy{
						Canary:      100,
						MaxParallel: 1,
					},
				},
			},
			existingDeployment: nil,
			expectedCanaryNodes: map[string]int{
				"foo": 1,
			},
			expectedCanaryNodeID: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nr := NewNodeReconciler(tc.existingDeployment)
			canaryNodes, canariesPerTG := nr.computeCanaryNodes(tc.required, tc.liveAllocs, tc.terminalAllocs, tc.nodes)
			must.Eq(t, tc.expectedCanaryNodes, canariesPerTG)
			if tc.liveAllocs != nil {
				for nodeID, tgName := range tc.expectedCanaryNodeID {
					must.True(t, canaryNodes[nodeID][tgName])
				}
			}
		})
	}
}

// Tests the reconciler creates new canaries when the job changes
func TestNodeReconciler_NewCanaries(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	job.TaskGroups[0].Update = &structs.UpdateStrategy{
		Canary:      20, // deploy to 20% of eligible nodes
		MaxParallel: 1,  // otherwise the update strategy will be considered nil
	}
	job.JobModifyIndex = 1

	// Create 10 nodes
	nodes := []*structs.Node{}
	for i := range 10 {
		node := mock.Node()
		node.ID = fmt.Sprintf("node_%d", i)
		node.Name = fmt.Sprintf("node_%d", i)
		nodes = append(nodes, node)
	}

	allocs := []*structs.Allocation{}
	for _, n := range nodes {
		a := mock.Alloc()
		a.Job = job
		a.Name = "my-job.web[0]"
		a.NodeID = n.ID
		a.NodeName = n.Name
		a.TaskGroup = job.TaskGroups[0].Name

		allocs = append(allocs, a)
	}

	// bump the job version up
	newJobVersion := job.Copy()
	newJobVersion.Version = job.Version + 1
	newJobVersion.JobModifyIndex = job.JobModifyIndex + 1

	// bump the version and add a new TG
	newJobWithNewTaskGroup := newJobVersion.Copy()
	newJobWithNewTaskGroup.Version = newJobVersion.Version + 1
	newJobWithNewTaskGroup.JobModifyIndex = newJobVersion.JobModifyIndex + 1
	tg := newJobVersion.TaskGroups[0].Copy()
	tg.Name = "other"
	tg.Update = &structs.UpdateStrategy{MaxParallel: 1}
	newJobWithNewTaskGroup.TaskGroups = append(newJobWithNewTaskGroup.TaskGroups, tg)

	// new job with no previous allocs and no canary update strategy
	jobWithNoUpdates := mock.SystemJob()
	jobWithNoUpdates.Name = "i-am-a-brand-new-job"
	jobWithNoUpdates.TaskGroups[0].Name = "i-am-a-brand-new-tg"
	jobWithNoUpdates.TaskGroups[0].Update = structs.DefaultUpdateStrategy

	// additional test to make sure there are no canaries being placed for v0
	// jobs
	freshJob := mock.SystemJob()
	freshJob.TaskGroups[0].Update = structs.DefaultUpdateStrategy
	freshNodes := []*structs.Node{}
	for range 2 {
		node := mock.Node()
		freshNodes = append(freshNodes, node)
	}

	testCases := []struct {
		name                                string
		job                                 *structs.Job
		nodes                               []*structs.Node
		existingDeployment                  *structs.Deployment
		expectedDesiredCanaries             map[string]int
		expectedDesiredTotal                map[string]int
		expectedDeploymentStatusDescription string
		expectedPlaceCount                  int
		expectedUpdateCount                 int
	}{
		{
			name:                                "new job version",
			job:                                 newJobVersion,
			nodes:                               nodes,
			existingDeployment:                  nil,
			expectedDesiredCanaries:             map[string]int{newJobVersion.TaskGroups[0].Name: 2},
			expectedDesiredTotal:                map[string]int{newJobVersion.TaskGroups[0].Name: 10},
			expectedDeploymentStatusDescription: structs.DeploymentStatusDescriptionRunningNeedsPromotion,
			expectedPlaceCount:                  0,
			expectedUpdateCount:                 2,
		},
		{
			name:               "new job version with a new TG (no existing allocs, no canaries)",
			job:                newJobWithNewTaskGroup,
			nodes:              nodes,
			existingDeployment: nil,
			expectedDesiredCanaries: map[string]int{
				newJobWithNewTaskGroup.TaskGroups[0].Name: 2,
				newJobWithNewTaskGroup.TaskGroups[1].Name: 0,
			},
			expectedDesiredTotal: map[string]int{
				newJobWithNewTaskGroup.TaskGroups[0].Name: 10,
				newJobWithNewTaskGroup.TaskGroups[1].Name: 10,
			},
			expectedDeploymentStatusDescription: structs.DeploymentStatusDescriptionRunningNeedsPromotion,
			expectedPlaceCount:                  10,
			expectedUpdateCount:                 2,
		},
		{
			name:               "brand new job with no update block",
			job:                jobWithNoUpdates,
			nodes:              nodes,
			existingDeployment: nil,
			expectedDesiredCanaries: map[string]int{
				jobWithNoUpdates.TaskGroups[0].Name: 0,
			},
			expectedDesiredTotal: map[string]int{
				jobWithNoUpdates.TaskGroups[0].Name: 10,
			},
			expectedDeploymentStatusDescription: structs.DeploymentStatusDescriptionRunning,
			expectedPlaceCount:                  10,
			expectedUpdateCount:                 0,
		},
		{
			name:               "fresh job with no updates, empty nodes",
			job:                freshJob,
			nodes:              freshNodes,
			existingDeployment: nil,
			expectedDesiredCanaries: map[string]int{
				freshJob.TaskGroups[0].Name: 0,
			},
			expectedDesiredTotal: map[string]int{
				freshJob.TaskGroups[0].Name: 2,
			},
			expectedDeploymentStatusDescription: structs.DeploymentStatusDescriptionRunning,
			expectedPlaceCount:                  2,
			expectedUpdateCount:                 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reconciler := NewNodeReconciler(tc.existingDeployment)
			r := reconciler.Compute(tc.job, tc.nodes, nil, nil, allocs, nil, false)
			must.NotNil(t, reconciler.DeploymentCurrent)
			must.Eq(t, tc.expectedPlaceCount, len(r.Place), must.Sprint("incorrect amount of r.Place"))
			must.Eq(t, tc.expectedUpdateCount, len(r.Update), must.Sprint("incorrect amount of r.Update"))
			must.Eq(t, tc.expectedDeploymentStatusDescription, reconciler.DeploymentCurrent.StatusDescription)
			for _, tg := range tc.job.TaskGroups {
				must.Eq(t, tc.expectedDesiredCanaries[tg.Name],
					reconciler.DeploymentCurrent.TaskGroups[tg.Name].DesiredCanaries,
					must.Sprintf("incorrect number of DesiredCanaries for %s", tg.Name))
				must.Eq(t, tc.expectedDesiredTotal[tg.Name],
					reconciler.DeploymentCurrent.TaskGroups[tg.Name].DesiredTotal,
					must.Sprintf("incorrect number of DesiredTotal for %s", tg.Name))
			}
		})
	}
}

// Tests the reconciler correctly promotes canaries
func TestNodeReconciler_CanaryPromotion(t *testing.T) {
	ci.Parallel(t)

	job := mock.SystemJob()
	job.TaskGroups[0].Update = &structs.UpdateStrategy{
		Canary:      20, // deploy to 20% of eligible nodes
		MaxParallel: 1,  // otherwise the update strategy will be considered nil
	}
	job.JobModifyIndex = 1

	// bump the job version up
	newJobVersion := job.Copy()
	newJobVersion.Version = job.Version + 1
	newJobVersion.JobModifyIndex = job.JobModifyIndex + 1

	// Create 5 nodes
	nodes := []*structs.Node{}
	for i := range 5 {
		node := mock.Node()
		node.ID = fmt.Sprintf("node_%d", i)
		node.Name = fmt.Sprintf("node_%d", i)
		nodes = append(nodes, node)
	}

	// Create v0 allocs on 2 of the nodes, and v1 (canary) allocs on 3 nodes
	allocs := []*structs.Allocation{}
	for _, n := range nodes[0:3] {
		a := mock.Alloc()
		a.Job = job
		a.Name = "my-job.web[0]"
		a.NodeID = n.ID
		a.NodeName = n.Name
		a.TaskGroup = job.TaskGroups[0].Name

		allocs = append(allocs, a)
	}
	for _, n := range nodes[3:] {
		a := mock.Alloc()
		a.Job = job
		a.Name = "my-job.web[0]"
		a.NodeID = n.ID
		a.NodeName = n.Name
		a.TaskGroup = job.TaskGroups[0].Name
		a.DeploymentStatus = &structs.AllocDeploymentStatus{Canary: true}
		a.Job.Version = newJobVersion.Version
		a.Job.JobModifyIndex = newJobVersion.JobModifyIndex

		allocs = append(allocs, a)
	}

	// promote canaries
	deployment := structs.NewDeployment(newJobVersion, 10, time.Now().Unix())
	deployment.TaskGroups[newJobVersion.TaskGroups[0].Name] = &structs.DeploymentState{
		Promoted:        true,
		HealthyAllocs:   5,
		DesiredTotal:    5,
		DesiredCanaries: 0,
	}

	// reconcile
	reconciler := NewNodeReconciler(deployment)
	reconciler.Compute(newJobVersion, nodes, nil, nil, allocs, nil, false)

	must.NotNil(t, reconciler.DeploymentCurrent)
	must.Eq(t, 5, reconciler.DeploymentCurrent.TaskGroups[newJobVersion.TaskGroups[0].Name].DesiredTotal)
	must.SliceContainsFunc(t, reconciler.DeploymentUpdates, structs.DeploymentStatusSuccessful,
		func(a *structs.DeploymentStatusUpdate, b string) bool { return a.Status == b },
	)
}
