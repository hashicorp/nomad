// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	canaryUpdate = &structs.UpdateStrategy{
		Canary:          2,
		MaxParallel:     2,
		HealthCheck:     structs.UpdateStrategyHealthCheck_Checks,
		MinHealthyTime:  10 * time.Second,
		HealthyDeadline: 10 * time.Minute,
		Stagger:         31 * time.Second,
	}

	noCanaryUpdate = &structs.UpdateStrategy{
		MaxParallel:     4,
		HealthCheck:     structs.UpdateStrategyHealthCheck_Checks,
		MinHealthyTime:  10 * time.Second,
		HealthyDeadline: 10 * time.Minute,
		Stagger:         31 * time.Second,
	}
)

func allocUpdateFnIgnore(*structs.Allocation, *structs.Job, *structs.TaskGroup) (bool, bool, *structs.Allocation) {
	return true, false, nil
}

func allocUpdateFnDestructive(*structs.Allocation, *structs.Job, *structs.TaskGroup) (bool, bool, *structs.Allocation) {
	return false, true, nil
}

func allocUpdateFnInplace(existing *structs.Allocation, _ *structs.Job, newTG *structs.TaskGroup) (bool, bool, *structs.Allocation) {
	// Create a shallow copy
	newAlloc := existing.CopySkipJob()
	newAlloc.AllocatedResources = &structs.AllocatedResources{
		Tasks: map[string]*structs.AllocatedTaskResources{},
		Shared: structs.AllocatedSharedResources{
			DiskMB: int64(newTG.EphemeralDisk.SizeMB),
		},
	}

	// Use the new task resources but keep the network from the old
	for _, task := range newTG.Tasks {
		networks := existing.AllocatedResources.Tasks[task.Name].Copy().Networks
		newAlloc.AllocatedResources.Tasks[task.Name] = &structs.AllocatedTaskResources{
			Cpu: structs.AllocatedCpuResources{
				CpuShares: int64(task.Resources.CPU),
			},
			Memory: structs.AllocatedMemoryResources{
				MemoryMB: int64(task.Resources.MemoryMB),
			},
			Networks: networks,
		}
	}

	return false, false, newAlloc
}

func allocUpdateFnMock(handled map[string]allocUpdateType, unhandled allocUpdateType) allocUpdateType {
	return func(existing *structs.Allocation, newJob *structs.Job, newTG *structs.TaskGroup) (bool, bool, *structs.Allocation) {
		if fn, ok := handled[existing.ID]; ok {
			return fn(existing, newJob, newTG)
		}

		return unhandled(existing, newJob, newTG)
	}
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
	t.Helper()
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

func assertNoCanariesStopped(t *testing.T, d *structs.Deployment, stop []allocStopResult) {
	t.Helper()
	canaryIndex := make(map[string]struct{})
	for _, state := range d.TaskGroups {
		for _, c := range state.PlacedCanaries {
			canaryIndex[c] = struct{}{}
		}
	}

	for _, s := range stop {
		if _, ok := canaryIndex[s.alloc.ID]; ok {
			t.Fatalf("Stopping canary alloc %q %q", s.alloc.ID, s.alloc.Name)
		}
	}
}

func assertPlaceResultsHavePreviousAllocs(t *testing.T, numPrevious int, place []allocPlaceResult) {
	t.Helper()
	names := make(map[string]struct{}, numPrevious)

	found := 0
	for _, p := range place {
		if _, ok := names[p.name]; ok {
			t.Fatalf("Name %q already placed", p.name)
		}
		names[p.name] = struct{}{}

		if p.previousAlloc == nil {
			continue
		}

		if act := p.previousAlloc.Name; p.name != act {
			t.Fatalf("Name mismatch on previous alloc; got %q; want %q", act, p.name)
		}
		found++
	}
	if numPrevious != found {
		t.Fatalf("wanted %d; got %d placements with previous allocs", numPrevious, found)
	}
}

func assertPlacementsAreRescheduled(t *testing.T, numRescheduled int, place []allocPlaceResult) {
	t.Helper()
	names := make(map[string]struct{}, numRescheduled)

	found := 0
	for _, p := range place {
		if _, ok := names[p.name]; ok {
			t.Fatalf("Name %q already placed", p.name)
		}
		names[p.name] = struct{}{}

		if p.previousAlloc == nil {
			continue
		}
		if p.reschedule {
			found++
		}

	}
	if numRescheduled != found {
		t.Fatalf("wanted %d; got %d placements that are rescheduled", numRescheduled, found)
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

func destructiveResultsToNames(destructive []allocDestructiveResult) []string {
	names := make([]string, 0, len(destructive))
	for _, d := range destructive {
		names = append(names, d.placeName)
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

func attributeUpdatesToNames(attributeUpdates map[string]*structs.Allocation) []string {
	names := make([]string, 0, len(attributeUpdates))
	for _, a := range attributeUpdates {
		names = append(names, a.Name)
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
	deploymentUpdates []*structs.DeploymentStatusUpdate
	place             int
	destructive       int
	inplace           int
	attributeUpdates  int
	disconnectUpdates int
	reconnectUpdates  int
	desiredTGUpdates  map[string]*structs.DesiredUpdates
	stop              int
}

func assertResults(t *testing.T, r *reconcileResults, exp *resultExpectation) {
	t.Helper()
	assertion := assert.New(t)

	if exp.createDeployment != nil && r.deployment == nil {
		t.Errorf("Expect a created deployment got none")
	} else if exp.createDeployment == nil && r.deployment != nil {
		t.Errorf("Expect no created deployment; got %#v", r.deployment)
	} else if exp.createDeployment != nil && r.deployment != nil {
		// Clear the deployment ID
		r.deployment.ID, exp.createDeployment.ID = "", ""
		if !reflect.DeepEqual(r.deployment, exp.createDeployment) {
			t.Errorf("Unexpected createdDeployment; got\n %#v\nwant\n%#v\nDiff: %v",
				r.deployment, exp.createDeployment, pretty.Diff(r.deployment, exp.createDeployment))
		}
	}

	assertion.EqualValues(exp.deploymentUpdates, r.deploymentUpdates, "Expected Deployment Updates")
	assertion.Len(r.place, exp.place, "Expected Placements")
	assertion.Len(r.destructiveUpdate, exp.destructive, "Expected Destructive")
	assertion.Len(r.inplaceUpdate, exp.inplace, "Expected Inplace Updates")
	assertion.Len(r.attributeUpdates, exp.attributeUpdates, "Expected Attribute Updates")
	assertion.Len(r.reconnectUpdates, exp.reconnectUpdates, "Expected Reconnect Updates")
	assertion.Len(r.disconnectUpdates, exp.disconnectUpdates, "Expected Disconnect Updates")
	assertion.Len(r.stop, exp.stop, "Expected Stops")
	assertion.EqualValues(exp.desiredTGUpdates, r.desiredTGUpdates, "Expected Desired TG Update Annotations")
}

func buildAllocations(job *structs.Job, count int, clientStatus, desiredStatus string, nodeScore float64) []*structs.Allocation {
	allocs := make([]*structs.Allocation, 0)

	for i := 0; i < count; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.ClientStatus = clientStatus
		alloc.DesiredStatus = desiredStatus

		alloc.Metrics = &structs.AllocMetric{
			ScoreMetaData: []*structs.NodeScoreMeta{
				{
					NodeID:    alloc.NodeID,
					NormScore: nodeScore,
					Scores: map[string]float64{
						alloc.NodeID: nodeScore,
					},
				},
			},
		}

		allocs = append(allocs, alloc)
	}

	return allocs
}

func buildDisconnectedNodes(allocs []*structs.Allocation, count int) map[string]*structs.Node {
	tainted := make(map[string]*structs.Node, count)
	for i := 0; i < count; i++ {
		n := mock.Node()
		n.ID = allocs[i].NodeID
		n.Status = structs.NodeStatusDisconnected
		tainted[n.ID] = n
	}
	return tainted
}

func buildResumableAllocations(count int, clientStatus, desiredStatus string, nodeScore float64) (*structs.Job, []*structs.Allocation) {
	job := mock.Job()
	job.TaskGroups[0].MaxClientDisconnect = pointer.Of(5 * time.Minute)
	job.TaskGroups[0].Count = count

	return job, buildAllocations(job, count, clientStatus, desiredStatus, nodeScore)
}

// Tests the reconciler properly handles placements for a job that has no
// existing allocations
func TestReconciler_Place_NoExisting(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	reconciler := NewAllocReconciler(
		testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, nil, nil, "", job.Priority, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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
	ci.Parallel(t)

	job := mock.Job()

	// Create 3 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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
	ci.Parallel(t)

	// Has desired 10
	job := mock.Job()

	// Create 20 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 20; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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
	ci.Parallel(t)

	// Set desired 0
	job := mock.Job()
	job.TaskGroups[0].Count = 0

	// Create 20 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 20; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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

// Tests the reconciler properly handles stopping allocations for a job that has
// scaled down to zero desired where allocs have duplicate names
func TestReconciler_ScaleDown_Zero_DuplicateNames(t *testing.T) {
	ci.Parallel(t)

	// Set desired 0
	job := mock.Job()
	job.TaskGroups[0].Count = 0

	// Create 20 existing allocations
	var allocs []*structs.Allocation
	var expectedStopped []int
	for i := 0; i < 20; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i%2))
		allocs = append(allocs, alloc)
		expectedStopped = append(expectedStopped, i%2)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             0,
		inplace:           0,
		stop:              20,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Stop: 20,
			},
		},
	})

	assertNamesHaveIndexes(t, expectedStopped, stopResultsToNames(r.stop))
}

// Tests the reconciler properly handles inplace upgrading allocations
func TestReconciler_Inplace(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnInplace, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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
	ci.Parallel(t)

	// Set desired 15
	job := mock.Job()
	job.TaskGroups[0].Count = 15

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnInplace, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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
	ci.Parallel(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnInplace, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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

// TestReconciler_Inplace_Rollback tests that a rollback to a previous version
// generates the expected placements for any already-running allocations of
// that version.
func TestReconciler_Inplace_Rollback(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Count = 4
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		DelayFunction: "exponential",
		Interval:      time.Second * 30,
		Delay:         time.Hour * 1,
		Attempts:      3,
		Unlimited:     true,
	}

	// Create 3 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 3; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}
	// allocs[0] is an allocation from version 0
	allocs[0].ClientStatus = structs.AllocClientStatusRunning

	// allocs[1] and allocs[2] are failed allocations for version 1 with
	// different rescheduling states
	allocs[1].ClientStatus = structs.AllocClientStatusFailed
	allocs[1].TaskStates = map[string]*structs.TaskState{
		"web": {FinishedAt: time.Now().Add(-10 * time.Minute)}}
	allocs[2].ClientStatus = structs.AllocClientStatusFailed

	// job is rolled back, we expect allocs[0] to be updated in-place
	allocUpdateFn := allocUpdateFnMock(map[string]allocUpdateType{
		allocs[0].ID: allocUpdateFnInplace,
	}, allocUpdateFnDestructive)

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFn,
		false, job.ID, job, nil, allocs, nil, uuid.Generate(), 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             2,
		inplace:           1,
		stop:              1,
		destructive:       1,
		attributeUpdates:  1,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:             2,
				Stop:              1,
				InPlaceUpdate:     1,
				DestructiveUpdate: 1,
			},
		},
	})

	assert.Len(t, r.desiredFollowupEvals, 1, "expected 1 follow-up eval")
	assertNamesHaveIndexes(t, intRange(0, 0), allocsToNames(r.inplaceUpdate))
	assertNamesHaveIndexes(t, intRange(2, 2), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(2, 3), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles destructive upgrading allocations
func TestReconciler_Destructive(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		destructive:       10,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				DestructiveUpdate: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 9), destructiveResultsToNames(r.destructiveUpdate))
}

// Tests the reconciler properly handles destructive upgrading allocations when max_parallel=0
func TestReconciler_DestructiveMaxParallel(t *testing.T) {
	ci.Parallel(t)

	job := mock.MaxParallelJob()

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		destructive:       10,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				DestructiveUpdate: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 9), destructiveResultsToNames(r.destructiveUpdate))
}

// Tests the reconciler properly handles destructive upgrading allocations while
// scaling up
func TestReconciler_Destructive_ScaleUp(t *testing.T) {
	ci.Parallel(t)

	// Set desired 15
	job := mock.Job()
	job.TaskGroups[0].Count = 15

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             5,
		destructive:       10,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:             5,
				DestructiveUpdate: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 9), destructiveResultsToNames(r.destructiveUpdate))
	assertNamesHaveIndexes(t, intRange(10, 14), placeResultsToNames(r.place))
}

// Tests the reconciler properly handles destructive upgrading allocations while
// scaling down
func TestReconciler_Destructive_ScaleDown(t *testing.T) {
	ci.Parallel(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		destructive:       5,
		stop:              5,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Stop:              5,
				DestructiveUpdate: 5,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(5, 9), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(0, 4), destructiveResultsToNames(r.destructiveUpdate))
}

// Tests the reconciler properly handles lost nodes with allocations
func TestReconciler_LostNode(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
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

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, tainted, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             2,
		inplace:           0,
		stop:              2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  2,
				Stop:   2,
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
	ci.Parallel(t)

	// Set desired 15
	job := mock.Job()
	job.TaskGroups[0].Count = 15

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
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

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, tainted, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             7,
		inplace:           0,
		stop:              2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  7,
				Stop:   2,
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
	ci.Parallel(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
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

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, tainted, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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
	ci.Parallel(t)

	job := mock.Job()

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	// Build a map of tainted nodes
	tainted := make(map[string]*structs.Node, 2)
	for i := 0; i < 2; i++ {
		n := mock.DrainNode()
		n.ID = allocs[i].NodeID
		allocs[i].DesiredTransition.Migrate = pointer.Of(true)
		tainted[n.ID] = n
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, tainted, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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
	assertPlaceResultsHavePreviousAllocs(t, 2, r.place)
	// These should not have the reschedule field set
	assertPlacementsAreRescheduled(t, 0, r.place)
}

// Tests the reconciler properly handles draining nodes with allocations while
// scaling up
func TestReconciler_DrainNode_ScaleUp(t *testing.T) {
	ci.Parallel(t)

	// Set desired 15
	job := mock.Job()
	job.TaskGroups[0].Count = 15

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	// Build a map of tainted nodes
	tainted := make(map[string]*structs.Node, 2)
	for i := 0; i < 2; i++ {
		n := mock.DrainNode()
		n.ID = allocs[i].NodeID
		allocs[i].DesiredTransition.Migrate = pointer.Of(true)
		tainted[n.ID] = n
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, tainted, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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
	assertPlaceResultsHavePreviousAllocs(t, 2, r.place)
	// These should not have the reschedule field set
	assertPlacementsAreRescheduled(t, 0, r.place)
}

// Tests the reconciler properly handles draining nodes with allocations while
// scaling down
func TestReconciler_DrainNode_ScaleDown(t *testing.T) {
	ci.Parallel(t)

	// Set desired 8
	job := mock.Job()
	job.TaskGroups[0].Count = 8

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	// Build a map of tainted nodes
	tainted := make(map[string]*structs.Node, 3)
	for i := 0; i < 3; i++ {
		n := mock.DrainNode()
		n.ID = allocs[i].NodeID
		allocs[i].DesiredTransition.Migrate = pointer.Of(true)
		tainted[n.ID] = n
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, tainted, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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
	assertPlaceResultsHavePreviousAllocs(t, 1, r.place)
	// These should not have the reschedule field set
	assertPlacementsAreRescheduled(t, 0, r.place)
}

// Tests the reconciler properly handles a task group being removed
func TestReconciler_RemovedTG(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()

	// Create 10 allocations for a tg that no longer exists
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	oldName := job.TaskGroups[0].Name
	newName := "different"
	job.TaskGroups[0].Name = newName

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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
	ci.Parallel(t)

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
			// Create 10 allocations
			var allocs []*structs.Allocation
			for i := 0; i < 10; i++ {
				alloc := mock.Alloc()
				alloc.Job = c.job
				alloc.JobID = c.jobID
				alloc.NodeID = uuid.Generate()
				alloc.Name = structs.AllocName(c.jobID, c.taskGroup, uint(i))
				alloc.TaskGroup = c.taskGroup
				allocs = append(allocs, alloc)
			}

			reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, c.jobID, c.job,
				nil, allocs, nil, "", 50, true)
			r := reconciler.Compute()

			// Assert the correct results
			assertResults(t, r, &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: nil,
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

// Tests the reconciler doesn't update allocs in terminal state
// when job is stopped or nil
func TestReconciler_JobStopped_TerminalAllocs(t *testing.T) {
	ci.Parallel(t)

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
			// Create 10 terminal allocations
			var allocs []*structs.Allocation
			for i := 0; i < 10; i++ {
				alloc := mock.Alloc()
				alloc.Job = c.job
				alloc.JobID = c.jobID
				alloc.NodeID = uuid.Generate()
				alloc.Name = structs.AllocName(c.jobID, c.taskGroup, uint(i))
				alloc.TaskGroup = c.taskGroup
				if i%2 == 0 {
					alloc.DesiredStatus = structs.AllocDesiredStatusStop
				} else {
					alloc.ClientStatus = structs.AllocClientStatusFailed
				}
				allocs = append(allocs, alloc)
			}

			reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, c.jobID, c.job,
				nil, allocs, nil, "", 50, true)
			r := reconciler.Compute()
			require.Len(t, r.stop, 0)
			// Assert the correct results
			assertResults(t, r, &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: nil,
				place:             0,
				inplace:           0,
				stop:              0,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					c.taskGroup: {},
				},
			})
		})
	}
}

