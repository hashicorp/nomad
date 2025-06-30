// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"encoding/binary"
	"fmt"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"pgregory.net/rapid"
)

const maxAllocs = 30

func TestAllocReconciler_PropTest(t *testing.T) {
	t.Run("batch jobs", rapid.MakeCheck(func(t *rapid.T) {
		ar := genAllocReconciler(structs.JobTypeBatch, &idGenerator{}).Draw(t, "reconciler")
		results := ar.Compute()

		if results == nil {
			t.Fatal("results should never be nil")
		}
		// TODO(tgross): this where the properties under test go
	}))

	t.Run("service jobs", rapid.MakeCheck(func(t *rapid.T) {
		ar := genAllocReconciler(structs.JobTypeService, &idGenerator{}).Draw(t, "reconciler")
		results := ar.Compute()

		/*
			SAFETY properties ("something bad never happens")
		*/
		if results == nil {
			t.Fatal("results should never be nil")
		}

		// stopped jobs
		if ar.jobState.Job.Stopped() {
			if ar.jobState.DeploymentCurrent != nil {
				if results.Deployment != nil {
					t.Fatal("stopped jobs with current deployments should never result in a new deployment")
				}
				if results.Stop == nil {
					t.Fatal("stopped jobs with current deployments should never have nil stopped allocs")
				}
			}
			if results.DesiredTGUpdates == nil {
				t.Fatal("stopped jobs should result in non-nil desired task group updates")
			}
		}

		for _, tg := range ar.jobState.Job.TaskGroups {
			if tg == nil && results.DesiredTGUpdates[tg.Name].Stop != 0 {
				t.Fatal("nil task groups should never have non-empty sets of allocs to stop")
			}
		}

		/*
			LIVENESS properties ("something good eventually happens")
		*/

	}))
}

func genAllocReconciler(jobType string, idg *idGenerator) *rapid.Generator[*AllocReconciler] {
	return rapid.Custom(func(t *rapid.T) *AllocReconciler {
		now := time.Now() // note: you can only use offsets from this

		nodes := rapid.SliceOfN(genNode(idg), 0, 5).Draw(t, "nodes")
		taintedNodes := helper.SliceToMap[map[string]*structs.Node](
			nodes, func(n *structs.Node) string { return n.ID })

		clusterState := ClusterState{
			TaintedNodes:                taintedNodes,
			SupportsDisconnectedClients: rapid.Bool().Draw(t, "supports_disconnected_clients"),
			Now:                         now,
		}
		job := genJob(jobType, idg).Draw(t, "job")
		oldJob := job.Copy()
		oldJob.Version--
		oldJob.CreateIndex = 100

		currentAllocs := rapid.SliceOfN(
			genExistingAllocMaybeTainted(idg, job, taintedNodes, now), 0, 15).Draw(t, "allocs")
		oldAllocs := rapid.SliceOfN(
			genExistingAllocMaybeTainted(idg, oldJob, taintedNodes, now), 0, 15).Draw(t, "old_allocs")

		// tie together a subset of allocations so we can exercise reconnection
		previousAllocID := ""
		for i, alloc := range currentAllocs {
			if i%3 == 0 {
				alloc.NextAllocation = previousAllocID
			} else {
				previousAllocID = alloc.ID
			}
		}

		allocs := append(currentAllocs, oldAllocs...)

		// note: either of these might return nil
		oldDeploy := genDeployment(idg, oldJob, oldAllocs).Draw(t, "old_deploy")
		currentDeploy := genDeployment(idg, job, currentAllocs).Draw(t, "current_deploy")

		reconcilerState := ReconcilerState{
			Job:               job,
			JobID:             job.ID,
			JobIsBatch:        job.Type == structs.JobTypeBatch,
			DeploymentOld:     oldDeploy,
			DeploymentCurrent: currentDeploy,
			DeploymentPaused:  currentDeploy != nil && currentDeploy.Status == structs.DeploymentStatusPaused,
			DeploymentFailed:  currentDeploy != nil && currentDeploy.Status == structs.DeploymentStatusFailed,
			ExistingAllocs:    allocs,
			EvalID:            idg.nextID(),
		}

		updateFn := rapid.SampledFrom([]AllocUpdateType{
			allocUpdateFnDestructive,
			allocUpdateFnIgnore,
			allocUpdateFnInplace,
		}).Draw(t, "update_function")

		logger := testlog.HCLogger(t)
		ar := NewAllocReconciler(logger,
			updateFn,
			reconcilerState,
			clusterState,
		)

		return ar
	})
}

