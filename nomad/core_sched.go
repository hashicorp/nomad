// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
	"golang.org/x/time/rate"
)

// CoreScheduler is a special "scheduler" that is registered
// as "_core". It is used to run various administrative work
// across the cluster.
type CoreScheduler struct {
	srv    *Server
	snap   *state.StateSnapshot
	logger log.Logger
}

// NewCoreScheduler is used to return a new system scheduler instance
func NewCoreScheduler(srv *Server, snap *state.StateSnapshot) scheduler.Scheduler {
	s := &CoreScheduler{
		srv:    srv,
		snap:   snap,
		logger: srv.logger.ResetNamed("core.sched"),
	}
	return s
}

// Process is used to implement the scheduler.Scheduler interface
func (c *CoreScheduler) Process(eval *structs.Evaluation) error {
	job := strings.Split(eval.JobID, ":") // extra data can be smuggled in w/ JobID
	switch job[0] {
	case structs.CoreJobEvalGC:
		return c.evalGC(eval)
	case structs.CoreJobNodeGC:
		return c.nodeGC(eval)
	case structs.CoreJobJobGC:
		return c.jobGC(eval)
	case structs.CoreJobDeploymentGC:
		return c.deploymentGC(eval)
	case structs.CoreJobCSIVolumeClaimGC:
		return c.csiVolumeClaimGC(eval)
	case structs.CoreJobCSIPluginGC:
		return c.csiPluginGC(eval)
	case structs.CoreJobOneTimeTokenGC:
		return c.expiredOneTimeTokenGC(eval)
	case structs.CoreJobLocalTokenExpiredGC:
		return c.expiredACLTokenGC(eval, false)
	case structs.CoreJobGlobalTokenExpiredGC:
		return c.expiredACLTokenGC(eval, true)
	case structs.CoreJobRootKeyRotateOrGC:
		return c.rootKeyRotateOrGC(eval)
	case structs.CoreJobVariablesRekey:
		return c.variablesRekey(eval)
	case structs.CoreJobForceGC:
		return c.forceGC(eval)
	default:
		return fmt.Errorf("core scheduler cannot handle job '%s'", eval.JobID)
	}
}

// forceGC is used to garbage collect all eligible objects.
func (c *CoreScheduler) forceGC(eval *structs.Evaluation) error {
	if err := c.jobGC(eval); err != nil {
		return err
	}
	if err := c.evalGC(eval); err != nil {
		return err
	}
	if err := c.deploymentGC(eval); err != nil {
		return err
	}
	if err := c.csiPluginGC(eval); err != nil {
		return err
	}
	if err := c.csiVolumeClaimGC(eval); err != nil {
		return err
	}
	if err := c.expiredOneTimeTokenGC(eval); err != nil {
		return err
	}
	if err := c.expiredACLTokenGC(eval, false); err != nil {
		return err
	}
	if err := c.expiredACLTokenGC(eval, true); err != nil {
		return err
	}
	if err := c.rootKeyGC(eval); err != nil {
		return err
	}
	// Node GC must occur after the others to ensure the allocations are
	// cleared.
	return c.nodeGC(eval)
}

// jobGC is used to garbage collect eligible jobs.
func (c *CoreScheduler) jobGC(eval *structs.Evaluation) error {
	// Get all the jobs eligible for garbage collection.
	ws := memdb.NewWatchSet()
	iter, err := c.snap.JobsByGC(ws, true)
	if err != nil {
		return err
	}

	oldThreshold := c.getThreshold(eval, "job",
		"job_gc_threshold", c.srv.config.JobGCThreshold)

	// Collect the allocations, evaluations and jobs to GC
	var gcAlloc, gcEval []string
	var gcJob []*structs.Job

OUTER:
	for i := iter.Next(); i != nil; i = iter.Next() {
		job := i.(*structs.Job)

		// Ignore new jobs.
		if job.CreateIndex > oldThreshold {
			continue
		}

		ws := memdb.NewWatchSet()
		evals, err := c.snap.EvalsByJob(ws, job.Namespace, job.ID)
		if err != nil {
			c.logger.Error("job GC failed to get evals for job", "job", job.ID, "error", err)
			continue
		}

		allEvalsGC := true
		var jobAlloc, jobEval []string
		for _, eval := range evals {
			gc, allocs, err := c.gcEval(eval, oldThreshold, true)
			if err != nil {
				continue OUTER
			} else if gc {
				jobEval = append(jobEval, eval.ID)
				jobAlloc = append(jobAlloc, allocs...)
			} else {
				allEvalsGC = false
				break
			}
		}

		// Job is eligible for garbage collection
		if allEvalsGC {
			gcJob = append(gcJob, job)
			gcAlloc = append(gcAlloc, jobAlloc...)
			gcEval = append(gcEval, jobEval...)
		}

	}

	// Fast-path the nothing case
	if len(gcEval) == 0 && len(gcAlloc) == 0 && len(gcJob) == 0 {
		return nil
	}

	c.logger.Debug("job GC found eligible objects",
		"jobs", len(gcJob), "evals", len(gcEval), "allocs", len(gcAlloc))

	// Reap the evals and allocs
	if err := c.evalReap(gcEval, gcAlloc); err != nil {
		return err
	}

	// Reap the jobs
	return c.jobReap(gcJob, eval.LeaderACL)
}