// Tests the reconciler properly handles jobs with multiple task groups
func TestReconciler_MultiTG(t *testing.T) {
	ci.Parallel(t)

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
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
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

// Tests the reconciler properly handles jobs with multiple task groups with
// only one having an update block and a deployment already being created
func TestReconciler_MultiTG_SingleUpdateBlock(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	tg2 := job.TaskGroups[0].Copy()
	tg2.Name = "foo"
	job.TaskGroups = append(job.TaskGroups, tg2)
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create all the allocs
	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		for j := 0; j < 10; j++ {
			alloc := mock.Alloc()
			alloc.Job = job
			alloc.JobID = job.ID
			alloc.NodeID = uuid.Generate()
			alloc.Name = structs.AllocName(job.ID, job.TaskGroups[i].Name, uint(j))
			alloc.TaskGroup = job.TaskGroups[i].Name
			allocs = append(allocs, alloc)
		}
	}

	d := structs.NewDeployment(job, 50)
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		DesiredTotal: 10,
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             0,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Ignore: 10,
			},
			tg2.Name: {
				Ignore: 10,
			},
		},
	})
}

// Tests delayed rescheduling of failed batch allocations
func TestReconciler_RescheduleLater_Batch(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Set desired 4
	job := mock.Job()
	job.TaskGroups[0].Count = 4
	now := time.Now()

	// Set up reschedule policy
	delayDur := 15 * time.Second
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{Attempts: 3, Interval: 24 * time.Hour, Delay: delayDur, DelayFunction: "constant"}
	tgName := job.TaskGroups[0].Name

	// Create 6 existing allocations - 2 running, 1 complete and 3 failed
	var allocs []*structs.Allocation
	for i := 0; i < 6; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}

	// Mark 3 as failed with restart tracking info
	allocs[0].ClientStatus = structs.AllocClientStatusFailed
	allocs[0].NextAllocation = allocs[1].ID
	allocs[1].ClientStatus = structs.AllocClientStatusFailed
	allocs[1].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: allocs[0].ID,
			PrevNodeID:  uuid.Generate(),
		},
	}}
	allocs[1].NextAllocation = allocs[2].ID
	allocs[2].ClientStatus = structs.AllocClientStatusFailed
	allocs[2].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now}}
	allocs[2].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-2 * time.Hour).UTC().UnixNano(),
			PrevAllocID: allocs[0].ID,
			PrevNodeID:  uuid.Generate(),
		},
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: allocs[1].ID,
			PrevNodeID:  uuid.Generate(),
		},
	}}

	// Mark one as complete
	allocs[5].ClientStatus = structs.AllocClientStatusComplete

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, true, job.ID, job,
		nil, allocs, nil, uuid.Generate(), 50, true)
	r := reconciler.Compute()

	// Two reschedule attempts were already made, one more can be made at a future time
	// Verify that the follow up eval has the expected waitUntil time
	evals := r.desiredFollowupEvals[tgName]
	require.NotNil(evals)
	require.Equal(1, len(evals))
	require.Equal(now.Add(delayDur), evals[0].WaitUntil)

	// Alloc 5 should not be replaced because it is terminal
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             0,
		inplace:           0,
		attributeUpdates:  1,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:         0,
				InPlaceUpdate: 0,
				Ignore:        4,
				Stop:          0,
			},
		},
	})
	assertNamesHaveIndexes(t, intRange(2, 2), attributeUpdatesToNames(r.attributeUpdates))

	// Verify that the followup evalID field is set correctly
	var annotated *structs.Allocation
	for _, a := range r.attributeUpdates {
		annotated = a
	}
	require.Equal(evals[0].ID, annotated.FollowupEvalID)
}

// Tests delayed rescheduling of failed batch allocations and batching of allocs
// with fail times that are close together
func TestReconciler_RescheduleLaterWithBatchedEvals_Batch(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Set desired 4
	job := mock.Job()
	job.TaskGroups[0].Count = 10
	now := time.Now()

	// Set up reschedule policy
	delayDur := 15 * time.Second
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{Attempts: 3, Interval: 24 * time.Hour, Delay: delayDur, DelayFunction: "constant"}
	tgName := job.TaskGroups[0].Name

	// Create 10 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}

	// Mark 5 as failed with fail times very close together
	for i := 0; i < 5; i++ {
		allocs[i].ClientStatus = structs.AllocClientStatusFailed
		allocs[i].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
			StartedAt:  now.Add(-1 * time.Hour),
			FinishedAt: now.Add(time.Duration(50*i) * time.Millisecond)}}
	}

	// Mark two more as failed several seconds later
	for i := 5; i < 7; i++ {
		allocs[i].ClientStatus = structs.AllocClientStatusFailed
		allocs[i].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
			StartedAt:  now.Add(-1 * time.Hour),
			FinishedAt: now.Add(10 * time.Second)}}
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, true, job.ID, job,
		nil, allocs, nil, uuid.Generate(), 50, true)
	r := reconciler.Compute()

	// Verify that two follow up evals were created
	evals := r.desiredFollowupEvals[tgName]
	require.NotNil(evals)
	require.Equal(2, len(evals))

	// Verify expected WaitUntil values for both batched evals
	require.Equal(now.Add(delayDur), evals[0].WaitUntil)
	secondBatchDuration := delayDur + 10*time.Second
	require.Equal(now.Add(secondBatchDuration), evals[1].WaitUntil)

	// Alloc 5 should not be replaced because it is terminal
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             0,
		inplace:           0,
		attributeUpdates:  7,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:         0,
				InPlaceUpdate: 0,
				Ignore:        10,
				Stop:          0,
			},
		},
	})
	assertNamesHaveIndexes(t, intRange(0, 6), attributeUpdatesToNames(r.attributeUpdates))

	// Verify that the followup evalID field is set correctly
	for _, alloc := range r.attributeUpdates {
		if allocNameToIndex(alloc.Name) < 5 {
			require.Equal(evals[0].ID, alloc.FollowupEvalID)
		} else if allocNameToIndex(alloc.Name) < 7 {
			require.Equal(evals[1].ID, alloc.FollowupEvalID)
		} else {
			t.Fatalf("Unexpected alloc name in Inplace results %v", alloc.Name)
		}
	}
}

// Tests rescheduling failed batch allocations
func TestReconciler_RescheduleNow_Batch(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	// Set desired 4
	job := mock.Job()
	job.TaskGroups[0].Count = 4
	now := time.Now()
	// Set up reschedule policy
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{Attempts: 3, Interval: 24 * time.Hour, Delay: 5 * time.Second, DelayFunction: "constant"}
	tgName := job.TaskGroups[0].Name
	// Create 6 existing allocations - 2 running, 1 complete and 3 failed
	var allocs []*structs.Allocation
	for i := 0; i < 6; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}
	// Mark 3 as failed with restart tracking info
	allocs[0].ClientStatus = structs.AllocClientStatusFailed
	allocs[0].NextAllocation = allocs[1].ID
	allocs[1].ClientStatus = structs.AllocClientStatusFailed
	allocs[1].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: allocs[0].ID,
			PrevNodeID:  uuid.Generate(),
		},
	}}
	allocs[1].NextAllocation = allocs[2].ID
	allocs[2].ClientStatus = structs.AllocClientStatusFailed
	allocs[2].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-5 * time.Second)}}
	allocs[2].FollowupEvalID = uuid.Generate()
	allocs[2].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-2 * time.Hour).UTC().UnixNano(),
			PrevAllocID: allocs[0].ID,
			PrevNodeID:  uuid.Generate(),
		},
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: allocs[1].ID,
			PrevNodeID:  uuid.Generate(),
		},
	}}
	// Mark one as complete
	allocs[5].ClientStatus = structs.AllocClientStatusComplete

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, true, job.ID, job,
		nil, allocs, nil, "", 50, true)
	reconciler.now = now
	r := reconciler.Compute()

	// Verify that no follow up evals were created
	evals := r.desiredFollowupEvals[tgName]
	require.Nil(evals)

	// Two reschedule attempts were made, one more can be made now
	// Alloc 5 should not be replaced because it is terminal
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		stop:              1,
		inplace:           0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  1,
				Stop:   1,
				Ignore: 3,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(2, 2), placeResultsToNames(r.place))
	assertPlaceResultsHavePreviousAllocs(t, 1, r.place)
	assertPlacementsAreRescheduled(t, 1, r.place)

}

// Tests rescheduling failed service allocations with desired state stop
func TestReconciler_RescheduleLater_Service(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Set up reschedule policy
	delayDur := 15 * time.Second
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{Attempts: 1, Interval: 24 * time.Hour, Delay: delayDur, MaxDelay: 1 * time.Hour}

	// Create 5 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}

	// Mark two as failed
	allocs[0].ClientStatus = structs.AllocClientStatusFailed

	// Mark one of them as already rescheduled once
	allocs[0].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: uuid.Generate(),
			PrevNodeID:  uuid.Generate(),
		},
	}}
	allocs[1].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now}}
	allocs[1].ClientStatus = structs.AllocClientStatusFailed

	// Mark one as desired state stop
	allocs[4].DesiredStatus = structs.AllocDesiredStatusStop

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, uuid.Generate(), 50, true)
	r := reconciler.Compute()

	// Should place a new placement and create a follow up eval for the delayed reschedule
	// Verify that the follow up eval has the expected waitUntil time
	evals := r.desiredFollowupEvals[tgName]
	require.NotNil(evals)
	require.Equal(1, len(evals))
	require.Equal(now.Add(delayDur), evals[0].WaitUntil)

	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		inplace:           0,
		attributeUpdates:  1,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:         1,
				InPlaceUpdate: 0,
				Ignore:        4,
				Stop:          0,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(4, 4), placeResultsToNames(r.place))
	assertNamesHaveIndexes(t, intRange(1, 1), attributeUpdatesToNames(r.attributeUpdates))

	// Verify that the followup evalID field is set correctly
	var annotated *structs.Allocation
	for _, a := range r.attributeUpdates {
		annotated = a
	}
	require.Equal(evals[0].ID, annotated.FollowupEvalID)
}

// Tests service allocations with client status complete
func TestReconciler_Service_ClientStatusComplete(t *testing.T) {
	ci.Parallel(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5

	// Set up reschedule policy
	delayDur := 15 * time.Second
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 1,
		Interval: 24 * time.Hour,
		Delay:    delayDur,
		MaxDelay: 1 * time.Hour,
	}

	// Create 5 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
		alloc.DesiredStatus = structs.AllocDesiredStatusRun
	}

	// Mark one as client status complete
	allocs[4].ClientStatus = structs.AllocClientStatusComplete

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Should place a new placement for the alloc that was marked complete
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:         1,
				InPlaceUpdate: 0,
				Ignore:        4,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(4, 4), placeResultsToNames(r.place))

}

// Tests service job placement with desired stop and client status complete
func TestReconciler_Service_DesiredStop_ClientStatusComplete(t *testing.T) {
	ci.Parallel(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5

	// Set up reschedule policy
	delayDur := 15 * time.Second
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts: 1,
		Interval: 24 * time.Hour,
		Delay:    delayDur,
		MaxDelay: 1 * time.Hour,
	}

	// Create 5 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
		alloc.DesiredStatus = structs.AllocDesiredStatusRun
	}

	// Mark one as failed but with desired status stop
	// Should not trigger rescheduling logic but should trigger a placement
	allocs[4].ClientStatus = structs.AllocClientStatusFailed
	allocs[4].DesiredStatus = structs.AllocDesiredStatusStop

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Should place a new placement for the alloc that was marked stopped
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:         1,
				InPlaceUpdate: 0,
				Ignore:        4,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(4, 4), placeResultsToNames(r.place))

	// Should not have any follow up evals created
	require := require.New(t)
	require.Equal(0, len(r.desiredFollowupEvals))
}

// Tests rescheduling failed service allocations with desired state stop
func TestReconciler_RescheduleNow_Service(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Set up reschedule policy and update block
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      1,
		Interval:      24 * time.Hour,
		Delay:         5 * time.Second,
		DelayFunction: "",
		MaxDelay:      1 * time.Hour,
		Unlimited:     false,
	}
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create 5 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}

	// Mark two as failed
	allocs[0].ClientStatus = structs.AllocClientStatusFailed

	// Mark one of them as already rescheduled once
	allocs[0].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: uuid.Generate(),
			PrevNodeID:  uuid.Generate(),
		},
	}}
	allocs[1].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}
	allocs[1].ClientStatus = structs.AllocClientStatusFailed

	// Mark one as desired state stop
	allocs[4].DesiredStatus = structs.AllocDesiredStatusStop

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Verify that no follow up evals were created
	evals := r.desiredFollowupEvals[tgName]
	require.Nil(evals)

	// Verify that one rescheduled alloc and one replacement for terminal alloc were placed
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             2,
		inplace:           0,
		stop:              1,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  2,
				Ignore: 3,
				Stop:   1,
			},
		},
	})

	// Rescheduled allocs should have previous allocs
	assertNamesHaveIndexes(t, intRange(1, 1, 4, 4), placeResultsToNames(r.place))
	assertPlaceResultsHavePreviousAllocs(t, 1, r.place)
	assertPlacementsAreRescheduled(t, 1, r.place)
}

// Tests rescheduling failed service allocations when there's clock drift (upto a second)
func TestReconciler_RescheduleNow_WithinAllowedTimeWindow(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Set up reschedule policy and update block
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      1,
		Interval:      24 * time.Hour,
		Delay:         5 * time.Second,
		DelayFunction: "",
		MaxDelay:      1 * time.Hour,
		Unlimited:     false,
	}
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create 5 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}

	// Mark one as failed
	allocs[0].ClientStatus = structs.AllocClientStatusFailed

	// Mark one of them as already rescheduled once
	allocs[0].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: uuid.Generate(),
			PrevNodeID:  uuid.Generate(),
		},
	}}
	// Set fail time to 4 seconds ago which falls within the reschedule window
	allocs[1].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-4 * time.Second)}}
	allocs[1].ClientStatus = structs.AllocClientStatusFailed

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	reconciler.now = now
	r := reconciler.Compute()

	// Verify that no follow up evals were created
	evals := r.desiredFollowupEvals[tgName]
	require.Nil(evals)

	// Verify that one rescheduled alloc was placed
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		inplace:           0,
		stop:              1,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  1,
				Stop:   1,
				Ignore: 4,
			},
		},
	})

	// Rescheduled allocs should have previous allocs
	assertNamesHaveIndexes(t, intRange(1, 1), placeResultsToNames(r.place))
	assertPlaceResultsHavePreviousAllocs(t, 1, r.place)
	assertPlacementsAreRescheduled(t, 1, r.place)
}

