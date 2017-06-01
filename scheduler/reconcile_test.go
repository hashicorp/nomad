package scheduler

import (
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
)

/*

TODO:
Basic Tests:
√   Place when there is nothing in the cluster
√   Place remainder when there is some in the cluster
√   Scale down from n to n-m where n != m
√   Scale down from n to zero
√   Inplace upgrade test
√   Inplace upgrade and scale up test
√   Inplace upgrade and scale down test
√   Destructive upgrade
√   Destructive upgrade and scale up test
√   Destructive upgrade and scale down test
√   Handle lost nodes
√   Handle lost nodes and scale up
√   Handle lost nodes and scale down
√   Handle draining nodes
√   Handle draining nodes and scale up
√   Handle draining nodes and scale down
√   Handle task group being removed
√   Handle job being stopped both as .Stopped and nil
√   Place more that one group

Canary Tests:
-  Stopped job cancels any existing deployment
-  JobIndex change cancels any existing deployment
-  Creates a deployment if any group has an update strategy
-  Paused deployment doesn't create any more canaries
-  Paused deployment doesn't do any placements
-  Paused deployment doesn't do destructive updates
-  Paused deployment does do migrations
-  Canary that is on a tainted node
-  Stop old canaries
-  Create new canaries on job change
-  Create new canaries on job change while scaling up
-  Create new canaries on job change while scaling down
-  Fill canaries if partial placement
-  Promote canaries unblocks max_parallel
-  Failed deployment should not place anything

*/

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func allocUpdateFnIgnore(*structs.Allocation, *structs.Job, *structs.TaskGroup) (bool, bool, *structs.Allocation) {
	return true, false, nil
}

func allocUpdateFnDestructive(*structs.Allocation, *structs.Job, *structs.TaskGroup) (bool, bool, *structs.Allocation) {
	return false, true, nil
}

func allocUpdateFnInplace(existing *structs.Allocation, _ *structs.Job, newTG *structs.TaskGroup) (bool, bool, *structs.Allocation) {
	// Create a shallow copy
	newAlloc := new(structs.Allocation)
	*newAlloc = *existing
	newAlloc.TaskResources = make(map[string]*structs.Resources)

	// Use the new task resources but keep the network from the old
	for _, task := range newTG.Tasks {
		r := task.Resources.Copy()
		r.Networks = existing.TaskResources[task.Name].Networks
		newAlloc.TaskResources[task.Name] = r
	}

	return false, false, newAlloc
}

var (
	// AllocationIndexRegex is a regular expression to find the allocation index.
	allocationIndexRegex = regexp.MustCompile(".+\\[(\\d+)\\]$")
)

// allocNameToIndex returns the index of the allocation.
func allocNameToIndex(name string) uint {
	matches := allocationIndexRegex.FindStringSubmatch(name)
	if len(matches) != 2 {
		return 0
	}

	index, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}

	return uint(index)
}

func assertNamesHaveIndexes(t *testing.T, indexes []int, names []string) {
	m := make(map[uint]int)
	for _, i := range indexes {
		m[uint(i)] += 1
	}

	for _, n := range names {
		index := allocNameToIndex(n)
		val, contained := m[index]
		if !contained {
			t.Fatalf("Unexpected index %d from name %s\nAll names: %v", index, n, names)
		}

		val--
		if val < 0 {
			t.Fatalf("Index %d repeated too many times\nAll names: %v", index, names)
		}
		m[index] = val
	}

	for k, remainder := range m {
		if remainder != 0 {
			t.Fatalf("Index %d has %d remaining uses expected\nAll names: %v", k, remainder, names)
		}
	}
}

func intRange(pairs ...int) []int {
	if len(pairs)%2 != 0 {
		return nil
	}

	var r []int
	for i := 0; i < len(pairs); i += 2 {
		for j := pairs[i]; j <= pairs[i+1]; j++ {
			r = append(r, j)
		}
	}
	return r
}

func placeResultsToNames(place []allocPlaceResult) []string {
	names := make([]string, 0, len(place))
	for _, p := range place {
		names = append(names, p.name)
	}
	return names
}

