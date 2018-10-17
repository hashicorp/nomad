package scheduler

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/stretchr/testify/require"
)

func TestStaticIterator_Reset(t *testing.T) {
	_, ctx := testContext(t)
	var nodes []*structs.Node
	for i := 0; i < 3; i++ {
		nodes = append(nodes, mock.Node())
	}
	static := NewStaticIterator(ctx, nodes)

	for i := 0; i < 6; i++ {
		static.Reset()
		for j := 0; j < i; j++ {
			static.Next()
		}
		static.Reset()

		out := collectFeasible(static)
		if len(out) != len(nodes) {
			t.Fatalf("out: %#v", out)
			t.Fatalf("missing nodes %d %#v", i, static)
		}

		ids := make(map[string]struct{})
		for _, o := range out {
			if _, ok := ids[o.ID]; ok {
				t.Fatalf("duplicate")
			}
			ids[o.ID] = struct{}{}
		}
	}
}

func TestStaticIterator_SetNodes(t *testing.T) {
	_, ctx := testContext(t)
	var nodes []*structs.Node
	for i := 0; i < 3; i++ {
		nodes = append(nodes, mock.Node())
	}
	static := NewStaticIterator(ctx, nodes)

	newNodes := []*structs.Node{mock.Node()}
	static.SetNodes(newNodes)

	out := collectFeasible(static)
	if !reflect.DeepEqual(out, newNodes) {
		t.Fatalf("bad: %#v", out)
	}
}

func TestRandomIterator(t *testing.T) {
	_, ctx := testContext(t)
	var nodes []*structs.Node
	for i := 0; i < 10; i++ {
		nodes = append(nodes, mock.Node())
	}

	nc := make([]*structs.Node, len(nodes))
	copy(nc, nodes)
	rand := NewRandomIterator(ctx, nc)

	out := collectFeasible(rand)
	if len(out) != len(nodes) {
		t.Fatalf("missing nodes")
	}
	if reflect.DeepEqual(out, nodes) {
		t.Fatalf("same order")
	}
}

func TestDriverChecker(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	nodes[0].Attributes["driver.foo"] = "1"
	nodes[1].Attributes["driver.foo"] = "0"
	nodes[2].Attributes["driver.foo"] = "true"
	nodes[3].Attributes["driver.foo"] = "False"

	drivers := map[string]struct{}{
		"exec": {},
		"foo":  {},
	}
	checker := NewDriverChecker(ctx, drivers)
	cases := []struct {
		Node   *structs.Node
		Result bool
	}{
		{
			Node:   nodes[0],
			Result: true,
		},
		{
			Node:   nodes[1],
			Result: false,
		},
		{
			Node:   nodes[2],
			Result: true,
		},
		{
			Node:   nodes[3],
			Result: false,
		},
	}

	for i, c := range cases {
		if act := checker.Feasible(c.Node); act != c.Result {
			t.Fatalf("case(%d) failed: got %v; want %v", i, act, c.Result)
		}
	}
}

func Test_HealthChecks(t *testing.T) {
	require := require.New(t)
	_, ctx := testContext(t)

	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	for _, e := range nodes {
		e.Drivers = make(map[string]*structs.DriverInfo)
	}
	nodes[0].Attributes["driver.foo"] = "1"
	nodes[0].Drivers["foo"] = &structs.DriverInfo{
		Detected:          true,
		Healthy:           true,
		HealthDescription: "running",
		UpdateTime:        time.Now(),
	}
	nodes[1].Attributes["driver.bar"] = "1"
	nodes[1].Drivers["bar"] = &structs.DriverInfo{
		Detected:          true,
		Healthy:           false,
		HealthDescription: "not running",
		UpdateTime:        time.Now(),
	}
	nodes[2].Attributes["driver.baz"] = "0"
	nodes[2].Drivers["baz"] = &structs.DriverInfo{
		Detected:          false,
		Healthy:           false,
		HealthDescription: "not running",
		UpdateTime:        time.Now(),
	}

	testDrivers := []string{"foo", "bar", "baz"}
	cases := []struct {
		Node   *structs.Node
		Result bool
	}{
		{
			Node:   nodes[0],
			Result: true,
		},
		{
			Node:   nodes[1],
			Result: false,
		},
		{
			Node:   nodes[2],
			Result: false,
		},
	}

	for i, c := range cases {
		drivers := map[string]struct{}{
			testDrivers[i]: {},
		}
		checker := NewDriverChecker(ctx, drivers)
		act := checker.Feasible(c.Node)
		require.Equal(act, c.Result)
	}
}

func TestConstraintChecker(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}

	nodes[0].Attributes["kernel.name"] = "freebsd"
	nodes[1].Datacenter = "dc2"
	nodes[2].NodeClass = "large"

	constraints := []*structs.Constraint{
		{
			Operand: "=",
			LTarget: "${node.datacenter}",
			RTarget: "dc1",
		},
		{
			Operand: "is",
			LTarget: "${attr.kernel.name}",
			RTarget: "linux",
		},
		{
			Operand: "is",
			LTarget: "${node.class}",
			RTarget: "large",
		},
	}
	checker := NewConstraintChecker(ctx, constraints)
	cases := []struct {
		Node   *structs.Node
		Result bool
	}{
		{
			Node:   nodes[0],
			Result: false,
		},
		{
			Node:   nodes[1],
			Result: false,
		},
		{
			Node:   nodes[2],
			Result: true,
		},
	}

	for i, c := range cases {
		if act := checker.Feasible(c.Node); act != c.Result {
			t.Fatalf("case(%d) failed: got %v; want %v", i, act, c.Result)
		}
	}
}