// Tests rescheduling failed service allocations when the eval ID matches and there's a large clock drift
func TestReconciler_RescheduleNow_EvalIDMatch(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Set up reschedule policy and update block
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      1,
		Interval:      24 * time.Hour,
		Delay:         5 * time.Second,
		DelayFunction: "",
		MaxDelay:      1 * time.Hour,
		Unlimited:     false,
	}
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create 5 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}

	// Mark one as failed
	allocs[0].ClientStatus = structs.AllocClientStatusFailed

	// Mark one of them as already rescheduled once
	allocs[0].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: uuid.Generate(),
			PrevNodeID:  uuid.Generate(),
		},
	}}
	// Set fail time to 5 seconds ago and eval ID
	evalID := uuid.Generate()
	allocs[1].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-5 * time.Second)}}
	allocs[1].ClientStatus = structs.AllocClientStatusFailed
	allocs[1].FollowupEvalID = evalID

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, evalID, 50, true)
	reconciler.now = now.Add(-30 * time.Second)
	r := reconciler.Compute()

	// Verify that no follow up evals were created
	evals := r.desiredFollowupEvals[tgName]
	require.Nil(evals)

	// Verify that one rescheduled alloc was placed
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		stop:              1,
		inplace:           0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  1,
				Stop:   1,
				Ignore: 4,
			},
		},
	})

	// Rescheduled allocs should have previous allocs
	assertNamesHaveIndexes(t, intRange(1, 1), placeResultsToNames(r.place))
	assertPlaceResultsHavePreviousAllocs(t, 1, r.place)
	assertPlacementsAreRescheduled(t, 1, r.place)
}

// Tests rescheduling failed service allocations when there are canaries
func TestReconciler_RescheduleNow_Service_WithCanaries(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Set up reschedule policy and update block
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      1,
		Interval:      24 * time.Hour,
		Delay:         5 * time.Second,
		DelayFunction: "",
		MaxDelay:      1 * time.Hour,
		Unlimited:     false,
	}
	job.TaskGroups[0].Update = canaryUpdate

	job2 := job.Copy()
	job2.Version++

	d := structs.NewDeployment(job2, 50)
	d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	s := &structs.DeploymentState{
		DesiredCanaries: 2,
		DesiredTotal:    5,
	}
	d.TaskGroups[job.TaskGroups[0].Name] = s

	// Create 5 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}

	// Mark three as failed
	allocs[0].ClientStatus = structs.AllocClientStatusFailed

	// Mark one of them as already rescheduled once
	allocs[0].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: uuid.Generate(),
			PrevNodeID:  uuid.Generate(),
		},
	}}
	allocs[1].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}
	allocs[1].ClientStatus = structs.AllocClientStatusFailed

	// Mark one as desired state stop
	allocs[4].ClientStatus = structs.AllocClientStatusFailed

	// Create 2 canary allocations
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.ClientStatus = structs.AllocClientStatusRunning
		alloc.DeploymentID = d.ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Canary:  true,
			Healthy: pointer.Of(false),
		}
		s.PlacedCanaries = append(s.PlacedCanaries, alloc.ID)
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job2,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Verify that no follow up evals were created
	evals := r.desiredFollowupEvals[tgName]
	require.Nil(evals)

	// Verify that one rescheduled alloc and one replacement for terminal alloc were placed
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             2,
		stop:              2,
		inplace:           0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  2,
				Stop:   2,
				Ignore: 5,
			},
		},
	})

	// Rescheduled allocs should have previous allocs
	assertNamesHaveIndexes(t, intRange(1, 1, 4, 4), placeResultsToNames(r.place))
	assertPlaceResultsHavePreviousAllocs(t, 2, r.place)
	assertPlacementsAreRescheduled(t, 2, r.place)
}

// Tests rescheduling failed canary service allocations
func TestReconciler_RescheduleNow_Service_Canaries(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Set up reschedule policy and update block
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Delay:         5 * time.Second,
		DelayFunction: "constant",
		MaxDelay:      1 * time.Hour,
		Unlimited:     true,
	}
	job.TaskGroups[0].Update = canaryUpdate

	job2 := job.Copy()
	job2.Version++

	d := structs.NewDeployment(job2, 50)
	d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	s := &structs.DeploymentState{
		DesiredCanaries: 2,
		DesiredTotal:    5,
	}
	d.TaskGroups[job.TaskGroups[0].Name] = s

	// Create 5 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}

	// Create 2 healthy canary allocations
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.ClientStatus = structs.AllocClientStatusRunning
		alloc.DeploymentID = d.ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Canary:  true,
			Healthy: pointer.Of(false),
		}
		s.PlacedCanaries = append(s.PlacedCanaries, alloc.ID)
		allocs = append(allocs, alloc)
	}

	// Mark the canaries as failed
	allocs[5].ClientStatus = structs.AllocClientStatusFailed
	allocs[5].DesiredTransition.Reschedule = pointer.Of(true)

	// Mark one of them as already rescheduled once
	allocs[5].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: uuid.Generate(),
			PrevNodeID:  uuid.Generate(),
		},
	}}

	allocs[6].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}
	allocs[6].ClientStatus = structs.AllocClientStatusFailed
	allocs[6].DesiredTransition.Reschedule = pointer.Of(true)

	// Create 4 unhealthy canary allocations that have already been replaced
	for i := 0; i < 4; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i%2))
		alloc.ClientStatus = structs.AllocClientStatusFailed
		alloc.DeploymentID = d.ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Canary:  true,
			Healthy: pointer.Of(false),
		}
		s.PlacedCanaries = append(s.PlacedCanaries, alloc.ID)
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job2,
		d, allocs, nil, "", 50, true)
	reconciler.now = now
	r := reconciler.Compute()

	// Verify that no follow up evals were created
	evals := r.desiredFollowupEvals[tgName]
	require.Nil(evals)

	// Verify that one rescheduled alloc and one replacement for terminal alloc were placed
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             2,
		stop:              2,
		inplace:           0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  2,
				Stop:   2,
				Ignore: 9,
			},
		},
	})

	// Rescheduled allocs should have previous allocs
	assertNamesHaveIndexes(t, intRange(0, 1), placeResultsToNames(r.place))
	assertPlaceResultsHavePreviousAllocs(t, 2, r.place)
	assertPlacementsAreRescheduled(t, 2, r.place)
}

// Tests rescheduling failed canary service allocations when one has reached its
// reschedule limit
func TestReconciler_RescheduleNow_Service_Canaries_Limit(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Set up reschedule policy and update block
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      1,
		Interval:      24 * time.Hour,
		Delay:         5 * time.Second,
		DelayFunction: "",
		MaxDelay:      1 * time.Hour,
		Unlimited:     false,
	}
	job.TaskGroups[0].Update = canaryUpdate

	job2 := job.Copy()
	job2.Version++

	d := structs.NewDeployment(job2, 50)
	d.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	s := &structs.DeploymentState{
		DesiredCanaries: 2,
		DesiredTotal:    5,
	}
	d.TaskGroups[job.TaskGroups[0].Name] = s

	// Create 5 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}

	// Create 2 healthy canary allocations
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.ClientStatus = structs.AllocClientStatusRunning
		alloc.DeploymentID = d.ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Canary:  true,
			Healthy: pointer.Of(false),
		}
		s.PlacedCanaries = append(s.PlacedCanaries, alloc.ID)
		allocs = append(allocs, alloc)
	}

	// Mark the canaries as failed
	allocs[5].ClientStatus = structs.AllocClientStatusFailed
	allocs[5].DesiredTransition.Reschedule = pointer.Of(true)

	// Mark one of them as already rescheduled once
	allocs[5].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: now.Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: uuid.Generate(),
			PrevNodeID:  uuid.Generate(),
		},
	}}

	allocs[6].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}
	allocs[6].ClientStatus = structs.AllocClientStatusFailed
	allocs[6].DesiredTransition.Reschedule = pointer.Of(true)

	// Create 4 unhealthy canary allocations that have already been replaced
	for i := 0; i < 4; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i%2))
		alloc.ClientStatus = structs.AllocClientStatusFailed
		alloc.DeploymentID = d.ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Canary:  true,
			Healthy: pointer.Of(false),
		}
		s.PlacedCanaries = append(s.PlacedCanaries, alloc.ID)
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job2,
		d, allocs, nil, "", 50, true)
	reconciler.now = now
	r := reconciler.Compute()

	// Verify that no follow up evals were created
	evals := r.desiredFollowupEvals[tgName]
	require.Nil(evals)

	// Verify that one rescheduled alloc and one replacement for terminal alloc were placed
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		stop:              1,
		inplace:           0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  1,
				Stop:   1,
				Ignore: 10,
			},
		},
	})

	// Rescheduled allocs should have previous allocs
	assertNamesHaveIndexes(t, intRange(1, 1), placeResultsToNames(r.place))
	assertPlaceResultsHavePreviousAllocs(t, 1, r.place)
	assertPlacementsAreRescheduled(t, 1, r.place)
}

// Tests failed service allocations that were already rescheduled won't be rescheduled again
func TestReconciler_DontReschedule_PreviouslyRescheduled(t *testing.T) {
	ci.Parallel(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5

	// Set up reschedule policy
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{Attempts: 5, Interval: 24 * time.Hour}

	// Create 7 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 7; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}
	// Mark two as failed and rescheduled
	allocs[0].ClientStatus = structs.AllocClientStatusFailed
	allocs[0].ID = allocs[1].ID
	allocs[1].ClientStatus = structs.AllocClientStatusFailed
	allocs[1].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: uuid.Generate(),
			PrevNodeID:  uuid.Generate(),
		},
	}}
	allocs[1].NextAllocation = allocs[2].ID

	// Mark one as desired state stop
	allocs[4].DesiredStatus = structs.AllocDesiredStatusStop

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Should place 1 - one is a new placement to make up the desired count of 5
	// failing allocs are not rescheduled
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  1,
				Ignore: 4,
			},
		},
	})

	// name index 0 is used for the replacement because its
	assertNamesHaveIndexes(t, intRange(0, 0), placeResultsToNames(r.place))
}

// Tests the reconciler cancels an old deployment when the job is being stopped
func TestReconciler_CancelDeployment_JobStop(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.Stop = true

	running := structs.NewDeployment(job, 50)
	failed := structs.NewDeployment(job, 50)
	failed.Status = structs.DeploymentStatusFailed

	cases := []struct {
		name             string
		job              *structs.Job
		jobID, taskGroup string
		deployment       *structs.Deployment
		cancel           bool
	}{
		{
			name:       "stopped job, running deployment",
			job:        job,
			jobID:      job.ID,
			taskGroup:  job.TaskGroups[0].Name,
			deployment: running,
			cancel:     true,
		},
		{
			name:       "nil job, running deployment",
			job:        nil,
			jobID:      "foo",
			taskGroup:  "bar",
			deployment: running,
			cancel:     true,
		},
		{
			name:       "stopped job, failed deployment",
			job:        job,
			jobID:      job.ID,
			taskGroup:  job.TaskGroups[0].Name,
			deployment: failed,
			cancel:     false,
		},
		{
			name:       "nil job, failed deployment",
			job:        nil,
			jobID:      "foo",
			taskGroup:  "bar",
			deployment: failed,
			cancel:     false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Create 10 allocations
			var allocs []*structs.Allocation
			for i := 0; i < 10; i++ {
				alloc := mock.Alloc()
				alloc.Job = c.job
				alloc.JobID = c.jobID
				alloc.NodeID = uuid.Generate()
				alloc.Name = structs.AllocName(c.jobID, c.taskGroup, uint(i))
				alloc.TaskGroup = c.taskGroup
				allocs = append(allocs, alloc)
			}

			reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, c.jobID, c.job,
				c.deployment, allocs, nil, "", 50, true)
			r := reconciler.Compute()

			var updates []*structs.DeploymentStatusUpdate
			if c.cancel {
				updates = []*structs.DeploymentStatusUpdate{
					{
						DeploymentID:      c.deployment.ID,
						Status:            structs.DeploymentStatusCancelled,
						StatusDescription: structs.DeploymentStatusDescriptionStoppedJob,
					},
				}
			}

			// Assert the correct results
			assertResults(t, r, &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: updates,
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

// Tests the reconciler cancels an old deployment when the job is updated
func TestReconciler_CancelDeployment_JobUpdate(t *testing.T) {
	ci.Parallel(t)

	// Create a base job
	job := mock.Job()

	// Create two deployments
	running := structs.NewDeployment(job, 50)
	failed := structs.NewDeployment(job, 50)
	failed.Status = structs.DeploymentStatusFailed

	// Make the job newer than the deployment
	job.Version += 10

	cases := []struct {
		name       string
		deployment *structs.Deployment
		cancel     bool
	}{
		{
			name:       "running deployment",
			deployment: running,
			cancel:     true,
		},
		{
			name:       "failed deployment",
			deployment: failed,
			cancel:     false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Create 10 allocations
			var allocs []*structs.Allocation
			for i := 0; i < 10; i++ {
				alloc := mock.Alloc()
				alloc.Job = job
				alloc.JobID = job.ID
				alloc.NodeID = uuid.Generate()
				alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
				alloc.TaskGroup = job.TaskGroups[0].Name
				allocs = append(allocs, alloc)
			}

			reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
				c.deployment, allocs, nil, "", 50, true)
			r := reconciler.Compute()

			var updates []*structs.DeploymentStatusUpdate
			if c.cancel {
				updates = []*structs.DeploymentStatusUpdate{
					{
						DeploymentID:      c.deployment.ID,
						Status:            structs.DeploymentStatusCancelled,
						StatusDescription: structs.DeploymentStatusDescriptionNewerJob,
					},
				}
			}

			// Assert the correct results
			assertResults(t, r, &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: updates,
				place:             0,
				inplace:           0,
				stop:              0,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					job.TaskGroups[0].Name: {
						Ignore: 10,
					},
				},
			})
		})
	}
}

// Tests the reconciler creates a deployment and does a rolling upgrade with
// destructive changes
func TestReconciler_CreateDeployment_RollingUpgrade_Destructive(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create 10 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	d := structs.NewDeployment(job, 50)
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		DesiredTotal: 10,
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  d,
		deploymentUpdates: nil,
		destructive:       4,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				DestructiveUpdate: 4,
				Ignore:            6,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 3), destructiveResultsToNames(r.destructiveUpdate))
}

// Tests the reconciler creates a deployment for inplace updates
func TestReconciler_CreateDeployment_RollingUpgrade_Inplace(t *testing.T) {
	ci.Parallel(t)

	jobOld := mock.Job()
	job := jobOld.Copy()
	job.Version++
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create 10 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = jobOld
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnInplace, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	d := structs.NewDeployment(job, 50)
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		DesiredTotal: 10,
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  d,
		deploymentUpdates: nil,
		place:             0,
		inplace:           10,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				InPlaceUpdate: 10,
			},
		},
	})
}

// Tests the reconciler creates a deployment when the job has a newer create index
func TestReconciler_CreateDeployment_NewerCreateIndex(t *testing.T) {
	ci.Parallel(t)

	jobOld := mock.Job()
	job := jobOld.Copy()
	job.TaskGroups[0].Update = noCanaryUpdate
	job.CreateIndex += 100

	// Create 5 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = jobOld
		alloc.JobID = jobOld.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	d := structs.NewDeployment(job, 50)
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		DesiredTotal: 5,
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  d,
		deploymentUpdates: nil,
		place:             5,
		destructive:       0,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				InPlaceUpdate:     0,
				Ignore:            5,
				Place:             5,
				DestructiveUpdate: 0,
			},
		},
	})
}

// Tests the reconciler doesn't creates a deployment if there are no changes
func TestReconciler_DontCreateDeployment_NoChanges(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create 10 allocations from the job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             0,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				DestructiveUpdate: 0,
				Ignore:            10,
			},
		},
	})
}

