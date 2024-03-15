// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconnectingpicker

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestPickReconnectingAlloc_NewerVersion(t *testing.T) {
	rp := New(hclog.NewNullLogger())
	ds := &structs.DisconnectStrategy{
		Reconcile: "best-score",
	}

	replacement := &structs.Allocation{
		Job: &structs.Job{
			Version:     2,
			CreateIndex: 2,
		},
	}

	testCases := []struct {
		name        string
		version     uint64
		createIndex uint64
		expected    *structs.Allocation
	}{
		{
			name:        "original_is_older",
			version:     1,
			createIndex: 1,
			expected:    replacement,
		},
		{
			name:        "original_has_older_version",
			version:     1,
			createIndex: 2,
			expected:    replacement,
		},
		{
			name:        "original_has_older_create_index",
			version:     2,
			createIndex: 1,
			expected:    replacement,
		},
	}

	for _, tc := range testCases {
		original := &structs.Allocation{
			Job: &structs.Job{
				Version:     tc.version,
				CreateIndex: tc.createIndex,
			},
		}

		result := rp.PickReconnectingAlloc(ds, original, replacement)

		must.Eq(t, tc.expected, result)
	}
}

func TestPickReconnectingAlloc_DifferentStrategies(t *testing.T) {
	rp := New(hclog.NewNullLogger())
	now := time.Now()

	original := &structs.Allocation{
		TaskGroup: "taskgroup1",
		Job: &structs.Job{
			Version:     1,
			CreateIndex: 1,
			TaskGroups: []*structs.TaskGroup{
				{
					Name: "taskgroup1",
					Tasks: []*structs.Task{
						{
							Name: "task1",
						},
					},
				},
			},
		},
		TaskStates: map[string]*structs.TaskState{
			"task1": {
				Restarts:  0,
				StartedAt: now.Add(-time.Hour),
			},
		},
		Metrics: &structs.AllocMetric{
			ScoreMetaData: []*structs.NodeScoreMeta{
				{
					NormScore: 10,
				},
			},
		},
	}

	replacement := &structs.Allocation{
		Job: &structs.Job{
			Version:     1,
			CreateIndex: 1,
			TaskGroups: []*structs.TaskGroup{
				{
					Name: "taskgroup1",
					Tasks: []*structs.Task{
						{
							Name: "task1",
						},
					},
				},
			},
		},
		TaskStates: map[string]*structs.TaskState{
			"task1": {
				Restarts:  0,
				StartedAt: now.Add(-30 * time.Minute),
			},
		},
		Metrics: &structs.AllocMetric{
			ScoreMetaData: []*structs.NodeScoreMeta{
				{
					NormScore: 20,
				},
			},
		},
	}

	testsCases := []struct {
		name     string
		strategy string

		expected *structs.Allocation
	}{
		{
			name:     "keep_the_allocation_with_the_best_score",
			strategy: structs.ReconcileOptionBestScore,
			expected: replacement,
		},
		{
			name:     "keep_the_original_allocation",
			strategy: structs.ReconcileOptionKeepOriginal,
			expected: original,
		},
		{
			name:     "keep_the_replacement_allocation",
			strategy: structs.ReconcileOptionKeepReplacement,
			expected: replacement,
		},
		{
			name:     "keep_the_longest_running_allocation",
			strategy: structs.ReconcileOptionLongestRunning,
			expected: original,
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			ds := &structs.DisconnectStrategy{
				Reconcile: tc.strategy,
			}

			result := rp.PickReconnectingAlloc(ds, original, replacement)
			must.Eq(t, tc.expected, result)

		})
	}
}

