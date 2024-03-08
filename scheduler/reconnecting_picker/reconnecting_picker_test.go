// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconnectingpicker

import (
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestPickReconnectingAlloc_NewerVersion(t *testing.T) {
	rp := New(nil)
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

func TestPickReconnectingAlloc_LongestRunning(t *testing.T) {
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
			"task1": {},
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
			"task1": {},
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
		name             string
		originalState    structs.TaskState
		replacementState structs.TaskState
		expected         *structs.Allocation
	}{
		{
			name: "original_with_no_restart",
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
			name: "original_with_older_restarts",
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
			name: "original_with_newer_restarts",
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
			name: "original_with_zero_start_time",
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
			name: "replacement_with_zero_start_time",
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
			name: "both_with_zero_start_time",
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
			original.TaskStates["task1"] = &tc.originalState
			replacement.TaskStates["task1"] = &tc.replacementState

			result := rp.PickReconnectingAlloc(&structs.DisconnectStrategy{
				Reconcile: structs.ReconcileOptionLongestRunning,
			}, original, replacement)

			must.Eq(t, tc.expected, result)
		})
	}

}