// Tests the reconciler doesn't place any more canaries when the deployment is
// paused or failed
func TestReconciler_PausedOrFailedDeployment_NoMoreCanaries(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate

	cases := []struct {
		name             string
		deploymentStatus string
		stop             uint64
	}{
		{
			name:             "paused deployment",
			deploymentStatus: structs.DeploymentStatusPaused,
			stop:             0,
		},
		{
			name:             "failed deployment",
			deploymentStatus: structs.DeploymentStatusFailed,
			stop:             1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Create a deployment that is paused/failed and has placed some canaries
			d := structs.NewDeployment(job, 50)
			d.Status = c.deploymentStatus
			d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
				Promoted:        false,
				DesiredCanaries: 2,
				DesiredTotal:    10,
				PlacedAllocs:    1,
			}

			// Create 10 allocations for the original job
			var allocs []*structs.Allocation
			for i := 0; i < 10; i++ {
				alloc := mock.Alloc()
				alloc.Job = job
				alloc.JobID = job.ID
				alloc.NodeID = uuid.Generate()
				alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
				alloc.TaskGroup = job.TaskGroups[0].Name
				allocs = append(allocs, alloc)
			}

			// Create one canary
			canary := mock.Alloc()
			canary.Job = job
			canary.JobID = job.ID
			canary.NodeID = uuid.Generate()
			canary.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, 0)
			canary.TaskGroup = job.TaskGroups[0].Name
			canary.DeploymentID = d.ID
			allocs = append(allocs, canary)
			d.TaskGroups[canary.TaskGroup].PlacedCanaries = []string{canary.ID}

			mockUpdateFn := allocUpdateFnMock(map[string]allocUpdateType{canary.ID: allocUpdateFnIgnore}, allocUpdateFnDestructive)
			reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, job.ID, job,
				d, allocs, nil, "", 50, true)
			r := reconciler.Compute()

			// Assert the correct results
			assertResults(t, r, &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: nil,
				place:             0,
				inplace:           0,
				stop:              int(c.stop),
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					job.TaskGroups[0].Name: {
						Ignore: 11 - c.stop,
						Stop:   c.stop,
					},
				},
			})
		})
	}
}

// Tests the reconciler doesn't place any more allocs when the deployment is
// paused or failed
func TestReconciler_PausedOrFailedDeployment_NoMorePlacements(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate
	job.TaskGroups[0].Count = 15

	cases := []struct {
		name             string
		deploymentStatus string
	}{
		{
			name:             "paused deployment",
			deploymentStatus: structs.DeploymentStatusPaused,
		},
		{
			name:             "failed deployment",
			deploymentStatus: structs.DeploymentStatusFailed,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Create a deployment that is paused and has placed some canaries
			d := structs.NewDeployment(job, 50)
			d.Status = c.deploymentStatus
			d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
				Promoted:     false,
				DesiredTotal: 15,
				PlacedAllocs: 10,
			}

			// Create 10 allocations for the new job
			var allocs []*structs.Allocation
			for i := 0; i < 10; i++ {
				alloc := mock.Alloc()
				alloc.Job = job
				alloc.JobID = job.ID
				alloc.NodeID = uuid.Generate()
				alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
				alloc.TaskGroup = job.TaskGroups[0].Name
				allocs = append(allocs, alloc)
			}

			reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
				d, allocs, nil, "", 50, true)
			r := reconciler.Compute()

			// Assert the correct results
			assertResults(t, r, &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: nil,
				place:             0,
				inplace:           0,
				stop:              0,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					job.TaskGroups[0].Name: {
						Ignore: 10,
					},
				},
			})
		})
	}
}

// Tests the reconciler doesn't do any more destructive updates when the
// deployment is paused or failed
func TestReconciler_PausedOrFailedDeployment_NoMoreDestructiveUpdates(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate

	cases := []struct {
		name             string
		deploymentStatus string
	}{
		{
			name:             "paused deployment",
			deploymentStatus: structs.DeploymentStatusPaused,
		},
		{
			name:             "failed deployment",
			deploymentStatus: structs.DeploymentStatusFailed,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Create a deployment that is paused and has placed some canaries
			d := structs.NewDeployment(job, 50)
			d.Status = c.deploymentStatus
			d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
				Promoted:     false,
				DesiredTotal: 10,
				PlacedAllocs: 1,
			}

			// Create 9 allocations for the original job
			var allocs []*structs.Allocation
			for i := 1; i < 10; i++ {
				alloc := mock.Alloc()
				alloc.Job = job
				alloc.JobID = job.ID
				alloc.NodeID = uuid.Generate()
				alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
				alloc.TaskGroup = job.TaskGroups[0].Name
				allocs = append(allocs, alloc)
			}

			// Create one for the new job
			newAlloc := mock.Alloc()
			newAlloc.Job = job
			newAlloc.JobID = job.ID
			newAlloc.NodeID = uuid.Generate()
			newAlloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, 0)
			newAlloc.TaskGroup = job.TaskGroups[0].Name
			newAlloc.DeploymentID = d.ID
			allocs = append(allocs, newAlloc)

			mockUpdateFn := allocUpdateFnMock(map[string]allocUpdateType{newAlloc.ID: allocUpdateFnIgnore}, allocUpdateFnDestructive)
			reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, job.ID, job,
				d, allocs, nil, "", 50, true)
			r := reconciler.Compute()

			// Assert the correct results
			assertResults(t, r, &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: nil,
				place:             0,
				inplace:           0,
				stop:              0,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					job.TaskGroups[0].Name: {
						Ignore: 10,
					},
				},
			})
		})
	}
}

// Tests the reconciler handles migrating a canary correctly on a draining node
func TestReconciler_DrainNode_Canary(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate

	// Create a deployment that is paused and has placed some canaries
	d := structs.NewDeployment(job, 50)
	s := &structs.DeploymentState{
		Promoted:        false,
		DesiredTotal:    10,
		DesiredCanaries: 2,
		PlacedAllocs:    2,
	}
	d.TaskGroups[job.TaskGroups[0].Name] = s

	// Create 10 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	// Create two canaries for the new job
	handled := make(map[string]allocUpdateType)
	for i := 0; i < 2; i++ {
		// Create one canary
		canary := mock.Alloc()
		canary.Job = job
		canary.JobID = job.ID
		canary.NodeID = uuid.Generate()
		canary.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		canary.TaskGroup = job.TaskGroups[0].Name
		canary.DeploymentID = d.ID
		s.PlacedCanaries = append(s.PlacedCanaries, canary.ID)
		allocs = append(allocs, canary)
		handled[canary.ID] = allocUpdateFnIgnore
	}

	// Build a map of tainted nodes that contains the last canary
	tainted := make(map[string]*structs.Node, 1)
	n := mock.DrainNode()
	n.ID = allocs[11].NodeID
	allocs[11].DesiredTransition.Migrate = pointer.Of(true)
	tainted[n.ID] = n

	mockUpdateFn := allocUpdateFnMock(handled, allocUpdateFnDestructive)
	reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, job.ID, job,
		d, allocs, tainted, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		inplace:           0,
		stop:              1,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Canary: 1,
				Ignore: 11,
			},
		},
	})
	assertNamesHaveIndexes(t, intRange(1, 1), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(1, 1), placeResultsToNames(r.place))
}

// Tests the reconciler handles migrating a canary correctly on a lost node
func TestReconciler_LostNode_Canary(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate

	// Create a deployment that is paused and has placed some canaries
	d := structs.NewDeployment(job, 50)
	s := &structs.DeploymentState{
		Promoted:        false,
		DesiredTotal:    10,
		DesiredCanaries: 2,
		PlacedAllocs:    2,
	}
	d.TaskGroups[job.TaskGroups[0].Name] = s

	// Create 10 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	// Create two canaries for the new job
	handled := make(map[string]allocUpdateType)
	for i := 0; i < 2; i++ {
		// Create one canary
		canary := mock.Alloc()
		canary.Job = job
		canary.JobID = job.ID
		canary.NodeID = uuid.Generate()
		canary.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		canary.TaskGroup = job.TaskGroups[0].Name
		s.PlacedCanaries = append(s.PlacedCanaries, canary.ID)
		canary.DeploymentID = d.ID
		allocs = append(allocs, canary)
		handled[canary.ID] = allocUpdateFnIgnore
	}

	// Build a map of tainted nodes that contains the last canary
	tainted := make(map[string]*structs.Node, 1)
	n := mock.Node()
	n.ID = allocs[11].NodeID
	n.Status = structs.NodeStatusDown
	tainted[n.ID] = n

	mockUpdateFn := allocUpdateFnMock(handled, allocUpdateFnDestructive)
	reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, job.ID, job,
		d, allocs, tainted, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		inplace:           0,
		stop:              1,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Canary: 1,
				Ignore: 11,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(1, 1), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(1, 1), placeResultsToNames(r.place))
}

// Tests the reconciler handles stopping canaries from older deployments
func TestReconciler_StopOldCanaries(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate

	// Create an old deployment that has placed some canaries
	d := structs.NewDeployment(job, 50)
	s := &structs.DeploymentState{
		Promoted:        false,
		DesiredTotal:    10,
		DesiredCanaries: 2,
		PlacedAllocs:    2,
	}
	d.TaskGroups[job.TaskGroups[0].Name] = s

	// Update the job
	job.Version += 10

	// Create 10 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	// Create canaries
	for i := 0; i < 2; i++ {
		// Create one canary
		canary := mock.Alloc()
		canary.Job = job
		canary.JobID = job.ID
		canary.NodeID = uuid.Generate()
		canary.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		canary.TaskGroup = job.TaskGroups[0].Name
		s.PlacedCanaries = append(s.PlacedCanaries, canary.ID)
		canary.DeploymentID = d.ID
		allocs = append(allocs, canary)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job, d,
		allocs, nil, "", 50, true)
	r := reconciler.Compute()

	newD := structs.NewDeployment(job, 50)
	newD.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	newD.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		DesiredCanaries: 2,
		DesiredTotal:    10,
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment: newD,
		deploymentUpdates: []*structs.DeploymentStatusUpdate{
			{
				DeploymentID:      d.ID,
				Status:            structs.DeploymentStatusCancelled,
				StatusDescription: structs.DeploymentStatusDescriptionNewerJob,
			},
		},
		place:   2,
		inplace: 0,
		stop:    2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Canary: 2,
				Stop:   2,
				Ignore: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1), stopResultsToNames(r.stop))
	assertNamesHaveIndexes(t, intRange(0, 1), placeResultsToNames(r.place))
}

// Tests the reconciler creates new canaries when the job changes
func TestReconciler_NewCanaries(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate

	// Create 10 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	newD := structs.NewDeployment(job, 50)
	newD.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	newD.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		DesiredCanaries: 2,
		DesiredTotal:    10,
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  newD,
		deploymentUpdates: nil,
		place:             2,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Canary: 2,
				Ignore: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1), placeResultsToNames(r.place))
}

// Tests the reconciler creates new canaries when the job changes and the
// canary count is greater than the task group count
func TestReconciler_NewCanaries_CountGreater(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Count = 3
	job.TaskGroups[0].Update = canaryUpdate.Copy()
	job.TaskGroups[0].Update.Canary = 7

	// Create 3 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 3; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	newD := structs.NewDeployment(job, 50)
	newD.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	state := &structs.DeploymentState{
		DesiredCanaries: 7,
		DesiredTotal:    3,
	}
	newD.TaskGroups[job.TaskGroups[0].Name] = state

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  newD,
		deploymentUpdates: nil,
		place:             7,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Canary: 7,
				Ignore: 3,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 2, 3, 6), placeResultsToNames(r.place))
}

// Tests the reconciler creates new canaries when the job changes for multiple
// task groups
func TestReconciler_NewCanaries_MultiTG(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate
	job.TaskGroups = append(job.TaskGroups, job.TaskGroups[0].Copy())
	job.TaskGroups[0].Name = "tg2"

	// Create 10 allocations from the old job for each tg
	var allocs []*structs.Allocation
	for j := 0; j < 2; j++ {
		for i := 0; i < 10; i++ {
			alloc := mock.Alloc()
			alloc.Job = job
			alloc.JobID = job.ID
			alloc.NodeID = uuid.Generate()
			alloc.Name = structs.AllocName(job.ID, job.TaskGroups[j].Name, uint(i))
			alloc.TaskGroup = job.TaskGroups[j].Name
			allocs = append(allocs, alloc)
		}
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	newD := structs.NewDeployment(job, 50)
	newD.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	state := &structs.DeploymentState{
		DesiredCanaries: 2,
		DesiredTotal:    10,
	}
	newD.TaskGroups[job.TaskGroups[0].Name] = state
	newD.TaskGroups[job.TaskGroups[1].Name] = state.Copy()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  newD,
		deploymentUpdates: nil,
		place:             4,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Canary: 2,
				Ignore: 10,
			},
			job.TaskGroups[1].Name: {
				Canary: 2,
				Ignore: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1, 0, 1), placeResultsToNames(r.place))
}

// Tests the reconciler creates new canaries when the job changes and scales up
func TestReconciler_NewCanaries_ScaleUp(t *testing.T) {
	ci.Parallel(t)

	// Scale the job up to 15
	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate
	job.TaskGroups[0].Count = 15

	// Create 10 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	newD := structs.NewDeployment(job, 50)
	newD.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	newD.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		DesiredCanaries: 2,
		DesiredTotal:    15,
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  newD,
		deploymentUpdates: nil,
		place:             2,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Canary: 2,
				Ignore: 10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1), placeResultsToNames(r.place))
}

// Tests the reconciler creates new canaries when the job changes and scales
// down
func TestReconciler_NewCanaries_ScaleDown(t *testing.T) {
	ci.Parallel(t)

	// Scale the job down to 5
	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate
	job.TaskGroups[0].Count = 5

	// Create 10 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	newD := structs.NewDeployment(job, 50)
	newD.StatusDescription = structs.DeploymentStatusDescriptionRunningNeedsPromotion
	newD.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		DesiredCanaries: 2,
		DesiredTotal:    5,
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  newD,
		deploymentUpdates: nil,
		place:             2,
		inplace:           0,
		stop:              5,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Canary: 2,
				Stop:   5,
				Ignore: 5,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1), placeResultsToNames(r.place))
	assertNamesHaveIndexes(t, intRange(5, 9), stopResultsToNames(r.stop))
}

// Tests the reconciler handles filling the names of partially placed canaries
func TestReconciler_NewCanaries_FillNames(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = &structs.UpdateStrategy{
		Canary:          4,
		MaxParallel:     2,
		HealthCheck:     structs.UpdateStrategyHealthCheck_Checks,
		MinHealthyTime:  10 * time.Second,
		HealthyDeadline: 10 * time.Minute,
	}

	// Create an existing deployment that has placed some canaries
	d := structs.NewDeployment(job, 50)
	s := &structs.DeploymentState{
		Promoted:        false,
		DesiredTotal:    10,
		DesiredCanaries: 4,
		PlacedAllocs:    2,
	}
	d.TaskGroups[job.TaskGroups[0].Name] = s

	// Create 10 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	// Create canaries but pick names at the ends
	for i := 0; i < 4; i += 3 {
		// Create one canary
		canary := mock.Alloc()
		canary.Job = job
		canary.JobID = job.ID
		canary.NodeID = uuid.Generate()
		canary.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		canary.TaskGroup = job.TaskGroups[0].Name
		s.PlacedCanaries = append(s.PlacedCanaries, canary.ID)
		canary.DeploymentID = d.ID
		allocs = append(allocs, canary)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             2,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Canary: 2,
				Ignore: 12,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(1, 2), placeResultsToNames(r.place))
}