func TestResolveConstraintTarget(t *testing.T) {
	type tcase struct {
		target string
		node   *structs.Node
		val    interface{}
		result bool
	}
	node := mock.Node()
	cases := []tcase{
		{
			target: "${node.unique.id}",
			node:   node,
			val:    node.ID,
			result: true,
		},
		{
			target: "${node.datacenter}",
			node:   node,
			val:    node.Datacenter,
			result: true,
		},
		{
			target: "${node.unique.name}",
			node:   node,
			val:    node.Name,
			result: true,
		},
		{
			target: "${node.class}",
			node:   node,
			val:    node.NodeClass,
			result: true,
		},
		{
			target: "${node.foo}",
			node:   node,
			result: false,
		},
		{
			target: "${attr.kernel.name}",
			node:   node,
			val:    node.Attributes["kernel.name"],
			result: true,
		},
		{
			target: "${attr.rand}",
			node:   node,
			result: false,
		},
		{
			target: "${meta.pci-dss}",
			node:   node,
			val:    node.Meta["pci-dss"],
			result: true,
		},
		{
			target: "${meta.rand}",
			node:   node,
			result: false,
		},
	}

	for _, tc := range cases {
		res, ok := resolveTarget(tc.target, tc.node)
		if ok != tc.result {
			t.Fatalf("TC: %#v, Result: %v %v", tc, res, ok)
		}
		if ok && !reflect.DeepEqual(res, tc.val) {
			t.Fatalf("TC: %#v, Result: %v %v", tc, res, ok)
		}
	}
}

func TestCheckConstraint(t *testing.T) {
	type tcase struct {
		op         string
		lVal, rVal interface{}
		result     bool
	}
	cases := []tcase{
		{
			op:   "=",
			lVal: "foo", rVal: "foo",
			result: true,
		},
		{
			op:   "is",
			lVal: "foo", rVal: "foo",
			result: true,
		},
		{
			op:   "==",
			lVal: "foo", rVal: "foo",
			result: true,
		},
		{
			op:   "!=",
			lVal: "foo", rVal: "foo",
			result: false,
		},
		{
			op:   "!=",
			lVal: "foo", rVal: "bar",
			result: true,
		},
		{
			op:   "not",
			lVal: "foo", rVal: "bar",
			result: true,
		},
		{
			op:   structs.ConstraintVersion,
			lVal: "1.2.3", rVal: "~> 1.0",
			result: true,
		},
		{
			op:   structs.ConstraintRegex,
			lVal: "foobarbaz", rVal: "[\\w]+",
			result: true,
		},
		{
			op:   "<",
			lVal: "foo", rVal: "bar",
			result: false,
		},
		{
			op:   structs.ConstraintSetContains,
			lVal: "foo,bar,baz", rVal: "foo,  bar  ",
			result: true,
		},
		{
			op:   structs.ConstraintSetContains,
			lVal: "foo,bar,baz", rVal: "foo,bam",
			result: false,
		},
	}

	for _, tc := range cases {
		_, ctx := testContext(t)
		if res := checkConstraint(ctx, tc.op, tc.lVal, tc.rVal); res != tc.result {
			t.Fatalf("TC: %#v, Result: %v", tc, res)
		}
	}
}

func TestCheckLexicalOrder(t *testing.T) {
	type tcase struct {
		op         string
		lVal, rVal interface{}
		result     bool
	}
	cases := []tcase{
		{
			op:   "<",
			lVal: "bar", rVal: "foo",
			result: true,
		},
		{
			op:   "<=",
			lVal: "foo", rVal: "foo",
			result: true,
		},
		{
			op:   ">",
			lVal: "bar", rVal: "foo",
			result: false,
		},
		{
			op:   ">=",
			lVal: "bar", rVal: "bar",
			result: true,
		},
		{
			op:   ">",
			lVal: 1, rVal: "foo",
			result: false,
		},
	}
	for _, tc := range cases {
		if res := checkLexicalOrder(tc.op, tc.lVal, tc.rVal); res != tc.result {
			t.Fatalf("TC: %#v, Result: %v", tc, res)
		}
	}
}

func TestCheckVersionConstraint(t *testing.T) {
	type tcase struct {
		lVal, rVal interface{}
		result     bool
	}
	cases := []tcase{
		{
			lVal: "1.2.3", rVal: "~> 1.0",
			result: true,
		},
		{
			lVal: "1.2.3", rVal: ">= 1.0, < 1.4",
			result: true,
		},
		{
			lVal: "2.0.1", rVal: "~> 1.0",
			result: false,
		},
		{
			lVal: "1.4", rVal: ">= 1.0, < 1.4",
			result: false,
		},
		{
			lVal: 1, rVal: "~> 1.0",
			result: true,
		},
	}
	for _, tc := range cases {
		_, ctx := testContext(t)
		if res := checkVersionMatch(ctx, tc.lVal, tc.rVal); res != tc.result {
			t.Fatalf("TC: %#v, Result: %v", tc, res)
		}
	}
}

func TestCheckRegexpConstraint(t *testing.T) {
	type tcase struct {
		lVal, rVal interface{}
		result     bool
	}
	cases := []tcase{
		{
			lVal: "foobar", rVal: "bar",
			result: true,
		},
		{
			lVal: "foobar", rVal: "^foo",
			result: true,
		},
		{
			lVal: "foobar", rVal: "^bar",
			result: false,
		},
		{
			lVal: "zipzap", rVal: "foo",
			result: false,
		},
		{
			lVal: 1, rVal: "foo",
			result: false,
		},
	}
	for _, tc := range cases {
		_, ctx := testContext(t)
		if res := checkRegexpMatch(ctx, tc.lVal, tc.rVal); res != tc.result {
			t.Fatalf("TC: %#v, Result: %v", tc, res)
		}
	}
}