// jobReap contacts the leader and issues a reap on the passed jobs
func (c *CoreScheduler) jobReap(jobs []*structs.Job, leaderACL string) error {
	// Call to the leader to issue the reap
	for _, req := range c.partitionJobReap(jobs, leaderACL, structs.MaxUUIDsPerWriteRequest) {
		var resp structs.JobBatchDeregisterResponse
		if err := c.srv.RPC("Job.BatchDeregister", req, &resp); err != nil {
			c.logger.Error("batch job reap failed", "error", err)
			return err
		}
	}

	return nil
}

// partitionJobReap returns a list of JobBatchDeregisterRequests to make,
// ensuring a single request does not contain too many jobs. This is necessary
// to ensure that the Raft transaction does not become too large.
func (c *CoreScheduler) partitionJobReap(jobs []*structs.Job, leaderACL string, batchSize int) []*structs.JobBatchDeregisterRequest {
	option := &structs.JobDeregisterOptions{Purge: true}
	var requests []*structs.JobBatchDeregisterRequest
	submittedJobs := 0
	for submittedJobs != len(jobs) {
		req := &structs.JobBatchDeregisterRequest{
			Jobs: make(map[structs.NamespacedID]*structs.JobDeregisterOptions),
			WriteRequest: structs.WriteRequest{
				Region:    c.srv.config.Region,
				AuthToken: leaderACL,
			},
		}
		requests = append(requests, req)
		available := batchSize

		if remaining := len(jobs) - submittedJobs; remaining > 0 {
			if remaining <= available {
				for _, job := range jobs[submittedJobs:] {
					jns := structs.NamespacedID{ID: job.ID, Namespace: job.Namespace}
					req.Jobs[jns] = option
				}
				submittedJobs += remaining
			} else {
				for _, job := range jobs[submittedJobs : submittedJobs+available] {
					jns := structs.NamespacedID{ID: job.ID, Namespace: job.Namespace}
					req.Jobs[jns] = option
				}
				submittedJobs += available
			}
		}
	}

	return requests
}

// evalGC is used to garbage collect old evaluations
func (c *CoreScheduler) evalGC(eval *structs.Evaluation) error {
	// Iterate over the evaluations
	ws := memdb.NewWatchSet()
	iter, err := c.snap.Evals(ws, false)
	if err != nil {
		return err
	}

	oldThreshold := c.getThreshold(eval, "eval",
		"eval_gc_threshold", c.srv.config.EvalGCThreshold)
	batchOldThreshold := c.getThreshold(eval, "eval",
		"batch_eval_gc_threshold", c.srv.config.BatchEvalGCThreshold)

	// Collect the allocations and evaluations to GC
	var gcAlloc, gcEval []string
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		eval := raw.(*structs.Evaluation)

		gcThreshold := oldThreshold
		if eval.Type == structs.JobTypeBatch {
			gcThreshold = batchOldThreshold
		}

		gc, allocs, err := c.gcEval(eval, gcThreshold, false)
		if err != nil {
			return err
		}

		if gc {
			gcEval = append(gcEval, eval.ID)
		}
		gcAlloc = append(gcAlloc, allocs...)
	}

	// Fast-path the nothing case
	if len(gcEval) == 0 && len(gcAlloc) == 0 {
		return nil
	}
	c.logger.Debug("eval GC found eligibile objects",
		"evals", len(gcEval), "allocs", len(gcAlloc))

	return c.evalReap(gcEval, gcAlloc)
}