// Tests the reconciler handles canary promotion by unblocking max_parallel
func TestReconciler_PromoteCanaries_Unblock(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate

	// Create an existing deployment that has placed some canaries and mark them
	// promoted
	d := structs.NewDeployment(job, 50)
	s := &structs.DeploymentState{
		Promoted:        true,
		DesiredTotal:    10,
		DesiredCanaries: 2,
		PlacedAllocs:    2,
	}
	d.TaskGroups[job.TaskGroups[0].Name] = s

	// Create 10 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	// Create the canaries
	handled := make(map[string]allocUpdateType)
	for i := 0; i < 2; i++ {
		// Create one canary
		canary := mock.Alloc()
		canary.Job = job
		canary.JobID = job.ID
		canary.NodeID = uuid.Generate()
		canary.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		canary.TaskGroup = job.TaskGroups[0].Name
		s.PlacedCanaries = append(s.PlacedCanaries, canary.ID)
		canary.DeploymentID = d.ID
		canary.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(true),
		}
		allocs = append(allocs, canary)
		handled[canary.ID] = allocUpdateFnIgnore
	}

	mockUpdateFn := allocUpdateFnMock(handled, allocUpdateFnDestructive)
	reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, job.ID, job,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		destructive:       2,
		stop:              2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Stop:              2,
				DestructiveUpdate: 2,
				Ignore:            8,
			},
		},
	})

	assertNoCanariesStopped(t, d, r.stop)
	assertNamesHaveIndexes(t, intRange(2, 3), destructiveResultsToNames(r.destructiveUpdate))
	assertNamesHaveIndexes(t, intRange(0, 1), stopResultsToNames(r.stop))
}

// Tests the reconciler handles canary promotion when the canary count equals
// the total correctly
func TestReconciler_PromoteCanaries_CanariesEqualCount(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate
	job.TaskGroups[0].Count = 2

	// Create an existing deployment that has placed some canaries and mark them
	// promoted
	d := structs.NewDeployment(job, 50)
	s := &structs.DeploymentState{
		Promoted:        true,
		DesiredTotal:    2,
		DesiredCanaries: 2,
		PlacedAllocs:    2,
		HealthyAllocs:   2,
	}
	d.TaskGroups[job.TaskGroups[0].Name] = s

	// Create 2 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 2; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	// Create the canaries
	handled := make(map[string]allocUpdateType)
	for i := 0; i < 2; i++ {
		// Create one canary
		canary := mock.Alloc()
		canary.Job = job
		canary.JobID = job.ID
		canary.NodeID = uuid.Generate()
		canary.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		canary.TaskGroup = job.TaskGroups[0].Name
		s.PlacedCanaries = append(s.PlacedCanaries, canary.ID)
		canary.DeploymentID = d.ID
		canary.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(true),
		}
		allocs = append(allocs, canary)
		handled[canary.ID] = allocUpdateFnIgnore
	}

	mockUpdateFn := allocUpdateFnMock(handled, allocUpdateFnDestructive)
	reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, job.ID, job,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	updates := []*structs.DeploymentStatusUpdate{
		{
			DeploymentID:      d.ID,
			Status:            structs.DeploymentStatusSuccessful,
			StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
		},
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: updates,
		place:             0,
		inplace:           0,
		stop:              2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Stop:   2,
				Ignore: 2,
			},
		},
	})

	assertNoCanariesStopped(t, d, r.stop)
	assertNamesHaveIndexes(t, intRange(0, 1), stopResultsToNames(r.stop))
}

// Tests the reconciler checks the health of placed allocs to determine the
// limit
func TestReconciler_DeploymentLimit_HealthAccounting(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate

	cases := []struct {
		healthy int
	}{
		{
			healthy: 0,
		},
		{
			healthy: 1,
		},
		{
			healthy: 2,
		},
		{
			healthy: 3,
		},
		{
			healthy: 4,
		},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%d healthy", c.healthy), func(t *testing.T) {
			// Create an existing deployment that has placed some canaries and mark them
			// promoted
			d := structs.NewDeployment(job, 50)
			d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
				Promoted:     true,
				DesiredTotal: 10,
				PlacedAllocs: 4,
			}

			// Create 6 allocations from the old job
			var allocs []*structs.Allocation
			for i := 4; i < 10; i++ {
				alloc := mock.Alloc()
				alloc.Job = job
				alloc.JobID = job.ID
				alloc.NodeID = uuid.Generate()
				alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
				alloc.TaskGroup = job.TaskGroups[0].Name
				allocs = append(allocs, alloc)
			}

			// Create the new allocs
			handled := make(map[string]allocUpdateType)
			for i := 0; i < 4; i++ {
				new := mock.Alloc()
				new.Job = job
				new.JobID = job.ID
				new.NodeID = uuid.Generate()
				new.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
				new.TaskGroup = job.TaskGroups[0].Name
				new.DeploymentID = d.ID
				if i < c.healthy {
					new.DeploymentStatus = &structs.AllocDeploymentStatus{
						Healthy: pointer.Of(true),
					}
				}
				allocs = append(allocs, new)
				handled[new.ID] = allocUpdateFnIgnore
			}

			mockUpdateFn := allocUpdateFnMock(handled, allocUpdateFnDestructive)
			reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, job.ID, job,
				d, allocs, nil, "", 50, true)
			r := reconciler.Compute()

			// Assert the correct results
			assertResults(t, r, &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: nil,
				destructive:       c.healthy,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					job.TaskGroups[0].Name: {
						DestructiveUpdate: uint64(c.healthy),
						Ignore:            uint64(10 - c.healthy),
					},
				},
			})

			if c.healthy != 0 {
				assertNamesHaveIndexes(t, intRange(4, 3+c.healthy), destructiveResultsToNames(r.destructiveUpdate))
			}
		})
	}
}

// Tests the reconciler handles an alloc on a tainted node during a rolling
// update
func TestReconciler_TaintedNode_RollingUpgrade(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create an existing deployment that has some placed allocs
	d := structs.NewDeployment(job, 50)
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		Promoted:     true,
		DesiredTotal: 10,
		PlacedAllocs: 7,
	}

	// Create 2 allocations from the old job
	var allocs []*structs.Allocation
	for i := 8; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	// Create the healthy replacements
	handled := make(map[string]allocUpdateType)
	for i := 0; i < 8; i++ {
		new := mock.Alloc()
		new.Job = job
		new.JobID = job.ID
		new.NodeID = uuid.Generate()
		new.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		new.TaskGroup = job.TaskGroups[0].Name
		new.DeploymentID = d.ID
		new.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(true),
		}
		allocs = append(allocs, new)
		handled[new.ID] = allocUpdateFnIgnore
	}

	// Build a map of tainted nodes
	tainted := make(map[string]*structs.Node, 3)
	for i := 0; i < 3; i++ {
		n := mock.Node()
		n.ID = allocs[2+i].NodeID
		if i == 0 {
			n.Status = structs.NodeStatusDown
		} else {
			n.DrainStrategy = mock.DrainNode().DrainStrategy
			allocs[2+i].DesiredTransition.Migrate = pointer.Of(true)
		}
		tainted[n.ID] = n
	}

	mockUpdateFn := allocUpdateFnMock(handled, allocUpdateFnDestructive)
	reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, job.ID, job,
		d, allocs, tainted, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             3,
		destructive:       2,
		stop:              3,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:             1, // Place the lost
				Stop:              1, // Stop the lost
				Migrate:           2, // Migrate the tainted
				DestructiveUpdate: 2,
				Ignore:            5,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(8, 9), destructiveResultsToNames(r.destructiveUpdate))
	assertNamesHaveIndexes(t, intRange(0, 2), placeResultsToNames(r.place))
	assertNamesHaveIndexes(t, intRange(0, 2), stopResultsToNames(r.stop))
}

// Tests the reconciler handles a failed deployment with allocs on tainted
// nodes
func TestReconciler_FailedDeployment_TaintedNodes(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create an existing failed deployment that has some placed allocs
	d := structs.NewDeployment(job, 50)
	d.Status = structs.DeploymentStatusFailed
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		Promoted:     true,
		DesiredTotal: 10,
		PlacedAllocs: 4,
	}

	// Create 6 allocations from the old job
	var allocs []*structs.Allocation
	for i := 4; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	// Create the healthy replacements
	handled := make(map[string]allocUpdateType)
	for i := 0; i < 4; i++ {
		new := mock.Alloc()
		new.Job = job
		new.JobID = job.ID
		new.NodeID = uuid.Generate()
		new.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		new.TaskGroup = job.TaskGroups[0].Name
		new.DeploymentID = d.ID
		new.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(true),
		}
		allocs = append(allocs, new)
		handled[new.ID] = allocUpdateFnIgnore
	}

	// Build a map of tainted nodes
	tainted := make(map[string]*structs.Node, 2)
	for i := 0; i < 2; i++ {
		n := mock.Node()
		n.ID = allocs[6+i].NodeID
		if i == 0 {
			n.Status = structs.NodeStatusDown
		} else {
			n.DrainStrategy = mock.DrainNode().DrainStrategy
			allocs[6+i].DesiredTransition.Migrate = pointer.Of(true)
		}
		tainted[n.ID] = n
	}

	mockUpdateFn := allocUpdateFnMock(handled, allocUpdateFnDestructive)
	reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, job.ID, job,
		d, allocs, tainted, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             2,
		inplace:           0,
		stop:              2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:   1,
				Migrate: 1,
				Stop:    1,
				Ignore:  8,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1), placeResultsToNames(r.place))
	assertNamesHaveIndexes(t, intRange(0, 1), stopResultsToNames(r.stop))
}

// Tests the reconciler handles a run after a deployment is complete
// successfully.
func TestReconciler_CompleteDeployment(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate

	d := structs.NewDeployment(job, 50)
	d.Status = structs.DeploymentStatusSuccessful
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		Promoted:        true,
		DesiredTotal:    10,
		DesiredCanaries: 2,
		PlacedAllocs:    10,
		HealthyAllocs:   10,
	}

	// Create allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		alloc.DeploymentID = d.ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(true),
		}
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             0,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Ignore: 10,
			},
		},
	})
}

// Tests that the reconciler marks a deployment as complete once there is
// nothing left to place even if there are failed allocations that are part of
// the deployment.
func TestReconciler_MarkDeploymentComplete_FailedAllocations(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate

	d := structs.NewDeployment(job, 50)
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		DesiredTotal:  10,
		PlacedAllocs:  20,
		HealthyAllocs: 10,
	}

	// Create 10 healthy allocs and 10 allocs that are failed
	var allocs []*structs.Allocation
	for i := 0; i < 20; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i%10))
		alloc.TaskGroup = job.TaskGroups[0].Name
		alloc.DeploymentID = d.ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{}
		if i < 10 {
			alloc.ClientStatus = structs.AllocClientStatusRunning
			alloc.DeploymentStatus.Healthy = pointer.Of(true)
		} else {
			alloc.DesiredStatus = structs.AllocDesiredStatusStop
			alloc.ClientStatus = structs.AllocClientStatusFailed
			alloc.DeploymentStatus.Healthy = pointer.Of(false)
		}

		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID,
		job, d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	updates := []*structs.DeploymentStatusUpdate{
		{
			DeploymentID:      d.ID,
			Status:            structs.DeploymentStatusSuccessful,
			StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
		},
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: updates,
		place:             0,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Ignore: 10,
			},
		},
	})
}

// Test that a failed deployment cancels non-promoted canaries
func TestReconciler_FailedDeployment_CancelCanaries(t *testing.T) {
	ci.Parallel(t)

	// Create a job with two task groups
	job := mock.Job()
	job.TaskGroups[0].Update = canaryUpdate
	job.TaskGroups = append(job.TaskGroups, job.TaskGroups[0].Copy())
	job.TaskGroups[1].Name = "two"

	// Create an existing failed deployment that has promoted one task group
	d := structs.NewDeployment(job, 50)
	d.Status = structs.DeploymentStatusFailed
	s0 := &structs.DeploymentState{
		Promoted:        true,
		DesiredTotal:    10,
		DesiredCanaries: 2,
		PlacedAllocs:    4,
	}
	s1 := &structs.DeploymentState{
		Promoted:        false,
		DesiredTotal:    10,
		DesiredCanaries: 2,
		PlacedAllocs:    2,
	}
	d.TaskGroups[job.TaskGroups[0].Name] = s0
	d.TaskGroups[job.TaskGroups[1].Name] = s1

	// Create 6 allocations from the old job
	var allocs []*structs.Allocation
	handled := make(map[string]allocUpdateType)
	for _, group := range []int{0, 1} {
		replacements := 4
		state := s0
		if group == 1 {
			replacements = 2
			state = s1
		}

		// Create the healthy replacements
		for i := 0; i < replacements; i++ {
			new := mock.Alloc()
			new.Job = job
			new.JobID = job.ID
			new.NodeID = uuid.Generate()
			new.Name = structs.AllocName(job.ID, job.TaskGroups[group].Name, uint(i))
			new.TaskGroup = job.TaskGroups[group].Name
			new.DeploymentID = d.ID
			new.DeploymentStatus = &structs.AllocDeploymentStatus{
				Healthy: pointer.Of(true),
			}
			allocs = append(allocs, new)
			handled[new.ID] = allocUpdateFnIgnore

			// Add the alloc to the canary list
			if i < 2 {
				state.PlacedCanaries = append(state.PlacedCanaries, new.ID)
			}
		}
		for i := replacements; i < 10; i++ {
			alloc := mock.Alloc()
			alloc.Job = job
			alloc.JobID = job.ID
			alloc.NodeID = uuid.Generate()
			alloc.Name = structs.AllocName(job.ID, job.TaskGroups[group].Name, uint(i))
			alloc.TaskGroup = job.TaskGroups[group].Name
			allocs = append(allocs, alloc)
		}
	}

	mockUpdateFn := allocUpdateFnMock(handled, allocUpdateFnDestructive)
	reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, job.ID, job,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             0,
		inplace:           0,
		stop:              2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Ignore: 10,
			},
			job.TaskGroups[1].Name: {
				Stop:   2,
				Ignore: 8,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 1), stopResultsToNames(r.stop))
}

// Test that a failed deployment and updated job works
func TestReconciler_FailedDeployment_NewJob(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create an existing failed deployment that has some placed allocs
	d := structs.NewDeployment(job, 50)
	d.Status = structs.DeploymentStatusFailed
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		Promoted:     true,
		DesiredTotal: 10,
		PlacedAllocs: 4,
	}

	// Create 6 allocations from the old job
	var allocs []*structs.Allocation
	for i := 4; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	// Create the healthy replacements
	for i := 0; i < 4; i++ {
		new := mock.Alloc()
		new.Job = job
		new.JobID = job.ID
		new.NodeID = uuid.Generate()
		new.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		new.TaskGroup = job.TaskGroups[0].Name
		new.DeploymentID = d.ID
		new.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(true),
		}
		allocs = append(allocs, new)
	}

	// Up the job version
	jobNew := job.Copy()
	jobNew.Version += 100

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, jobNew,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	dnew := structs.NewDeployment(jobNew, 50)
	dnew.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		DesiredTotal: 10,
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  dnew,
		deploymentUpdates: nil,
		destructive:       4,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				DestructiveUpdate: 4,
				Ignore:            6,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 3), destructiveResultsToNames(r.destructiveUpdate))
}

// Tests the reconciler marks a deployment as complete
func TestReconciler_MarkDeploymentComplete(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate

	d := structs.NewDeployment(job, 50)
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		Promoted:      true,
		DesiredTotal:  10,
		PlacedAllocs:  10,
		HealthyAllocs: 10,
	}

	// Create allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		alloc.DeploymentID = d.ID
		alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(true),
		}
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	updates := []*structs.DeploymentStatusUpdate{
		{
			DeploymentID:      d.ID,
			Status:            structs.DeploymentStatusSuccessful,
			StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
		},
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: updates,
		place:             0,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Ignore: 10,
			},
		},
	})
}

// Tests the reconciler handles changing a job such that a deployment is created
// while doing a scale up but as the second eval.
func TestReconciler_JobChange_ScaleUp_SecondEval(t *testing.T) {
	ci.Parallel(t)

	// Scale the job up to 15
	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate
	job.TaskGroups[0].Count = 30

	// Create a deployment that is paused and has placed some canaries
	d := structs.NewDeployment(job, 50)
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		Promoted:     false,
		DesiredTotal: 30,
		PlacedAllocs: 20,
	}

	// Create 10 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	// Create 20 from new job
	handled := make(map[string]allocUpdateType)
	for i := 10; i < 30; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.DeploymentID = d.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
		handled[alloc.ID] = allocUpdateFnIgnore
	}

	mockUpdateFn := allocUpdateFnMock(handled, allocUpdateFnDestructive)
	reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, job.ID, job,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				// All should be ignored because nothing has been marked as
				// healthy.
				Ignore: 30,
			},
		},
	})
}

