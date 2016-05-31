package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/watch"
	"github.com/hashicorp/nomad/scheduler"
)

// Job endpoint is used for job interactions
type Job struct {
	srv *Server
}

// Register is used to upsert a job for scheduling
func (j *Job) Register(args *structs.JobRegisterRequest, reply *structs.JobRegisterResponse) error {
	if done, err := j.srv.forward("Job.Register", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "register"}, time.Now())

	// Validate the arguments
	if args.Job == nil {
		return fmt.Errorf("missing job for registration")
	}

	// Initialize the job fields (sets defaults and any necessary init work).
	args.Job.InitFields()

	// Validate the job.
	if err := validateJob(args.Job); err != nil {
		return err
	}

	// Commit this update via Raft
	_, index, err := j.srv.raftApply(structs.JobRegisterRequestType, args)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Register failed: %v", err)
		return err
	}

	// Populate the reply with job information
	reply.JobModifyIndex = index

	// If the job is periodic, we don't create an eval.
	if args.Job.IsPeriodic() {
		return nil
	}

	// Create a new evaluation
	eval := &structs.Evaluation{
		ID:             structs.GenerateUUID(),
		Priority:       args.Job.Priority,
		Type:           args.Job.Type,
		TriggeredBy:    structs.EvalTriggerJobRegister,
		JobID:          args.Job.ID,
		JobModifyIndex: index,
		Status:         structs.EvalStatusPending,
	}
	update := &structs.EvalUpdateRequest{
		Evals:        []*structs.Evaluation{eval},
		WriteRequest: structs.WriteRequest{Region: args.Region},
	}

	// Commit this evaluation via Raft
	// XXX: There is a risk of partial failure where the JobRegister succeeds
	// but that the EvalUpdate does not.
	_, evalIndex, err := j.srv.raftApply(structs.EvalUpdateRequestType, update)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Eval create failed: %v", err)
		return err
	}

	// Populate the reply with eval information
	reply.EvalID = eval.ID
	reply.EvalCreateIndex = evalIndex
	reply.Index = evalIndex
	return nil
}

// Evaluate is used to force a job for re-evaluation
func (j *Job) Evaluate(args *structs.JobEvaluateRequest, reply *structs.JobRegisterResponse) error {
	if done, err := j.srv.forward("Job.Evaluate", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "evaluate"}, time.Now())

	// Validate the arguments
	if args.JobID == "" {
		return fmt.Errorf("missing job ID for evaluation")
	}

	// Lookup the job
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	job, err := snap.JobByID(args.JobID)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	if job.IsPeriodic() {
		return fmt.Errorf("can't evaluate periodic job")
	}

	// Create a new evaluation
	eval := &structs.Evaluation{
		ID:             structs.GenerateUUID(),
		Priority:       job.Priority,
		Type:           job.Type,
		TriggeredBy:    structs.EvalTriggerJobRegister,
		JobID:          job.ID,
		JobModifyIndex: job.ModifyIndex,
		Status:         structs.EvalStatusPending,
	}
	update := &structs.EvalUpdateRequest{
		Evals:        []*structs.Evaluation{eval},
		WriteRequest: structs.WriteRequest{Region: args.Region},
	}

	// Commit this evaluation via Raft
	_, evalIndex, err := j.srv.raftApply(structs.EvalUpdateRequestType, update)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Eval create failed: %v", err)
		return err
	}

	// Setup the reply
	reply.EvalID = eval.ID
	reply.EvalCreateIndex = evalIndex
	reply.JobModifyIndex = job.ModifyIndex
	reply.Index = evalIndex
	return nil
}

