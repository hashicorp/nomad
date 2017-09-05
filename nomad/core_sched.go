package nomad

import (
	"fmt"
	"math"
	"time"

	memdb "github.com/hashicorp/go-memdb"
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
	srv  *Server
	snap *state.StateSnapshot
}

// NewCoreScheduler is used to return a new system scheduler instance
func NewCoreScheduler(srv *Server, snap *state.StateSnapshot) scheduler.Scheduler {
	s := &CoreScheduler{
		srv:  srv,
		snap: snap,
	}
	return s
}

// Process is used to implement the scheduler.Scheduler interface
func (c *CoreScheduler) Process(eval *structs.Evaluation) error {
	switch eval.JobID {
	case structs.CoreJobEvalGC:
		return c.evalGC(eval)
	case structs.CoreJobNodeGC:
		return c.nodeGC(eval)
	case structs.CoreJobJobGC:
		return c.jobGC(eval)
	case structs.CoreJobDeploymentGC:
		return c.deploymentGC(eval)
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
		c.srv.logger.Println("[DEBUG] sched.core: forced job GC")
	} else {
		// Get the time table to calculate GC cutoffs.
		tt := c.srv.fsm.TimeTable()
		cutoff := time.Now().UTC().Add(-1 * c.srv.config.JobGCThreshold)
		oldThreshold = tt.NearestIndex(cutoff)
		c.srv.logger.Printf("[DEBUG] sched.core: job GC: scanning before index %d (%v)",
			oldThreshold, c.srv.config.JobGCThreshold)
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
			c.srv.logger.Printf("[ERR] sched.core: failed to get evals for job %s: %v", job.ID, err)
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
	c.srv.logger.Printf("[DEBUG] sched.core: job GC: %d jobs, %d evaluations, %d allocs eligible",
		len(gcJob), len(gcEval), len(gcAlloc))

	// Reap the evals and allocs
	if err := c.evalReap(gcEval, gcAlloc); err != nil {
		return err
	}

	// Call to the leader to deregister the jobs.
	for _, job := range gcJob {
		req := structs.JobDeregisterRequest{
			JobID: job.ID,
			Purge: true,
			WriteRequest: structs.WriteRequest{
				Region:    c.srv.config.Region,
				Namespace: job.Namespace,
			},
		}
		var resp structs.JobDeregisterResponse
		if err := c.srv.RPC("Job.Deregister", &req, &resp); err != nil {
			c.srv.logger.Printf("[ERR] sched.core: job deregister failed: %v", err)
			return err
		}
	}

	return nil
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
		c.srv.logger.Println("[DEBUG] sched.core: forced eval GC")
	} else {
		// Compute the old threshold limit for GC using the FSM
		// time table.  This is a rough mapping of a time to the
		// Raft index it belongs to.
		tt := c.srv.fsm.TimeTable()
		cutoff := time.Now().UTC().Add(-1 * c.srv.config.EvalGCThreshold)
		oldThreshold = tt.NearestIndex(cutoff)
		c.srv.logger.Printf("[DEBUG] sched.core: eval GC: scanning before index %d (%v)",
			oldThreshold, c.srv.config.EvalGCThreshold)
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
	c.srv.logger.Printf("[DEBUG] sched.core: eval GC: %d evaluations, %d allocs eligible",
		len(gcEval), len(gcAlloc))

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

	// If the eval is from a running "batch" job we don't want to garbage
	// collect its allocations. If there is a long running batch job and its
	// terminal allocations get GC'd the scheduler would re-run the
	// allocations.
	if eval.Type == structs.JobTypeBatch {
		// Check if the job is running
		job, err := c.snap.JobByID(ws, eval.Namespace, eval.JobID)
		if err != nil {
			return false, nil, err
		}

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
			return false, nil, nil
		}
	}

	// Get the allocations by eval
	allocs, err := c.snap.AllocsByEval(ws, eval.ID)
	if err != nil {
		c.srv.logger.Printf("[ERR] sched.core: failed to get allocs for eval %s: %v",
			eval.ID, err)
		return false, nil, err
	}

	// Scan the allocations to ensure they are terminal and old
	gcEval := true
	var gcAllocIDs []string
	for _, alloc := range allocs {
		if !alloc.TerminalStatus() || alloc.ModifyIndex > thresholdIndex {
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

// evalReap contacts the leader and issues a reap on the passed evals and
// allocs.
func (c *CoreScheduler) evalReap(evals, allocs []string) error {
	// Call to the leader to issue the reap
	for _, req := range c.partitionEvalReap(evals, allocs) {
		var resp structs.GenericResponse
		if err := c.srv.RPC("Eval.Reap", req, &resp); err != nil {
			c.srv.logger.Printf("[ERR] sched.core: eval reap failed: %v", err)
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
		c.srv.logger.Println("[DEBUG] sched.core: forced node GC")
	} else {
		// Compute the old threshold limit for GC using the FSM
		// time table.  This is a rough mapping of a time to the
		// Raft index it belongs to.
		tt := c.srv.fsm.TimeTable()
		cutoff := time.Now().UTC().Add(-1 * c.srv.config.NodeGCThreshold)
		oldThreshold = tt.NearestIndex(cutoff)
		c.srv.logger.Printf("[DEBUG] sched.core: node GC: scanning before index %d (%v)",
			oldThreshold, c.srv.config.NodeGCThreshold)
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
			c.srv.logger.Printf("[ERR] sched.core: failed to get allocs for node %s: %v",
				eval.ID, err)
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
	c.srv.logger.Printf("[DEBUG] sched.core: node GC: %d nodes eligible", len(gcNode))

	// Call to the leader to issue the reap
	for _, nodeID := range gcNode {
		req := structs.NodeDeregisterRequest{
			NodeID: nodeID,
			WriteRequest: structs.WriteRequest{
				Region: c.srv.config.Region,
			},
		}
		var resp structs.NodeUpdateResponse
		if err := c.srv.RPC("Node.Deregister", &req, &resp); err != nil {
			c.srv.logger.Printf("[ERR] sched.core: node '%s' reap failed: %v", nodeID, err)
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
		c.srv.logger.Println("[DEBUG] sched.core: forced deployment GC")
	} else {
		// Compute the old threshold limit for GC using the FSM
		// time table.  This is a rough mapping of a time to the
		// Raft index it belongs to.
		tt := c.srv.fsm.TimeTable()
		cutoff := time.Now().UTC().Add(-1 * c.srv.config.DeploymentGCThreshold)
		oldThreshold = tt.NearestIndex(cutoff)
		c.srv.logger.Printf("[DEBUG] sched.core: deployment GC: scanning before index %d (%v)",
			oldThreshold, c.srv.config.DeploymentGCThreshold)
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
			c.srv.logger.Printf("[ERR] sched.core: failed to get allocs for deployment %s: %v",
				deploy.ID, err)
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
	c.srv.logger.Printf("[DEBUG] sched.core: deployment GC: %d deployments eligible", len(gcDeployment))
	return c.deploymentReap(gcDeployment)
}

// deploymentReap contacts the leader and issues a reap on the passed
// deployments.
func (c *CoreScheduler) deploymentReap(deployments []string) error {
	// Call to the leader to issue the reap
	for _, req := range c.partitionDeploymentReap(deployments) {
		var resp structs.GenericResponse
		if err := c.srv.RPC("Deployment.Reap", req, &resp); err != nil {
			c.srv.logger.Printf("[ERR] sched.core: deployment reap failed: %v", err)
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
