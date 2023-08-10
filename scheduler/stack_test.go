// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func BenchmarkServiceStack_With_ComputedClass(b *testing.B) {
	// Key doesn't escape computed node class.
	benchmarkServiceStack_MetaKeyConstraint(b, "key", 5000, 64)
}

func BenchmarkServiceStack_WithOut_ComputedClass(b *testing.B) {
	// Key escapes computed node class.
	benchmarkServiceStack_MetaKeyConstraint(b, "unique.key", 5000, 64)
}

// benchmarkServiceStack_MetaKeyConstraint creates the passed number of nodes
// and sets the meta data key to have nodePartitions number of values. It then
// benchmarks the stack by selecting a job that constrains against one of the
// partitions.
func benchmarkServiceStack_MetaKeyConstraint(b *testing.B, key string, numNodes, nodePartitions int) {
	_, ctx := testContext(b)
	stack := NewGenericStack(false, ctx)

	// Create 4 classes of nodes.
	nodes := make([]*structs.Node, numNodes)
	for i := 0; i < numNodes; i++ {
		n := mock.Node()
		n.Meta[key] = fmt.Sprintf("%d", i%nodePartitions)
		nodes[i] = n
	}
	stack.SetNodes(nodes)

	// Create a job whose constraint meets two node classes.
	job := mock.Job()
	job.Constraints[0] = &structs.Constraint{
		LTarget: fmt.Sprintf("${meta.%v}", key),
		RTarget: "1",
		Operand: "<",
	}
	stack.SetJob(job)

	b.ResetTimer()
	selectOptions := &SelectOptions{}
	for i := 0; i < b.N; i++ {
		stack.Select(job.TaskGroups[0], selectOptions)
	}
}

func TestServiceStack_SetNodes(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	stack := NewGenericStack(false, ctx)

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
	ci.Parallel(t)

	_, ctx := testContext(t)
	stack := NewGenericStack(false, ctx)

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
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
	}
	stack := NewGenericStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	stack.SetJob(job)
	selectOptions := &SelectOptions{}
	node := stack.Select(job.TaskGroups[0], selectOptions)
	if node == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}

	// Note: On Windows time.Now currently has a best case granularity of 1ms.
	// We skip the following assertion on Windows because this test usually
	// runs too fast to measure an allocation time on Windows.
	met := ctx.Metrics()
	if runtime.GOOS != "windows" && met.AllocationTime == 0 {
		t.Fatalf("missing time")
	}
}

func TestServiceStack_Select_PreferringNodes(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
	}
	stack := NewGenericStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	stack.SetJob(job)

	// Create a preferred node
	preferredNode := mock.Node()
	prefNodes := []*structs.Node{preferredNode}
	selectOptions := &SelectOptions{PreferredNodes: prefNodes}
	option := stack.Select(job.TaskGroups[0], selectOptions)
	if option == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}
	if option.Node.ID != preferredNode.ID {
		t.Fatalf("expected: %v, actual: %v", option.Node.ID, preferredNode.ID)
	}

	// Make sure select doesn't have a side effect on preferred nodes
	require.Equal(t, prefNodes, selectOptions.PreferredNodes)

	// Change the preferred node's kernel to windows and ensure the allocations
	// are placed elsewhere
	preferredNode1 := preferredNode.Copy()
	preferredNode1.Attributes["kernel.name"] = "windows"
	preferredNode1.ComputeClass()
	prefNodes1 := []*structs.Node{preferredNode1}
	selectOptions = &SelectOptions{PreferredNodes: prefNodes1}
	option = stack.Select(job.TaskGroups[0], selectOptions)
	if option == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}

	if option.Node.ID != nodes[0].ID {
		t.Fatalf("expected: %#v, actual: %#v", nodes[0], option.Node)
	}
	require.Equal(t, prefNodes1, selectOptions.PreferredNodes)
}

func TestServiceStack_Select_MetricsReset(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	stack := NewGenericStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	stack.SetJob(job)
	selectOptions := &SelectOptions{}
	n1 := stack.Select(job.TaskGroups[0], selectOptions)
	m1 := ctx.Metrics()
	if n1 == nil {
		t.Fatalf("missing node %#v", m1)
	}

	if m1.NodesEvaluated != 2 {
		t.Fatalf("should only be 2")
	}

	n2 := stack.Select(job.TaskGroups[0], selectOptions)
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
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	zero := nodes[0]
	zero.Attributes["driver.foo"] = "1"
	if err := zero.ComputeClass(); err != nil {
		t.Fatalf("ComputedClass() failed: %v", err)
	}

	stack := NewGenericStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Driver = "foo"
	stack.SetJob(job)

	selectOptions := &SelectOptions{}
	node := stack.Select(job.TaskGroups[0], selectOptions)
	if node == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}

	if node.Node != zero {
		t.Fatalf("bad")
	}
}