func stopResultsToNames(stop []allocStopResult) []string {
	names := make([]string, 0, len(stop))
	for _, s := range stop {
		names = append(names, s.alloc.Name)
	}
	return names
}

func allocsToNames(allocs []*structs.Allocation) []string {
	names := make([]string, 0, len(allocs))
	for _, a := range allocs {
		names = append(names, a.Name)
	}
	return names
}

type resultExpectation struct {
	createDeployment  *structs.Deployment
	deploymentUpdates int
	place             int
	inplace           int
	stop              int
	desiredTGUpdates  map[string]*structs.DesiredUpdates
}

func assertResults(t *testing.T, r *reconcileResults, exp *resultExpectation) {

	if exp.createDeployment != nil && r.createDeployment == nil {
		t.Fatalf("Expect a created deployment got none")
	} else if exp.createDeployment == nil && r.createDeployment != nil {
		t.Fatalf("Expect no created deployment; got %v", r.createDeployment)
	} else if !reflect.DeepEqual(r.createDeployment, exp.createDeployment) {
		t.Fatalf("Unexpected createdDeployment: %v", pretty.Diff(r.createDeployment, exp.createDeployment))
	}

	if l := len(r.deploymentUpdates); l != exp.deploymentUpdates {
		t.Fatalf("Expect %d deployment updates; got %v", exp.deploymentUpdates, r.deploymentUpdates)
	}
	if l := len(r.place); l != exp.place {
		t.Fatalf("Expected %d placements; got %d", exp.place, l)
	}
	if l := len(r.inplaceUpdate); l != exp.inplace {
		t.Fatalf("Expected %d inplaceUpdate; got %d", exp.inplace, l)
	}
	if l := len(r.stop); l != exp.stop {
		t.Fatalf("Expected %d stops; got %d", exp.stop, l)
	}
	if l := len(r.desiredTGUpdates); l != len(exp.desiredTGUpdates) {
		t.Fatalf("Expected %d task group desired tg updates annotations; got %d", len(exp.desiredTGUpdates), l)
	}

	// Check the desired updates happened
	for group, desired := range exp.desiredTGUpdates {
		act, ok := r.desiredTGUpdates[group]
		if !ok {
			t.Fatalf("Expected desired updates for group %q", group)
		}

		if !reflect.DeepEqual(act, desired) {
			t.Fatalf("Unexpected annotations for group %q: %v", group, pretty.Diff(act, desired))
		}
	}
}

// Tests the reconciler properly handles placements for a job that has no
// existing allocations
func TestReconciler_Place_NoExisting(t *testing.T) {
	job := mock.Job()
	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, nil, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             10,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 9), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles placements for a job that has some
// existing allocations
func TestReconciler_Place_Existing(t *testing.T) {
	job := mock.Job()

	// Create 3 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, allocs, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             5,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  5,
				Ignore: 5,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(5, 9), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles stopping allocations for a job that has
// scaled down
func TestReconciler_ScaleDown_Partial(t *testing.T) {
	// Has desired 10
	job := mock.Job()

	// Create 20 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 20; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, allocs, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             0,
		inplace:           0,
		stop:              10,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Ignore: 10,
				Stop:   10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(10, 19), stopResultsToNames(r.stop))
}

// Tests the reconciler properly handles stopping allocations for a job that has
// scaled down to zero desired
func TestReconciler_ScaleDown_Zero(t *testing.T) {
	// Set desired 0
	job := mock.Job()
	job.TaskGroups[0].Count = 0

	// Create 20 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 20; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, allocs, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             0,
		inplace:           0,
		stop:              20,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Stop: 20,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 19), stopResultsToNames(r.stop))
}

// Tests the reconciler properly handles inplace upgrading allocations
func TestReconciler_Inplace(t *testing.T) {
	job := mock.Job()

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnInplace, false, job.ID, job, nil, allocs, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             0,
		inplace:           10,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				InPlaceUpdate: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 9), allocsToNames(r.inplaceUpdate))
}

