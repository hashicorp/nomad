package api

import (
	"reflect"
	"sort"
	"testing"

	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"
)

func TestAllocations_List(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Allocations()

	// Querying when no allocs exist returns nothing
	allocs, qm, err := a.List(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(allocs); n != 0 {
		t.Fatalf("expected 0 allocs, got: %d", n)
	}

	// TODO: do something that causes an allocation to actually happen
	// so we can query for them.
	return

	//job := &Job{
	//ID:   stringToPtr("job1"),
	//Name: stringToPtr("Job #1"),
	//Type: stringToPtr(JobTypeService),
	//}
	//eval, _, err := c.Jobs().Register(job, nil)
	//if err != nil {
	//t.Fatalf("err: %s", err)
	//}

	//// List the allocations again
	//allocs, qm, err = a.List(nil)
	//if err != nil {
	//t.Fatalf("err: %s", err)
	//}
	//if qm.LastIndex == 0 {
	//t.Fatalf("bad index: %d", qm.LastIndex)
	//}

	//// Check that we got the allocation back
	//if len(allocs) == 0 || allocs[0].EvalID != eval {
	//t.Fatalf("bad: %#v", allocs)
	//}
}

func TestAllocations_PrefixList(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()
	a := c.Allocations()

	// Querying when no allocs exist returns nothing
	allocs, qm, err := a.PrefixList("")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if qm.LastIndex != 0 {
		t.Fatalf("bad index: %d", qm.LastIndex)
	}
	if n := len(allocs); n != 0 {
		t.Fatalf("expected 0 allocs, got: %d", n)
	}

	// TODO: do something that causes an allocation to actually happen
	// so we can query for them.
	return

	//job := &Job{
	//ID:   stringToPtr("job1"),
	//Name: stringToPtr("Job #1"),
	//Type: stringToPtr(JobTypeService),
	//}

	//eval, _, err := c.Jobs().Register(job, nil)
	//if err != nil {
	//t.Fatalf("err: %s", err)
	//}

	//// List the allocations by prefix
	//allocs, qm, err = a.PrefixList("foobar")
	//if err != nil {
	//t.Fatalf("err: %s", err)
	//}
	//if qm.LastIndex == 0 {
	//t.Fatalf("bad index: %d", qm.LastIndex)
	//}

	//// Check that we got the allocation back
	//if len(allocs) == 0 || allocs[0].EvalID != eval {
	//t.Fatalf("bad: %#v", allocs)
	//}
}

func TestAllocations_CreateIndexSort(t *testing.T) {
	t.Parallel()
	allocs := []*AllocationListStub{
		{CreateIndex: 2},
		{CreateIndex: 1},
		{CreateIndex: 5},
	}
	sort.Sort(AllocIndexSort(allocs))

	expect := []*AllocationListStub{
		{CreateIndex: 5},
		{CreateIndex: 2},
		{CreateIndex: 1},
	}
	if !reflect.DeepEqual(allocs, expect) {
		t.Fatalf("\n\n%#v\n\n%#v", allocs, expect)
	}
}

func TestAllocations_RescheduleInfo(t *testing.T) {
	t.Parallel()
	// Create a job, task group and alloc
	job := &Job{
		Name:      stringToPtr("foo"),
		Namespace: stringToPtr(DefaultNamespace),
		ID:        stringToPtr("bar"),
		ParentID:  stringToPtr("lol"),
		TaskGroups: []*TaskGroup{
			{
				Name: stringToPtr("bar"),
				Tasks: []*Task{
					{
						Name: "task1",
					},
				},
			},
		},
	}
	job.Canonicalize()

	uuidGen := func() string {
		ret, err := uuid.GenerateUUID()
		if err != nil {
			t.Fatal(err)
		}
		return ret
	}

	alloc := &Allocation{
		ID:        uuidGen(),
		Namespace: DefaultNamespace,
		EvalID:    uuidGen(),
		Name:      "foo-bar[1]",
		NodeID:    uuidGen(),
		TaskGroup: *job.TaskGroups[0].Name,
		JobID:     *job.ID,
		Job:       job,
	}

	type testCase struct {
		desc              string
		reschedulePolicy  *ReschedulePolicy
		rescheduleTracker *RescheduleTracker
		time              time.Time
		expAttempted      int
		expTotal          int
	}

	testCases := []testCase{
		{
			desc:         "no reschedule policy",
			expAttempted: 0,
			expTotal:     0,
		},
		{
			desc: "no reschedule events",
			reschedulePolicy: &ReschedulePolicy{
				Attempts: intToPtr(3),
				Interval: timeToPtr(15 * time.Minute),
			},
			expAttempted: 0,
			expTotal:     3,
		},
		{
			desc: "all reschedule events within interval",
			reschedulePolicy: &ReschedulePolicy{
				Attempts: intToPtr(3),
				Interval: timeToPtr(15 * time.Minute),
			},
			time: time.Now(),
			rescheduleTracker: &RescheduleTracker{
				Events: []*RescheduleEvent{
					{
						RescheduleTime: time.Now().Add(-5 * time.Minute).UTC().UnixNano(),
					},
				},
			},
			expAttempted: 1,
			expTotal:     3,
		},
		{
			desc: "some reschedule events outside interval",
			reschedulePolicy: &ReschedulePolicy{
				Attempts: intToPtr(3),
				Interval: timeToPtr(15 * time.Minute),
			},
			time: time.Now(),
			rescheduleTracker: &RescheduleTracker{
				Events: []*RescheduleEvent{
					{
						RescheduleTime: time.Now().Add(-45 * time.Minute).UTC().UnixNano(),
					},
					{
						RescheduleTime: time.Now().Add(-30 * time.Minute).UTC().UnixNano(),
					},
					{
						RescheduleTime: time.Now().Add(-10 * time.Minute).UTC().UnixNano(),
					},
					{
						RescheduleTime: time.Now().Add(-5 * time.Minute).UTC().UnixNano(),
					},
				},
			},
			expAttempted: 2,
			expTotal:     3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require := require.New(t)
			alloc.RescheduleTracker = tc.rescheduleTracker
			job.TaskGroups[0].ReschedulePolicy = tc.reschedulePolicy
			attempted, total := alloc.RescheduleInfo(tc.time)
			require.Equal(tc.expAttempted, attempted)
			require.Equal(tc.expTotal, total)
		})
	}

}

func TestAllocations_ShouldMigrate(t *testing.T) {
	t.Parallel()
	require.True(t, DesiredTransition{Migrate: boolToPtr(true)}.ShouldMigrate())
	require.False(t, DesiredTransition{}.ShouldMigrate())
	require.False(t, DesiredTransition{Migrate: boolToPtr(false)}.ShouldMigrate())
}