// gcEval returns whether the eval should be garbage collected given a raft
// threshold index. The eval disqualifies for garbage collection if it or its
// allocs are not older than the threshold. If the eval should be garbage
// collected, the associated alloc ids that should also be removed are also
// returned
func (c *CoreScheduler) gcEval(eval *structs.Evaluation, thresholdIndex uint64, allowBatch bool) (
	bool, []string, error) {
	// Ignore non-terminal and new evaluations
	if !eval.TerminalStatus() || eval.ModifyIndex > thresholdIndex {
		return false, nil, nil
	}

	// Create a watchset
	ws := memdb.NewWatchSet()

	// Look up the job
	job, err := c.snap.JobByID(ws, eval.Namespace, eval.JobID)
	if err != nil {
		return false, nil, err
	}

	// Get the allocations by eval
	allocs, err := c.snap.AllocsByEval(ws, eval.ID)
	if err != nil {
		c.logger.Error("failed to get allocs for eval",
			"eval_id", eval.ID, "error", err)
		return false, nil, err
	}

	// If the eval is from a running "batch" job we don't want to garbage
	// collect its most current allocations. If there is a long running batch job and its
	// terminal allocations get GC'd the scheduler would re-run the allocations. However,
	// we do want to GC old Evals and Allocs if there are newer ones due to update.
	//
	// The age of the evaluation must also reach the threshold configured to be GCed so that
	// one may debug old evaluations and referenced allocations.
	if eval.Type == structs.JobTypeBatch {
		// Check if the job is running

		// Can collect if either holds:
		//   - Job doesn't exist
		//   - Job is Stopped and dead
		//   - allowBatch and the job is dead
		//
		// If we cannot collect outright, check if a partial GC may occur
		collect := job == nil || job.Status == structs.JobStatusDead && (job.Stop || allowBatch)
		if !collect {
			oldAllocs := olderVersionTerminalAllocs(allocs, job, thresholdIndex)
			gcEval := (len(oldAllocs) == len(allocs))
			return gcEval, oldAllocs, nil
		}
	}

	// Scan the allocations to ensure they are terminal and old
	gcEval := true
	var gcAllocIDs []string
	for _, alloc := range allocs {
		if !allocGCEligible(alloc, job, time.Now(), thresholdIndex) {
			// Can't GC the evaluation since not all of the allocations are
			// terminal
			gcEval = false
		} else {
			// The allocation is eligible to be GC'd
			gcAllocIDs = append(gcAllocIDs, alloc.ID)
		}
	}

	return gcEval, gcAllocIDs, nil
}

// olderVersionTerminalAllocs returns a list of terminal allocations that belong to the evaluation and may be
// GCed.
func olderVersionTerminalAllocs(allocs []*structs.Allocation, job *structs.Job, thresholdIndex uint64) []string {
	var ret []string
	for _, alloc := range allocs {
		if alloc.CreateIndex < job.JobModifyIndex && alloc.ModifyIndex < thresholdIndex && alloc.TerminalStatus() {
			ret = append(ret, alloc.ID)
		}
	}
	return ret
}

// evalReap contacts the leader and issues a reap on the passed evals and
// allocs.
func (c *CoreScheduler) evalReap(evals, allocs []string) error {
	// Call to the leader to issue the reap
	for _, req := range c.partitionEvalReap(evals, allocs, structs.MaxUUIDsPerWriteRequest) {
		var resp structs.GenericResponse
		if err := c.srv.RPC("Eval.Reap", req, &resp); err != nil {
			c.logger.Error("eval reap failed", "error", err)
			return err
		}
	}

	return nil
}

// partitionEvalReap returns a list of EvalReapRequest to make, ensuring a single
// request does not contain too many allocations and evaluations. This is
// necessary to ensure that the Raft transaction does not become too large.
func (c *CoreScheduler) partitionEvalReap(evals, allocs []string, batchSize int) []*structs.EvalReapRequest {
	var requests []*structs.EvalReapRequest
	submittedEvals, submittedAllocs := 0, 0
	for submittedEvals != len(evals) || submittedAllocs != len(allocs) {
		req := &structs.EvalReapRequest{
			WriteRequest: structs.WriteRequest{
				Region: c.srv.config.Region,
			},
		}
		requests = append(requests, req)
		available := batchSize

		// Add the allocs first
		if remaining := len(allocs) - submittedAllocs; remaining > 0 {
			if remaining <= available {
				req.Allocs = allocs[submittedAllocs:]
				available -= remaining
				submittedAllocs += remaining
			} else {
				req.Allocs = allocs[submittedAllocs : submittedAllocs+available]
				submittedAllocs += available

				// Exhausted space so skip adding evals
				continue
			}
		}

		// Add the evals
		if remaining := len(evals) - submittedEvals; remaining > 0 {
			if remaining <= available {
				req.Evals = evals[submittedEvals:]
				submittedEvals += remaining
			} else {
				req.Evals = evals[submittedEvals : submittedEvals+available]
				submittedEvals += available
			}
		}
	}

	return requests
}