// This test puts allocations on the node to test if it detects infeasibility of
// nodes correctly and picks the only feasible one
func TestDistinctHostsIterator_JobDistinctHosts(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	static := NewStaticIterator(ctx, nodes)

	// Create a job with a distinct_hosts constraint and two task groups.
	tg1 := &structs.TaskGroup{Name: "bar"}
	tg2 := &structs.TaskGroup{Name: "baz"}

	job := &structs.Job{
		ID:          "foo",
		Namespace:   structs.DefaultNamespace,
		Constraints: []*structs.Constraint{{Operand: structs.ConstraintDistinctHosts}},
		TaskGroups:  []*structs.TaskGroup{tg1, tg2},
	}

	// Add allocs placing tg1 on node1 and tg2 on node2. This should make the
	// job unsatisfiable on all nodes but node3
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
		},

		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
		},
	}
	plan.NodeAllocation[nodes[1].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
		},

		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
		},
	}

	proposed := NewDistinctHostsIterator(ctx, static)
	proposed.SetTaskGroup(tg1)
	proposed.SetJob(job)

	out := collectFeasible(proposed)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}

	if out[0].ID != nodes[2].ID {
		t.Fatalf("wrong node picked")
	}
}

func TestDistinctHostsIterator_JobDistinctHosts_InfeasibleCount(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	static := NewStaticIterator(ctx, nodes)

	// Create a job with a distinct_hosts constraint and three task groups.
	tg1 := &structs.TaskGroup{Name: "bar"}
	tg2 := &structs.TaskGroup{Name: "baz"}
	tg3 := &structs.TaskGroup{Name: "bam"}

	job := &structs.Job{
		ID:          "foo",
		Namespace:   structs.DefaultNamespace,
		Constraints: []*structs.Constraint{{Operand: structs.ConstraintDistinctHosts}},
		TaskGroups:  []*structs.TaskGroup{tg1, tg2, tg3},
	}

	// Add allocs placing tg1 on node1 and tg2 on node2. This should make the
	// job unsatisfiable for tg3
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			ID:        uuid.Generate(),
		},
	}
	plan.NodeAllocation[nodes[1].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			ID:        uuid.Generate(),
		},
	}

	proposed := NewDistinctHostsIterator(ctx, static)
	proposed.SetTaskGroup(tg3)
	proposed.SetJob(job)

	// It should not be able to place 3 tasks with only two nodes.
	out := collectFeasible(proposed)
	if len(out) != 0 {
		t.Fatalf("Bad: %#v", out)
	}
}

func TestDistinctHostsIterator_TaskGroupDistinctHosts(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	static := NewStaticIterator(ctx, nodes)

	// Create a task group with a distinct_hosts constraint.
	tg1 := &structs.TaskGroup{
		Name: "example",
		Constraints: []*structs.Constraint{
			{Operand: structs.ConstraintDistinctHosts},
		},
	}
	tg2 := &structs.TaskGroup{Name: "baz"}

	// Add a planned alloc to node1.
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     "foo",
		},
	}

	// Add a planned alloc to node2 with the same task group name but a
	// different job.
	plan.NodeAllocation[nodes[1].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     "bar",
		},
	}

	proposed := NewDistinctHostsIterator(ctx, static)
	proposed.SetTaskGroup(tg1)
	proposed.SetJob(&structs.Job{
		ID:        "foo",
		Namespace: structs.DefaultNamespace,
	})

	out := collectFeasible(proposed)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}

	// Expect it to skip the first node as there is a previous alloc on it for
	// the same task group.
	if out[0] != nodes[1] {
		t.Fatalf("Bad: %v", out)
	}

	// Since the other task group doesn't have the constraint, both nodes should
	// be feasible.
	proposed.Reset()
	proposed.SetTaskGroup(tg2)
	out = collectFeasible(proposed)
	if len(out) != 2 {
		t.Fatalf("Bad: %#v", out)
	}
}

// This test puts creates allocations across task groups that use a property
// value to detect if the constraint at the job level properly considers all
// task groups.
func TestDistinctPropertyIterator_JobDistinctProperty(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}

	for i, n := range nodes {
		n.Meta["rack"] = fmt.Sprintf("%d", i)

		// Add to state store
		if err := state.UpsertNode(uint64(100+i), n); err != nil {
			t.Fatalf("failed to upsert node: %v", err)
		}
	}

	static := NewStaticIterator(ctx, nodes)

	// Create a job with a distinct_property constraint and a task groups.
	tg1 := &structs.TaskGroup{Name: "bar"}
	tg2 := &structs.TaskGroup{Name: "baz"}

	job := &structs.Job{
		ID:        "foo",
		Namespace: structs.DefaultNamespace,
		Constraints: []*structs.Constraint{
			{
				Operand: structs.ConstraintDistinctProperty,
				LTarget: "${meta.rack}",
			},
		},
		TaskGroups: []*structs.TaskGroup{tg1, tg2},
	}

	// Add allocs placing tg1 on node1 and 2 and tg2 on node3 and 4. This should make the
	// job unsatisfiable on all nodes but node5. Also mix the allocations
	// existing in the plan and the state store.
	plan := ctx.Plan()
	alloc1ID := uuid.Generate()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        alloc1ID,
			NodeID:    nodes[0].ID,
		},

		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[0].ID,
		},
	}
	plan.NodeAllocation[nodes[2].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[2].ID,
		},

		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[2].ID,
		},
	}

	// Put an allocation on Node 5 but make it stopped in the plan
	stoppingAllocID := uuid.Generate()
	plan.NodeUpdate[nodes[4].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        stoppingAllocID,
			NodeID:    nodes[4].ID,
		},
	}

	upserting := []*structs.Allocation{
		// Have one of the allocations exist in both the plan and the state
		// store. This resembles an allocation update
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        alloc1ID,
			EvalID:    uuid.Generate(),
			NodeID:    nodes[0].ID,
		},

		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[1].ID,
		},

		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[1].ID,
		},
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[3].ID,
		},

		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[3].ID,
		},
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        stoppingAllocID,
			EvalID:    uuid.Generate(),
			NodeID:    nodes[4].ID,
		},
	}
	if err := state.UpsertAllocs(1000, upserting); err != nil {
		t.Fatalf("failed to UpsertAllocs: %v", err)
	}

	proposed := NewDistinctPropertyIterator(ctx, static)
	proposed.SetJob(job)
	proposed.SetTaskGroup(tg2)
	proposed.Reset()

	out := collectFeasible(proposed)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0].ID != nodes[4].ID {
		t.Fatalf("wrong node picked")
	}
}