// Tests the reconciler doesn't stop allocations when doing a rolling upgrade
// where the count of the old job allocs is < desired count.
func TestReconciler_RollingUpgrade_MissingAllocs(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create 7 allocations from the old job
	var allocs []*structs.Allocation
	for i := 0; i < 7; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	d := structs.NewDeployment(job, 50)
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		DesiredTotal: 10,
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  d,
		deploymentUpdates: nil,
		place:             3,
		destructive:       1,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:             3,
				DestructiveUpdate: 1,
				Ignore:            6,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(7, 9), placeResultsToNames(r.place))
	assertNamesHaveIndexes(t, intRange(0, 0), destructiveResultsToNames(r.destructiveUpdate))
}

// Tests that the reconciler handles rerunning a batch job in the case that the
// allocations are from an older instance of the job.
func TestReconciler_Batch_Rerun(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.Type = structs.JobTypeBatch
	job.TaskGroups[0].Update = nil

	// Create 10 allocations from the old job and have them be complete
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		alloc.ClientStatus = structs.AllocClientStatusComplete
		alloc.DesiredStatus = structs.AllocDesiredStatusStop
		allocs = append(allocs, alloc)
	}

	// Create a copy of the job that is "new"
	job2 := job.Copy()
	job2.CreateIndex++

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, true, job2.ID, job2,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             10,
		destructive:       0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:             10,
				DestructiveUpdate: 0,
				Ignore:            10,
			},
		},
	})

	assertNamesHaveIndexes(t, intRange(0, 9), placeResultsToNames(r.place))
}

// Test that a failed deployment will not result in rescheduling failed allocations
func TestReconciler_FailedDeployment_DontReschedule(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate

	tgName := job.TaskGroups[0].Name
	now := time.Now()
	// Create an existing failed deployment that has some placed allocs
	d := structs.NewDeployment(job, 50)
	d.Status = structs.DeploymentStatusFailed
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		Promoted:     true,
		DesiredTotal: 5,
		PlacedAllocs: 4,
	}

	// Create 4 allocations and mark two as failed
	var allocs []*structs.Allocation
	for i := 0; i < 4; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		alloc.DeploymentID = d.ID
		allocs = append(allocs, alloc)
	}

	//create some allocations that are reschedulable now
	allocs[2].ClientStatus = structs.AllocClientStatusFailed
	allocs[2].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}

	allocs[3].ClientStatus = structs.AllocClientStatusFailed
	allocs[3].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert that no rescheduled placements were created
	assertResults(t, r, &resultExpectation{
		place:             0,
		createDeployment:  nil,
		deploymentUpdates: nil,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Ignore: 2,
			},
		},
	})
}

// Test that a running deployment with failed allocs will not result in
// rescheduling failed allocations unless they are marked as reschedulable.
func TestReconciler_DeploymentWithFailedAllocs_DontReschedule(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Mock deployment with failed allocs, but deployment watcher hasn't marked it as failed yet
	d := structs.NewDeployment(job, 50)
	d.Status = structs.DeploymentStatusRunning
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		Promoted:     false,
		DesiredTotal: 10,
		PlacedAllocs: 10,
	}

	// Create 10 allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		alloc.DeploymentID = d.ID
		alloc.ClientStatus = structs.AllocClientStatusFailed
		alloc.TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
			StartedAt:  now.Add(-1 * time.Hour),
			FinishedAt: now.Add(-10 * time.Second)}}
		allocs = append(allocs, alloc)
	}

	// Mark half of them as reschedulable
	for i := 0; i < 5; i++ {
		allocs[i].DesiredTransition.Reschedule = pointer.Of(true)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert that no rescheduled placements were created
	assertResults(t, r, &resultExpectation{
		place:             5,
		stop:              5,
		createDeployment:  nil,
		deploymentUpdates: nil,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  5,
				Stop:   5,
				Ignore: 5,
			},
		},
	})
}

// Test that a failed deployment cancels non-promoted canaries
func TestReconciler_FailedDeployment_AutoRevert_CancelCanaries(t *testing.T) {
	ci.Parallel(t)

	// Create a job
	job := mock.Job()
	job.TaskGroups[0].Count = 3
	job.TaskGroups[0].Update = &structs.UpdateStrategy{
		Canary:          3,
		MaxParallel:     2,
		HealthCheck:     structs.UpdateStrategyHealthCheck_Checks,
		MinHealthyTime:  10 * time.Second,
		HealthyDeadline: 10 * time.Minute,
		Stagger:         31 * time.Second,
	}

	// Create v1 of the job
	jobv1 := job.Copy()
	jobv1.Version = 1
	jobv1.TaskGroups[0].Meta = map[string]string{"version": "1"}

	// Create v2 of the job
	jobv2 := job.Copy()
	jobv2.Version = 2
	jobv2.TaskGroups[0].Meta = map[string]string{"version": "2"}

	d := structs.NewDeployment(jobv2, 50)
	state := &structs.DeploymentState{
		Promoted:      true,
		DesiredTotal:  3,
		PlacedAllocs:  3,
		HealthyAllocs: 3,
	}
	d.TaskGroups[job.TaskGroups[0].Name] = state

	// Create the original
	var allocs []*structs.Allocation
	for i := 0; i < 3; i++ {
		new := mock.Alloc()
		new.Job = jobv2
		new.JobID = job.ID
		new.NodeID = uuid.Generate()
		new.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		new.TaskGroup = job.TaskGroups[0].Name
		new.DeploymentID = d.ID
		new.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(true),
		}
		new.ClientStatus = structs.AllocClientStatusRunning
		allocs = append(allocs, new)

	}
	for i := 0; i < 3; i++ {
		new := mock.Alloc()
		new.Job = jobv1
		new.JobID = jobv1.ID
		new.NodeID = uuid.Generate()
		new.Name = structs.AllocName(jobv1.ID, jobv1.TaskGroups[0].Name, uint(i))
		new.TaskGroup = job.TaskGroups[0].Name
		new.DeploymentID = uuid.Generate()
		new.DeploymentStatus = &structs.AllocDeploymentStatus{
			Healthy: pointer.Of(false),
		}
		new.DesiredStatus = structs.AllocDesiredStatusStop
		new.ClientStatus = structs.AllocClientStatusFailed
		allocs = append(allocs, new)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, jobv2,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	updates := []*structs.DeploymentStatusUpdate{
		{
			DeploymentID:      d.ID,
			Status:            structs.DeploymentStatusSuccessful,
			StatusDescription: structs.DeploymentStatusDescriptionSuccessful,
		},
	}

	// Assert the correct results
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: updates,
		place:             0,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Stop:          0,
				InPlaceUpdate: 0,
				Ignore:        3,
			},
		},
	})
}

// Test that a successful deployment with failed allocs will result in
// rescheduling failed allocations
func TestReconciler_SuccessfulDeploymentWithFailedAllocs_Reschedule(t *testing.T) {
	ci.Parallel(t)

	job := mock.Job()
	job.TaskGroups[0].Update = noCanaryUpdate
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Mock deployment with failed allocs, but deployment watcher hasn't marked it as failed yet
	d := structs.NewDeployment(job, 50)
	d.Status = structs.DeploymentStatusSuccessful
	d.TaskGroups[job.TaskGroups[0].Name] = &structs.DeploymentState{
		Promoted:     false,
		DesiredTotal: 10,
		PlacedAllocs: 10,
	}

	// Create 10 allocations
	var allocs []*structs.Allocation
	for i := 0; i < 10; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		alloc.TaskGroup = job.TaskGroups[0].Name
		alloc.DeploymentID = d.ID
		alloc.ClientStatus = structs.AllocClientStatusFailed
		alloc.TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
			StartedAt:  now.Add(-1 * time.Hour),
			FinishedAt: now.Add(-10 * time.Second)}}
		allocs = append(allocs, alloc)
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnDestructive, false, job.ID, job,
		d, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Assert that rescheduled placements were created
	assertResults(t, r, &resultExpectation{
		place:             10,
		stop:              10,
		createDeployment:  nil,
		deploymentUpdates: nil,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  10,
				Stop:   10,
				Ignore: 0,
			},
		},
	})
	assertPlaceResultsHavePreviousAllocs(t, 10, r.place)
}

// Tests force rescheduling a failed alloc that is past its reschedule limit
func TestReconciler_ForceReschedule_Service(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5
	tgName := job.TaskGroups[0].Name

	// Set up reschedule policy and update block
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      1,
		Interval:      24 * time.Hour,
		Delay:         5 * time.Second,
		DelayFunction: "",
		MaxDelay:      1 * time.Hour,
		Unlimited:     false,
	}
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create 5 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}

	// Mark one as failed and past its reschedule limit so not eligible to reschedule
	allocs[0].ClientStatus = structs.AllocClientStatusFailed
	allocs[0].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: uuid.Generate(),
			PrevNodeID:  uuid.Generate(),
		},
	}}

	// Mark DesiredTransition ForceReschedule
	allocs[0].DesiredTransition = structs.DesiredTransition{ForceReschedule: pointer.Of(true)}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Verify that no follow up evals were created
	evals := r.desiredFollowupEvals[tgName]
	require.Nil(evals)

	// Verify that one rescheduled alloc was created because of the forced reschedule
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		stop:              1,
		inplace:           0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  1,
				Stop:   1,
				Ignore: 4,
			},
		},
	})

	// Rescheduled allocs should have previous allocs
	assertNamesHaveIndexes(t, intRange(0, 0), placeResultsToNames(r.place))
	assertPlaceResultsHavePreviousAllocs(t, 1, r.place)
	assertPlacementsAreRescheduled(t, 1, r.place)
}

// Tests behavior of service failure with rescheduling policy preventing rescheduling:
// new allocs should be placed to satisfy the job count, and current allocations are
// left unmodified
func TestReconciler_RescheduleNot_Service(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)

	// Set desired 5
	job := mock.Job()
	job.TaskGroups[0].Count = 5
	tgName := job.TaskGroups[0].Name
	now := time.Now()

	// Set up reschedule policy and update block
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      0,
		Interval:      24 * time.Hour,
		Delay:         5 * time.Second,
		DelayFunction: "",
		MaxDelay:      1 * time.Hour,
		Unlimited:     false,
	}
	job.TaskGroups[0].Update = noCanaryUpdate

	// Create 5 existing allocations
	var allocs []*structs.Allocation
	for i := 0; i < 5; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}

	// Mark two as failed
	allocs[0].ClientStatus = structs.AllocClientStatusFailed

	// Mark one of them as already rescheduled once
	allocs[0].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: uuid.Generate(),
			PrevNodeID:  uuid.Generate(),
		},
	}}
	allocs[1].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-10 * time.Second)}}
	allocs[1].ClientStatus = structs.AllocClientStatusFailed

	// Mark one as desired state stop
	allocs[4].DesiredStatus = structs.AllocDesiredStatusStop

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	r := reconciler.Compute()

	// Verify that no follow up evals were created
	evals := r.desiredFollowupEvals[tgName]
	require.Nil(evals)

	// no rescheduling, ignore all 4 allocs
	// but place one to substitute allocs[4] that was stopped explicitly
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             1,
		inplace:           0,
		stop:              0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  1,
				Ignore: 4,
				Stop:   0,
			},
		},
	})

	// none of the placement should have preallocs or rescheduled
	assertPlaceResultsHavePreviousAllocs(t, 0, r.place)
	assertPlacementsAreRescheduled(t, 0, r.place)
}

// Tests behavior of batch failure with rescheduling policy preventing rescheduling:
// current allocations are left unmodified and no follow up
func TestReconciler_RescheduleNot_Batch(t *testing.T) {
	ci.Parallel(t)

	require := require.New(t)
	// Set desired 4
	job := mock.Job()
	job.TaskGroups[0].Count = 4
	now := time.Now()
	// Set up reschedule policy
	job.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Attempts:      0,
		Interval:      24 * time.Hour,
		Delay:         5 * time.Second,
		DelayFunction: "constant",
	}
	tgName := job.TaskGroups[0].Name
	// Create 6 existing allocations - 2 running, 1 complete and 3 failed
	var allocs []*structs.Allocation
	for i := 0; i < 6; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.NodeID = uuid.Generate()
		alloc.Name = structs.AllocName(job.ID, job.TaskGroups[0].Name, uint(i))
		allocs = append(allocs, alloc)
		alloc.ClientStatus = structs.AllocClientStatusRunning
	}
	// Mark 3 as failed with restart tracking info
	allocs[0].ClientStatus = structs.AllocClientStatusFailed
	allocs[0].NextAllocation = allocs[1].ID
	allocs[1].ClientStatus = structs.AllocClientStatusFailed
	allocs[1].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: allocs[0].ID,
			PrevNodeID:  uuid.Generate(),
		},
	}}
	allocs[1].NextAllocation = allocs[2].ID
	allocs[2].ClientStatus = structs.AllocClientStatusFailed
	allocs[2].TaskStates = map[string]*structs.TaskState{tgName: {State: "start",
		StartedAt:  now.Add(-1 * time.Hour),
		FinishedAt: now.Add(-5 * time.Second)}}
	allocs[2].FollowupEvalID = uuid.Generate()
	allocs[2].RescheduleTracker = &structs.RescheduleTracker{Events: []*structs.RescheduleEvent{
		{RescheduleTime: time.Now().Add(-2 * time.Hour).UTC().UnixNano(),
			PrevAllocID: allocs[0].ID,
			PrevNodeID:  uuid.Generate(),
		},
		{RescheduleTime: time.Now().Add(-1 * time.Hour).UTC().UnixNano(),
			PrevAllocID: allocs[1].ID,
			PrevNodeID:  uuid.Generate(),
		},
	}}
	// Mark one as complete
	allocs[5].ClientStatus = structs.AllocClientStatusComplete

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, true, job.ID, job,
		nil, allocs, nil, "", 50, true)
	reconciler.now = now
	r := reconciler.Compute()

	// Verify that no follow up evals were created
	evals := r.desiredFollowupEvals[tgName]
	require.Nil(evals)

	// No reschedule attempts were made and all allocs are untouched
	assertResults(t, r, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             0,
		stop:              0,
		inplace:           0,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:  0,
				Stop:   0,
				Ignore: 4,
			},
		},
	})
}

// Tests that when a node disconnects running allocations are queued to transition to unknown.
func TestReconciler_Node_Disconnect_Updates_Alloc_To_Unknown(t *testing.T) {
	job, allocs := buildResumableAllocations(3, structs.AllocClientStatusRunning, structs.AllocDesiredStatusRun, 2)
	// Build a map of disconnected nodes
	nodes := buildDisconnectedNodes(allocs, 2)

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job,
		nil, allocs, nodes, "", 50, true)
	reconciler.now = time.Now().UTC()
	results := reconciler.Compute()

	// Verify that 1 follow up eval was created with the values we expect.
	evals := results.desiredFollowupEvals[job.TaskGroups[0].Name]
	require.Len(t, evals, 1)
	expectedTime := reconciler.now.Add(5 * time.Minute)

	eval := evals[0]
	require.NotNil(t, eval.WaitUntil)
	require.Equal(t, expectedTime, eval.WaitUntil)

	// Validate that the queued disconnectUpdates have the right client status,
	// and that they have a valid FollowUpdEvalID.
	for _, disconnectUpdate := range results.disconnectUpdates {
		require.Equal(t, structs.AllocClientStatusUnknown, disconnectUpdate.ClientStatus)
		require.NotEmpty(t, disconnectUpdate.FollowupEvalID)
		require.Equal(t, eval.ID, disconnectUpdate.FollowupEvalID)
	}

	// 2 to place, 2 to update, 1 to ignore
	assertResults(t, results, &resultExpectation{
		createDeployment:  nil,
		deploymentUpdates: nil,
		place:             2,
		stop:              0,
		inplace:           0,
		disconnectUpdates: 2,

		// 2 to place and 1 to ignore
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				Place:         2,
				Stop:          0,
				Ignore:        1,
				InPlaceUpdate: 0,
			},
		},
	})
}

