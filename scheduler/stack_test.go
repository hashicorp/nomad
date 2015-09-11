package scheduler

import (
	"reflect"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestServiceStack_SetNodes(t *testing.T) {
	_, ctx := testContext(t)
	stack := NewGenericStack(false, ctx, nil)

	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	stack.SetNodes(nodes)

	// Check that our scan limit is updated
	if stack.limit.limit != 3 {
		t.Fatalf("bad limit %d", stack.limit.limit)
	}

	out := collectFeasible(stack.source)
	if !reflect.DeepEqual(out, nodes) {
		t.Fatalf("bad: %#v", out)
	}
}

func TestServiceStack_SetJob(t *testing.T) {
	_, ctx := testContext(t)
	stack := NewGenericStack(false, ctx, nil)

	job := mock.Job()
	stack.SetJob(job)

	if stack.binPack.priority != job.Priority {
		t.Fatalf("bad")
	}
	if !reflect.DeepEqual(stack.jobConstraint.constraints, job.Constraints) {
		t.Fatalf("bad")
	}
}

func TestServiceStack_Select_Size(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
	}
	stack := NewGenericStack(false, ctx, nodes)

	job := mock.Job()
	stack.SetJob(job)
	node, size := stack.Select(job.TaskGroups[0])
	if node == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}
	if size == nil {
		t.Fatalf("missing size")
	}

	if size.CPU != 0.5 || size.MemoryMB != 256 {
		t.Fatalf("bad: %#v", size)
	}

	met := ctx.Metrics()
	if met.AllocationTime == 0 {
		t.Fatalf("missing time")
	}
}

func TestServiceStack_Select_MetricsReset(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	stack := NewGenericStack(false, ctx, nodes)

	job := mock.Job()
	stack.SetJob(job)
	n1, _ := stack.Select(job.TaskGroups[0])
	m1 := ctx.Metrics()
	if n1 == nil {
		t.Fatalf("missing node %#v", m1)
	}

	if m1.NodesEvaluated != 2 {
		t.Fatalf("should only be 2")
	}

	n2, _ := stack.Select(job.TaskGroups[0])
	m2 := ctx.Metrics()
	if n2 == nil {
		t.Fatalf("missing node %#v", m2)
	}

	// If we don't reset, this would be 4
	if m2.NodesEvaluated != 2 {
		t.Fatalf("should only be 2")
	}
}

func TestServiceStack_Select_DriverFilter(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	zero := nodes[0]
	zero.Attributes["driver.foo"] = "1"

	stack := NewGenericStack(false, ctx, nodes)

	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Driver = "foo"
	stack.SetJob(job)

	node, _ := stack.Select(job.TaskGroups[0])
	if node == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}

	if node.Node != zero {
		t.Fatalf("bad")
	}
}

func TestServiceStack_Select_ConstraintFilter(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	zero := nodes[0]
	zero.Attributes["kernel.name"] = "freebsd"

	stack := NewGenericStack(false, ctx, nodes)

	job := mock.Job()
	job.Constraints[0].RTarget = "freebsd"
	stack.SetJob(job)

	node, _ := stack.Select(job.TaskGroups[0])
	if node == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}

	if node.Node != zero {
		t.Fatalf("bad")
	}

	met := ctx.Metrics()
	if met.NodesFiltered != 1 {
		t.Fatalf("bad: %#v", met)
	}
	if met.ClassFiltered["linux-medium-pci"] != 1 {
		t.Fatalf("bad: %#v", met)
	}
	if met.ConstraintFiltered["$attr.kernel.name = freebsd"] != 1 {
		t.Fatalf("bad: %#v", met)
	}
}

func TestServiceStack_Select_BinPack_Overflow(t *testing.T) {
	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	zero := nodes[0]
	one := nodes[1]
	one.Reserved = one.Resources

	stack := NewGenericStack(false, ctx, nodes)

	job := mock.Job()
	stack.SetJob(job)

	node, _ := stack.Select(job.TaskGroups[0])
	if node == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}

	if node.Node != zero {
		t.Fatalf("bad")
	}

	met := ctx.Metrics()
	if met.NodesExhausted != 1 {
		t.Fatalf("bad: %#v", met)
	}
	if met.ClassExhausted["linux-medium-pci"] != 1 {
		t.Fatalf("bad: %#v", met)
	}
	if len(met.Scores) != 1 {
		t.Fatalf("bad: %#v", met)
	}
}
