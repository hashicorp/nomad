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
	"github.com/shoenig/test/must"
	"pgregory.net/rapid"
)

const maxAllocs = 30

func TestAllocReconciler_PropTest(t *testing.T) {

	// collectExpected returns a convenience map that may hold multiple "states" for
	// the same alloc (ex. all three of "total" and "terminal" and "failed")
	collectExpected := func(t *rapid.T, ar *AllocReconciler) map[string]map[string]int {
		t.Helper()

		perTaskGroup := map[string]map[string]int{}
		for _, tg := range ar.jobState.Job.TaskGroups {
			perTaskGroup[tg.Name] = map[string]int{"expect_count": tg.Count}
			if tg.Update != nil {
				perTaskGroup[tg.Name]["max_canaries"] = tg.Update.Canary
			}
		}
		for _, alloc := range ar.jobState.ExistingAllocs {
			if _, ok := perTaskGroup[alloc.TaskGroup]; !ok {
				// existing task group doesn't exist in new job
				perTaskGroup[alloc.TaskGroup] = map[string]int{"expect_count": 0}
			}
			perTaskGroup[alloc.TaskGroup]["exist_total"]++
			perTaskGroup[alloc.TaskGroup]["exist_"+alloc.ClientStatus]++
			perTaskGroup[alloc.TaskGroup]["exist_desired_"+alloc.DesiredStatus]++
			if alloc.TerminalStatus() {
				perTaskGroup[alloc.TaskGroup]["exist_terminal"]++
			} else {
				perTaskGroup[alloc.TaskGroup]["exist_non_terminal"]++
			}
			if alloc.ClientTerminalStatus() {
				perTaskGroup[alloc.TaskGroup]["exist_client_terminal"]++
			} else {
				perTaskGroup[alloc.TaskGroup]["exist_non_client_terminal"]++
			}
			if alloc.ServerTerminalStatus() {
				perTaskGroup[alloc.TaskGroup]["exist_server_terminal"]++
			} else {
				perTaskGroup[alloc.TaskGroup]["exist_non_server_terminal"]++
			}

			if alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Canary {
				perTaskGroup[alloc.TaskGroup]["exist_canary"]++
			}
		}

		// these only assert our categories are reasonable

		for _, counts := range perTaskGroup {
			must.Eq(t, counts["exist_total"],
				(counts["exist_pending"] +
					counts["exist_running"] +
					counts["exist_complete"] +
					counts["exist_failed"] +
					counts["exist_lost"] +
					counts["exist_unknown"]),
				must.Sprintf("exist_total doesn't add up: %+v", counts))

			must.Eq(t, counts["exist_client_terminal"],
				(counts["exist_complete"] +
					counts["exist_failed"] +
					counts["exist_lost"]),
				must.Sprintf("exist_client_terminal doesn't add up: %+v", counts))
		}

		return perTaskGroup
	}

	// sharedSafetyProperties asserts safety properties ("something bad never
	// happens") that apply to all job types that use the cluster reconciler
	sharedSafetyProperties := func(t *rapid.T, ar *AllocReconciler, results *ReconcileResults, perTaskGroup map[string]map[string]int) {
		t.Helper()

		// stopped jobs
		if ar.jobState.Job.Stopped() {
			if ar.jobState.DeploymentCurrent != nil {
				if results.Deployment != nil {
					t.Fatal("stopped jobs with current deployments should never result in a new deployment")
				}
				if results.Stop == nil {
					t.Fatal("stopped jobs with current deployments should always have stopped allocs")
				}
			}
		}

		must.NotNil(t, results.DesiredTGUpdates,
			must.Sprint("desired task group updates should always be initialized"))

		if ar.jobState.DeploymentFailed && results.Deployment != nil {
			t.Fatal("failed deployments should never result in new deployments")
		}

		if !ar.clusterState.SupportsDisconnectedClients && results.ReconnectUpdates != nil {
			t.Fatal("task groups that don't support disconnected clients should never result in reconnect updates")
		}

		if ar.jobState.DeploymentCurrent == nil && ar.jobState.DeploymentOld == nil && len(ar.jobState.ExistingAllocs) == 0 {
			count := 0
			for _, tg := range ar.jobState.Job.TaskGroups {
				count += tg.Count
			}
			if len(results.Place) > count {
				t.Fatal("for new jobs, amount of allocs to place should never exceed total tg count")
			}
		}

		for tgName, counts := range perTaskGroup {
			tgUpdates := results.DesiredTGUpdates[tgName]

			tprintf := func(msg string) must.Setting {
				return must.Sprintf(msg+" (%s) %v => %+v", tgName, counts, tgUpdates)
			}

			// when the job is stopped or scaled to zero we can make stronger
			// assertions, so split out these checks
			if counts["expect_count"] == 0 || ar.jobState.Job.Stopped() {

				must.Eq(t, 0, int(tgUpdates.Place),
					tprintf("no placements on stop or scale-to-zero"))
				must.Eq(t, 0, int(tgUpdates.Canary),
					tprintf("no canaries on stop or scale-to-zero"))
				must.Eq(t, 0, int(tgUpdates.DestructiveUpdate),
					tprintf("no destructive updates on stop or scale-to-zero"))
				must.Eq(t, 0, int(tgUpdates.Migrate),
					tprintf("no migrating on stop or scale-to-zero"))
				must.Eq(t, 0, int(tgUpdates.RescheduleLater),
					tprintf("no rescheduling later on stop or scale-to-zero"))
				must.Eq(t, 0, int(tgUpdates.Preemptions),
					tprintf("no preemptions on stop or scale-to-zero"))

				continue
			}

			must.LessEq(t, counts["expect_count"], int(tgUpdates.Place),
				tprintf("group placements should never exceed group count"))

			must.LessEq(t, counts["max_canaries"], int(tgUpdates.Canary),
				tprintf("canaries should never exceed expected canaries"))

			must.LessEq(t, counts["max_canaries"], int(tgUpdates.Canary)+counts["exist_canary"],
				tprintf("canaries+existing canaries should never exceed expected canaries"))

			must.LessEq(t, counts["expect_count"], int(tgUpdates.DestructiveUpdate),
				tprintf("destructive updates should never exceed group count"))

			must.LessEq(t, counts["expect_count"]+counts["max_canaries"],
				int(tgUpdates.Canary)+int(tgUpdates.Place)+int(tgUpdates.DestructiveUpdate),
				tprintf("place+canaries+destructive should never exceed group count + expected canaries"))

			must.LessEq(t, counts["exist_non_client_terminal"], int(tgUpdates.Reconnect),
				tprintf("reconnected should never exceed non-client-terminal"))

			must.LessEq(t, counts["exist_total"], int(tgUpdates.InPlaceUpdate),
				tprintf("in-place updates should never exceed existing allocs"))

			must.LessEq(t, counts["exist_total"], int(tgUpdates.DestructiveUpdate),
				tprintf("destructive updates should never exceed existing allocs"))

			must.LessEq(t, counts["exist_total"], int(tgUpdates.Migrate),
				tprintf("migrations should never exceed existing allocs"))

			must.LessEq(t, counts["exist_total"], int(tgUpdates.Ignore),
				tprintf("ignore should never exceed existing allocs"))

			must.GreaterEq(t, tgUpdates.Migrate, tgUpdates.Stop,
				tprintf("migrated allocs should be stopped"))

			must.GreaterEq(t, tgUpdates.RescheduleLater, tgUpdates.Ignore,
				tprintf("reschedule-later allocs should be ignored"))
		}

	}

	t.Run("batch jobs", rapid.MakeCheck(func(t *rapid.T) {
		ar := genAllocReconciler(structs.JobTypeBatch, &idGenerator{}).Draw(t, "reconciler")
		perTaskGroup := collectExpected(t, ar)
		results := ar.Compute()
		must.NotNil(t, results, must.Sprint("results should never be nil"))

		sharedSafetyProperties(t, ar, results, perTaskGroup)
	}))

	t.Run("service jobs", rapid.MakeCheck(func(t *rapid.T) {
		ar := genAllocReconciler(structs.JobTypeService, &idGenerator{}).Draw(t, "reconciler")
		perTaskGroup := collectExpected(t, ar)
		results := ar.Compute()
		must.NotNil(t, results, must.Sprint("results should never be nil"))

		sharedSafetyProperties(t, ar, results, perTaskGroup)
	}))
}

