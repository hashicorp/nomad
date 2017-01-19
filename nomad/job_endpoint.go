package nomad

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/golang/snappy"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/watch"
	"github.com/hashicorp/nomad/scheduler"
)

const (
	// RegisterEnforceIndexErrPrefix is the prefix to use in errors caused by
	// enforcing the job modify index during registers.
	RegisterEnforceIndexErrPrefix = "Enforcing job modify index"

	// DispatchPayloadSizeLimit is the maximum size of the uncompressed input
	// data payload.
	DispatchPayloadSizeLimit = 16 * 1024
)

var (
	// vaultConstraint is the implicit constraint added to jobs requesting a
	// Vault token
	vaultConstraint = &structs.Constraint{
		LTarget: "${attr.vault.version}",
		RTarget: ">= 0.6.1",
		Operand: structs.ConstraintVersion,
	}
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
	args.Job.Canonicalize()

	// Add implicit constraints
	setImplicitConstraints(args.Job)

	// Validate the job.
	if err := validateJob(args.Job); err != nil {
		return err
	}

	if args.EnforceIndex {
		// Lookup the job
		snap, err := j.srv.fsm.State().Snapshot()
		if err != nil {
			return err
		}
		job, err := snap.JobByID(args.Job.ID)
		if err != nil {
			return err
		}
		jmi := args.JobModifyIndex
		if job != nil {
			if jmi == 0 {
				return fmt.Errorf("%s 0: job already exists", RegisterEnforceIndexErrPrefix)
			} else if jmi != job.JobModifyIndex {
				return fmt.Errorf("%s %d: job exists with conflicting job modify index: %d",
					RegisterEnforceIndexErrPrefix, jmi, job.JobModifyIndex)
			}
		} else if jmi != 0 {
			return fmt.Errorf("%s %d: job does not exist", RegisterEnforceIndexErrPrefix, jmi)
		}
	}

	// Ensure that the job has permissions for the requested Vault tokens
	policies := args.Job.VaultPolicies()
	if len(policies) != 0 {
		vconf := j.srv.config.VaultConfig
		if !vconf.IsEnabled() {
			return fmt.Errorf("Vault not enabled and Vault policies requested")
		}

		// Have to check if the user has permissions
		if !vconf.AllowsUnauthenticated() {
			if args.Job.VaultToken == "" {
				return fmt.Errorf("Vault policies requested but missing Vault Token")
			}

			vault := j.srv.vault
			s, err := vault.LookupToken(context.Background(), args.Job.VaultToken)
			if err != nil {
				return err
			}

			allowedPolicies, err := PoliciesFrom(s)
			if err != nil {
				return err
			}

			// If we are given a root token it can access all policies
			if !lib.StrContains(allowedPolicies, "root") {
				flatPolicies := structs.VaultPoliciesSet(policies)
				subset, offending := helper.SliceStringIsSubset(allowedPolicies, flatPolicies)
				if !subset {
					return fmt.Errorf("Passed Vault Token doesn't allow access to the following policies: %s",
						strings.Join(offending, ", "))
				}
			}
		}
	}

	// Clear the Vault token
	args.Job.VaultToken = ""

	// Commit this update via Raft
	_, index, err := j.srv.raftApply(structs.JobRegisterRequestType, args)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Register failed: %v", err)
		return err
	}

	// Populate the reply with job information
	reply.JobModifyIndex = index

	// If the job is periodic or a constructor, we don't create an eval.
	if args.Job.IsPeriodic() || args.Job.IsConstructor() {
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

// setImplicitConstraints adds implicit constraints to the job based on the
// features it is requesting.
func setImplicitConstraints(j *structs.Job) {
	// Get the required Vault Policies
	policies := j.VaultPolicies()

	// Get the required signals
	signals := j.RequiredSignals()

	// Hot path
	if len(signals) == 0 && len(policies) == 0 {
		return
	}

	// Add Vault constraints
	for _, tg := range j.TaskGroups {
		_, ok := policies[tg.Name]
		if !ok {
			// Not requesting Vault
			continue
		}

		found := false
		for _, c := range tg.Constraints {
			if c.Equal(vaultConstraint) {
				found = true
				break
			}
		}

		if !found {
			tg.Constraints = append(tg.Constraints, vaultConstraint)
		}
	}

	// Add signal constraints
	for _, tg := range j.TaskGroups {
		tgSignals, ok := signals[tg.Name]
		if !ok {
			// Not requesting Vault
			continue
		}

		// Flatten the signals
		required := helper.MapStringStringSliceValueSet(tgSignals)
		sigConstraint := getSignalConstraint(required)

		found := false
		for _, c := range tg.Constraints {
			if c.Equal(sigConstraint) {
				found = true
				break
			}
		}

		if !found {
			tg.Constraints = append(tg.Constraints, sigConstraint)
		}
	}
}

// getSignalConstraint builds a suitable constraint based on the required
// signals
func getSignalConstraint(signals []string) *structs.Constraint {
	return &structs.Constraint{
		Operand: structs.ConstraintSetContains,
		LTarget: "${attr.os.signals}",
		RTarget: strings.Join(signals, ","),
	}
}

// Summary retreives the summary of a job
func (j *Job) Summary(args *structs.JobSummaryRequest,
	reply *structs.JobSummaryResponse) error {
	if done, err := j.srv.forward("Job.Summary", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job_summary", "get_job_summary"}, time.Now())
	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		watch:     watch.NewItems(watch.Item{JobSummary: args.JobID}),
		run: func() error {
			snap, err := j.srv.fsm.State().Snapshot()
			if err != nil {
				return err
			}

			// Look for job summary
			out, err := snap.JobSummaryByID(args.JobID)
			if err != nil {
				return err
			}

			// Setup the output
			reply.JobSummary = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the job_summary table
				index, err := snap.Index("job_summary")
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
	} else if job.IsConstructor() {
		return fmt.Errorf("can't evaluate constructor job")
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

	// If the job is periodic or a construcotr, we don't create an eval.
	if job != nil && (job.IsPeriodic() || job.IsConstructor()) {
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
				summary, err := snap.JobSummaryByID(job.ID)
				if err != nil {
					return fmt.Errorf("unable to look up summary for job: %v", job.ID)
				}
				jobs = append(jobs, job.Stub(summary))
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
			allocs, err := snap.AllocsByJob(args.JobID, args.AllAllocs)
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

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		watch:     watch.NewItems(watch.Item{EvalJob: args.JobID}),
		run: func() error {
			// Capture the evals
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
		}}

	return j.srv.blockingRPC(&opts)
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
	args.Job.Canonicalize()

	// Add implicit constraints
	setImplicitConstraints(args.Job)

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
	var updatedIndex uint64
	if oldJob != nil {
		index = oldJob.JobModifyIndex
		updatedIndex = oldJob.JobModifyIndex + 1
	}

	// Insert the updated Job into the snapshot
	snap.UpsertJob(updatedIndex, args.Job)

	// Create an eval and mark it as requiring annotations and insert that as well
	eval := &structs.Evaluation{
		ID:             structs.GenerateUUID(),
		Priority:       args.Job.Priority,
		Type:           args.Job.Type,
		TriggeredBy:    structs.EvalTriggerJobRegister,
		JobID:          args.Job.ID,
		JobModifyIndex: updatedIndex,
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
		return err
	}

	if err := sched.Process(eval); err != nil {
		return err
	}

	// Annotate and store the diff
	if plans := len(planner.Plans); plans != 1 {
		return fmt.Errorf("scheduler resulted in an unexpected number of plans: %v", plans)
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

	// Grab the failures
	if len(planner.Evals) != 1 {
		return fmt.Errorf("scheduler resulted in an unexpected number of eval updates: %v", planner.Evals)
	}
	updatedEval := planner.Evals[0]

	// If it is a periodic job calculate the next launch
	if args.Job.IsPeriodic() && args.Job.Periodic.Enabled {
		reply.NextPeriodicLaunch = args.Job.Periodic.Next(time.Now().UTC())
	}

	reply.FailedTGAllocs = updatedEval.FailedTGAllocs
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

	// Get the signals required
	signals := job.RequiredSignals()

	// Validate the driver configurations.
	for _, tg := range job.TaskGroups {
		// Get the signals for the task group
		tgSignals, tgOk := signals[tg.Name]

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

			// The task group didn't have any task that required signals
			if !tgOk {
				continue
			}

			// This task requires signals. Ensure the driver is capable
			if required, ok := tgSignals[task.Name]; ok {
				abilities := d.Abilities()
				if !abilities.SendSignals {
					formatted := fmt.Errorf("group %q -> task %q: driver %q doesn't support sending signals. Requested signals are %v",
						tg.Name, task.Name, task.Driver, strings.Join(required, ", "))
					multierror.Append(validationErrors, formatted)
				}
			}
		}
	}

	if job.Type == structs.JobTypeCore {
		multierror.Append(validationErrors, fmt.Errorf("job type cannot be core"))
	}

	if len(job.Payload) != 0 {
		multierror.Append(validationErrors, fmt.Errorf("job can't be submitted with a payload, only dispatched"))
	}

	return validationErrors.ErrorOrNil()
}

// Dispatch is used to dispatch a job based on a constructor job.
func (j *Job) Dispatch(args *structs.JobDispatchRequest, reply *structs.JobDispatchResponse) error {
	if done, err := j.srv.forward("Job.Dispatch", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "dispatch"}, time.Now())

	// Lookup the job
	if args.JobID == "" {
		return fmt.Errorf("missing constructor job ID")
	}

	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	constructor, err := snap.JobByID(args.JobID)
	if err != nil {
		return err
	}
	if constructor == nil {
		return fmt.Errorf("constructor job not found")
	}

	if !constructor.IsConstructor() {
		return fmt.Errorf("Specified job %q is not a constructor job", args.JobID)
	}

	// Validate the arguments
	if err := validateDispatchRequest(args, constructor); err != nil {
		return err
	}

	// Derive the child job and commit it via Raft
	dispatchJob := constructor.Copy()
	dispatchJob.Constructor = nil
	dispatchJob.ID = structs.DispatchedID(constructor.ID, time.Now())
	dispatchJob.ParentID = constructor.ID
	dispatchJob.Name = dispatchJob.ID

	// Merge in the meta data
	for k, v := range args.Meta {
		if dispatchJob.Meta == nil {
			dispatchJob.Meta = make(map[string]string, len(args.Meta))
		}
		dispatchJob.Meta[k] = v
	}

	// Compress the payload
	dispatchJob.Payload = snappy.Encode(nil, args.Payload)

	regReq := &structs.JobRegisterRequest{
		Job:          dispatchJob,
		WriteRequest: args.WriteRequest,
	}

	// Commit this update via Raft
	_, jobCreateIndex, err := j.srv.raftApply(structs.JobRegisterRequestType, regReq)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Dispatched job register failed: %v", err)
		return err
	}

	// Create a new evaluation
	eval := &structs.Evaluation{
		ID:             structs.GenerateUUID(),
		Priority:       dispatchJob.Priority,
		Type:           dispatchJob.Type,
		TriggeredBy:    structs.EvalTriggerJobRegister,
		JobID:          dispatchJob.ID,
		JobModifyIndex: jobCreateIndex,
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
	reply.JobCreateIndex = jobCreateIndex
	reply.DispatchedJobID = dispatchJob.ID
	reply.Index = evalIndex
	return nil
}

// validateDispatchRequest returns whether the request is valid given the
// jobs constructor
func validateDispatchRequest(req *structs.JobDispatchRequest, job *structs.Job) error {
	// Check the payload constraint is met
	hasInputData := len(req.Payload) != 0
	if job.Constructor.Payload == structs.DispatchPayloadRequired && !hasInputData {
		return fmt.Errorf("Payload is not provided but required by constructor")
	} else if job.Constructor.Payload == structs.DispatchPayloadForbidden && hasInputData {
		return fmt.Errorf("Payload provided but forbidden by constructor")
	}

	// Check the payload doesn't exceed the size limit
	if l := len(req.Payload); l > DispatchPayloadSizeLimit {
		return fmt.Errorf("Payload exceeds maximum size; %d > %d", l, DispatchPayloadSizeLimit)
	}

	// Check if the metadata is a set
	keys := make(map[string]struct{}, len(req.Meta))
	for k := range keys {
		if _, ok := keys[k]; ok {
			return fmt.Errorf("Duplicate key %q in passed metadata", k)
		}
		keys[k] = struct{}{}
	}

	required := helper.SliceStringToSet(job.Constructor.MetaRequired)
	optional := helper.SliceStringToSet(job.Constructor.MetaOptional)

	// Check the metadata key constraints are met
	unpermitted := make(map[string]struct{})
	for k := range req.Meta {
		_, req := required[k]
		_, opt := optional[k]
		if !req && !opt {
			unpermitted[k] = struct{}{}
		}
	}

	if len(unpermitted) != 0 {
		flat := make([]string, 0, len(unpermitted))
		for k := range unpermitted {
			flat = append(flat, k)
		}

		return fmt.Errorf("Dispatch request included unpermitted metadata keys: %v", flat)
	}

	missing := make(map[string]struct{})
	for _, k := range job.Constructor.MetaRequired {
		if _, ok := req.Meta[k]; !ok {
			missing[k] = struct{}{}
		}
	}

	if len(missing) != 0 {
		flat := make([]string, 0, len(missing))
		for k := range missing {
			flat = append(flat, k)
		}

		return fmt.Errorf("Dispatch did not provide required meta keys: %v", flat)
	}

	return nil
}