func TestReconciler_Disconnect_UpdateJobAfterReconnect(t *testing.T) {
	ci.Parallel(t)

	// Create 2 allocs and simulate one have being previously disconnected and
	// then reconnected.
	job, allocs := buildResumableAllocations(2, structs.AllocClientStatusRunning, structs.AllocDesiredStatusRun, 2)
	allocs[0].AllocStates = []*structs.AllocState{
		{
			Field: structs.AllocStateFieldClientStatus,
			Value: structs.AllocClientStatusUnknown,
			Time:  time.Now().Add(-5 * time.Minute),
		},
		{
			Field: structs.AllocStateFieldClientStatus,
			Value: structs.AllocClientStatusRunning,
			Time:  time.Now(),
		},
	}

	reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnInplace, false, job.ID, job,
		nil, allocs, nil, "", 50, true)
	results := reconciler.Compute()

	// Assert both allocations will be updated.
	assertResults(t, results, &resultExpectation{
		inplace: 2,
		desiredTGUpdates: map[string]*structs.DesiredUpdates{
			job.TaskGroups[0].Name: {
				InPlaceUpdate: 2,
			},
		},
	})
}

// Tests that when a node disconnects/reconnects allocations for that node are
// reconciled according to the business rules.
func TestReconciler_Disconnected_Client(t *testing.T) {
	disconnectAllocState := []*structs.AllocState{{
		Field: structs.AllocStateFieldClientStatus,
		Value: structs.AllocClientStatusUnknown,
		Time:  time.Now(),
	}}

	type testCase struct {
		name                         string
		allocCount                   int
		disconnectedAllocCount       int
		jobVersionIncrement          uint64
		nodeScoreIncrement           float64
		disconnectedAllocStatus      string
		disconnectedAllocStates      []*structs.AllocState
		serverDesiredStatus          string
		isBatch                      bool
		nodeStatusDisconnected       bool
		replace                      bool
		failReplacement              bool
		taintReplacement             bool
		disconnectReplacement        bool
		replaceFailedReplacement     bool
		shouldStopOnDisconnectedNode bool
		maxDisconnect                *time.Duration
		expected                     *resultExpectation
	}

	testCases := []testCase{
		{
			name:                         "reconnect-original-no-replacement",
			allocCount:                   2,
			replace:                      false,
			disconnectedAllocCount:       2,
			disconnectedAllocStatus:      structs.AllocClientStatusRunning,
			disconnectedAllocStates:      disconnectAllocState,
			serverDesiredStatus:          structs.AllocDesiredStatusRun,
			shouldStopOnDisconnectedNode: false,
			expected: &resultExpectation{
				reconnectUpdates: 2,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Ignore: 2,
					},
				},
			},
		},
		{
			name:                         "resume-original-and-stop-replacement",
			allocCount:                   3,
			replace:                      true,
			disconnectedAllocCount:       1,
			disconnectedAllocStatus:      structs.AllocClientStatusRunning,
			disconnectedAllocStates:      disconnectAllocState,
			serverDesiredStatus:          structs.AllocDesiredStatusRun,
			shouldStopOnDisconnectedNode: false,
			expected: &resultExpectation{
				stop:             1,
				reconnectUpdates: 1,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Stop:   1,
						Ignore: 3,
					},
				},
			},
		},
		{
			name:                         "stop-original-with-lower-node-score",
			allocCount:                   4,
			replace:                      true,
			disconnectedAllocCount:       1,
			disconnectedAllocStatus:      structs.AllocClientStatusRunning,
			disconnectedAllocStates:      disconnectAllocState,
			serverDesiredStatus:          structs.AllocDesiredStatusRun,
			shouldStopOnDisconnectedNode: true,
			nodeScoreIncrement:           1,
			expected: &resultExpectation{
				stop: 1,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Stop:   1,
						Ignore: 4,
					},
				},
			},
		},
		{
			name:                         "stop-original-failed-on-reconnect",
			allocCount:                   4,
			replace:                      true,
			disconnectedAllocCount:       2,
			disconnectedAllocStatus:      structs.AllocClientStatusFailed,
			disconnectedAllocStates:      disconnectAllocState,
			serverDesiredStatus:          structs.AllocDesiredStatusRun,
			shouldStopOnDisconnectedNode: true,
			expected: &resultExpectation{
				stop: 2,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Stop:   2,
						Ignore: 4,
					},
				},
			},
		},
		{
			name:                         "reschedule-original-failed-if-not-replaced",
			allocCount:                   4,
			replace:                      false,
			disconnectedAllocCount:       2,
			disconnectedAllocStatus:      structs.AllocClientStatusFailed,
			disconnectedAllocStates:      disconnectAllocState,
			serverDesiredStatus:          structs.AllocDesiredStatusRun,
			shouldStopOnDisconnectedNode: true,
			expected: &resultExpectation{
				stop:  2,
				place: 2,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Ignore: 2,
						Place:  2,
						Stop:   2,
					},
				},
			},
		},
		{
			name:                    "ignore-reconnect-completed",
			allocCount:              2,
			replace:                 false,
			disconnectedAllocCount:  2,
			disconnectedAllocStatus: structs.AllocClientStatusComplete,
			disconnectedAllocStates: disconnectAllocState,
			serverDesiredStatus:     structs.AllocDesiredStatusRun,
			isBatch:                 true,
			expected: &resultExpectation{
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Ignore: 2,
					},
				},
			},
		},
		{
			name:                    "keep-original-alloc-and-stop-failed-replacement",
			allocCount:              3,
			replace:                 true,
			failReplacement:         true,
			disconnectedAllocCount:  2,
			disconnectedAllocStatus: structs.AllocClientStatusRunning,
			disconnectedAllocStates: disconnectAllocState,
			serverDesiredStatus:     structs.AllocDesiredStatusRun,
			expected: &resultExpectation{
				reconnectUpdates: 2,
				stop:             2,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Ignore: 3,
						Stop:   2,
					},
				},
			},
		},
		{
			name:                    "keep-original-and-stop-reconnecting-replacement",
			allocCount:              2,
			replace:                 true,
			disconnectReplacement:   true,
			disconnectedAllocCount:  1,
			disconnectedAllocStatus: structs.AllocClientStatusRunning,
			disconnectedAllocStates: disconnectAllocState,
			serverDesiredStatus:     structs.AllocDesiredStatusRun,
			expected: &resultExpectation{
				reconnectUpdates: 1,
				stop:             1,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Ignore: 2,
						Stop:   1,
					},
				},
			},
		},
		{
			name:                    "keep-original-and-stop-tainted-replacement",
			allocCount:              3,
			replace:                 true,
			taintReplacement:        true,
			disconnectedAllocCount:  2,
			disconnectedAllocStatus: structs.AllocClientStatusRunning,
			disconnectedAllocStates: disconnectAllocState,
			serverDesiredStatus:     structs.AllocDesiredStatusRun,
			expected: &resultExpectation{
				reconnectUpdates: 2,
				stop:             2,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Ignore: 3,
						Stop:   2,
					},
				},
			},
		},
		{
			name:                         "stop-original-alloc-with-old-job-version",
			allocCount:                   5,
			replace:                      true,
			disconnectedAllocCount:       2,
			disconnectedAllocStatus:      structs.AllocClientStatusRunning,
			disconnectedAllocStates:      disconnectAllocState,
			serverDesiredStatus:          structs.AllocDesiredStatusRun,
			shouldStopOnDisconnectedNode: true,
			jobVersionIncrement:          1,
			expected: &resultExpectation{
				stop: 2,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Ignore: 5,
						Stop:   2,
					},
				},
			},
		},
		{
			name:                         "stop-original-alloc-with-old-job-version-reconnect-eval",
			allocCount:                   5,
			replace:                      true,
			disconnectedAllocCount:       2,
			disconnectedAllocStatus:      structs.AllocClientStatusRunning,
			disconnectedAllocStates:      disconnectAllocState,
			serverDesiredStatus:          structs.AllocDesiredStatusRun,
			shouldStopOnDisconnectedNode: true,
			jobVersionIncrement:          1,
			expected: &resultExpectation{
				stop: 2,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Stop:   2,
						Ignore: 5,
					},
				},
			},
		},
		{
			name:                         "stop-original-alloc-with-old-job-version-and-failed-replacements-replaced",
			allocCount:                   5,
			replace:                      true,
			failReplacement:              true,
			replaceFailedReplacement:     true,
			disconnectedAllocCount:       2,
			disconnectedAllocStatus:      structs.AllocClientStatusRunning,
			disconnectedAllocStates:      disconnectAllocState,
			serverDesiredStatus:          structs.AllocDesiredStatusRun,
			shouldStopOnDisconnectedNode: true,
			jobVersionIncrement:          1,
			expected: &resultExpectation{
				stop: 2,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Stop:   2,
						Ignore: 5,
					},
				},
			},
		},
		{
			name:                         "stop-original-pending-alloc-for-disconnected-node",
			allocCount:                   2,
			replace:                      true,
			disconnectedAllocCount:       1,
			disconnectedAllocStatus:      structs.AllocClientStatusPending,
			disconnectedAllocStates:      disconnectAllocState,
			serverDesiredStatus:          structs.AllocDesiredStatusRun,
			shouldStopOnDisconnectedNode: true,
			nodeStatusDisconnected:       true,
			expected: &resultExpectation{
				stop: 1,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Stop:   1,
						Ignore: 2,
					},
				},
			},
		},
		{
			name:                         "stop-failed-original-and-failed-replacements-and-place-new",
			allocCount:                   5,
			replace:                      true,
			failReplacement:              true,
			disconnectedAllocCount:       2,
			disconnectedAllocStatus:      structs.AllocClientStatusFailed,
			disconnectedAllocStates:      disconnectAllocState,
			serverDesiredStatus:          structs.AllocDesiredStatusRun,
			shouldStopOnDisconnectedNode: true,
			expected: &resultExpectation{
				stop:  4,
				place: 2,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Stop:   4,
						Place:  2,
						Ignore: 3,
					},
				},
			},
		},
		{
			name:                         "stop-expired-allocs",
			allocCount:                   5,
			replace:                      true,
			disconnectedAllocCount:       2,
			disconnectedAllocStatus:      structs.AllocClientStatusUnknown,
			disconnectedAllocStates:      disconnectAllocState,
			serverDesiredStatus:          structs.AllocDesiredStatusRun,
			shouldStopOnDisconnectedNode: true,
			nodeStatusDisconnected:       true,
			maxDisconnect:                pointer.Of(2 * time.Second),
			expected: &resultExpectation{
				stop: 2,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Stop:   2,
						Ignore: 5,
					},
				},
			},
		},
		{
			name:                    "replace-allocs-on-disconnected-node",
			allocCount:              5,
			replace:                 false,
			disconnectedAllocCount:  2,
			disconnectedAllocStatus: structs.AllocClientStatusRunning,
			disconnectedAllocStates: []*structs.AllocState{},
			serverDesiredStatus:     structs.AllocDesiredStatusRun,
			nodeStatusDisconnected:  true,
			expected: &resultExpectation{
				place:             2,
				disconnectUpdates: 2,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					"web": {
						Place:  2,
						Ignore: 3,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotEqual(t, 0, tc.allocCount, "invalid test case: alloc count must be greater than zero")

			testNode := mock.Node()
			if tc.nodeStatusDisconnected == true {
				testNode.Status = structs.NodeStatusDisconnected
			}

			// Create resumable allocs
			job, allocs := buildResumableAllocations(tc.allocCount, structs.AllocClientStatusRunning, structs.AllocDesiredStatusRun, 2)

			origAllocs := set.New[string](len(allocs))
			for _, alloc := range allocs {
				origAllocs.Insert(alloc.ID)
			}

			if tc.isBatch {
				job.Type = structs.JobTypeBatch
			}

			// Set alloc state
			disconnectedAllocCount := tc.disconnectedAllocCount
			for _, alloc := range allocs {
				alloc.DesiredStatus = tc.serverDesiredStatus

				if tc.maxDisconnect != nil {
					alloc.Job.TaskGroups[0].MaxClientDisconnect = tc.maxDisconnect
				}

				if disconnectedAllocCount > 0 {
					alloc.ClientStatus = tc.disconnectedAllocStatus
					alloc.AllocStates = tc.disconnectedAllocStates
					// Set the node id on all the disconnected allocs to the node under test.
					alloc.NodeID = testNode.ID
					alloc.NodeName = "disconnected"

					disconnectedAllocCount--
				}
			}

			// Place the allocs on another node.
			if tc.replace {
				replacements := make([]*structs.Allocation, 0)
				for _, alloc := range allocs {
					if alloc.NodeID != testNode.ID {
						continue
					}
					replacement := alloc.Copy()
					replacement.ID = uuid.Generate()
					replacement.NodeID = uuid.Generate()
					replacement.ClientStatus = structs.AllocClientStatusRunning
					replacement.PreviousAllocation = alloc.ID
					replacement.AllocStates = nil
					replacement.TaskStates = nil
					replacement.CreateIndex += 1
					alloc.NextAllocation = replacement.ID

					if tc.jobVersionIncrement != 0 {
						replacement.Job.Version = replacement.Job.Version + tc.jobVersionIncrement
					}
					if tc.nodeScoreIncrement != 0 {
						replacement.Metrics.ScoreMetaData[0].NormScore = replacement.Metrics.ScoreMetaData[0].NormScore + tc.nodeScoreIncrement
					}
					if tc.taintReplacement {
						replacement.DesiredTransition.Migrate = pointer.Of(true)
					}
					if tc.disconnectReplacement {
						replacement.AllocStates = tc.disconnectedAllocStates
					}

					// If we want to test intermediate replacement failures simulate that.
					if tc.failReplacement {
						replacement.ClientStatus = structs.AllocClientStatusFailed

						if tc.replaceFailedReplacement {
							nextReplacement := replacement.Copy()
							nextReplacement.ID = uuid.Generate()
							nextReplacement.ClientStatus = structs.AllocClientStatusRunning
							nextReplacement.DesiredStatus = structs.AllocDesiredStatusRun
							nextReplacement.PreviousAllocation = replacement.ID
							nextReplacement.CreateIndex += 1

							replacement.NextAllocation = nextReplacement.ID
							replacement.DesiredStatus = structs.AllocDesiredStatusStop

							replacements = append(replacements, nextReplacement)
						}
					}

					replacements = append(replacements, replacement)
				}

				allocs = append(allocs, replacements...)
			}

			reconciler := NewAllocReconciler(testlog.HCLogger(t), allocUpdateFnIgnore, tc.isBatch, job.ID, job,
				nil, allocs, map[string]*structs.Node{testNode.ID: testNode}, "", 50, true)

			reconciler.now = time.Now()
			if tc.maxDisconnect != nil {
				reconciler.now = time.Now().Add(*tc.maxDisconnect * 20)
			}

			results := reconciler.Compute()
			assertResults(t, results, tc.expected)

			for _, stopResult := range results.stop {
				// Skip replacement allocs.
				if !origAllocs.Contains(stopResult.alloc.ID) {
					continue
				}

				if tc.shouldStopOnDisconnectedNode {
					require.Equal(t, testNode.ID, stopResult.alloc.NodeID)
				} else {
					require.NotEqual(t, testNode.ID, stopResult.alloc.NodeID)
				}

				require.Equal(t, job.Version, stopResult.alloc.Job.Version)
			}
		})
	}
}