// nodeGC is used to garbage collect old nodes
func (c *CoreScheduler) nodeGC(eval *structs.Evaluation) error {
	// Iterate over the evaluations
	ws := memdb.NewWatchSet()
	iter, err := c.snap.Nodes(ws)
	if err != nil {
		return err
	}

	oldThreshold := c.getThreshold(eval, "node",
		"node_gc_threshold", c.srv.config.NodeGCThreshold)

	// Collect the nodes to GC
	var gcNode []string
OUTER:
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		node := raw.(*structs.Node)

		// Ignore non-terminal and new nodes
		if !node.TerminalStatus() || node.ModifyIndex > oldThreshold {
			continue
		}

		// Get the allocations by node
		ws := memdb.NewWatchSet()
		allocs, err := c.snap.AllocsByNode(ws, node.ID)
		if err != nil {
			c.logger.Error("failed to get allocs for node",
				"node_id", node.ID, "error", err)
			continue
		}

		// If there are any non-terminal allocations, skip the node. If the node
		// is terminal and the allocations are not, the scheduler may not have
		// run yet to transition the allocs on the node to terminal. We delay
		// GC'ing until this happens.
		for _, alloc := range allocs {
			if !alloc.TerminalStatus() {
				continue OUTER
			}
		}

		// Node is eligible for garbage collection
		gcNode = append(gcNode, node.ID)
	}

	// Fast-path the nothing case
	if len(gcNode) == 0 {
		return nil
	}
	c.logger.Debug("node GC found eligible nodes", "nodes", len(gcNode))
	return c.nodeReap(eval, gcNode)
}

func (c *CoreScheduler) nodeReap(eval *structs.Evaluation, nodeIDs []string) error {
	// For old clusters, send single deregistration messages COMPAT(0.11)
	minVersionBatchNodeDeregister := version.Must(version.NewVersion("0.9.4"))
	if !ServersMeetMinimumVersion(c.srv.Members(), c.srv.Region(), minVersionBatchNodeDeregister, true) {
		for _, id := range nodeIDs {
			req := structs.NodeDeregisterRequest{
				NodeID: id,
				WriteRequest: structs.WriteRequest{
					Region:    c.srv.config.Region,
					AuthToken: eval.LeaderACL,
				},
			}
			var resp structs.NodeUpdateResponse
			if err := c.srv.RPC("Node.Deregister", &req, &resp); err != nil {
				c.logger.Error("node reap failed", "node_id", id, "error", err)
				return err
			}
		}
		return nil
	}

	// Call to the leader to issue the reap
	for _, ids := range partitionAll(structs.MaxUUIDsPerWriteRequest, nodeIDs) {
		req := structs.NodeBatchDeregisterRequest{
			NodeIDs: ids,
			WriteRequest: structs.WriteRequest{
				Region:    c.srv.config.Region,
				AuthToken: eval.LeaderACL,
			},
		}
		var resp structs.NodeUpdateResponse
		if err := c.srv.RPC("Node.BatchDeregister", &req, &resp); err != nil {
			c.logger.Error("node reap failed", "node_ids", ids, "error", err)
			return err
		}
	}
	return nil
}

// deploymentGC is used to garbage collect old deployments
func (c *CoreScheduler) deploymentGC(eval *structs.Evaluation) error {
	// Iterate over the deployments
	ws := memdb.NewWatchSet()
	iter, err := c.snap.Deployments(ws, state.SortDefault)
	if err != nil {
		return err
	}

	oldThreshold := c.getThreshold(eval, "deployment",
		"deployment_gc_threshold", c.srv.config.DeploymentGCThreshold)

	// Collect the deployments to GC
	var gcDeployment []string

OUTER:
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		deploy := raw.(*structs.Deployment)

		// Ignore non-terminal and new deployments
		if deploy.Active() || deploy.ModifyIndex > oldThreshold {
			continue
		}

		// Ensure there are no allocs referencing this deployment.
		allocs, err := c.snap.AllocsByDeployment(ws, deploy.ID)
		if err != nil {
			c.logger.Error("failed to get allocs for deployment",
				"deployment_id", deploy.ID, "error", err)
			continue
		}

		// Ensure there is no allocation referencing the deployment.
		for _, alloc := range allocs {
			if !alloc.TerminalStatus() {
				continue OUTER
			}
		}

		// Deployment is eligible for garbage collection
		gcDeployment = append(gcDeployment, deploy.ID)
	}

	// Fast-path the nothing case
	if len(gcDeployment) == 0 {
		return nil
	}
	c.logger.Debug("deployment GC found eligible deployments", "deployments", len(gcDeployment))
	return c.deploymentReap(gcDeployment)
}