func genDeployment(idg *idGenerator, job *structs.Job, allocs []*structs.Allocation) *rapid.Generator[*structs.Deployment] {
	return rapid.Custom(func(t *rapid.T) *structs.Deployment {
		if rapid.Bool().Draw(t, "deploy_is_nil") {
			return nil
		}

		unusedAllocs := helper.SliceToMap[map[string]*structs.Allocation](
			allocs, func(a *structs.Allocation) string { return a.ID })

		dstates := map[string]*structs.DeploymentState{}
		for _, tg := range job.TaskGroups {
			dstate := &structs.DeploymentState{
				AutoRevert:        tg.Update.AutoRevert,
				AutoPromote:       tg.Update.AutoPromote,
				ProgressDeadline:  tg.Update.ProgressDeadline,
				RequireProgressBy: time.Time{},
				Promoted:          rapid.Bool().Draw(t, "promoted"),
				PlacedCanaries:    []string{},
				DesiredCanaries:   tg.Update.Canary,
				DesiredTotal:      tg.Count,
				PlacedAllocs:      0,
				HealthyAllocs:     0,
				UnhealthyAllocs:   0,
			}
			for id, alloc := range unusedAllocs {
				if alloc.TaskGroup == tg.Name {
					dstate.PlacedAllocs++
					if alloc.ClientTerminalStatus() {
						dstate.UnhealthyAllocs++
					} else if alloc.ClientStatus == structs.AllocClientStatusRunning {
						dstate.HealthyAllocs++
					}
					// consume the allocs as canaries first
					if len(dstate.PlacedCanaries) < dstate.DesiredCanaries {
						dstate.PlacedCanaries = append(dstate.PlacedCanaries, id)
					}

					delete(unusedAllocs, id)
				}
			}

			dstates[tg.Name] = dstate
		}

		return &structs.Deployment{
			ID:                 idg.nextID(),
			Namespace:          job.Namespace,
			JobID:              job.ID,
			JobVersion:         job.Version,
			JobModifyIndex:     0,
			JobSpecModifyIndex: 0,
			JobCreateIndex:     job.CreateIndex,
			IsMultiregion:      false,
			TaskGroups:         dstates,
			Status: rapid.SampledFrom([]string{
				structs.DeploymentStatusRunning,
				structs.DeploymentStatusPending,
				structs.DeploymentStatusInitializing,
				structs.DeploymentStatusPaused,
				structs.DeploymentStatusFailed,
				structs.DeploymentStatusSuccessful,
			}).Draw(t, "deployment_status"),
			StatusDescription: "",
			EvalPriority:      0,
			CreateIndex:       job.CreateIndex,
			ModifyIndex:       0,
			CreateTime:        0,
			ModifyTime:        0,
		}
	})
}

func genNode(idg *idGenerator) *rapid.Generator[*structs.Node] {
	return rapid.Custom(func(t *rapid.T) *structs.Node {

		status := rapid.SampledFrom([]string{
			structs.NodeStatusReady,
			structs.NodeStatusReady,
			structs.NodeStatusReady,
			structs.NodeStatusDown,
			structs.NodeStatusDisconnected}).Draw(t, "node_status")

		// for the node to be both tainted and ready, it must be draining
		var drainStrat *structs.DrainStrategy
		if status == structs.NodeStatusReady && weightedBool(30).Draw(t, "is_draining") {
			drainStrat = &structs.DrainStrategy{ // TODO(tgross): what else should we specify?
				DrainSpec: structs.DrainSpec{
					Deadline:         0,
					IgnoreSystemJobs: false,
				},
				ForceDeadline: time.Time{},
				StartedAt:     time.Time{},
			}
		}

		return &structs.Node{
			ID:                    idg.nextID(),
			Status:                status,
			DrainStrategy:         drainStrat,
			SchedulingEligibility: structs.NodeSchedulingEligible,
		}
	})
}

