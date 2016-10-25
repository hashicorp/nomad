package scheduler

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
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
		"exec": struct{}{},
		"foo":  struct{}{},
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
		&structs.Constraint{
			Operand: "=",
			LTarget: "${node.datacenter}",
			RTarget: "dc1",
		},
		&structs.Constraint{
			Operand: "is",
			LTarget: "${attr.kernel.name}",
			RTarget: "linux",
		},
		&structs.Constraint{
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
		res, ok := resolveConstraintTarget(tc.target, tc.node)
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
		if res := checkVersionConstraint(ctx, tc.lVal, tc.rVal); res != tc.result {
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
		if res := checkRegexpConstraint(ctx, tc.lVal, tc.rVal); res != tc.result {
			t.Fatalf("TC: %#v, Result: %v", tc, res)
		}
	}
}

func TestProposedAllocConstraint_JobDistinctHosts(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
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
		Constraints: []*structs.Constraint{{Operand: structs.ConstraintDistinctHosts}},
		TaskGroups:  []*structs.TaskGroup{tg1, tg2},
	}

	propsed := NewProposedAllocConstraintIterator(ctx, static)
	propsed.SetTaskGroup(tg1)
	propsed.SetJob(job)

	out := collectFeasible(propsed)
	if len(out) != 4 {
		t.Fatalf("Bad: %#v", out)
	}

	selected := make(map[string]struct{}, 4)
	for _, option := range out {
		if _, ok := selected[option.ID]; ok {
			t.Fatalf("selected node %v for more than one alloc", option)
		}
		selected[option.ID] = struct{}{}
	}
}

func TestProposedAllocConstraint_JobDistinctHosts_Infeasible(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	static := NewStaticIterator(ctx, nodes)

	// Create a job with a distinct_hosts constraint and two task groups.
	tg1 := &structs.TaskGroup{Name: "bar"}
	tg2 := &structs.TaskGroup{Name: "baz"}

	job := &structs.Job{
		ID:          "foo",
		Constraints: []*structs.Constraint{{Operand: structs.ConstraintDistinctHosts}},
		TaskGroups:  []*structs.TaskGroup{tg1, tg2},
	}

	// Add allocs placing tg1 on node1 and tg2 on node2. This should make the
	// job unsatisfiable.
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		&structs.Allocation{
			TaskGroup: tg1.Name,
			JobID:     job.ID,
			ID:        structs.GenerateUUID(),
		},

		// Should be ignored as it is a different job.
		&structs.Allocation{
			TaskGroup: tg2.Name,
			JobID:     "ignore 2",
			ID:        structs.GenerateUUID(),
		},
	}
	plan.NodeAllocation[nodes[1].ID] = []*structs.Allocation{
		&structs.Allocation{
			TaskGroup: tg2.Name,
			JobID:     job.ID,
			ID:        structs.GenerateUUID(),
		},

		// Should be ignored as it is a different job.
		&structs.Allocation{
			TaskGroup: tg1.Name,
			JobID:     "ignore 2",
			ID:        structs.GenerateUUID(),
		},
	}

	propsed := NewProposedAllocConstraintIterator(ctx, static)
	propsed.SetTaskGroup(tg1)
	propsed.SetJob(job)

	out := collectFeasible(propsed)
	if len(out) != 0 {
		t.Fatalf("Bad: %#v", out)
	}
}

func TestProposedAllocConstraint_JobDistinctHosts_InfeasibleCount(t *testing.T) {
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
		Constraints: []*structs.Constraint{{Operand: structs.ConstraintDistinctHosts}},
		TaskGroups:  []*structs.TaskGroup{tg1, tg2, tg3},
	}

	propsed := NewProposedAllocConstraintIterator(ctx, static)
	propsed.SetTaskGroup(tg1)
	propsed.SetJob(job)

	// It should not be able to place 3 tasks with only two nodes.
	out := collectFeasible(propsed)
	if len(out) != 2 {
		t.Fatalf("Bad: %#v", out)
	}
}

func TestProposedAllocConstraint_TaskGroupDistinctHosts(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	static := NewStaticIterator(ctx, nodes)

	// Create a task group with a distinct_hosts constraint.
	taskGroup := &structs.TaskGroup{
		Name: "example",
		Constraints: []*structs.Constraint{
			{Operand: structs.ConstraintDistinctHosts},
		},
	}

	// Add a planned alloc to node1.
	plan := ctx.Plan()
	plan.NodeAllocation[nodes[0].ID] = []*structs.Allocation{
		&structs.Allocation{
			TaskGroup: taskGroup.Name,
			JobID:     "foo",
		},
	}

	// Add a planned alloc to node2 with the same task group name but a
	// different job.
	plan.NodeAllocation[nodes[1].ID] = []*structs.Allocation{
		&structs.Allocation{
			TaskGroup: taskGroup.Name,
			JobID:     "bar",
		},
	}

	propsed := NewProposedAllocConstraintIterator(ctx, static)
	propsed.SetTaskGroup(taskGroup)
	propsed.SetJob(&structs.Job{ID: "foo"})

	out := collectFeasible(propsed)
	if len(out) != 1 {
		t.Fatalf("Bad: %#v", out)
	}

	// Expect it to skip the first node as there is a previous alloc on it for
	// the same task group.
	if out[0] != nodes[1] {
		t.Fatalf("Bad: %v", out)
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

func newMockFeasiblityChecker(values ...bool) *mockFeasibilityChecker {
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
	mocked := newMockFeasiblityChecker(false)
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
	mocked := newMockFeasiblityChecker(false)
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
	jobMock := newMockFeasiblityChecker(true)
	tgMock := newMockFeasiblityChecker(false)
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
	jobMock := newMockFeasiblityChecker(true)
	tgMock := newMockFeasiblityChecker(false)
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
	jobMock := newMockFeasiblityChecker(true)
	tgMock := newMockFeasiblityChecker(true)
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