// deploymentReap contacts the leader and issues a reap on the passed
// deployments.
func (c *CoreScheduler) deploymentReap(deployments []string) error {
	// Call to the leader to issue the reap
	for _, req := range c.partitionDeploymentReap(deployments, structs.MaxUUIDsPerWriteRequest) {
		var resp structs.GenericResponse
		if err := c.srv.RPC("Deployment.Reap", req, &resp); err != nil {
			c.logger.Error("deployment reap failed", "error", err)
			return err
		}
	}

	return nil
}

// partitionDeploymentReap returns a list of DeploymentDeleteRequest to make,
// ensuring a single request does not contain too many deployments. This is
// necessary to ensure that the Raft transaction does not become too large.
func (c *CoreScheduler) partitionDeploymentReap(deployments []string, batchSize int) []*structs.DeploymentDeleteRequest {
	var requests []*structs.DeploymentDeleteRequest
	submittedDeployments := 0
	for submittedDeployments != len(deployments) {
		req := &structs.DeploymentDeleteRequest{
			WriteRequest: structs.WriteRequest{
				Region: c.srv.config.Region,
			},
		}
		requests = append(requests, req)
		available := batchSize

		if remaining := len(deployments) - submittedDeployments; remaining > 0 {
			if remaining <= available {
				req.Deployments = deployments[submittedDeployments:]
				submittedDeployments += remaining
			} else {
				req.Deployments = deployments[submittedDeployments : submittedDeployments+available]
				submittedDeployments += available
			}
		}
	}

	return requests
}

// allocGCEligible returns if the allocation is eligible to be garbage collected
// according to its terminal status and its reschedule trackers
func allocGCEligible(a *structs.Allocation, job *structs.Job, gcTime time.Time, thresholdIndex uint64) bool {
	// Not in a terminal status and old enough
	if !a.TerminalStatus() || a.ModifyIndex > thresholdIndex {
		return false
	}

	// If the allocation is still running on the client we can not garbage
	// collect it.
	if a.ClientStatus == structs.AllocClientStatusRunning {
		return false
	}

	// If the job is deleted, stopped or dead all allocs can be removed
	if job == nil || job.Stop || job.Status == structs.JobStatusDead {
		return true
	}

	// If the allocation's desired state is Stop, it can be GCed even if it
	// has failed and hasn't been rescheduled. This can happen during job updates
	if a.DesiredStatus == structs.AllocDesiredStatusStop {
		return true
	}

	// If the alloc hasn't failed then we don't need to consider it for rescheduling
	// Rescheduling needs to copy over information from the previous alloc so that it
	// can enforce the reschedule policy
	if a.ClientStatus != structs.AllocClientStatusFailed {
		return true
	}

	var reschedulePolicy *structs.ReschedulePolicy
	tg := job.LookupTaskGroup(a.TaskGroup)

	if tg != nil {
		reschedulePolicy = tg.ReschedulePolicy
	}
	// No reschedule policy or rescheduling is disabled
	if reschedulePolicy == nil || (!reschedulePolicy.Unlimited && reschedulePolicy.Attempts == 0) {
		return true
	}
	// Restart tracking information has been carried forward
	if a.NextAllocation != "" {
		return true
	}

	// This task has unlimited rescheduling and the alloc has not been replaced, so we can't GC it yet
	if reschedulePolicy.Unlimited {
		return false
	}

	// No restarts have been attempted yet
	if a.RescheduleTracker == nil || len(a.RescheduleTracker.Events) == 0 {
		return false
	}

	// Don't GC if most recent reschedule attempt is within time interval
	interval := reschedulePolicy.Interval
	lastIndex := len(a.RescheduleTracker.Events)
	lastRescheduleEvent := a.RescheduleTracker.Events[lastIndex-1]
	timeDiff := gcTime.UTC().UnixNano() - lastRescheduleEvent.RescheduleTime

	return timeDiff > interval.Nanoseconds()
}