func genJob(jobType string, idg *idGenerator) *rapid.Generator[*structs.Job] {
	return rapid.Custom(func(t *rapid.T) *structs.Job {
		return &structs.Job{
			ID:          "jobID",
			Name:        "jobID",
			Type:        jobType,
			TaskGroups:  rapid.SliceOfN(genTaskGroup(idg), 1, 3).Draw(t, "task_groups"),
			Version:     3, // this gives us room to have older allocs
			Stop:        weightedBool(30).Draw(t, "job_stopped"),
			CreateIndex: 1000,
		}
	})
}

// weightedBool returns a biased boolean picker
func weightedBool(truePct int) *rapid.Generator[bool] {
	return rapid.Custom(func(t *rapid.T) bool {
		i := rapid.IntRange(0, 100).Draw(t, "weighting")
		return i <= truePct
	})
}

// maybeDuration returns either an empty duration or the fixed amount
func maybeDuration(nonePct, dur int) *rapid.Generator[time.Duration] {
	return rapid.Custom(func(t *rapid.T) time.Duration {
		i := rapid.IntRange(0, 100).Draw(t, "weighting")
		if i <= nonePct {
			return time.Duration(0)
		}
		return time.Duration(dur)
	})
}

func genTaskGroup(idg *idGenerator) *rapid.Generator[*structs.TaskGroup] {
	return rapid.Custom(func(t *rapid.T) *structs.TaskGroup {
		tgCount := rapid.IntRange(0, maxAllocs).Draw(t, "tg_count")

		return &structs.TaskGroup{
			Count:  tgCount,
			Name:   idg.nextName(),
			Update: genUpdateBlock(tgCount).Draw(t, "tg_update_block"),
			Disconnect: &structs.DisconnectStrategy{
				LostAfter: maybeDuration(50, 300).Draw(t, "disconnect:lost_after"),
				Replace:   pointer.Of(rapid.Bool().Draw(t, "disconnect:replace")),
				Reconcile: structs.ReconcileOptionBestScore,
			},
			// we'll use a fairly static policy and then use the alloc
			// reschedule tracker to introduce dimensions to test
			ReschedulePolicy: &structs.ReschedulePolicy{
				Attempts:      3,
				Interval:      time.Hour,
				Delay:         90 * time.Second,
				DelayFunction: "constant",
				MaxDelay:      time.Hour,
				Unlimited:     rapid.Bool().Draw(t, "reschedule.unlimited"),
			},
			EphemeralDisk: &structs.EphemeralDisk{}, // avoids a panic for in-place updates
		}
	})
}