// This test creates allocations across task groups that use a property value to
// detect if the constraint at the job level properly considers all task groups
// when the constraint allows a count greater than one
func TestDistinctPropertyIterator_JobDistinctProperty_Count(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}

	for i, n := range nodes {
		n.Meta["rack"] = fmt.Sprintf("%d", i)

		// Add to state store
		if err := state.UpsertNode(uint64(100+i), n); err != nil {
			t.Fatalf("failed to upsert node: %v", err)
		}
	}

	static := NewStaticIterator(ctx, nodes)

	// Create a job with a distinct_property constraint and a task groups.
	tg1 := &structs.TaskGroup{Name: "bar"}
	tg2 := &structs.TaskGroup{Name: "baz"}

	job := &structs.Job{
		ID:        "foo",
		Namespace: structs.DefaultNamespace,
		Constraints: []*structs.Constraint{
			{
				Operand: structs.ConstraintDistinctProperty,
				LTarget: "${meta.rack}",
				RTarget: "2",
			},
		},
		TaskGroups: []*structs.TaskGroup{tg1, tg2},
	}

	// Add allocs placing two allocations on both node 1 and 2 and only one on
	// node 3. This should make the job unsatisfiable on all nodes but node5.
	// Also mix the allocations existing in the plan and the state store.
	plan := ctx.Plan()
	alloc1ID := uuid.Generate()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        alloc1ID,
			NodeID:    nodes[0].ID,
		},

		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        alloc1ID,
			NodeID:    nodes[0].ID,
		},

		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[0].ID,
		},
	}
	plan.NodeAllocation[nodes[1].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[1].ID,
		},

		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[1].ID,
		},

		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[1].ID,
		},
	}
	plan.NodeAllocation[nodes[2].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[2].ID,
		},

		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[2].ID,
		},
	}

	// Put an allocation on Node 3 but make it stopped in the plan
	stoppingAllocID := uuid.Generate()
	plan.NodeUpdate[nodes[2].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        stoppingAllocID,
			NodeID:    nodes[2].ID,
		},
	}

	upserting := []*structs.Allocation{
		// Have one of the allocations exist in both the plan and the state
		// store. This resembles an allocation update
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        alloc1ID,
			EvalID:    uuid.Generate(),
			NodeID:    nodes[0].ID,
		},

		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[1].ID,
		},

		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[0].ID,
		},

		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[1].ID,
		},
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[1].ID,
		},
	}
	if err := state.UpsertAllocs(1000, upserting); err != nil {
		t.Fatalf("failed to UpsertAllocs: %v", err)
	}

	proposed := NewDistinctPropertyIterator(ctx, static)
	proposed.SetJob(job)
	proposed.SetTaskGroup(tg2)
	proposed.Reset()

	out := collectFeasible(proposed)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0].ID != nodes[2].ID {
		t.Fatalf("wrong node picked")
	}
}

// This test checks that if a node has an allocation on it that gets stopped,
// there is a plan to re-use that for a new allocation, that the next select
// won't select that node.
func TestDistinctPropertyIterator_JobDistinctProperty_RemoveAndReplace(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
	}

	nodes[0].Meta["rack"] = "1"

	// Add to state store
	if err := state.UpsertNode(uint64(100), nodes[0]); err != nil {
		t.Fatalf("failed to upsert node: %v", err)
	}

	static := NewStaticIterator(ctx, nodes)

	// Create a job with a distinct_property constraint and a task groups.
	tg1 := &structs.TaskGroup{Name: "bar"}
	job := &structs.Job{
		Namespace: structs.DefaultNamespace,
		ID:        "foo",
		Constraints: []*structs.Constraint{
			{
				Operand: structs.ConstraintDistinctProperty,
				LTarget: "${meta.rack}",
			},
		},
		TaskGroups: []*structs.TaskGroup{tg1},
	}

	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[0].ID,
		},
	}

	stoppingAllocID := uuid.Generate()
	plan.NodeUpdate[nodes[0].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        stoppingAllocID,
			NodeID:    nodes[0].ID,
		},
	}

	upserting := []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        stoppingAllocID,
			EvalID:    uuid.Generate(),
			NodeID:    nodes[0].ID,
		},
	}
	if err := state.UpsertAllocs(1000, upserting); err != nil {
		t.Fatalf("failed to UpsertAllocs: %v", err)
	}

	proposed := NewDistinctPropertyIterator(ctx, static)
	proposed.SetJob(job)
	proposed.SetTaskGroup(tg1)
	proposed.Reset()

	out := collectFeasible(proposed)
	if len(out) != 0 {
		t.Fatalf("Bad: %#v", out)
	}
}