// Tests the reconciler properly handles inplace upgrading allocations while
// scaling up
func TestReconciler_Inplace_ScaleUp(t *testing.T) {
	// Set desired 15
	job := mock.Job()
	job.TaskGroups[0].Count = 15

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnInplace, false, job.ID, job, nil, allocs, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             5,
		inplace:           10,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:         5,
				InPlaceUpdate: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 9), allocsToNames(r.inplaceUpdate))
	assertNamesHaveIndexes(t, intRange(10, 14), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles inplace upgrading allocations while
// scaling down
func TestReconciler_Inplace_ScaleDown(t *testing.T) {
	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnInplace, false, job.ID, job, nil, allocs, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             0,
		inplace:           5,
		stop:              5,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Stop:          5,
				InPlaceUpdate: 5,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 4), allocsToNames(r.inplaceUpdate))
	assertNamesHaveIndexes(t, intRange(5, 9), stopResultsToNames(r.stop))
}

// Tests the reconciler properly handles destructive upgrading allocations
func TestReconciler_Destructive(t *testing.T) {
	job := mock.Job()

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnDestructive, false, job.ID, job, nil, allocs, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             10,
		inplace:           0,
		stop:              10,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				DestructiveUpdate: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 9), placeResultsToNames(r.place))
	assertNamesHaveIndexes(t, intRange(0, 9), stopResultsToNames(r.stop))
}

// Tests the reconciler properly handles destructive upgrading allocations while
// scaling up
func TestReconciler_Destructive_ScaleUp(t *testing.T) {
	// Set desired 15
	job := mock.Job()
	job.TaskGroups[0].Count = 15

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnDestructive, false, job.ID, job, nil, allocs, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             15,
		inplace:           0,
		stop:              10,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:             5,
				DestructiveUpdate: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 9), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(0, 14), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles destructive upgrading allocations while
// scaling down
func TestReconciler_Destructive_ScaleDown(t *testing.T) {
	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnDestructive, false, job.ID, job, nil, allocs, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             5,
		inplace:           0,
		stop:              10,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Stop:              5,
				DestructiveUpdate: 5,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 9), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(0, 4), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles lost nodes with allocations
func TestReconciler_LostNode(t *testing.T) {
	job := mock.Job()

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	// Build a map of tainted nodes
	tainted := make(map[string]*structs.Node, 2)
	for i := 0; i < 2; i++ {
		n := mock.Node()
		n.ID = allocs[i].NodeID
		n.Status = structs.NodeStatusDown
		tainted[n.ID] = n
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, allocs, tainted)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             2,
		inplace:           0,
		stop:              2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  2,
				Ignore: 8,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(0, 1), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles lost nodes with allocations while
// scaling up
func TestReconciler_LostNode_ScaleUp(t *testing.T) {
	// Set desired 15
	job := mock.Job()
	job.TaskGroups[0].Count = 15

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	// Build a map of tainted nodes
	tainted := make(map[string]*structs.Node, 2)
	for i := 0; i < 2; i++ {
		n := mock.Node()
		n.ID = allocs[i].NodeID
		n.Status = structs.NodeStatusDown
		tainted[n.ID] = n
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, allocs, tainted)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             7,
		inplace:           0,
		stop:              2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  7,
				Ignore: 8,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(0, 1, 10, 14), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles lost nodes with allocations while
// scaling down
func TestReconciler_LostNode_ScaleDown(t *testing.T) {
	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	// Build a map of tainted nodes
	tainted := make(map[string]*structs.Node, 2)
	for i := 0; i < 2; i++ {
		n := mock.Node()
		n.ID = allocs[i].NodeID
		n.Status = structs.NodeStatusDown
		tainted[n.ID] = n
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, allocs, tainted)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             0,
		inplace:           0,
		stop:              5,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Stop:   5,
				Ignore: 5,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1, 7, 9), stopResultsToNames(r.stop))
}