// csiVolumeClaimGC is used to garbage collect CSI volume claims
func (c *CoreScheduler) csiVolumeClaimGC(eval *structs.Evaluation) error {

	gcClaims := func(ns, volID string) error {
		req := &structs.CSIVolumeClaimRequest{
			VolumeID: volID,
			Claim:    structs.CSIVolumeClaimGC,
			State:    structs.CSIVolumeClaimStateUnpublishing,
			WriteRequest: structs.WriteRequest{
				Namespace: ns,
				Region:    c.srv.Region(),
				AuthToken: eval.LeaderACL,
			},
		}
		err := c.srv.RPC("CSIVolume.Claim", req, &structs.CSIVolumeClaimResponse{})
		return err
	}

	c.logger.Trace("garbage collecting unclaimed CSI volume claims", "eval.JobID", eval.JobID)

	// Volume ID smuggled in with the eval's own JobID
	evalVolID := strings.Split(eval.JobID, ":")

	// COMPAT(1.0): 0.11.0 shipped with 3 fields. tighten this check to len == 2
	if len(evalVolID) > 1 {
		volID := evalVolID[1]
		return gcClaims(eval.Namespace, volID)
	}

	ws := memdb.NewWatchSet()

	iter, err := c.snap.CSIVolumes(ws)
	if err != nil {
		return err
	}

	oldThreshold := c.getThreshold(eval, "CSI volume claim",
		"csi_volume_claim_gc_threshold", c.srv.config.CSIVolumeClaimGCThreshold)

	for i := iter.Next(); i != nil; i = iter.Next() {
		vol := i.(*structs.CSIVolume)

		// Ignore new volumes
		if vol.CreateIndex > oldThreshold {
			continue
		}

		// we only call the claim release RPC if the volume has claims
		// that no longer have valid allocations. otherwise we'd send
		// out a lot of do-nothing RPCs.
		vol, err := c.snap.CSIVolumeDenormalize(ws, vol.Copy())
		if err != nil {
			return err
		}
		if len(vol.PastClaims) > 0 {
			err = gcClaims(vol.Namespace, vol.ID)
			if err != nil {
				return err
			}
		}

	}
	return nil

}

// csiPluginGC is used to garbage collect unused plugins
func (c *CoreScheduler) csiPluginGC(eval *structs.Evaluation) error {

	ws := memdb.NewWatchSet()

	iter, err := c.snap.CSIPlugins(ws)
	if err != nil {
		return err
	}

	oldThreshold := c.getThreshold(eval, "CSI plugin",
		"csi_plugin_gc_threshold", c.srv.config.CSIPluginGCThreshold)

	for i := iter.Next(); i != nil; i = iter.Next() {
		plugin := i.(*structs.CSIPlugin)

		// Ignore new plugins
		if plugin.CreateIndex > oldThreshold {
			continue
		}

		req := &structs.CSIPluginDeleteRequest{ID: plugin.ID,
			QueryOptions: structs.QueryOptions{
				Region:    c.srv.Region(),
				AuthToken: eval.LeaderACL,
			}}
		err := c.srv.RPC("CSIPlugin.Delete", req, &structs.CSIPluginDeleteResponse{})
		if err != nil {
			if strings.Contains(err.Error(), "plugin in use") {
				continue
			}
			c.logger.Error("failed to GC plugin", "plugin_id", plugin.ID, "error", err)
			return err
		}
	}
	return nil
}

func (c *CoreScheduler) expiredOneTimeTokenGC(eval *structs.Evaluation) error {
	req := &structs.OneTimeTokenExpireRequest{
		WriteRequest: structs.WriteRequest{
			Region:    c.srv.Region(),
			AuthToken: eval.LeaderACL,
		},
	}
	return c.srv.RPC("ACL.ExpireOneTimeTokens", req, &structs.GenericResponse{})
}

// expiredACLTokenGC handles running the garbage collector for expired ACL
// tokens. It can be used for both local and global tokens and includes
// behaviour to account for periodic and user actioned garbage collection
// invocations.
func (c *CoreScheduler) expiredACLTokenGC(eval *structs.Evaluation, global bool) error {

	// If ACLs are not enabled, we do not need to continue and should exit
	// early. This is not an error condition as callers can blindly call this
	// function without checking the configuration. If the caller wants this to
	// be an error, they should check this config value themselves.
	if !c.srv.config.ACLEnabled {
		return nil
	}

	// If the function has been triggered for global tokens, but we are not the
	// authoritative region, we should exit. This is not an error condition as
	// callers can blindly call this function without checking the
	// configuration. If the caller wants this to be an error, they should
	// check this config value themselves.
	if global && c.srv.config.AuthoritativeRegion != c.srv.Region() {
		return nil
	}

	// The object name is logged within the getThreshold function, therefore we
	// want to be clear what token type this trigger is for.
	tokenScope := "local"
	if global {
		tokenScope = "global"
	}

	expiryThresholdIdx := c.getThreshold(eval, tokenScope+" expired ACL tokens",
		"acl_token_expiration_gc_threshold", c.srv.config.ACLTokenExpirationGCThreshold)

	expiredIter, err := c.snap.ACLTokensByExpired(global)
	if err != nil {
		return err
	}

	var (
		expiredAccessorIDs []string
		num                int
	)

	// The memdb iterator contains all tokens which include an expiration time,
	// however, as the caller, we do not know at which point in the array the
	// tokens are no longer expired. This time therefore forms the basis at
	// which we draw the line in the iteration loop and find the final expired
	// token that is eligible for deletion.
	now := time.Now().UTC()

	for raw := expiredIter.Next(); raw != nil; raw = expiredIter.Next() {
		token := raw.(*structs.ACLToken)

		// The iteration order of the indexes mean if we come across an
		// unexpired token, we can exit as we have found all currently expired
		// tokens.
		if !token.IsExpired(now) {
			break
		}

		// Check if the token is recent enough to skip, otherwise we'll delete
		// it.
		if token.CreateIndex > expiryThresholdIdx {
			continue
		}

		// Add the token accessor ID to the tracking array, thus marking it
		// ready for deletion.
		expiredAccessorIDs = append(expiredAccessorIDs, token.AccessorID)

		// Increment the counter. If this is at or above our limit, we return
		// what we have so far.
		if num++; num >= structs.ACLMaxExpiredBatchSize {
			break
		}
	}

	// There is no need to call the RPC endpoint if we do not have any tokens
	// to delete.
	if len(expiredAccessorIDs) < 1 {
		return nil
	}

	// Log a nice, friendly debug message which could be useful when debugging
	// garbage collection in environments with a high rate of token creation
	// and expiration.
	c.logger.Debug("expired ACL token GC found eligible tokens",
		"num", len(expiredAccessorIDs), "global", global)

	// Set up and make the RPC request which will return any error performing
	// the deletion.
	req := structs.ACLTokenDeleteRequest{
		AccessorIDs: expiredAccessorIDs,
		WriteRequest: structs.WriteRequest{
			Region:    c.srv.Region(),
			AuthToken: eval.LeaderACL,
		},
	}
	return c.srv.RPC(structs.ACLDeleteTokensRPCMethod, req, &structs.GenericResponse{})
}

