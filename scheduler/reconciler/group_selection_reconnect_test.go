// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func cohortAlloc(score float64, start time.Time) *structs.Allocation {
	alloc := mock.Alloc()
	alloc.ClientStatus = structs.AllocClientStatusRunning
	alloc.Metrics = &structs.AllocMetric{ScoreMetaData: []*structs.NodeScoreMeta{{NormScore: score}}}
	alloc.TaskStates = map[string]*structs.TaskState{alloc.Job.LookupTaskGroup(alloc.TaskGroup).Tasks[0].Name: {State: structs.TaskStateRunning, StartedAt: start}}
	return alloc
}

func TestPickGroupSelectionCohort_SingleReplicaNativeParity(t *testing.T) {
	now := time.Now()
	picker := newReconnectingPicker(hclog.NewNullLogger())
	for _, strategy := range []string{structs.ReconcileOptionKeepOriginal, structs.ReconcileOptionKeepReplacement, structs.ReconcileOptionBestScore, structs.ReconcileOptionLongestRunning} {
		t.Run(strategy, func(t *testing.T) {
			ds := &structs.DisconnectStrategy{Reconcile: strategy}
			for _, oldScore := range []float64{0.1, 0.8} {
				for _, newScore := range []float64{0.1, 0.8} {
					for _, oldRunning := range []bool{false, true} {
						for _, newRunning := range []bool{false, true} {
							old, new := cohortAlloc(oldScore, now.Add(-time.Hour)), cohortAlloc(newScore, now.Add(-time.Minute))
							if !oldRunning {
								old.ClientStatus = structs.AllocClientStatusPending
							}
							if !newRunning {
								new.ClientStatus = structs.AllocClientStatusPending
							}
							expected := picker.pickReconnectingAlloc(ds, old, new) == old
							must.Eq(t, expected, PickGroupSelectionCohort(ds, []*structs.Allocation{old}, []*structs.Allocation{new}))
						}
					}
				}
			}
		})
	}
}

func TestPickGroupSelectionCohort_Aggregates(t *testing.T) {
	now := time.Now()
	old := []*structs.Allocation{cohortAlloc(0.9, now.Add(-time.Minute)), cohortAlloc(0.1, now.Add(-time.Hour))}
	replacement := []*structs.Allocation{cohortAlloc(0.6, now.Add(-10*time.Minute)), cohortAlloc(0.6, now.Add(-10*time.Minute)), cohortAlloc(0.6, now.Add(-10*time.Minute))}
	// A lowest-index comparison would choose the original's 0.9. The mean
	// selects the replacement and does not favor its greater replica count.
	must.False(t, PickGroupSelectionCohort(&structs.DisconnectStrategy{Reconcile: structs.ReconcileOptionBestScore}, old, replacement))
	// The second original replica started first, despite the first replica
	// starting more recently than every replacement.
	must.True(t, PickGroupSelectionCohort(&structs.DisconnectStrategy{Reconcile: structs.ReconcileOptionLongestRunning}, old, replacement))
	old[0].Metrics = nil
	old[1].Metrics = nil
	must.False(t, PickGroupSelectionCohort(&structs.DisconnectStrategy{Reconcile: structs.ReconcileOptionBestScore}, old, replacement))
	for _, alloc := range replacement {
		alloc.Metrics = nil
	}
	must.True(t, PickGroupSelectionCohort(&structs.DisconnectStrategy{Reconcile: structs.ReconcileOptionBestScore}, old, replacement))
}
