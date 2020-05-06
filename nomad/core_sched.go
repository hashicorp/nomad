package nomad

import (
	"fmt"
	"math"
	"strings"
	"time"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
)

var (
	// maxIdsPerReap is the maximum number of evals and allocations to reap in a
	// single Raft transaction. This is to ensure that the Raft message does not
	// become too large.
	maxIdsPerReap = (1024 * 256) / 36 // 0.25 MB of ids.
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

	var oldThreshold uint64
	if eval.JobID == structs.CoreJobForceGC {
		// The GC was forced, so set the threshold to its maximum so everything
		// will GC.
		oldThreshold = math.MaxUint64
		c.logger.Debug("forced job GC")
	} else {
		// Get the time table to calculate GC cutoffs.
		tt := c.srv.fsm.TimeTable()
		cutoff := time.Now().UTC().Add(-1 * c.srv.config.JobGCThreshold)
		oldThreshold = tt.NearestIndex(cutoff)
		c.logger.Debug("job GC scanning before cutoff index",
			"index", oldThreshold, "job_gc_threshold", c.srv.config.JobGCThreshold)
	}

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
			}

			if gc {
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
	for _, req := range c.partitionJobReap(jobs, leaderACL) {
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
func (c *CoreScheduler) partitionJobReap(jobs []*structs.Job, leaderACL string) []*structs.JobBatchDeregisterRequest {
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
		available := maxIdsPerReap

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
	iter, err := c.snap.Evals(ws)
	if err != nil {
		return err
	}

	var oldThreshold uint64
	if eval.JobID == structs.CoreJobForceGC {
		// The GC was forced, so set the threshold to its maximum so everything
		// will GC.
		oldThreshold = math.MaxUint64
		c.logger.Debug("forced eval GC")
	} else {
		// Compute the old threshold limit for GC using the FSM
		// time table.  This is a rough mapping of a time to the
		// Raft index it belongs to.
		tt := c.srv.fsm.TimeTable()
		cutoff := time.Now().UTC().Add(-1 * c.srv.config.EvalGCThreshold)
		oldThreshold = tt.NearestIndex(cutoff)
		c.logger.Debug("eval GC scanning before cutoff index",
			"index", oldThreshold, "eval_gc_threshold", c.srv.config.EvalGCThreshold)
	}

	// Collect the allocations and evaluations to GC
	var gcAlloc, gcEval []string
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		eval := raw.(*structs.Evaluation)

		// The Evaluation GC should not handle batch jobs since those need to be
		// garbage collected in one shot
		gc, allocs, err := c.gcEval(eval, oldThreshold, false)
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
	// collect its allocations. If there is a long running batch job and its
	// terminal allocations get GC'd the scheduler would re-run the
	// allocations.
	if eval.Type == structs.JobTypeBatch {
		// Check if the job is running

		// Can collect if:
		// Job doesn't exist
		// Job is Stopped and dead
		// allowBatch and the job is dead
		collect := false
		if job == nil {
			collect = true
		} else if job.Status != structs.JobStatusDead {
			collect = false
		} else if job.Stop {
			collect = true
		} else if allowBatch {
			collect = true
		}

		// We don't want to gc anything related to a job which is not dead
		// If the batch job doesn't exist we can GC it regardless of allowBatch
		if !collect {
			// Find allocs associated with older (based on createindex) and GC them if terminal
			oldAllocs := olderVersionTerminalAllocs(allocs, job)
			return false, oldAllocs, nil
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

// olderVersionTerminalAllocs returns terminal allocations whose job create index
// is older than the job's create index
func olderVersionTerminalAllocs(allocs []*structs.Allocation, job *structs.Job) []string {
	var ret []string
	for _, alloc := range allocs {
		if alloc.Job != nil && alloc.Job.CreateIndex < job.CreateIndex && alloc.TerminalStatus() {
			ret = append(ret, alloc.ID)
		}
	}
	return ret
}

// evalReap contacts the leader and issues a reap on the passed evals and
// allocs.
func (c *CoreScheduler) evalReap(evals, allocs []string) error {
	// Call to the leader to issue the reap
	for _, req := range c.partitionEvalReap(evals, allocs) {
		var resp structs.GenericResponse
		if err := c.srv.RPC("Eval.Reap", req, &resp); err != nil {
			c.logger.Error("eval reap failed", "error", err)
			return err
		}
	}

	return nil
}

// partitionEvalReap returns a list of EvalDeleteRequest to make, ensuring a single
// request does not contain too many allocations and evaluations. This is
// necessary to ensure that the Raft transaction does not become too large.
func (c *CoreScheduler) partitionEvalReap(evals, allocs []string) []*structs.EvalDeleteRequest {
	var requests []*structs.EvalDeleteRequest
	submittedEvals, submittedAllocs := 0, 0
	for submittedEvals != len(evals) || submittedAllocs != len(allocs) {
		req := &structs.EvalDeleteRequest{
			WriteRequest: structs.WriteRequest{
				Region: c.srv.config.Region,
			},
		}
		requests = append(requests, req)
		available := maxIdsPerReap

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

	var oldThreshold uint64
	if eval.JobID == structs.CoreJobForceGC {
		// The GC was forced, so set the threshold to its maximum so everything
		// will GC.
		oldThreshold = math.MaxUint64
		c.logger.Debug("forced node GC")
	} else {
		// Compute the old threshold limit for GC using the FSM
		// time table.  This is a rough mapping of a time to the
		// Raft index it belongs to.
		tt := c.srv.fsm.TimeTable()
		cutoff := time.Now().UTC().Add(-1 * c.srv.config.NodeGCThreshold)
		oldThreshold = tt.NearestIndex(cutoff)
		c.logger.Debug("node GC scanning before cutoff index",
			"index", oldThreshold, "node_gc_threshold", c.srv.config.NodeGCThreshold)
	}

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
	if !ServersMeetMinimumVersion(c.srv.Members(), minVersionBatchNodeDeregister, true) {
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
	for _, ids := range partitionAll(maxIdsPerReap, nodeIDs) {
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
	iter, err := c.snap.Deployments(ws)
	if err != nil {
		return err
	}

	var oldThreshold uint64
	if eval.JobID == structs.CoreJobForceGC {
		// The GC was forced, so set the threshold to its maximum so everything
		// will GC.
		oldThreshold = math.MaxUint64
		c.logger.Debug("forced deployment GC")
	} else {
		// Compute the old threshold limit for GC using the FSM
		// time table.  This is a rough mapping of a time to the
		// Raft index it belongs to.
		tt := c.srv.fsm.TimeTable()
		cutoff := time.Now().UTC().Add(-1 * c.srv.config.DeploymentGCThreshold)
		oldThreshold = tt.NearestIndex(cutoff)
		c.logger.Debug("deployment GC scanning before cutoff index",
			"index", oldThreshold, "deployment_gc_threshold", c.srv.config.DeploymentGCThreshold)
	}

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
	for _, req := range c.partitionDeploymentReap(deployments) {
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
func (c *CoreScheduler) partitionDeploymentReap(deployments []string) []*structs.DeploymentDeleteRequest {
	var requests []*structs.DeploymentDeleteRequest
	submittedDeployments := 0
	for submittedDeployments != len(deployments) {
		req := &structs.DeploymentDeleteRequest{
			WriteRequest: structs.WriteRequest{
				Region: c.srv.config.Region,
			},
		}
		requests = append(requests, req)
		available := maxIdsPerReap

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

// TODO: we need a periodic trigger to iterate over all the volumes and split
// them up into separate work items, same as we do for jobs.

// csiVolumeClaimGC is used to garbage collect CSI volume claims
func (c *CoreScheduler) csiVolumeClaimGC(eval *structs.Evaluation) error {
	c.logger.Trace("garbage collecting unclaimed CSI volume claims", "eval.JobID", eval.JobID)

	// Volume ID smuggled in with the eval's own JobID
	evalVolID := strings.Split(eval.JobID, ":")

	// COMPAT(1.0): 0.11.0 shipped with 3 fields. tighten this check to len == 2
	if len(evalVolID) < 2 {
		// TODO(tgross): implement this
		c.logger.Trace("garbage collecting CSI volume claims")
		return nil
	}

	volID := evalVolID[1]
	req := &structs.CSIVolumeClaimRequest{
		VolumeID: volID,
		Claim:    structs.CSIVolumeClaimRelease,
	}
	req.Namespace = eval.Namespace
	req.Region = c.srv.config.Region

	err := c.srv.RPC("CSIVolume.Claim", req, &structs.CSIVolumeClaimResponse{})
	return err
}

// csiPluginGC is used to garbage collect unused plugins
func (c *CoreScheduler) csiPluginGC(eval *structs.Evaluation) error {

	ws := memdb.NewWatchSet()

	iter, err := c.snap.CSIPlugins(ws)
	if err != nil {
		return err
	}

	// Get the time table to calculate GC cutoffs.
	var oldThreshold uint64
	if eval.JobID == structs.CoreJobForceGC {
		// The GC was forced, so set the threshold to its maximum so
		// everything will GC.
		oldThreshold = math.MaxUint64
		c.logger.Debug("forced plugin GC")
	} else {
		tt := c.srv.fsm.TimeTable()
		cutoff := time.Now().UTC().Add(-1 * c.srv.config.CSIPluginGCThreshold)
		oldThreshold = tt.NearestIndex(cutoff)
	}

	c.logger.Debug("CSI plugin GC scanning before cutoff index",
		"index", oldThreshold, "csi_plugin_gc_threshold", c.srv.config.CSIPluginGCThreshold)

	for i := iter.Next(); i != nil; i = iter.Next() {
		plugin := i.(*structs.CSIPlugin)

		// Ignore new plugins
		if plugin.CreateIndex > oldThreshold {
			continue
		}

		req := &structs.CSIPluginDeleteRequest{ID: plugin.ID}
		req.Region = c.srv.Region()
		err := c.srv.RPC("CSIPlugin.Delete", req, &structs.CSIPluginDeleteResponse{})
		if err != nil {
			if err.Error() == "plugin in use" {
				continue
			}
			c.logger.Error("failed to GC plugin", "plugin_id", plugin.ID, "error", err)
			return err
		}
	}
	return nil
}