// Tests that a client disconnect while a canary is in progress generates the result.
func TestReconciler_Client_Disconnect_Canaries(t *testing.T) {

	type testCase struct {
		name            string
		nodes           []string
		deploymentState *structs.DeploymentState
		deployedAllocs  map[*structs.Node][]*structs.Allocation
		canaryAllocs    map[*structs.Node][]*structs.Allocation
		expectedResult  *resultExpectation
	}

	running := structs.AllocClientStatusRunning
	complete := structs.AllocClientStatusComplete
	unknown := structs.AllocClientStatusUnknown
	pending := structs.AllocClientStatusPending
	run := structs.AllocDesiredStatusRun
	stop := structs.AllocDesiredStatusStop

	maxClientDisconnect := 10 * time.Minute

	readyNode := mock.Node()
	readyNode.Name = "ready-" + readyNode.ID
	readyNode.Status = structs.NodeStatusReady

	disconnectedNode := mock.Node()
	disconnectedNode.Name = "disconnected-" + disconnectedNode.ID
	disconnectedNode.Status = structs.NodeStatusDisconnected

	// Job with allocations and max_client_disconnect
	job := mock.Job()

	updatedJob := job.Copy()
	updatedJob.Version = updatedJob.Version + 1

	testCases := []testCase{
		{
			name: "3-placed-1-disconnect",
			deploymentState: &structs.DeploymentState{
				AutoRevert:        false,
				AutoPromote:       false,
				Promoted:          false,
				ProgressDeadline:  5 * time.Minute,
				RequireProgressBy: time.Now().Add(5 * time.Minute),
				PlacedCanaries:    []string{},
				DesiredCanaries:   1,
				DesiredTotal:      6,
				PlacedAllocs:      3,
				HealthyAllocs:     2,
				UnhealthyAllocs:   0,
			},
			deployedAllocs: map[*structs.Node][]*structs.Allocation{
				readyNode: {
					// filtered as terminal
					{Name: "my-job.web[0]", ClientStatus: complete, DesiredStatus: stop},
					// Ignored
					{Name: "my-job.web[2]", ClientStatus: running, DesiredStatus: stop},
					// destructive, but discarded because canarying
					{Name: "my-job.web[4]", ClientStatus: running, DesiredStatus: run},
				},
				disconnectedNode: {
					// filtered as terminal
					{Name: "my-job.web[1]", ClientStatus: complete, DesiredStatus: stop},
					// Gets a placement, and a disconnect update
					{Name: "my-job.web[3]", ClientStatus: running, DesiredStatus: run},
					// Gets a placement, and a disconnect update
					{Name: "my-job.web[5]", ClientStatus: running, DesiredStatus: run},
				},
			},
			canaryAllocs: map[*structs.Node][]*structs.Allocation{
				readyNode: {
					// Ignored
					{Name: "my-job.web[0]", ClientStatus: running, DesiredStatus: run},
					// Ignored
					{Name: "my-job.web[2]", ClientStatus: pending, DesiredStatus: run},
				},
				disconnectedNode: {
					// Gets a placement, and a disconnect update
					{Name: "my-job.web[1]", ClientStatus: running, DesiredStatus: run},
				},
			},
			expectedResult: &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: nil,
				place:             3,
				destructive:       0,
				stop:              0,
				inplace:           0,
				attributeUpdates:  0,
				disconnectUpdates: 3,
				reconnectUpdates:  0,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					updatedJob.TaskGroups[0].Name: {
						Place:  3,
						Canary: 0,
						Ignore: 3,
					},
				},
			},
		},
		{
			name: "ignore-unknown",
			deploymentState: &structs.DeploymentState{
				AutoRevert:        false,
				AutoPromote:       false,
				Promoted:          false,
				ProgressDeadline:  5 * time.Minute,
				RequireProgressBy: time.Now().Add(5 * time.Minute),
				PlacedCanaries:    []string{},
				DesiredCanaries:   1,
				DesiredTotal:      6,
				PlacedAllocs:      3,
				HealthyAllocs:     2,
				UnhealthyAllocs:   0,
			},
			deployedAllocs: map[*structs.Node][]*structs.Allocation{
				readyNode: {
					// filtered as terminal
					{Name: "my-job.web[0]", ClientStatus: complete, DesiredStatus: stop},
					// Ignored
					{Name: "my-job.web[2]", ClientStatus: running, DesiredStatus: stop},
					// destructive, but discarded because canarying
					{Name: "my-job.web[4]", ClientStatus: running, DesiredStatus: run},
				},
				disconnectedNode: {
					// filtered as terminal
					{Name: "my-job.web[1]", ClientStatus: complete, DesiredStatus: stop},
					// Gets a placement, and a disconnect update
					{Name: "my-job.web[3]", ClientStatus: running, DesiredStatus: run},
					// Gets a placement, and a disconnect update
					{Name: "my-job.web[5]", ClientStatus: running, DesiredStatus: run},
				},
			},
			canaryAllocs: map[*structs.Node][]*structs.Allocation{
				readyNode: {
					// Ignored
					{Name: "my-job.web[0]", ClientStatus: running, DesiredStatus: run},
					// Ignored
					{Name: "my-job.web[2]", ClientStatus: pending, DesiredStatus: run},
				},
				disconnectedNode: {
					// Ignored
					{Name: "my-job.web[1]", ClientStatus: unknown, DesiredStatus: run},
				},
			},
			expectedResult: &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: nil,
				place:             2,
				destructive:       0,
				stop:              0,
				inplace:           0,
				attributeUpdates:  0,
				disconnectUpdates: 2,
				reconnectUpdates:  0,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					updatedJob.TaskGroups[0].Name: {
						Place:  2,
						Canary: 0,
						Ignore: 4,
					},
				},
			},
		},
		{
			name: "4-placed-2-pending-lost",
			deploymentState: &structs.DeploymentState{
				AutoRevert:        false,
				AutoPromote:       false,
				Promoted:          false,
				ProgressDeadline:  5 * time.Minute,
				RequireProgressBy: time.Now().Add(5 * time.Minute),
				PlacedCanaries:    []string{},
				DesiredCanaries:   2,
				DesiredTotal:      6,
				PlacedAllocs:      4,
				HealthyAllocs:     2,
				UnhealthyAllocs:   0,
			},
			deployedAllocs: map[*structs.Node][]*structs.Allocation{
				readyNode: {
					// filtered as terminal
					{Name: "my-job.web[0]", ClientStatus: complete, DesiredStatus: stop},
					// filtered as terminal
					{Name: "my-job.web[2]", ClientStatus: complete, DesiredStatus: stop},
					// destructive, but discarded because canarying
					{Name: "my-job.web[4]", ClientStatus: running, DesiredStatus: run},
				},
				disconnectedNode: {
					// filtered as terminal
					{Name: "my-job.web[1]", ClientStatus: complete, DesiredStatus: stop},
					// Gets a placement, and a disconnect update
					{Name: "my-job.web[3]", ClientStatus: running, DesiredStatus: run},
					// Gets a placement, and a disconnect update
					{Name: "my-job.web[5]", ClientStatus: running, DesiredStatus: run},
				},
			},
			canaryAllocs: map[*structs.Node][]*structs.Allocation{
				readyNode: {
					// Ignored
					{Name: "my-job.web[0]", ClientStatus: running, DesiredStatus: run},
					// Ignored
					{Name: "my-job.web[2]", ClientStatus: running, DesiredStatus: run},
				},
				disconnectedNode: {
					// Stop/Lost because pending
					{Name: "my-job.web[1]", ClientStatus: pending, DesiredStatus: run},
					// Stop/Lost because pending
					{Name: "my-job.web[3]", ClientStatus: pending, DesiredStatus: run},
				},
			},
			expectedResult: &resultExpectation{
				createDeployment:  nil,
				deploymentUpdates: nil,
				place:             2,
				destructive:       0,
				stop:              2,
				inplace:           0,
				attributeUpdates:  0,
				disconnectUpdates: 2,
				reconnectUpdates:  0,
				desiredTGUpdates: map[string]*structs.DesiredUpdates{
					updatedJob.TaskGroups[0].Name: {
						Place:  2,
						Canary: 0,
						Ignore: 3,
						// The 2 stops in this test are transient failures, but
						// the deployment can still progress. We don't include
						// them in the stop count since DesiredTGUpdates is used
						// to report deployment progress or final deployment state.
						Stop: 0,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set the count dynamically to the number from the original deployment.
			job.TaskGroups[0].Count = len(tc.deployedAllocs[readyNode]) + len(tc.deployedAllocs[disconnectedNode])
			job.TaskGroups[0].MaxClientDisconnect = &maxClientDisconnect
			job.TaskGroups[0].Update = &structs.UpdateStrategy{
				MaxParallel:     1,
				Canary:          tc.deploymentState.DesiredCanaries,
				MinHealthyTime:  3 * time.Second,
				HealthyDeadline: 20 * time.Second,
				AutoRevert:      true,
				AutoPromote:     true,
			}

			updatedJob.TaskGroups[0].Count = len(tc.deployedAllocs[readyNode]) + len(tc.deployedAllocs[disconnectedNode])
			updatedJob.TaskGroups[0].MaxClientDisconnect = &maxClientDisconnect
			updatedJob.TaskGroups[0].Update = &structs.UpdateStrategy{
				MaxParallel:     1,
				Canary:          tc.deploymentState.DesiredCanaries,
				MinHealthyTime:  3 * time.Second,
				HealthyDeadline: 20 * time.Second,
				AutoRevert:      true,
				AutoPromote:     true,
			}

			// Populate Alloc IDS, Node IDs, Job on deployed allocs
			allocsConfigured := 0
			for node, allocs := range tc.deployedAllocs {
				for _, alloc := range allocs {
					alloc.ID = uuid.Generate()
					alloc.NodeID = node.ID
					alloc.NodeName = node.Name
					alloc.JobID = job.ID
					alloc.Job = job
					alloc.TaskGroup = job.TaskGroups[0].Name
					allocsConfigured++
				}
			}

			require.Equal(t, tc.deploymentState.DesiredTotal, allocsConfigured, "invalid alloc configuration: expect %d got %d", tc.deploymentState.DesiredTotal, allocsConfigured)

			// Populate Alloc IDS, Node IDs, Job on canaries
			canariesConfigured := 0
			handled := make(map[string]allocUpdateType)
			for node, allocs := range tc.canaryAllocs {
				for _, alloc := range allocs {
					alloc.ID = uuid.Generate()
					alloc.NodeID = node.ID
					alloc.NodeName = node.Name
					alloc.JobID = updatedJob.ID
					alloc.Job = updatedJob
					alloc.TaskGroup = updatedJob.TaskGroups[0].Name
					alloc.DeploymentStatus = &structs.AllocDeploymentStatus{
						Canary: true,
					}
					if alloc.ClientStatus == structs.AllocClientStatusRunning {
						alloc.DeploymentStatus.Healthy = pointer.Of(true)
					}
					tc.deploymentState.PlacedCanaries = append(tc.deploymentState.PlacedCanaries, alloc.ID)
					handled[alloc.ID] = allocUpdateFnIgnore
					canariesConfigured++
				}
			}

			// Validate tc.canaryAllocs against tc.deploymentState
			require.Equal(t, tc.deploymentState.PlacedAllocs, canariesConfigured, "invalid canary configuration: expect %d got %d", tc.deploymentState.PlacedAllocs, canariesConfigured)

			deployment := structs.NewDeployment(updatedJob, 50)
			deployment.TaskGroups[updatedJob.TaskGroups[0].Name] = tc.deploymentState

			// Build a map of tainted nodes that contains the last canary
			tainted := make(map[string]*structs.Node, 1)
			tainted[disconnectedNode.ID] = disconnectedNode

			allocs := make([]*structs.Allocation, 0)

			allocs = append(allocs, tc.deployedAllocs[readyNode]...)
			allocs = append(allocs, tc.deployedAllocs[disconnectedNode]...)
			allocs = append(allocs, tc.canaryAllocs[readyNode]...)
			allocs = append(allocs, tc.canaryAllocs[disconnectedNode]...)

			mockUpdateFn := allocUpdateFnMock(handled, allocUpdateFnDestructive)
			reconciler := NewAllocReconciler(testlog.HCLogger(t), mockUpdateFn, false, updatedJob.ID, updatedJob,
				deployment, allocs, tainted, "", 50, true)
			result := reconciler.Compute()

			// Assert the correct results
			assertResults(t, result, tc.expectedResult)

			// Validate that placements are either for placed canaries for the
			// updated job, or for disconnected allocs for the original job
			// and that they have a disconnect update.
			for _, placeResult := range result.place {
				found := false
				require.NotNil(t, placeResult.previousAlloc)
				for _, deployed := range tc.deployedAllocs[disconnectedNode] {
					if deployed.ID == placeResult.previousAlloc.ID {
						found = true
						require.Equal(t, job.Version, placeResult.previousAlloc.Job.Version)
						require.Equal(t, disconnectedNode.ID, placeResult.previousAlloc.NodeID)
						_, exists := result.disconnectUpdates[placeResult.previousAlloc.ID]
						require.True(t, exists)
						break
					}
				}
				for _, canary := range tc.canaryAllocs[disconnectedNode] {
					if canary.ID == placeResult.previousAlloc.ID {
						found = true
						require.Equal(t, updatedJob.Version, placeResult.previousAlloc.Job.Version)
						require.Equal(t, disconnectedNode.ID, placeResult.previousAlloc.NodeID)
						_, exists := result.disconnectUpdates[placeResult.previousAlloc.ID]
						require.True(t, exists)
						break
					}
				}
				require.True(t, found)
			}

			// Validate that stops are for pending disconnects
			for _, stopResult := range result.stop {
				require.Equal(t, pending, stopResult.alloc.ClientStatus)
			}
		})
	}
}

// Tests the reconciler properly handles the logic for computeDeploymentPaused
// for various job types.
func TestReconciler_ComputeDeploymentPaused(t *testing.T) {
	ci.Parallel(t)

	type testCase struct {
		name            string
		jobType         string
		isMultiregion   bool
		isPeriodic      bool
		isParameterized bool
		expected        bool
	}

	multiregionCfg := mock.MultiregionJob().Multiregion
	periodicCfg := mock.PeriodicJob().Periodic
	parameterizedCfg := &structs.ParameterizedJobConfig{
		Payload: structs.DispatchPayloadRequired,
	}

	testCases := []testCase{
		{
			name:            "single region service is not paused",
			jobType:         structs.JobTypeService,
			isMultiregion:   false,
			isPeriodic:      false,
			isParameterized: false,
			expected:        false,
		},
		{
			name:            "multiregion service is paused",
			jobType:         structs.JobTypeService,
			isMultiregion:   true,
			isPeriodic:      false,
			isParameterized: false,
			expected:        true,
		},
		{
			name:            "single region batch job is not paused",
			jobType:         structs.JobTypeBatch,
			isMultiregion:   false,
			isPeriodic:      false,
			isParameterized: false,
			expected:        false,
		},
		{
			name:            "multiregion batch job is not paused",
			jobType:         structs.JobTypeBatch,
			isMultiregion:   false,
			isPeriodic:      false,
			isParameterized: false,
			expected:        false,
		},
		{
			name:            "multiregion parameterized batch is not paused",
			jobType:         structs.JobTypeBatch,
			isMultiregion:   true,
			isPeriodic:      false,
			isParameterized: true,
			expected:        false,
		},
		{
			name:            "multiregion periodic batch is not paused",
			jobType:         structs.JobTypeBatch,
			isMultiregion:   true,
			isPeriodic:      true,
			isParameterized: false,
			expected:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var job *structs.Job

			if tc.jobType == structs.JobTypeService {
				job = mock.Job()
			} else if tc.jobType == structs.JobTypeBatch {
				job = mock.BatchJob()
			}

			require.NotNil(t, job, "invalid job type", tc.jobType)

			var deployment *structs.Deployment
			if tc.isMultiregion {
				job.Multiregion = multiregionCfg

				// This deployment is created by the Job.Register RPC and
				// fetched by the scheduler before handing it to the
				// reconciler.
				if job.UsesDeployments() {
					deployment = structs.NewDeployment(job, 100)
					deployment.Status = structs.DeploymentStatusInitializing
					deployment.StatusDescription = structs.DeploymentStatusDescriptionPendingForPeer
				}
			}

			if tc.isPeriodic {
				job.Periodic = periodicCfg
			}

			if tc.isParameterized {
				job.ParameterizedJob = parameterizedCfg
			}

			reconciler := NewAllocReconciler(
				testlog.HCLogger(t), allocUpdateFnIgnore, false, job.ID, job, deployment,
				nil, nil, "", job.Priority, true)

			_ = reconciler.Compute()

			require.Equal(t, tc.expected, reconciler.deploymentPaused)
		})
	}
}