// rootKeyRotateOrGC is used to rotate or garbage collect root keys
func (c *CoreScheduler) rootKeyRotateOrGC(eval *structs.Evaluation) error {

	// a rotation will be sent to the leader so our view of state
	// is no longer valid. we ack this core job and will pick up
	// the GC work on the next interval
	wasRotated, err := c.rootKeyRotate(eval)
	if err != nil {
		return err
	}
	if wasRotated {
		return nil
	}
	return c.rootKeyGC(eval)
}

func (c *CoreScheduler) rootKeyGC(eval *structs.Evaluation) error {

	oldThreshold := c.getThreshold(eval, "root key",
		"root_key_gc_threshold", c.srv.config.RootKeyGCThreshold)

	ws := memdb.NewWatchSet()
	iter, err := c.snap.RootKeyMetas(ws)
	if err != nil {
		return err
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		keyMeta := raw.(*structs.RootKeyMeta)
		if keyMeta.Active() || keyMeta.Rekeying() {
			continue // never GC the active key or one we're rekeying
		}
		if keyMeta.CreateIndex > oldThreshold {
			continue // don't GC recent keys
		}

		inUse, err := c.snap.IsRootKeyMetaInUse(keyMeta.KeyID)
		if err != nil {
			return err
		}
		if inUse {
			continue
		}

		req := &structs.KeyringDeleteRootKeyRequest{
			KeyID: keyMeta.KeyID,
			WriteRequest: structs.WriteRequest{
				Region:    c.srv.config.Region,
				AuthToken: eval.LeaderACL,
			},
		}
		if err := c.srv.RPC("Keyring.Delete",
			req, &structs.KeyringDeleteRootKeyResponse{}); err != nil {
			c.logger.Error("root key delete failed", "error", err)
			return err
		}
	}

	return nil
}

// rootKeyRotate checks if the active key is old enough that we need
// to kick off a rotation.
func (c *CoreScheduler) rootKeyRotate(eval *structs.Evaluation) (bool, error) {

	rotationThreshold := c.getThreshold(eval, "root key",
		"root_key_rotation_threshold", c.srv.config.RootKeyRotationThreshold)

	ws := memdb.NewWatchSet()
	activeKey, err := c.snap.GetActiveRootKeyMeta(ws)
	if err != nil {
		return false, err
	}
	if activeKey == nil {
		return false, nil // no active key
	}
	if activeKey.CreateIndex >= rotationThreshold {
		return false, nil // key is too new
	}

	req := &structs.KeyringRotateRootKeyRequest{
		WriteRequest: structs.WriteRequest{
			Region:    c.srv.config.Region,
			AuthToken: eval.LeaderACL,
		},
	}
	if err := c.srv.RPC("Keyring.Rotate",
		req, &structs.KeyringRotateRootKeyResponse{}); err != nil {
		c.logger.Error("root key rotation failed", "error", err)
		return false, err
	}

	return true, nil
}