// Deregister is used to remove a job the cluster.
func (j *Job) Deregister(args *structs.JobDeregisterRequest, reply *structs.JobDeregisterResponse) error {
	if done, err := j.srv.forward("Job.Deregister", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "deregister"}, time.Now())

	// Validate the arguments
	if args.JobID == "" {
		return fmt.Errorf("missing job ID for evaluation")
	}

	// Lookup the job
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	job, err := snap.JobByID(args.JobID)
	if err != nil {
		return err
	}

	// Commit this update via Raft
	_, index, err := j.srv.raftApply(structs.JobDeregisterRequestType, args)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Deregister failed: %v", err)
		return err
	}

	// Populate the reply with job information
	reply.JobModifyIndex = index

	// If the job is periodic, we don't create an eval.
	if job != nil && job.IsPeriodic() {
		return nil
	}

	// Create a new evaluation
	// XXX: The job priority / type is strange for this, since it's not a high
	// priority even if the job was. The scheduler itself also doesn't matter,
	// since all should be able to handle deregistration in the same way.
	eval := &structs.Evaluation{
		ID:             structs.GenerateUUID(),
		Priority:       structs.JobDefaultPriority,
		Type:           structs.JobTypeService,
		TriggeredBy:    structs.EvalTriggerJobDeregister,
		JobID:          args.JobID,
		JobModifyIndex: index,
		Status:         structs.EvalStatusPending,
	}
	update := &structs.EvalUpdateRequest{
		Evals:        []*structs.Evaluation{eval},
		WriteRequest: structs.WriteRequest{Region: args.Region},
	}

	// Commit this evaluation via Raft
	_, evalIndex, err := j.srv.raftApply(structs.EvalUpdateRequestType, update)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Eval create failed: %v", err)
		return err
	}

	// Populate the reply with eval information
	reply.EvalID = eval.ID
	reply.EvalCreateIndex = evalIndex
	reply.Index = evalIndex
	return nil
}