func TestPickReconnectingAlloc_BestScore(t *testing.T) {
	rp := New(hclog.NewNullLogger())

	original := &structs.Allocation{
		Job: &structs.Job{
			Version:     1,
			CreateIndex: 1,
		},
		TaskGroup: "taskgroup1",
		Metrics: &structs.AllocMetric{
			ScoreMetaData: []*structs.NodeScoreMeta{
				{
					NormScore: 10,
				},
			},
		},
	}

	replacement := original.Copy()

	testsCases := []struct {
		name                    string
		originalClientStatus    string
		replacementClientStatus string
		replacementScore        float64
		expected                *structs.Allocation
	}{
		{
			name:                    "replacement_has_better_score_and_running",
			replacementScore:        20,
			originalClientStatus:    structs.AllocClientStatusRunning,
			replacementClientStatus: structs.AllocClientStatusRunning,
			expected:                replacement,
		},
		{
			name:                    "original_has_better_score_and_running",
			originalClientStatus:    structs.AllocClientStatusRunning,
			replacementClientStatus: structs.AllocClientStatusRunning,
			replacementScore:        5,
			expected:                original,
		},
		{
			name:                    "replacement_has_better_score_but_replacement_not_running",
			originalClientStatus:    structs.AllocClientStatusRunning,
			replacementClientStatus: structs.AllocClientStatusPending,
			replacementScore:        20,
			expected:                original,
		},
		{
			name:                    "replacement_has_better_score_and_original_not_running",
			originalClientStatus:    structs.AllocClientStatusPending,
			replacementClientStatus: structs.AllocClientStatusRunning,
			replacementScore:        20,
			expected:                replacement,
		},
		{
			name:                    "original_has_better_score_but_not_running",
			originalClientStatus:    structs.AllocClientStatusPending,
			replacementClientStatus: structs.AllocClientStatusRunning,
			replacementScore:        5,
			expected:                original,
		},
		{
			name:                    "original_has_better_score_and_replacement_not_running",
			originalClientStatus:    structs.AllocClientStatusRunning,
			replacementClientStatus: structs.AllocClientStatusPending,
			replacementScore:        5,
			expected:                original,
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			original.ClientStatus = tc.originalClientStatus

			replacement.ClientStatus = tc.replacementClientStatus
			replacement.Metrics.ScoreMetaData[0].NormScore = tc.replacementScore

			result := rp.PickReconnectingAlloc(&structs.DisconnectStrategy{
				Reconcile: structs.ReconcileOptionBestScore,
			}, original, replacement)

			must.Eq(t, tc.expected, result)
		})
	}
}

