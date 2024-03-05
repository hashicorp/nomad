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
	original := &structs.Allocation{
		Job: &structs.Job{
			Version:     1,
			CreateIndex: 1,
		},
	}
	replacement := &structs.Allocation{
		Job: &structs.Job{
			Version:     2,
			CreateIndex: 2,
		},
	}
	result := rp.PickReconnectingAlloc(ds, original, replacement)
	if result != replacement {
		t.Fatalf("expected replacement, got %v", result)
	}
}

func TestPickReconnectingAlloc(t *testing.T) {
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
				Events: []*structs.TaskEvent{
					{
						Type: structs.TaskStarted,
						Time: now.Add(-time.Hour).UnixNano(),
					},
					{
						Type: structs.TaskClientReconnected,
						Time: now.Add(-20 * time.Minute).UnixNano(),
					},
				},
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
				Events: []*structs.TaskEvent{
					{
						Type: structs.TaskStarted,
						Time: now.Add(-30 * time.Minute).UnixNano(),
					},
				},
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