func TestAllocReconciler_cancelUnneededCanaries(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		idg := &idGenerator{}
		job := genJob(
			rapid.SampledFrom([]string{structs.JobTypeService, structs.JobTypeBatch}).Draw(t, "job_type"),
			idg,
		).Draw(t, "job")

		clusterState := genClusterState(idg, time.Now()).Draw(t, "cluster_state")
		jobState := genReconcilerState(idg, job, clusterState).Draw(t, "reconciler_state")

		logger := testlog.HCLogger(t)
		ar := NewAllocReconciler(logger, allocUpdateFnInplace, jobState, clusterState)

		m := newAllocMatrix(job, jobState.ExistingAllocs)
		group := job.TaskGroups[0].Name
		all := m[group] // <-- allocset of all allocs for tg
		all, _ = filterOldTerminalAllocs(jobState, all)

		// runs the method under test
		canaries, _, stopAllocs := ar.cancelUnneededCanaries(all, new(structs.DesiredUpdates))

		expectedStopped := []string{}
		if jobState.DeploymentOld != nil {
			for _, dstate := range jobState.DeploymentOld.TaskGroups {
				if !dstate.Promoted {
					expectedStopped = append(expectedStopped, dstate.PlacedCanaries...)
				}
			}
		}
		if jobState.DeploymentCurrent != nil && jobState.DeploymentCurrent.Status == structs.DeploymentStatusFailed {
			for _, dstate := range jobState.DeploymentCurrent.TaskGroups {
				if !dstate.Promoted {
					expectedStopped = append(expectedStopped, dstate.PlacedCanaries...)
				}
			}
		}
		stopSet := all.fromKeys(expectedStopped)
		all = all.difference(stopSet)

		expectedCanaries := []string{}
		if jobState.DeploymentCurrent != nil {
			for _, dstate := range jobState.DeploymentCurrent.TaskGroups {
				expectedCanaries = append(expectedCanaries, dstate.PlacedCanaries...)
			}
		}
		canarySet := all.fromKeys(expectedCanaries)
		canariesOnUntaintedNodes, migrate, lost, _, _, _, _ := filterByTainted(canarySet, clusterState)

		stopSet = stopSet.union(migrate, lost)

		must.Eq(t, len(stopAllocs), len(stopSet))
		must.Eq(t, len(canaries), len(canariesOnUntaintedNodes))
	})
}

