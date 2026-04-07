// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package batchtimeout

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestShouldStopAlloc(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	cases := []struct {
		name     string
		allocFn  func() *structs.Allocation
		expected bool
	}{
		{
			name: "batch alloc times out after max run duration",
			allocFn: func() *structs.Allocation {
				alloc := mock.Alloc()
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].MaxRunDuration = pointer.Of(5 * time.Minute)
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.DesiredStatus = structs.AllocDesiredStatusRun
				alloc.TaskStates = map[string]*structs.TaskState{
					"web": {
						State:     structs.TaskStateRunning,
						StartedAt: now.Add(-10 * time.Minute),
					},
				}
				return alloc
			},
			expected: true,
		},
		{
			name: "sysbatch alloc times out after max run duration",
			allocFn: func() *structs.Allocation {
				alloc := mock.SysBatchAlloc()
				alloc.Job.TaskGroups[0].MaxRunDuration = pointer.Of(5 * time.Minute)
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.DesiredStatus = structs.AllocDesiredStatusRun
				alloc.TaskStates = map[string]*structs.TaskState{
					"ping-example": {
						State:     structs.TaskStateRunning,
						StartedAt: now.Add(-10 * time.Minute),
					},
				}
				return alloc
			},
			expected: true,
		},
		{
			name: "alloc without max run duration does not time out",
			allocFn: func() *structs.Allocation {
				alloc := mock.Alloc()
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].MaxRunDuration = nil
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.DesiredStatus = structs.AllocDesiredStatusRun
				alloc.TaskStates = map[string]*structs.TaskState{
					"web": {
						State:     structs.TaskStateRunning,
						StartedAt: now.Add(-10 * time.Minute),
					},
				}
				return alloc
			},
			expected: false,
		},
		{
			name: "service alloc does not time out",
			allocFn: func() *structs.Allocation {
				alloc := mock.Alloc()
				alloc.Job.Type = structs.JobTypeService
				alloc.Job.TaskGroups[0].MaxRunDuration = pointer.Of(5 * time.Minute)
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.DesiredStatus = structs.AllocDesiredStatusRun
				alloc.TaskStates = map[string]*structs.TaskState{
					"web": {
						State:     structs.TaskStateRunning,
						StartedAt: now.Add(-10 * time.Minute),
					},
				}
				return alloc
			},
			expected: false,
		},
		{
			name: "pending alloc does not time out",
			allocFn: func() *structs.Allocation {
				alloc := mock.Alloc()
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].MaxRunDuration = pointer.Of(5 * time.Minute)
				alloc.ClientStatus = structs.AllocClientStatusPending
				alloc.DesiredStatus = structs.AllocDesiredStatusRun
				alloc.TaskStates = map[string]*structs.TaskState{
					"web": {
						State:     structs.TaskStateRunning,
						StartedAt: now.Add(-10 * time.Minute),
					},
				}
				return alloc
			},
			expected: false,
		},
		{
			name: "stopping alloc does not time out again",
			allocFn: func() *structs.Allocation {
				alloc := mock.Alloc()
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].MaxRunDuration = pointer.Of(5 * time.Minute)
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.DesiredStatus = structs.AllocDesiredStatusStop
				alloc.TaskStates = map[string]*structs.TaskState{
					"web": {
						State:     structs.TaskStateRunning,
						StartedAt: now.Add(-10 * time.Minute),
					},
				}
				return alloc
			},
			expected: false,
		},
		{
			name: "alloc within max run duration does not time out",
			allocFn: func() *structs.Allocation {
				alloc := mock.Alloc()
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].MaxRunDuration = pointer.Of(15 * time.Minute)
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.DesiredStatus = structs.AllocDesiredStatusRun
				alloc.TaskStates = map[string]*structs.TaskState{
					"web": {
						State:     structs.TaskStateRunning,
						StartedAt: now.Add(-10 * time.Minute),
					},
				}
				return alloc
			},
			expected: false,
		},
		{
			name: "alloc with no running task state does not time out",
			allocFn: func() *structs.Allocation {
				alloc := mock.Alloc()
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].MaxRunDuration = pointer.Of(5 * time.Minute)
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.DesiredStatus = structs.AllocDesiredStatusRun
				alloc.TaskStates = map[string]*structs.TaskState{
					"web": {
						State:     structs.TaskStatePending,
						StartedAt: now.Add(-10 * time.Minute),
					},
				}
				return alloc
			},
			expected: false,
		},
		{
			name: "alloc with zero start time does not time out",
			allocFn: func() *structs.Allocation {
				alloc := mock.Alloc()
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].MaxRunDuration = pointer.Of(5 * time.Minute)
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.DesiredStatus = structs.AllocDesiredStatusRun
				alloc.TaskStates = map[string]*structs.TaskState{
					"web": {
						State: structs.TaskStateRunning,
					},
				}
				return alloc
			},
			expected: false,
		},
		{
			name: "alloc with mixed task states does not time out until all tasks are running",
			allocFn: func() *structs.Allocation {
				alloc := mock.Alloc()
				alloc.Job.Type = structs.JobTypeBatch
				alloc.Job.TaskGroups[0].Tasks = append(alloc.Job.TaskGroups[0].Tasks, &structs.Task{Name: "sidecar"})
				alloc.Job.TaskGroups[0].MaxRunDuration = pointer.Of(5 * time.Minute)
				alloc.ClientStatus = structs.AllocClientStatusRunning
				alloc.DesiredStatus = structs.AllocDesiredStatusRun
				alloc.TaskStates = map[string]*structs.TaskState{
					"web": {
						State:     structs.TaskStateRunning,
						StartedAt: now.Add(-10 * time.Minute),
					},
					"sidecar": {
						State: structs.TaskStatePending,
					},
				}
				return alloc
			},
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expected, shouldStopAlloc(now, tc.allocFn()))
		})
	}
}

func TestAllocRunningSince(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	t.Run("returns latest running task start time", func(t *testing.T) {
		alloc := mock.Alloc()
		alloc.TaskStates = map[string]*structs.TaskState{
			"web": {
				State:     structs.TaskStateRunning,
				StartedAt: now.Add(-10 * time.Minute),
			},
			"sidecar": {
				State:     structs.TaskStateRunning,
				StartedAt: now.Add(-5 * time.Minute),
			},
		}

		startedAt, ok := allocRunningSince(alloc)
		must.True(t, ok)
		must.Eq(t, now.Add(-5*time.Minute), startedAt)
	})

	t.Run("returns false when task states are missing", func(t *testing.T) {
		alloc := mock.Alloc()
		alloc.TaskStates = nil

		_, ok := allocRunningSince(alloc)
		must.False(t, ok)
	})

	t.Run("returns false when any task is not running", func(t *testing.T) {
		alloc := mock.Alloc()
		alloc.TaskStates = map[string]*structs.TaskState{
			"web": {
				State:     structs.TaskStateRunning,
				StartedAt: now.Add(-10 * time.Minute),
			},
			"sidecar": {
				State: structs.TaskStatePending,
			},
		}

		_, ok := allocRunningSince(alloc)
		must.False(t, ok)
	})

	t.Run("returns false when any running task has zero start time", func(t *testing.T) {
		alloc := mock.Alloc()
		alloc.TaskStates = map[string]*structs.TaskState{
			"web": {
				State: structs.TaskStateRunning,
			},
		}

		_, ok := allocRunningSince(alloc)
		must.False(t, ok)
	})
}