func genExistingAlloc(idg *idGenerator, job *structs.Job, nodeID string, now time.Time) *rapid.Generator[*structs.Allocation] {
	return rapid.Custom(func(t *rapid.T) *structs.Allocation {
		clientStatus := rapid.SampledFrom([]string{
			structs.AllocClientStatusPending,
			structs.AllocClientStatusRunning,
			structs.AllocClientStatusComplete,
			structs.AllocClientStatusFailed,
			structs.AllocClientStatusLost,
			structs.AllocClientStatusUnknown}).Draw(t, "alloc_client_status")

		desiredStatus := rapid.SampledFrom([]string{
			structs.AllocDesiredStatusRun,
			structs.AllocDesiredStatusEvict,
			structs.AllocDesiredStatusStop,
		}).Draw(t, "desired_status")

		hasDisconnect := weightedBool(40).Draw(t, "has_disconnect")
		var allocStates []*structs.AllocState
		if hasDisconnect {
			allocStates = append(allocStates, &structs.AllocState{
				Field: structs.AllocStateFieldClientStatus,
				Value: "unknown",
			})
		}

		tg := rapid.SampledFrom(helper.ConvertSlice(
			job.TaskGroups, func(g *structs.TaskGroup) string { return g.Name })).Draw(t, "tg")

		alloc := &structs.Allocation{
			ID:            idg.nextID(),
			Name:          idg.nextAllocName(tg),
			NodeID:        nodeID,
			JobID:         job.ID,
			Job:           job,
			TaskGroup:     tg,
			ClientStatus:  clientStatus,
			DesiredStatus: desiredStatus,
			AllocStates:   allocStates,
			// TODO(tgross): need to figure out a way to set these sensibly
			// DesiredTransition: structs.DesiredTransition{
			// 	Migrate:         new(bool),
			// 	Reschedule:      new(bool),
			// 	ForceReschedule: new(bool),
			// 	NoShutdownDelay: new(bool),
			// },
		}
		if alloc.ClientTerminalStatus() {
			numEvents := rapid.IntRange(0, 3).Draw(t, "reschedule_tracker_events")
			if numEvents != 0 {
				alloc.RescheduleTracker = &structs.RescheduleTracker{
					Events: []*structs.RescheduleEvent{}}
			}
			for i := range numEvents {
				alloc.RescheduleTracker.Events = append(alloc.RescheduleTracker.Events,
					&structs.RescheduleEvent{
						RescheduleTime: now.Add(time.Minute * time.Duration(-i)).UnixNano(),
						PrevAllocID:    idg.nextID(),
						PrevNodeID:     idg.nextID(),
					},
				)
			}
		}

		return alloc
	})
}

func genExistingAllocMaybeTainted(idg *idGenerator, job *structs.Job, taintedNodes map[string]*structs.Node, now time.Time) *rapid.Generator[*structs.Allocation] {
	return rapid.Custom(func(t *rapid.T) *structs.Allocation {

		// determine if we're going to place this existing alloc on a tainted node
		// or an untainted node (make up an ID)
		onTainted := rapid.Bool().Draw(t, "onTainted")
		var nodeID string
		if onTainted && len(taintedNodes) != 0 {
			nodeID = rapid.SampledFrom(slices.Collect(maps.Keys(taintedNodes))).Draw(t, "nodeID")
		} else {
			nodeID = idg.nextID()
		}

		return genExistingAlloc(idg, job, nodeID, now).Draw(t, "alloc")
	})
}

func genUpdateBlock(tgCount int) *rapid.Generator[*structs.UpdateStrategy] {
	return rapid.Custom(func(t *rapid.T) *structs.UpdateStrategy {
		mp := rapid.IntRange(0, tgCount).Draw(t, "max_parallel")
		canaries := rapid.IntRange(0, mp).Draw(t, "canaries")

		return &structs.UpdateStrategy{
			Stagger:     0, // TODO(tgross): need to set this for sysbatch/system
			MaxParallel: mp,
			AutoRevert:  rapid.Bool().Draw(t, "auto_revert"),
			AutoPromote: rapid.Bool().Draw(t, "auto_promote"),
			Canary:      canaries,
		}
	})
}

// idGenerator is used to generate unique-per-test IDs and names that don't
// impact the test results, without using the rapid library generators. This
// prevents them from being used as a dimension for fuzzing, which allows us to
// shrink only dimensions we care about on failure.
type idGenerator struct {
	index uint64
}

// nextName is used to generate a unique-per-test short string
func (idg *idGenerator) nextName() string {
	idg.index++
	return fmt.Sprintf("name-%d", idg.index)
}

func (idg *idGenerator) nextAllocName(tg string) string {
	idg.index++
	return fmt.Sprintf("%s[%d]", tg, idg.index)
}

// nextID is used to generate a unique-per-test UUID
func (idg *idGenerator) nextID() string {
	idg.index++
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint64(buf, idg.index)

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}