func TestPickReconnectingAlloc_LongestRunning(t *testing.T) {
	rp := New(hclog.NewNullLogger())
	now := time.Now()
	fmt.Println(now)
	taskGroupNoLeader := &structs.TaskGroup{
		Name: "taskGroupNoLeader",
		Tasks: []*structs.Task{
			{
				Name: "task1",
			},
			{
				Name: "task2",
			},
			{
				Name: "task3",
			},
		},
	}

	taskGroupWithLeader := &structs.TaskGroup{
		Name: "taskGroupWithLeader",
		Tasks: []*structs.Task{
			{
				Name: "task1",
			},
			{
				Name:   "task2",
				Leader: true,
			},
			{
				Name: "task3",
			},
		},
	}

	emptyTaskGroup := &structs.TaskGroup{
		Name: "emptyTaskGroup",
	}

	original := &structs.Allocation{
		Job: &structs.Job{
			Version:     1,
			CreateIndex: 1,
			TaskGroups: []*structs.TaskGroup{
				taskGroupNoLeader,
				taskGroupWithLeader,
				emptyTaskGroup,
			},
		},
		TaskStates: map[string]*structs.TaskState{
			"task2": {},
		},
		Metrics: &structs.AllocMetric{
			ScoreMetaData: []*structs.NodeScoreMeta{
				{
					NormScore: 10,
				},
			},
		},
	}

	replacement := original.Copy()
	replacement.Metrics.ScoreMetaData[0].NormScore = 20

	testsCases := []struct {
		name             string
		taskGroupName    string
		originalState    structs.TaskState
		replacementState structs.TaskState
		expected         *structs.Allocation
	}{
		{
			name:          "original_with_no_restart",
			taskGroupName: "taskGroupNoLeader",
			replacementState: structs.TaskState{
				StartedAt:   now.Add(-30 * time.Minute),
				Restarts:    2,
				LastRestart: now.Add(-10 * time.Minute),
			},
			originalState: structs.TaskState{
				StartedAt: now.Add(-time.Hour),
				Restarts:  0,
			},
			expected: original,
		},
		{
			name:          "original_with_no_restart_on_leader",
			taskGroupName: "taskGroupWithLeader",
			replacementState: structs.TaskState{
				StartedAt:   now.Add(-30 * time.Minute),
				Restarts:    2,
				LastRestart: now.Add(-10 * time.Minute),
			},
			originalState: structs.TaskState{
				StartedAt: now.Add(-time.Hour),
				Restarts:  0,
			},
			expected: original,
		},
		{
			name:          "empty_task_group",
			taskGroupName: "emptyTaskGroup",
			replacementState: structs.TaskState{
				StartedAt:   now.Add(-30 * time.Minute),
				Restarts:    2,
				LastRestart: now.Add(-10 * time.Minute),
			},
			originalState: structs.TaskState{
				StartedAt: now.Add(-time.Hour),
				Restarts:  0,
			},
			expected: replacement,
		},
		{
			name:          "original_with_no_restart_on_leader",
			taskGroupName: "taskGroupNoLeader",
			replacementState: structs.TaskState{
				StartedAt:   now.Add(-30 * time.Minute),
				Restarts:    2,
				LastRestart: now.Add(-10 * time.Minute),
			},
			originalState: structs.TaskState{
				StartedAt: now.Add(-time.Hour),
				Restarts:  0,
			},
			expected: original,
		},
		{
			name:          "original_with_older_restarts",
			taskGroupName: "taskGroupNoLeader",
			replacementState: structs.TaskState{
				StartedAt:   now.Add(-30 * time.Minute),
				Restarts:    2,
				LastRestart: now.Add(-10 * time.Minute),
			},
			originalState: structs.TaskState{
				StartedAt:   now.Add(-time.Hour),
				Restarts:    4,
				LastRestart: now.Add(-50 * time.Minute),
			},
			expected: original,
		},
		{
			name:          "original_with_newer_restarts",
			taskGroupName: "taskGroupNoLeader",
			replacementState: structs.TaskState{
				StartedAt:   now.Add(-30 * time.Minute),
				Restarts:    2,
				LastRestart: now.Add(-10 * time.Minute),
			},
			originalState: structs.TaskState{
				StartedAt:   now.Add(-time.Hour),
				Restarts:    4,
				LastRestart: now.Add(-5 * time.Minute),
			},
			expected: replacement,
		},
		{
			name:          "original_with_zero_start_time",
			taskGroupName: "taskGroupNoLeader",
			replacementState: structs.TaskState{
				StartedAt:   now.Add(-30 * time.Minute),
				Restarts:    2,
				LastRestart: now.Add(-10 * time.Minute),
			},
			originalState: structs.TaskState{
				StartedAt: time.Time{},
				Restarts:  0,
			},
			expected: replacement,
		},
		{
			name:          "replacement_with_zero_start_time",
			taskGroupName: "taskGroupNoLeader",
			replacementState: structs.TaskState{
				StartedAt: time.Time{},
				Restarts:  0,
			},
			originalState: structs.TaskState{
				StartedAt:   now.Add(-30 * time.Minute),
				Restarts:    2,
				LastRestart: now.Add(-10 * time.Minute),
			},
			expected: original,
		},
		{
			name:          "both_with_zero_start_time_pick_best_score",
			taskGroupName: "taskGroupNoLeader",
			replacementState: structs.TaskState{
				StartedAt: time.Time{},
				Restarts:  0,
			},
			originalState: structs.TaskState{
				StartedAt: time.Time{},
				Restarts:  0,
			},
			expected: replacement,
		},
	}

	for _, tc := range testsCases {
		t.Run(tc.name, func(t *testing.T) {
			original.TaskGroup = tc.taskGroupName
			replacement.TaskGroup = tc.taskGroupName

			original.TaskStates["task2"] = &tc.originalState
			replacement.TaskStates["task2"] = &tc.replacementState

			result := rp.PickReconnectingAlloc(&structs.DisconnectStrategy{
				Reconcile: structs.ReconcileOptionLongestRunning,
			}, original, replacement)

			must.Eq(t, tc.expected, result)
		})
	}
}