func TestServiceStack_Select_CSI(t *testing.T) {
	ci.Parallel(t)

	state, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}

	// Create a volume in the state store
	index := uint64(999)
	v := structs.NewCSIVolume("foo[0]", index)
	v.Namespace = structs.DefaultNamespace
	v.AccessMode = structs.CSIVolumeAccessModeMultiNodeSingleWriter
	v.AttachmentMode = structs.CSIVolumeAttachmentModeFilesystem
	v.PluginID = "bar"
	err := state.UpsertCSIVolume(999, []*structs.CSIVolume{v})
	require.NoError(t, err)

	// Create a node with healthy fingerprints for both controller and node plugins
	zero := nodes[0]
	zero.CSIControllerPlugins = map[string]*structs.CSIInfo{"bar": {
		PluginID:           "bar",
		Healthy:            true,
		RequiresTopologies: false,
		ControllerInfo: &structs.CSIControllerInfo{
			SupportsReadOnlyAttach: true,
			SupportsListVolumes:    true,
		},
	}}
	zero.CSINodePlugins = map[string]*structs.CSIInfo{"bar": {
		PluginID:           "bar",
		Healthy:            true,
		RequiresTopologies: false,
		NodeInfo: &structs.CSINodeInfo{
			ID:                      zero.ID,
			MaxVolumes:              2,
			AccessibleTopology:      nil,
			RequiresNodeStageVolume: false,
		},
	}}

	// Add the node to the state store to index the healthy plugins and mark the volume "foo" healthy
	err = state.UpsertNode(structs.MsgTypeTestSetup, 1000, zero)
	require.NoError(t, err)

	// Use the node to build the stack and test
	if err := zero.ComputeClass(); err != nil {
		t.Fatalf("ComputedClass() failed: %v", err)
	}

	stack := NewGenericStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	job.TaskGroups[0].Count = 2
	job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{"foo": {
		Name:     "bar",
		Type:     structs.VolumeTypeCSI,
		Source:   "foo",
		ReadOnly: true,
		PerAlloc: true,
	}}

	stack.SetJob(job)

	selectOptions := &SelectOptions{
		AllocName: structs.AllocName(job.Name, job.TaskGroups[0].Name, 0)}
	node := stack.Select(job.TaskGroups[0], selectOptions)
	if node == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}

	if node.Node != zero {
		t.Fatalf("bad")
	}
}

func TestServiceStack_Select_ConstraintFilter(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	zero := nodes[0]
	zero.Attributes["kernel.name"] = "freebsd"
	if err := zero.ComputeClass(); err != nil {
		t.Fatalf("ComputedClass() failed: %v", err)
	}

	stack := NewGenericStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	job.Constraints[0].RTarget = "freebsd"
	stack.SetJob(job)
	selectOptions := &SelectOptions{}
	node := stack.Select(job.TaskGroups[0], selectOptions)
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
	if met.ConstraintFiltered["${attr.kernel.name} = freebsd"] != 1 {
		t.Fatalf("bad: %#v", met)
	}
}

func TestServiceStack_Select_BinPack_Overflow(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	zero := nodes[0]
	one := nodes[1]
	one.ReservedResources = &structs.NodeReservedResources{
		Cpu: structs.NodeReservedCpuResources{
			CpuShares: one.NodeResources.Cpu.CpuShares,
		},
	}

	stack := NewGenericStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	stack.SetJob(job)
	selectOptions := &SelectOptions{}
	node := stack.Select(job.TaskGroups[0], selectOptions)
	ctx.Metrics().PopulateScoreMetaData()
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
	// Expect score metadata for one node
	if len(met.ScoreMetaData) != 1 {
		t.Fatalf("bad: %#v", met)
	}
}

func TestSystemStack_SetNodes(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	stack := NewSystemStack(false, ctx)

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

	out := collectFeasible(stack.source)
	if !reflect.DeepEqual(out, nodes) {
		t.Fatalf("bad: %#v", out)
	}
}

func TestSystemStack_SetJob(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	stack := NewSystemStack(false, ctx)

	job := mock.Job()
	stack.SetJob(job)

	if stack.binPack.priority != job.Priority {
		t.Fatalf("bad")
	}
	if !reflect.DeepEqual(stack.jobConstraint.constraints, job.Constraints) {
		t.Fatalf("bad")
	}
}