// This test creates previous allocations selecting certain property values to
// test if it detects infeasibility of property values correctly and picks the
// only feasible one
func TestDistinctPropertyIterator_JobDistinctProperty_Infeasible(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}

	for i, n := range nodes {
		n.Meta["rack"] = fmt.Sprintf("%d", i)

		// Add to state store
		if err := state.UpsertNode(uint64(100+i), n); err != nil {
			t.Fatalf("failed to upsert node: %v", err)
		}
	}

	static := NewStaticIterator(ctx, nodes)

	// Create a job with a distinct_property constraint and a task groups.
	tg1 := &structs.TaskGroup{Name: "bar"}
	tg2 := &structs.TaskGroup{Name: "baz"}
	tg3 := &structs.TaskGroup{Name: "bam"}

	job := &structs.Job{
		Namespace: structs.DefaultNamespace,
		ID:        "foo",
		Constraints: []*structs.Constraint{
			{
				Operand: structs.ConstraintDistinctProperty,
				LTarget: "${meta.rack}",
			},
		},
		TaskGroups: []*structs.TaskGroup{tg1, tg2, tg3},
	}

	// Add allocs placing tg1 on node1 and tg2 on node2. This should make the
	// job unsatisfiable for tg3.
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[0].ID,
		},
	}
	upserting := []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[1].ID,
		},
	}
	if err := state.UpsertAllocs(1000, upserting); err != nil {
		t.Fatalf("failed to UpsertAllocs: %v", err)
	}

	proposed := NewDistinctPropertyIterator(ctx, static)
	proposed.SetJob(job)
	proposed.SetTaskGroup(tg3)
	proposed.Reset()

	out := collectFeasible(proposed)
	if len(out) != 0 {
		t.Fatalf("Bad: %#v", out)
	}
}

// This test creates previous allocations selecting certain property values to
// test if it detects infeasibility of property values correctly and picks the
// only feasible one
func TestDistinctPropertyIterator_JobDistinctProperty_Infeasible_Count(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}

	for i, n := range nodes {
		n.Meta["rack"] = fmt.Sprintf("%d", i)

		// Add to state store
		if err := state.UpsertNode(uint64(100+i), n); err != nil {
			t.Fatalf("failed to upsert node: %v", err)
		}
	}

	static := NewStaticIterator(ctx, nodes)

	// Create a job with a distinct_property constraint and a task groups.
	tg1 := &structs.TaskGroup{Name: "bar"}
	tg2 := &structs.TaskGroup{Name: "baz"}
	tg3 := &structs.TaskGroup{Name: "bam"}

	job := &structs.Job{
		Namespace: structs.DefaultNamespace,
		ID:        "foo",
		Constraints: []*structs.Constraint{
			{
				Operand: structs.ConstraintDistinctProperty,
				LTarget: "${meta.rack}",
				RTarget: "2",
			},
		},
		TaskGroups: []*structs.TaskGroup{tg1, tg2, tg3},
	}

	// Add allocs placing two tg1's on node1 and two tg2's on node2. This should
	// make the job unsatisfiable for tg3.
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[0].ID,
		},
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[0].ID,
		},
	}
	upserting := []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[1].ID,
		},
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[1].ID,
		},
	}
	if err := state.UpsertAllocs(1000, upserting); err != nil {
		t.Fatalf("failed to UpsertAllocs: %v", err)
	}

	proposed := NewDistinctPropertyIterator(ctx, static)
	proposed.SetJob(job)
	proposed.SetTaskGroup(tg3)
	proposed.Reset()

	out := collectFeasible(proposed)
	if len(out) != 0 {
		t.Fatalf("Bad: %#v", out)
	}
}

// This test creates previous allocations selecting certain property values to
// test if it detects infeasibility of property values correctly and picks the
// only feasible one when the constraint is at the task group.
func TestDistinctPropertyIterator_TaskGroupDistinctProperty(t *testing.T) {
	state, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}

	for i, n := range nodes {
		n.Meta["rack"] = fmt.Sprintf("%d", i)

		// Add to state store
		if err := state.UpsertNode(uint64(100+i), n); err != nil {
			t.Fatalf("failed to upsert node: %v", err)
		}
	}

	static := NewStaticIterator(ctx, nodes)

	// Create a job with a task group with the distinct_property constraint
	tg1 := &structs.TaskGroup{
		Name: "example",
		Constraints: []*structs.Constraint{
			{
				Operand: structs.ConstraintDistinctProperty,
				LTarget: "${meta.rack}",
			},
		},
	}
	tg2 := &structs.TaskGroup{Name: "baz"}

	job := &structs.Job{
		Namespace:  structs.DefaultNamespace,
		ID:         "foo",
		TaskGroups: []*structs.TaskGroup{tg1, tg2},
	}

	// Add allocs placing tg1 on node1 and 2. This should make the
	// job unsatisfiable on all nodes but node3. Also mix the allocations
	// existing in the plan and the state store.
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			NodeID:    nodes[0].ID,
		},
	}

	// Put an allocation on Node 3 but make it stopped in the plan
	stoppingAllocID := uuid.Generate()
	plan.NodeUpdate[nodes[2].ID] = []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        stoppingAllocID,
			NodeID:    nodes[2].ID,
		},
	}

	upserting := []*structs.Allocation{
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[1].ID,
		},

		// Should be ignored as it is a different job.
		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     "ignore 2",
			Job:       job,
			ID:        uuid.Generate(),
			EvalID:    uuid.Generate(),
			NodeID:    nodes[2].ID,
		},

		{
			Namespace: structs.DefaultNamespace,
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			Job:       job,
			ID:        stoppingAllocID,
			EvalID:    uuid.Generate(),
			NodeID:    nodes[2].ID,
		},
	}
	if err := state.UpsertAllocs(1000, upserting); err != nil {
		t.Fatalf("failed to UpsertAllocs: %v", err)
	}

	proposed := NewDistinctPropertyIterator(ctx, static)
	proposed.SetJob(job)
	proposed.SetTaskGroup(tg1)
	proposed.Reset()

	out := collectFeasible(proposed)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}
	if out[0].ID != nodes[2].ID {
		t.Fatalf("wrong node picked")
	}

	// Since the other task group doesn't have the constraint, both nodes should
	// be feasible.
	proposed.SetTaskGroup(tg2)
	proposed.Reset()

	out = collectFeasible(proposed)
	if len(out) != 3 {
		t.Fatalf("Bad: %#v", out)
	}
}