// Tests the reconciler properly handles draining nodes with allocations
func TestReconciler_DrainNode(t *testing.T) {
	job := mock.Job()

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	// Build a map of tainted nodes
	tainted := make(map[string]*structs.Node, 2)
	for i := 0; i < 2; i++ {
		n := mock.Node()
		n.ID = allocs[i].NodeID
		n.Drain = true
		tainted[n.ID] = n
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, allocs, tainted)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             2,
		inplace:           0,
		stop:              2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Migrate: 2,
				Ignore:  8,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(0, 1), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles draining nodes with allocations while
// scaling up
func TestReconciler_DrainNode_ScaleUp(t *testing.T) {
	// Set desired 15
	job := mock.Job()
	job.TaskGroups[0].Count = 15

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	// Build a map of tainted nodes
	tainted := make(map[string]*structs.Node, 2)
	for i := 0; i < 2; i++ {
		n := mock.Node()
		n.ID = allocs[i].NodeID
		n.Drain = true
		tainted[n.ID] = n
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, allocs, tainted)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             7,
		inplace:           0,
		stop:              2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:   5,
				Migrate: 2,
				Ignore:  8,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(0, 1, 10, 14), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles draining nodes with allocations while
// scaling down
func TestReconciler_DrainNode_ScaleDown(t *testing.T) {
	// Set desired 8
	job := mock.Job()
	job.TaskGroups[0].Count = 8

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	// Build a map of tainted nodes
	tainted := make(map[string]*structs.Node, 3)
	for i := 0; i < 3; i++ {
		n := mock.Node()
		n.ID = allocs[i].NodeID
		n.Drain = true
		tainted[n.ID] = n
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, allocs, tainted)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             1,
		inplace:           0,
		stop:              3,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Migrate: 1,
				Stop:    2,
				Ignore:  7,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 2), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(0, 0), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles a task group being removed
func TestReconciler_RemovedTG(t *testing.T) {
	job := mock.Job()

	// Create 10 allocations for a tg that no longer exists
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	oldName := job.TaskGroups[0].Name
	newName := "different"
	job.TaskGroups[0].Name = newName

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, allocs, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             10,
		inplace:           0,
		stop:              10,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			oldName: {
				Stop: 10,
			},
			newName: {
				Place: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 9), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(0, 9), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles a job in stopped states
func TestReconciler_JobStopped(t *testing.T) {
	job := mock.Job()
	job.Stop = true

	cases := []struct {
		name             string
		job              *structs.Job
		jobID, taskGroup string
	}{
		{
			name:      "stopped job",
			job:       job,
			jobID:     job.ID,
			taskGroup: job.TaskGroups[0].Name,
		},
		{
			name:      "nil job",
			job:       nil,
			jobID:     "foo",
			taskGroup: "bar",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Create 10 allocations for a tg that no longer exists
			var allocs []*structs.Allocation
			for i := 0; i < 10; i++ {
				alloc := mock.Alloc()
				alloc.Job = c.job
				alloc.JobID = c.jobID
				alloc.NodeID = structs.GenerateUUID()
				alloc.Name = structs.AllocName(c.jobID, c.taskGroup, uint(i))
				alloc.TaskGroup = c.taskGroup
				allocs = append(allocs, alloc)
			}

			reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, c.jobID, c.job, nil, allocs, nil)
			r := reconciler.Compute()

			// Assert the correct results
			assertResults(t, r, &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: 0,
				place:             0,
				inplace:           0,
				stop:              10,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					c.taskGroup: {
						Stop: 10,
					},
				},
			})

			assertNamesHaveIndexes(t, intRange(0, 9), stopResultsToNames(r.stop))
		})
	}
}

// Tests the reconciler properly handles jobs with multiple task groups
func TestReconciler_MultiTG(t *testing.T) {
	job := mock.Job()
	tg2 := job.TaskGroups[0].Copy()
	tg2.Name = "foo"
	job.TaskGroups = append(job.TaskGroups, tg2)

	// Create 2 existing allocations for the first tg
	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = structs.GenerateUUID()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testLogger(), allocUpdateFnIgnore, false, job.ID, job, nil, allocs, nil)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: 0,
		place:             18,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  8,
				Ignore: 2,
			},
			tg2.Name: {
				Place: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(2, 9, 0, 9), placeResultsToNames(r.place))
}