// variablesReKey is optionally run after rotating the active
// root key. It iterates over all the variables for the keys in the
// re-keying state, decrypts them, and re-encrypts them in batches
// with the currently active key. This job does not GC the keys, which
// is handled in the normal periodic GC job.
func (c *CoreScheduler) variablesRekey(eval *structs.Evaluation) error {

	ws := memdb.NewWatchSet()
	iter, err := c.snap.RootKeyMetas(ws)
	if err != nil {
		return err
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		keyMeta := raw.(*structs.RootKeyMeta)
		if !keyMeta.Rekeying() {
			continue
		}
		varIter, err := c.snap.GetVariablesByKeyID(ws, keyMeta.KeyID)
		if err != nil {
			return err
		}
		err = c.rotateVariables(varIter, eval)
		if err != nil {
			return err
		}

	}

	return nil
}

// rotateVariables runs over an iterator of variables and decrypts them, and
// then sends them back to be re-encrypted with the currently active key,
// checking for conflicts
func (c *CoreScheduler) rotateVariables(iter memdb.ResultIterator, eval *structs.Evaluation) error {

	args := &structs.VariablesApplyRequest{
		Op: structs.VarOpCAS,
		WriteRequest: structs.WriteRequest{
			Region:    c.srv.config.Region,
			AuthToken: eval.LeaderACL,
		},
	}

	// We may have to work on a very large number of variables. There's no
	// BatchApply RPC because it makes for an awkward API around conflict
	// detection, and even if we did, we'd be blocking this scheduler goroutine
	// for a very long time using the same snapshot. This would increase the
	// risk that any given batch hits a conflict because of a concurrent change
	// and make it more likely that we fail the eval. For large sets, this would
	// likely mean the eval would run out of retries.
	//
	// Instead, we'll rate limit RPC requests and have a timeout. If we still
	// haven't finished the set by the timeout, emit a new eval.
	ctx, cancel := context.WithTimeout(context.Background(), c.srv.GetConfig().EvalNackTimeout/2)
	defer cancel()
	limiter := rate.NewLimiter(rate.Limit(100), 100)

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		select {
		case <-ctx.Done():
			newEval := &structs.Evaluation{
				ID:          uuid.Generate(),
				Namespace:   "-",
				Priority:    structs.CoreJobPriority,
				Type:        structs.JobTypeCore,
				TriggeredBy: structs.EvalTriggerScheduled,
				JobID:       eval.JobID,
				Status:      structs.EvalStatusPending,
				LeaderACL:   eval.LeaderACL,
			}
			return c.srv.RPC("Eval.Create", &structs.EvalUpdateRequest{
				Evals:     []*structs.Evaluation{newEval},
				EvalToken: uuid.Generate(),
				WriteRequest: structs.WriteRequest{
					Region:    c.srv.config.Region,
					AuthToken: eval.LeaderACL,
				},
			}, &structs.GenericResponse{})

		default:
		}

		ev := raw.(*structs.VariableEncrypted)
		cleartext, err := c.srv.encrypter.Decrypt(ev.Data, ev.KeyID)
		if err != nil {
			return err
		}
		dv := &structs.VariableDecrypted{
			VariableMetadata: ev.VariableMetadata,
		}
		dv.Items = make(map[string]string)
		err = json.Unmarshal(cleartext, &dv.Items)
		if err != nil {
			return err
		}
		args.Var = dv
		reply := &structs.VariablesApplyResponse{}

		if err := limiter.Wait(ctx); err != nil {
			return err
		}

		err = c.srv.RPC("Variables.Apply", args, reply)
		if err != nil {
			return err
		}
		if reply.IsConflict() {
			// we've already rotated the key by the time we took this
			// evaluation's snapshot, so any conflict is going to be on a write
			// made with the new key, so there's nothing for us to do here
			continue
		}
	}

	return nil
}

// getThreshold returns the index threshold for determining whether an
// object is old enough to GC
func (c *CoreScheduler) getThreshold(eval *structs.Evaluation, objectName, configName string, configThreshold time.Duration) uint64 {
	var oldThreshold uint64
	if eval.JobID == structs.CoreJobForceGC {
		// The GC was forced, so set the threshold to its maximum so
		// everything will GC.
		oldThreshold = math.MaxUint64
		c.logger.Debug(fmt.Sprintf("forced %s GC", objectName))
	} else {
		// Compute the old threshold limit for GC using the FSM
		// time table.  This is a rough mapping of a time to the
		// Raft index it belongs to.
		tt := c.srv.fsm.TimeTable()
		cutoff := time.Now().UTC().Add(-1 * configThreshold)
		oldThreshold = tt.NearestIndex(cutoff)
		c.logger.Debug(
			fmt.Sprintf("%s GC scanning before cutoff index", objectName),
			"index", oldThreshold,
			configName, configThreshold)
	}
	return oldThreshold
}