func collectFeasible(iter FeasibleIterator) (out []*structs.Node) {
	for {
		next := iter.Next()
		if next == nil {
			break
		}
		out = append(out, next)
	}
	return
}

// mockFeasibilityChecker is a FeasibilityChecker that returns predetermined
// feasibility values.
type mockFeasibilityChecker struct {
	retVals []bool
	i       int
}

func newMockFeasibilityChecker(values ...bool) *mockFeasibilityChecker {
	return &mockFeasibilityChecker{retVals: values}
}

func (c *mockFeasibilityChecker) Feasible(*structs.Node) bool {
	if c.i >= len(c.retVals) {
		c.i++
		return false
	}

	f := c.retVals[c.i]
	c.i++
	return f
}

// calls returns how many times the checker was called.
func (c *mockFeasibilityChecker) calls() int { return c.i }

func TestFeasibilityWrapper_JobIneligible(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{mock.Node()}
	static := NewStaticIterator(ctx, nodes)
	mocked := newMockFeasibilityChecker(false)
	wrapper := NewFeasibilityWrapper(ctx, static, []FeasibilityChecker{mocked}, nil)

	// Set the job to ineligible
	ctx.Eligibility().SetJobEligibility(false, nodes[0].ComputedClass)

	// Run the wrapper.
	out := collectFeasible(wrapper)

	if out != nil || mocked.calls() != 0 {
		t.Fatalf("bad: %#v %d", out, mocked.calls())
	}
}

func TestFeasibilityWrapper_JobEscapes(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{mock.Node()}
	static := NewStaticIterator(ctx, nodes)
	mocked := newMockFeasibilityChecker(false)
	wrapper := NewFeasibilityWrapper(ctx, static, []FeasibilityChecker{mocked}, nil)

	// Set the job to escaped
	cc := nodes[0].ComputedClass
	ctx.Eligibility().job[cc] = EvalComputedClassEscaped

	// Run the wrapper.
	out := collectFeasible(wrapper)

	if out != nil || mocked.calls() != 1 {
		t.Fatalf("bad: %#v", out)
	}

	// Ensure that the job status didn't change from escaped even though the
	// option failed.
	if status := ctx.Eligibility().JobStatus(cc); status != EvalComputedClassEscaped {
		t.Fatalf("job status is %v; want %v", status, EvalComputedClassEscaped)
	}
}

func TestFeasibilityWrapper_JobAndTg_Eligible(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{mock.Node()}
	static := NewStaticIterator(ctx, nodes)
	jobMock := newMockFeasibilityChecker(true)
	tgMock := newMockFeasibilityChecker(false)
	wrapper := NewFeasibilityWrapper(ctx, static, []FeasibilityChecker{jobMock}, []FeasibilityChecker{tgMock})

	// Set the job to escaped
	cc := nodes[0].ComputedClass
	ctx.Eligibility().job[cc] = EvalComputedClassEligible
	ctx.Eligibility().SetTaskGroupEligibility(true, "foo", cc)
	wrapper.SetTaskGroup("foo")

	// Run the wrapper.
	out := collectFeasible(wrapper)

	if out == nil || tgMock.calls() != 0 {
		t.Fatalf("bad: %#v %v", out, tgMock.calls())
	}
}

func TestFeasibilityWrapper_JobEligible_TgIneligible(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{mock.Node()}
	static := NewStaticIterator(ctx, nodes)
	jobMock := newMockFeasibilityChecker(true)
	tgMock := newMockFeasibilityChecker(false)
	wrapper := NewFeasibilityWrapper(ctx, static, []FeasibilityChecker{jobMock}, []FeasibilityChecker{tgMock})

	// Set the job to escaped
	cc := nodes[0].ComputedClass
	ctx.Eligibility().job[cc] = EvalComputedClassEligible
	ctx.Eligibility().SetTaskGroupEligibility(false, "foo", cc)
	wrapper.SetTaskGroup("foo")

	// Run the wrapper.
	out := collectFeasible(wrapper)

	if out != nil || tgMock.calls() != 0 {
		t.Fatalf("bad: %#v %v", out, tgMock.calls())
	}
}

func TestFeasibilityWrapper_JobEligible_TgEscaped(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{mock.Node()}
	static := NewStaticIterator(ctx, nodes)
	jobMock := newMockFeasibilityChecker(true)
	tgMock := newMockFeasibilityChecker(true)
	wrapper := NewFeasibilityWrapper(ctx, static, []FeasibilityChecker{jobMock}, []FeasibilityChecker{tgMock})

	// Set the job to escaped
	cc := nodes[0].ComputedClass
	ctx.Eligibility().job[cc] = EvalComputedClassEligible
	ctx.Eligibility().taskGroups["foo"] =
		map[string]ComputedClassFeasibility{cc: EvalComputedClassEscaped}
	wrapper.SetTaskGroup("foo")

	// Run the wrapper.
	out := collectFeasible(wrapper)

	if out == nil || tgMock.calls() != 1 {
		t.Fatalf("bad: %#v %v", out, tgMock.calls())
	}

	if e, ok := ctx.Eligibility().taskGroups["foo"][cc]; !ok || e != EvalComputedClassEscaped {
		t.Fatalf("bad: %v %v", e, ok)
	}
}

func TestSetContainsAny(t *testing.T) {
	require.True(t, checkSetContainsAny("a", "a"))
	require.True(t, checkSetContainsAny("a,b", "a"))
	require.True(t, checkSetContainsAny("  a,b  ", "a "))
	require.True(t, checkSetContainsAny("a", "a"))
	require.False(t, checkSetContainsAny("b", "a"))
}