func TestSystemStack_Select_Size(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*structs.Node{mock.Node()}
	stack := NewSystemStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	stack.SetJob(job)
	selectOptions := &SelectOptions{}
	node := stack.Select(job.TaskGroups[0], selectOptions)
	if node == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}

	// Note: On Windows time.Now currently has a best case granularity of 1ms.
	// We skip the following assertion on Windows because this test usually
	// runs too fast to measure an allocation time on Windows.
	met := ctx.Metrics()
	if runtime.GOOS != "windows" && met.AllocationTime == 0 {
		t.Fatalf("missing time")
	}
}

func TestSystemStack_Select_MetricsReset(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
		mock.Node(),
		mock.Node(),
	}
	stack := NewSystemStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	stack.SetJob(job)
	selectOptions := &SelectOptions{}
	n1 := stack.Select(job.TaskGroups[0], selectOptions)
	m1 := ctx.Metrics()
	if n1 == nil {
		t.Fatalf("missing node %#v", m1)
	}

	if m1.NodesEvaluated != 1 {
		t.Fatalf("should only be 1")
	}

	n2 := stack.Select(job.TaskGroups[0], selectOptions)
	m2 := ctx.Metrics()
	if n2 == nil {
		t.Fatalf("missing node %#v", m2)
	}

	// If we don't reset, this would be 2
	if m2.NodesEvaluated != 1 {
		t.Fatalf("should only be 2")
	}
}

func TestSystemStack_Select_DriverFilter(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
	}
	zero := nodes[0]
	zero.Attributes["driver.foo"] = "1"

	stack := NewSystemStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	job.TaskGroups[0].Tasks[0].Driver = "foo"
	stack.SetJob(job)

	selectOptions := &SelectOptions{}
	node := stack.Select(job.TaskGroups[0], selectOptions)
	if node == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}

	if node.Node != zero {
		t.Fatalf("bad")
	}

	zero.Attributes["driver.foo"] = "0"
	if err := zero.ComputeClass(); err != nil {
		t.Fatalf("ComputedClass() failed: %v", err)
	}

	stack = NewSystemStack(false, ctx)
	stack.SetNodes(nodes)
	stack.SetJob(job)
	node = stack.Select(job.TaskGroups[0], selectOptions)
	if node != nil {
		t.Fatalf("node not filtered %#v", node)
	}
}

func TestSystemStack_Select_ConstraintFilter(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	zero := nodes[1]
	zero.Attributes["kernel.name"] = "freebsd"
	if err := zero.ComputeClass(); err != nil {
		t.Fatalf("ComputedClass() failed: %v", err)
	}

	stack := NewSystemStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	job.Constraints[0].RTarget = "freebsd"
	stack.SetJob(job)

	selectOptions := &SelectOptions{}
	node := stack.Select(job.TaskGroups[0], selectOptions)
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
	if met.ConstraintFiltered["${attr.kernel.name} = freebsd"] != 1 {
		t.Fatalf("bad: %#v", met)
	}
}

func TestSystemStack_Select_BinPack_Overflow(t *testing.T) {
	ci.Parallel(t)

	_, ctx := testContext(t)
	nodes := []*structs.Node{
		mock.Node(),
		mock.Node(),
	}
	zero := nodes[0]
	zero.ReservedResources = &structs.NodeReservedResources{
		Cpu: structs.NodeReservedCpuResources{
			CpuShares: zero.NodeResources.Cpu.CpuShares,
		},
	}
	one := nodes[1]

	stack := NewSystemStack(false, ctx)
	stack.SetNodes(nodes)

	job := mock.Job()
	stack.SetJob(job)

	selectOptions := &SelectOptions{}
	node := stack.Select(job.TaskGroups[0], selectOptions)
	ctx.Metrics().PopulateScoreMetaData()
	if node == nil {
		t.Fatalf("missing node %#v", ctx.Metrics())
	}

	if node.Node != one {
		t.Fatalf("bad")
	}

	met := ctx.Metrics()
	if met.NodesExhausted != 1 {
		t.Fatalf("bad: %#v", met)
	}
	if met.ClassExhausted["linux-medium-pci"] != 1 {
		t.Fatalf("bad: %#v", met)
	}
	// Should have two scores, one from bin packing and one from normalization
	if len(met.ScoreMetaData) != 1 {
		t.Fatalf("bad: %#v", met)
	}
}