func genAllocReconciler(jobType string, idg *idGenerator) *rapid.Generator[*AllocReconciler] {
	return rapid.Custom(func(t *rapid.T) *AllocReconciler {
		now := time.Now() // note: you can only use offsets from this

		clusterState := genClusterState(idg, now).Draw(t, "cluster_state")
		job := genJob(jobType, idg).Draw(t, "job")

		reconcilerState := genReconcilerState(idg, job, clusterState).Draw(t, "reconciler_state")
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

func genClusterState(idg *idGenerator, now time.Time) *rapid.Generator[ClusterState] {
	return rapid.Custom(func(t *rapid.T) ClusterState {
		nodes := rapid.SliceOfN(genNode(idg), 0, 5).Draw(t, "nodes")
		taintedNodes := helper.SliceToMap[map[string]*structs.Node](
			nodes, func(n *structs.Node) string { return n.ID })

		return ClusterState{
			TaintedNodes:                taintedNodes,
			SupportsDisconnectedClients: rapid.Bool().Draw(t, "supports_disconnected_clients"),
			Now:                         now,
		}
	})
}

func genReconcilerState(idg *idGenerator, job *structs.Job, clusterState ClusterState) *rapid.Generator[ReconcilerState] {
	return rapid.Custom(func(t *rapid.T) ReconcilerState {
		oldJob := job.Copy()
		oldJob.Version--
		oldJob.JobModifyIndex = 100
		oldJob.CreateIndex = 100

		currentAllocs := rapid.SliceOfN(
			genExistingAllocMaybeTainted(idg, job, clusterState.TaintedNodes, clusterState.Now), 0, 15).Draw(t, "allocs")
		oldAllocs := rapid.SliceOfN(
			genExistingAllocMaybeTainted(idg, oldJob, clusterState.TaintedNodes, clusterState.Now), 0, 15).Draw(t, "old_allocs")

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

		return ReconcilerState{
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
			ID:             "jobID",
			Name:           "jobID",
			Type:           jobType,
			TaskGroups:     rapid.SliceOfN(genTaskGroup(idg), 1, 3).Draw(t, "task_groups"),
			Version:        3, // this gives us room to have older allocs
			Stop:           weightedBool(30).Draw(t, "job_stopped"),
			CreateIndex:    1000,
			JobModifyIndex: 1000,
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
				Time:  now.Add(time.Minute * time.Duration(-rapid.IntRange(0, 5).Draw(t, ""))),
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

func TestAllocReconciler_ReconnectingProps(t *testing.T) {

	rapid.Check(t, func(t *rapid.T) {
		now := time.Now()

		idg := &idGenerator{}
		job := genJob(structs.JobTypeBatch, idg).Draw(t, "job")
		tg := job.TaskGroups[0]
		tg.Disconnect.Reconcile = rapid.SampledFrom([]string{
			"", structs.ReconcileOptionBestScore, structs.ReconcileOptionLongestRunning,
			structs.ReconcileOptionKeepOriginal, structs.ReconcileOptionKeepReplacement},
		).Draw(t, "strategy")

		tg.Tasks = []*structs.Task{{Name: "task"}}

		reconnecting, all := allocSet{}, allocSet{}
		reconnectingAllocs := rapid.SliceOfN(
			genExistingAlloc(idg, job, idg.nextID(), now), 1, 10).Draw(t, "allocs")
		for _, alloc := range reconnectingAllocs {
			numRestarts := rapid.IntRange(0, 2).Draw(t, "")
			startTime := now.Add(-time.Minute * time.Duration(rapid.IntRange(2, 5).Draw(t, "")))
			lastRestart := startTime.Add(time.Minute)
			alloc.TaskStates = map[string]*structs.TaskState{"task": {
				Restarts:    uint64(numRestarts),
				LastRestart: lastRestart,
				StartedAt:   startTime,
			}}
			reconnecting[alloc.ID] = alloc
		}

		allAllocs := rapid.SliceOfN(
			genExistingAlloc(idg, job, idg.nextID(), now), 0, 10).Draw(t, "allocs")
		for i, alloc := range allAllocs {
			numRestarts := rapid.IntRange(0, 2).Draw(t, "")
			startTime := now.Add(-time.Minute * time.Duration(rapid.IntRange(2, 5).Draw(t, "")))
			lastRestart := startTime.Add(time.Minute)
			alloc.TaskStates = map[string]*structs.TaskState{"task": {
				Restarts:    uint64(numRestarts),
				LastRestart: lastRestart,
				StartedAt:   startTime,
			}}

			// wire up the next/previous relationship for a subset
			if i%2 == 0 && len(reconnecting) > i {
				alloc.PreviousAllocation = reconnectingAllocs[i].ID
				reconnecting[alloc.PreviousAllocation].NextAllocation = alloc.ID
			}
			all[alloc.ID] = alloc
		}

		logger := testlog.HCLogger(t)
		ar := NewAllocReconciler(logger,
			allocUpdateFnInplace, // not relevant to function
			ReconcilerState{Job: job},
			ClusterState{Now: now},
		)

		keep, stop, stopResults := ar.reconcileReconnecting(reconnecting, all, tg)

		for reconnectedID := range reconnecting {
			_, isKeep := keep[reconnectedID]
			_, isStop := stop[reconnectedID]
			if isKeep && isStop {
				t.Fatal("reconnecting alloc should not be both kept and stopped")
			}
			if !(isKeep || isStop) {
				t.Fatal("reconnecting alloc must be either kept or stopped")
			}
		}

		for keepID := range keep {
			if _, ok := reconnecting[keepID]; !ok {
				t.Fatal("only reconnecting allocations are allowed to be present in the returned reconnect set.")
			}
		}
		for stopID := range stop {
			if alloc, ok := reconnecting[stopID]; ok {
				nextID := alloc.NextAllocation
				_, nextIsKeep := keep[nextID]
				_, nextIsStop := stop[nextID]
				if nextIsKeep || nextIsStop {
					t.Fatal("replacements should not be in either set")
				}
			}
		}
		must.Eq(t, len(stop), len(stopResults),
			must.Sprint("every stop should have a stop result"))

	})
}