func TestDeviceChecker(t *testing.T) {
	getTg := func(devices ...*structs.RequestedDevice) *structs.TaskGroup {
		return &structs.TaskGroup{
			Name: "example",
			Tasks: []*structs.Task{
				{
					Resources: &structs.Resources{
						Devices: devices,
					},
				},
			},
		}
	}

	// Just type
	gpuTypeReq := &structs.RequestedDevice{
		Name:  "gpu",
		Count: 1,
	}
	fpgaTypeReq := &structs.RequestedDevice{
		Name:  "fpga",
		Count: 1,
	}

	// vendor/type
	gpuVendorTypeReq := &structs.RequestedDevice{
		Name:  "nvidia/gpu",
		Count: 1,
	}
	fpgaVendorTypeReq := &structs.RequestedDevice{
		Name:  "nvidia/fpga",
		Count: 1,
	}

	// vendor/type/model
	gpuFullReq := &structs.RequestedDevice{
		Name:  "nvidia/gpu/1080ti",
		Count: 1,
	}
	fpgaFullReq := &structs.RequestedDevice{
		Name:  "nvidia/fpga/F100",
		Count: 1,
	}

	// Just type but high count
	gpuTypeHighCountReq := &structs.RequestedDevice{
		Name:  "gpu",
		Count: 3,
	}

	getNode := func(devices ...*structs.NodeDeviceResource) *structs.Node {
		n := mock.Node()
		n.NodeResources.Devices = devices
		return n
	}

	nvidia := &structs.NodeDeviceResource{
		Vendor: "nvidia",
		Type:   "gpu",
		Name:   "1080ti",
		Attributes: map[string]*psstructs.Attribute{
			"memory":        psstructs.NewIntAttribute(4, psstructs.UnitGiB),
			"pci_bandwidth": psstructs.NewIntAttribute(995, psstructs.UnitMiBPerS),
			"cores_clock":   psstructs.NewIntAttribute(800, psstructs.UnitMHz),
		},
		Instances: []*structs.NodeDevice{
			{
				ID:      uuid.Generate(),
				Healthy: true,
			},
			{
				ID:      uuid.Generate(),
				Healthy: true,
			},
		},
	}

	nvidiaUnhealthy := &structs.NodeDeviceResource{
		Vendor: "nvidia",
		Type:   "gpu",
		Name:   "1080ti",
		Instances: []*structs.NodeDevice{
			{
				ID:      uuid.Generate(),
				Healthy: false,
			},
			{
				ID:      uuid.Generate(),
				Healthy: false,
			},
		},
	}

	cases := []struct {
		Name             string
		Result           bool
		NodeDevices      []*structs.NodeDeviceResource
		RequestedDevices []*structs.RequestedDevice
	}{
		{
			Name:             "no devices on node",
			Result:           false,
			NodeDevices:      nil,
			RequestedDevices: []*structs.RequestedDevice{gpuTypeReq},
		},
		{
			Name:             "no requested devices on empty node",
			Result:           true,
			NodeDevices:      nil,
			RequestedDevices: nil,
		},
		{
			Name:             "gpu devices by type",
			Result:           true,
			NodeDevices:      []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{gpuTypeReq},
		},
		{
			Name:             "wrong devices by type",
			Result:           false,
			NodeDevices:      []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{fpgaTypeReq},
		},
		{
			Name:             "devices by type unhealthy node",
			Result:           false,
			NodeDevices:      []*structs.NodeDeviceResource{nvidiaUnhealthy},
			RequestedDevices: []*structs.RequestedDevice{gpuTypeReq},
		},
		{
			Name:             "gpu devices by vendor/type",
			Result:           true,
			NodeDevices:      []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{gpuVendorTypeReq},
		},
		{
			Name:             "wrong devices by vendor/type",
			Result:           false,
			NodeDevices:      []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{fpgaVendorTypeReq},
		},
		{
			Name:             "gpu devices by vendor/type/model",
			Result:           true,
			NodeDevices:      []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{gpuFullReq},
		},
		{
			Name:             "wrong devices by vendor/type/model",
			Result:           false,
			NodeDevices:      []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{fpgaFullReq},
		},
		{
			Name:             "too many requested",
			Result:           false,
			NodeDevices:      []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{gpuTypeHighCountReq},
		},
		{
			Name:        "meets constraints requirement",
			Result:      true,
			NodeDevices: []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{
				{
					Name:  "nvidia/gpu",
					Count: 1,
					Constraints: []*structs.Constraint{
						{
							Operand: "=",
							LTarget: "${driver.model}",
							RTarget: "1080ti",
						},
						{
							Operand: ">",
							LTarget: "${driver.attr.memory}",
							RTarget: "1320.5 MB",
						},
						{
							Operand: "<=",
							LTarget: "${driver.attr.pci_bandwidth}",
							RTarget: ".98   GiB/s",
						},
						{
							Operand: "=",
							LTarget: "${driver.attr.cores_clock}",
							RTarget: "800MHz",
						},
					},
				},
			},
		},
		{
			Name:        "meets constraints requirement multiple count",
			Result:      true,
			NodeDevices: []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{
				{
					Name:  "nvidia/gpu",
					Count: 2,
					Constraints: []*structs.Constraint{
						{
							Operand: "=",
							LTarget: "${driver.model}",
							RTarget: "1080ti",
						},
						{
							Operand: ">",
							LTarget: "${driver.attr.memory}",
							RTarget: "1320.5 MB",
						},
						{
							Operand: "<=",
							LTarget: "${driver.attr.pci_bandwidth}",
							RTarget: ".98   GiB/s",
						},
						{
							Operand: "=",
							LTarget: "${driver.attr.cores_clock}",
							RTarget: "800MHz",
						},
					},
				},
			},
		},
		{
			Name:        "meets constraints requirement over count",
			Result:      false,
			NodeDevices: []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{
				{
					Name:  "nvidia/gpu",
					Count: 5,
					Constraints: []*structs.Constraint{
						{
							Operand: "=",
							LTarget: "${driver.model}",
							RTarget: "1080ti",
						},
						{
							Operand: ">",
							LTarget: "${driver.attr.memory}",
							RTarget: "1320.5 MB",
						},
						{
							Operand: "<=",
							LTarget: "${driver.attr.pci_bandwidth}",
							RTarget: ".98   GiB/s",
						},
						{
							Operand: "=",
							LTarget: "${driver.attr.cores_clock}",
							RTarget: "800MHz",
						},
					},
				},
			},
		},
		{
			Name:        "does not meet first constraint",
			Result:      false,
			NodeDevices: []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{
				{
					Name:  "nvidia/gpu",
					Count: 1,
					Constraints: []*structs.Constraint{
						{
							Operand: "=",
							LTarget: "${driver.model}",
							RTarget: "2080ti",
						},
						{
							Operand: ">",
							LTarget: "${driver.attr.memory}",
							RTarget: "1320.5 MB",
						},
						{
							Operand: "<=",
							LTarget: "${driver.attr.pci_bandwidth}",
							RTarget: ".98   GiB/s",
						},
						{
							Operand: "=",
							LTarget: "${driver.attr.cores_clock}",
							RTarget: "800MHz",
						},
					},
				},
			},
		},
		{
			Name:        "does not meet second constraint",
			Result:      false,
			NodeDevices: []*structs.NodeDeviceResource{nvidia},
			RequestedDevices: []*structs.RequestedDevice{
				{
					Name:  "nvidia/gpu",
					Count: 1,
					Constraints: []*structs.Constraint{
						{
							Operand: "=",
							LTarget: "${driver.model}",
							RTarget: "1080ti",
						},
						{
							Operand: "<",
							LTarget: "${driver.attr.memory}",
							RTarget: "1320.5 MB",
						},
						{
							Operand: "<=",
							LTarget: "${driver.attr.pci_bandwidth}",
							RTarget: ".98   GiB/s",
						},
						{
							Operand: "=",
							LTarget: "${driver.attr.cores_clock}",
							RTarget: "800MHz",
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			_, ctx := testContext(t)
			checker := NewDeviceChecker(ctx)
			checker.SetTaskGroup(getTg(c.RequestedDevices...))
			if act := checker.Feasible(getNode(c.NodeDevices...)); act != c.Result {
				t.Fatalf("got %v; want %v", act, c.Result)
			}
		})
	}
}

func TestCheckAttributeConstraint(t *testing.T) {
	type tcase struct {
		op         string
		lVal, rVal *psstructs.Attribute
		result     bool
	}
	cases := []tcase{
		{
			op:     "=",
			lVal:   psstructs.NewStringAttribute("foo"),
			rVal:   psstructs.NewStringAttribute("foo"),
			result: true,
		},
		{
			op:     "is",
			lVal:   psstructs.NewStringAttribute("foo"),
			rVal:   psstructs.NewStringAttribute("foo"),
			result: true,
		},
		{
			op:     "==",
			lVal:   psstructs.NewStringAttribute("foo"),
			rVal:   psstructs.NewStringAttribute("foo"),
			result: true,
		},
		{
			op:     "!=",
			lVal:   psstructs.NewStringAttribute("foo"),
			rVal:   psstructs.NewStringAttribute("foo"),
			result: false,
		},
		{
			op:     "!=",
			lVal:   psstructs.NewStringAttribute("foo"),
			rVal:   psstructs.NewStringAttribute("bar"),
			result: true,
		},
		{
			op:     "not",
			lVal:   psstructs.NewStringAttribute("foo"),
			rVal:   psstructs.NewStringAttribute("bar"),
			result: true,
		},
		{
			op:     structs.ConstraintVersion,
			lVal:   psstructs.NewStringAttribute("1.2.3"),
			rVal:   psstructs.NewStringAttribute("~> 1.0"),
			result: true,
		},
		{
			op:     structs.ConstraintRegex,
			lVal:   psstructs.NewStringAttribute("foobarbaz"),
			rVal:   psstructs.NewStringAttribute("[\\w]+"),
			result: true,
		},
		{
			op:     "<",
			lVal:   psstructs.NewStringAttribute("foo"),
			rVal:   psstructs.NewStringAttribute("bar"),
			result: false,
		},
		{
			op:     structs.ConstraintSetContains,
			lVal:   psstructs.NewStringAttribute("foo,bar,baz"),
			rVal:   psstructs.NewStringAttribute("foo,  bar  "),
			result: true,
		},
		{
			op:     structs.ConstraintSetContainsAll,
			lVal:   psstructs.NewStringAttribute("foo,bar,baz"),
			rVal:   psstructs.NewStringAttribute("foo,  bar  "),
			result: true,
		},
		{
			op:     structs.ConstraintSetContains,
			lVal:   psstructs.NewStringAttribute("foo,bar,baz"),
			rVal:   psstructs.NewStringAttribute("foo,bam"),
			result: false,
		},
		{
			op:     structs.ConstraintSetContainsAny,
			lVal:   psstructs.NewStringAttribute("foo,bar,baz"),
			rVal:   psstructs.NewStringAttribute("foo,bam"),
			result: true,
		},
	}

	for _, tc := range cases {
		_, ctx := testContext(t)
		if res := checkAttributeConstraint(ctx, tc.op, tc.lVal, tc.rVal); res != tc.result {
			t.Fatalf("TC: %#v, Result: %v", tc, res)
		}
	}
}
