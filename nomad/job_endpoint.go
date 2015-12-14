package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/watch"
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
	if err := args.Job.Validate(); err != nil {
		return err
	}

	// Initialize all the fields of services
	args.Job.InitAllServiceFields()

	if args.Job.Type == structs.JobTypeCore {
		return fmt.Errorf("job type cannot be core")
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
	if job == nil {
		return fmt.Errorf("job not found")
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
	if job.IsPeriodic() {
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
			iter, err := snap.Jobs()
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