// GetJob is used to request information about a specific job
func (j *Job) GetJob(args *structs.JobSpecificRequest,
	reply *structs.SingleJobResponse) error {
	if done, err := j.srv.forward("Job.GetJob", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "get_job"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		watch:     watch.NewItems(watch.Item{Job: args.JobID}),
		run: func() error {

			// Look for the job
			snap, err := j.srv.fsm.State().Snapshot()
			if err != nil {
				return err
			}
			out, err := snap.JobByID(args.JobID)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Job = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the nodes table
				index, err := snap.Index("jobs")
				if err != nil {
					return err
				}
				reply.Index = index
			}

			// Set the query response
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return j.srv.blockingRPC(&opts)
}

// List is used to list the jobs registered in the system
func (j *Job) List(args *structs.JobListRequest,
	reply *structs.JobListResponse) error {
	if done, err := j.srv.forward("Job.List", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "list"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		watch:     watch.NewItems(watch.Item{Table: "jobs"}),
		run: func() error {
			// Capture all the jobs
			snap, err := j.srv.fsm.State().Snapshot()
			if err != nil {
				return err
			}
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = snap.JobsByIDPrefix(prefix)
			} else {
				iter, err = snap.Jobs()
			}
			if err != nil {
				return err
			}

			var jobs []*structs.JobListStub
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				job := raw.(*structs.Job)
				jobs = append(jobs, job.Stub())
			}
			reply.Jobs = jobs

			// Use the last index that affected the jobs table
			index, err := snap.Index("jobs")
			if err != nil {
				return err
			}
			reply.Index = index

			// Set the query response
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return j.srv.blockingRPC(&opts)
}

// Allocations is used to list the allocations for a job
func (j *Job) Allocations(args *structs.JobSpecificRequest,
	reply *structs.JobAllocationsResponse) error {
	if done, err := j.srv.forward("Job.Allocations", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "allocations"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		watch:     watch.NewItems(watch.Item{AllocJob: args.JobID}),
		run: func() error {
			// Capture the allocations
			snap, err := j.srv.fsm.State().Snapshot()
			if err != nil {
				return err
			}
			allocs, err := snap.AllocsByJob(args.JobID)
			if err != nil {
				return err
			}

			// Convert to stubs
			if len(allocs) > 0 {
				reply.Allocations = make([]*structs.AllocListStub, 0, len(allocs))
				for _, alloc := range allocs {
					reply.Allocations = append(reply.Allocations, alloc.Stub())
				}
			}

			// Use the last index that affected the allocs table
			index, err := snap.Index("allocs")
			if err != nil {
				return err
			}
			reply.Index = index

			// Set the query response
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil

		}}
	return j.srv.blockingRPC(&opts)
}

// Evaluations is used to list the evaluations for a job
func (j *Job) Evaluations(args *structs.JobSpecificRequest,
	reply *structs.JobEvaluationsResponse) error {
	if done, err := j.srv.forward("Job.Evaluations", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "evaluations"}, time.Now())

	// Capture the evaluations
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	reply.Evaluations, err = snap.EvalsByJob(args.JobID)
	if err != nil {
		return err
	}

	// Use the last index that affected the evals table
	index, err := snap.Index("evals")
	if err != nil {
		return err
	}
	reply.Index = index

	// Set the query response
	j.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// Plan is used to cause a dry-run evaluation of the Job and return the results
// with a potential diff containing annotations.
func (j *Job) Plan(args *structs.JobPlanRequest, reply *structs.JobPlanResponse) error {
	if done, err := j.srv.forward("Job.Plan", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "plan"}, time.Now())

	// Validate the arguments
	if args.Job == nil {
		return fmt.Errorf("Job required for plan")
	}

	// Initialize the job fields (sets defaults and any necessary init work).
	args.Job.InitFields()

	// Validate the job.
	if err := validateJob(args.Job); err != nil {
		return err
	}

	// Acquire a snapshot of the state
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	// Get the original job
	oldJob, err := snap.JobByID(args.Job.ID)
	if err != nil {
		return err
	}

	var index uint64
	if oldJob != nil {
		index = oldJob.JobModifyIndex + 1
	}

	// Insert the updated Job into the snapshot
	snap.UpsertJob(index, args.Job)

	// Do the dry run
	planner, err := j.dryrunJob(args.Job, snap)
	if err != nil {
		return err
	}

	annotations := planner.Plans[0].Annotations
	if args.Diff {
		jobDiff, err := oldJob.Diff(args.Job, true)
		if err != nil {
			return fmt.Errorf("failed to create job diff: %v", err)
		}

		if err := scheduler.Annotate(jobDiff, annotations); err != nil {
			return fmt.Errorf("failed to annotate job diff: %v", err)
		}
		reply.Diff = jobDiff
	}

	reply.JobModifyIndex = index
	reply.Annotations = annotations
	reply.CreatedEvals = planner.CreateEvals
	reply.Index = index
	return nil
}

// validateJob validates a Job and task drivers and returns an error if there is
// a validation problem or if the Job is of a type a user is not allowed to
// submit.
func validateJob(job *structs.Job) error {
	validationErrors := new(multierror.Error)
	if err := job.Validate(); err != nil {
		multierror.Append(validationErrors, err)
	}

	// Validate the driver configurations.
	for _, tg := range job.TaskGroups {
		for _, task := range tg.Tasks {
			d, err := driver.NewDriver(
				task.Driver,
				driver.NewEmptyDriverContext(),
			)
			if err != nil {
				msg := "failed to create driver for task %q in group %q for validation: %v"
				multierror.Append(validationErrors, fmt.Errorf(msg, tg.Name, task.Name, err))
				continue
			}

			if err := d.Validate(task.Config); err != nil {
				formatted := fmt.Errorf("group %q -> task %q -> config: %v", tg.Name, task.Name, err)
				multierror.Append(validationErrors, formatted)
			}
		}
	}

	if job.Type == structs.JobTypeCore {
		multierror.Append(validationErrors, fmt.Errorf("job type cannot be core"))
	}

	return validationErrors.ErrorOrNil()
}

// Status returns a summary of the status of the job's allocations (how many
// pending, running, complete, failed).
func (j *Job) Status(args *structs.JobSpecificRequest,
	reply *structs.JobStatusResponse) error {
	if done, err := j.srv.forward("Job.Status", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "status"}, time.Now())

	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	out, err := snap.JobByID(args.JobID)
	if err != nil {
		return err
	}

	if out == nil {
		return fmt.Errorf("job %q not found", args.JobID)
	}

	status, err := j.computeJobStatus(snap, out)
	if err != nil {
		return err
	}

	*reply = *status
	reply.Index = out.ModifyIndex

	// Set the query response
	j.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// computeJobStatus takes a state snapshot and the job and computes is high
// level status
func (j *Job) computeJobStatus(snap *state.StateSnapshot, job *structs.Job) (*structs.JobStatusResponse, error) {
	if job == nil {
		return nil, fmt.Errorf("job can not be nil")
	}

	status := &structs.JobStatusResponse{
		Status:     job.Status,
		TaskGroups: make(map[string]structs.AllocStateCounts, len(job.TaskGroups)),
	}
	allocs, err := snap.AllocsByJob(job.ID)
	if err != nil {
		return nil, err
	}

	// Periodic jobs can't have running instances
	if job.IsPeriodic() {
		return status, nil
	}

	var tgStat structs.AllocStateCounts
	for _, alloc := range allocs {
		// Starting =  Desired Running && Client Pending
		// Running = Desired Running && Client Running
		// Complete = Desired running && client complete
		if alloc.DesiredStatus == structs.AllocDesiredStatusRun {
			tgStat = status.TaskGroups[alloc.TaskGroup]

			switch alloc.ClientStatus {
			case structs.AllocClientStatusPending:
				status.Starting++
				tgStat.Starting++
			case structs.AllocClientStatusRunning:
				status.Running++
				tgStat.Running++
			case structs.AllocClientStatusComplete:
				status.Complete++
				tgStat.Complete++
			}

		}

		// Failed = desired failed || client failed
		if alloc.DesiredStatus == structs.AllocDesiredStatusFailed ||
			alloc.ClientStatus == structs.AllocClientStatusFailed {
			status.Failed++
			tgStat.Failed++
		}

		status.TaskGroups[alloc.TaskGroup] = tgStat
	}

	// If the job is a system job, pending isn't applicable
	if job.Type == structs.JobTypeSystem {
		return status, nil
	}

	// Pending = Hasn't been scheduled
	// Determine what is pending by dry-running the scheduler.
	planner, err := j.dryrunJob(job, snap)
	if err != nil {
		return nil, err
	}

	annotations := planner.Plans[0].Annotations

	for tg, update := range annotations.DesiredTGUpdates {
		status.Pending += update.Place
		tgStat = status.TaskGroups[tg]
		tgStat.Pending += update.Place
		status.TaskGroups[tg] = tgStat
	}

	return status, nil
}

// dryrunJob takes a job and a state snapshot and invokes the scheduler in
// dryrun mode. The harness is returned which contains the created evaluation
// and annotations
func (j *Job) dryrunJob(job *structs.Job, snap *state.StateSnapshot) (*scheduler.Harness, error) {
	eval := &structs.Evaluation{
		ID:             structs.GenerateUUID(),
		Priority:       job.Priority,
		Type:           job.Type,
		TriggeredBy:    structs.EvalTriggerJobRegister,
		JobID:          job.ID,
		JobModifyIndex: job.ModifyIndex,
		Status:         structs.EvalStatusPending,
		AnnotatePlan:   true,
	}

	// Create an in-memory Planner that returns no errors and stores the
	// submitted plan and created evals.
	planner := &scheduler.Harness{
		State: &snap.StateStore,
	}

	// Create the scheduler and run it
	sched, err := scheduler.NewScheduler(eval.Type, j.srv.logger, snap, planner)
	if err != nil {
		return nil, err
	}

	if err := sched.Process(eval); err != nil {
		return nil, err
	}

	// Annotate and store the diff
	if plans := len(planner.Plans); plans != 1 {
		return nil, fmt.Errorf("scheduler resulted in an unexpected number of plans: %d", plans)
	}

	return planner, nil
}
