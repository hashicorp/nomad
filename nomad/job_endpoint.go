package nomad

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/golang/snappy"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/driver"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
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
	canonicalizeWarnings := args.Job.Canonicalize()

	// Add implicit constraints
	setImplicitConstraints(args.Job)

	// Validate the job and capture any warnings
	err, warnings := validateJob(args.Job)
	if err != nil {
		return err
	}

	// Set the warning message
	reply.Warnings = structs.MergeMultierrorWarnings(warnings, canonicalizeWarnings)

	// Lookup the job
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	ws := memdb.NewWatchSet()
	existingJob, err := snap.JobByID(ws, args.Job.ID)
	if err != nil {
		return err
	}

	// If EnforceIndex set, check it before trying to apply
	if args.EnforceIndex {
		jmi := args.JobModifyIndex
		if existingJob != nil {
			if jmi == 0 {
				return fmt.Errorf("%s 0: job already exists", RegisterEnforceIndexErrPrefix)
			} else if jmi != existingJob.JobModifyIndex {
				return fmt.Errorf("%s %d: job exists with conflicting job modify index: %d",
					RegisterEnforceIndexErrPrefix, jmi, existingJob.JobModifyIndex)
			}
		} else if jmi != 0 {
			return fmt.Errorf("%s %d: job does not exist", RegisterEnforceIndexErrPrefix, jmi)
		}
	}

	// Validate job transitions if its an update
	if existingJob != nil {
		if err := validateJobUpdate(existingJob, args.Job); err != nil {
			return err
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

	// Check if the job has changed at all
	if existingJob == nil || existingJob.SpecChanged(args.Job) {
		// Set the submit time
		args.Job.SetSubmitTime()

		// Commit this update via Raft
		_, index, err := j.srv.raftApply(structs.JobRegisterRequestType, args)
		if err != nil {
			j.srv.logger.Printf("[ERR] nomad.job: Register failed: %v", err)
			return err
		}

		// Populate the reply with job information
		reply.JobModifyIndex = index
	} else {
		reply.JobModifyIndex = existingJob.JobModifyIndex
	}

	// If the job is periodic or parameterized, we don't create an eval.
	if args.Job.IsPeriodic() || args.Job.IsParameterized() {
		return nil
	}

	// Create a new evaluation
	eval := &structs.Evaluation{
		ID:             structs.GenerateUUID(),
		Priority:       args.Job.Priority,
		Type:           args.Job.Type,
		TriggeredBy:    structs.EvalTriggerJobRegister,
		JobID:          args.Job.ID,
		JobModifyIndex: reply.JobModifyIndex,
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
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for job summary
			out, err := state.JobSummaryByID(ws, args.JobID)
			if err != nil {
				return err
			}

			// Setup the output
			reply.JobSummary = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the job_summary table
				index, err := state.Index("job_summary")
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

// Validate validates a job
func (j *Job) Validate(args *structs.JobValidateRequest, reply *structs.JobValidateResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "job", "validate"}, time.Now())

	// Initialize the job fields (sets defaults and any necessary init work).
	canonicalizeWarnings := args.Job.Canonicalize()

	// Add implicit constraints
	setImplicitConstraints(args.Job)

	// Validate the job and capture any warnings
	err, warnings := validateJob(args.Job)
	if err != nil {
		if merr, ok := err.(*multierror.Error); ok {
			for _, err := range merr.Errors {
				reply.ValidationErrors = append(reply.ValidationErrors, err.Error())
			}
			reply.Error = merr.Error()
		} else {
			reply.ValidationErrors = append(reply.ValidationErrors, err.Error())
			reply.Error = err.Error()
		}
	}

	// Set the warning message
	reply.Warnings = structs.MergeMultierrorWarnings(warnings, canonicalizeWarnings)
	reply.DriverConfigValidated = true
	return nil
}

// Revert is used to revert the job to a prior version
func (j *Job) Revert(args *structs.JobRevertRequest, reply *structs.JobRegisterResponse) error {
	if done, err := j.srv.forward("Job.Revert", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "revert"}, time.Now())

	// Validate the arguments
	if args.JobID == "" {
		return fmt.Errorf("missing job ID for revert")
	}

	// Lookup the job by version
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	cur, err := snap.JobByID(ws, args.JobID)
	if err != nil {
		return err
	}
	if cur == nil {
		return fmt.Errorf("job %q not found", args.JobID)
	}
	if args.JobVersion == cur.Version {
		return fmt.Errorf("can't revert to current version")
	}

	jobV, err := snap.JobByIDAndVersion(ws, args.JobID, args.JobVersion)
	if err != nil {
		return err
	}
	if jobV == nil {
		return fmt.Errorf("job %q at version %d not found", args.JobID, args.JobVersion)
	}

	// Build the register request
	reg := &structs.JobRegisterRequest{
		Job:          jobV.Copy(),
		WriteRequest: args.WriteRequest,
	}

	// If the request is enforcing the existing version do a check.
	if args.EnforcePriorVersion != nil {
		if cur.Version != *args.EnforcePriorVersion {
			return fmt.Errorf("Current job has version %d; enforcing version %d", cur.Version, *args.EnforcePriorVersion)
		}

		reg.EnforceIndex = true
		reg.JobModifyIndex = cur.JobModifyIndex
	}

	// Register the version.
	return j.Register(reg, reply)
}

// Stable is used to mark the job version as stable
func (j *Job) Stable(args *structs.JobStabilityRequest, reply *structs.JobStabilityResponse) error {
	if done, err := j.srv.forward("Job.Stable", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "stable"}, time.Now())

	// Validate the arguments
	if args.JobID == "" {
		return fmt.Errorf("missing job ID for marking job as stable")
	}

	// Lookup the job by version
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	jobV, err := snap.JobByIDAndVersion(ws, args.JobID, args.JobVersion)
	if err != nil {
		return err
	}
	if jobV == nil {
		return fmt.Errorf("job %q at version %d not found", args.JobID, args.JobVersion)
	}

	// Commit this stability request via Raft
	_, modifyIndex, err := j.srv.raftApply(structs.JobStabilityRequestType, args)
	if err != nil {
		j.srv.logger.Printf("[ERR] nomad.job: Job stability request failed: %v", err)
		return err
	}

	// Setup the reply
	reply.Index = modifyIndex
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
	ws := memdb.NewWatchSet()
	job, err := snap.JobByID(ws, args.JobID)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	if job.IsPeriodic() {
		return fmt.Errorf("can't evaluate periodic job")
	} else if job.IsParameterized() {
		return fmt.Errorf("can't evaluate parameterized job")
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
		return fmt.Errorf("missing job ID for deregistering")
	}

	// Lookup the job
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	ws := memdb.NewWatchSet()
	job, err := snap.JobByID(ws, args.JobID)
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

	// If the job is periodic or parameterized, we don't create an eval.
	if job != nil && (job.IsPeriodic() || job.IsParameterized()) {
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
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for the job
			out, err := state.JobByID(ws, args.JobID)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Job = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the nodes table
				index, err := state.Index("jobs")
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

// GetJobVersions is used to retrieve all tracked versions of a job.
func (j *Job) GetJobVersions(args *structs.JobVersionsRequest,
	reply *structs.JobVersionsResponse) error {
	if done, err := j.srv.forward("Job.GetJobVersions", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "get_job_versions"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for the job
			out, err := state.JobVersionsByID(ws, args.JobID)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Versions = out
			if len(out) != 0 {
				reply.Index = out[0].ModifyIndex

				// Compute the diffs
				if args.Diffs {
					for i := 0; i < len(out)-1; i++ {
						old, new := out[i+1], out[i]
						d, err := old.Diff(new, true)
						if err != nil {
							return fmt.Errorf("failed to create job diff: %v", err)
						}
						reply.Diffs = append(reply.Diffs, d)
					}
				}
			} else {
				// Use the last index that affected the nodes table
				index, err := state.Index("job_version")
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
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture all the jobs
			var err error
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = state.JobsByIDPrefix(ws, prefix)
			} else {
				iter, err = state.Jobs(ws)
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
				summary, err := state.JobSummaryByID(ws, job.ID)
				if err != nil {
					return fmt.Errorf("unable to look up summary for job: %v", job.ID)
				}
				jobs = append(jobs, job.Stub(summary))
			}
			reply.Jobs = jobs

			// Use the last index that affected the jobs table
			index, err := state.Index("jobs")
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
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture the allocations
			allocs, err := state.AllocsByJob(ws, args.JobID, args.AllAllocs)
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
			index, err := state.Index("allocs")
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
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture the evals
			var err error
			reply.Evaluations, err = state.EvalsByJob(ws, args.JobID)
			if err != nil {
				return err
			}

			// Use the last index that affected the evals table
			index, err := state.Index("evals")
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

// Deployments is used to list the deployments for a job
func (j *Job) Deployments(args *structs.JobSpecificRequest,
	reply *structs.DeploymentListResponse) error {
	if done, err := j.srv.forward("Job.Deployments", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "deployments"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture the deployments
			deploys, err := state.DeploymentsByJobID(ws, args.JobID)
			if err != nil {
				return err
			}

			// Use the last index that affected the deployment table
			index, err := state.Index("deployment")
			if err != nil {
				return err
			}
			reply.Index = index
			reply.Deployments = deploys

			// Set the query response
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil

		}}
	return j.srv.blockingRPC(&opts)
}

// LatestDeployment is used to retrieve the latest deployment for a job
func (j *Job) LatestDeployment(args *structs.JobSpecificRequest,
	reply *structs.SingleDeploymentResponse) error {
	if done, err := j.srv.forward("Job.LatestDeployment", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "latest_deployment"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture the deployments
			deploys, err := state.DeploymentsByJobID(ws, args.JobID)
			if err != nil {
				return err
			}

			// Use the last index that affected the deployment table
			index, err := state.Index("deployment")
			if err != nil {
				return err
			}
			reply.Index = index
			if len(deploys) > 0 {
				sort.Slice(deploys, func(i, j int) bool {
					return deploys[i].CreateIndex > deploys[j].CreateIndex
				})
				reply.Deployment = deploys[0]
			}

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
	canonicalizeWarnings := args.Job.Canonicalize()

	// Add implicit constraints
	setImplicitConstraints(args.Job)

	// Validate the job and capture any warnings
	err, warnings := validateJob(args.Job)
	if err != nil {
		return err
	}

	// Set the warning message
	reply.Warnings = structs.MergeMultierrorWarnings(warnings, canonicalizeWarnings)

	// Acquire a snapshot of the state
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	// Get the original job
	ws := memdb.NewWatchSet()
	oldJob, err := snap.JobByID(ws, args.Job.ID)
	if err != nil {
		return err
	}

	var index uint64
	var updatedIndex uint64

	if oldJob != nil {
		index = oldJob.JobModifyIndex

		// We want to reuse deployments where possible, so only insert the job if
		// it has changed or the job didn't exist
		if oldJob.SpecChanged(args.Job) {
			// Insert the updated Job into the snapshot
			updatedIndex = oldJob.JobModifyIndex + 1
			snap.UpsertJob(updatedIndex, args.Job)
		}
	} else if oldJob == nil {
		// Insert the updated Job into the snapshot
		snap.UpsertJob(100, args.Job)
	}

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
		reply.NextPeriodicLaunch = args.Job.Periodic.Next(time.Now().In(args.Job.Periodic.GetLocation()))
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
func validateJob(job *structs.Job) (invalid, warnings error) {
	validationErrors := new(multierror.Error)
	if err := job.Validate(); err != nil {
		multierror.Append(validationErrors, err)
	}

	// Get any warnings
	warnings = job.Warnings()

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

	return validationErrors.ErrorOrNil(), warnings
}

// validateJobUpdate ensures updates to a job are valid.
func validateJobUpdate(old, new *structs.Job) error {
	// Type transitions are disallowed
	if old.Type != new.Type {
		return fmt.Errorf("cannot update job from type %q to %q", old.Type, new.Type)
	}

	// Transitioning to/from periodic is disallowed
	if old.IsPeriodic() && !new.IsPeriodic() {
		return fmt.Errorf("cannot update non-periodic job to being periodic")
	}
	if new.IsPeriodic() && !old.IsPeriodic() {
		return fmt.Errorf("cannot update periodic job to being non-periodic")
	}

	// Transitioning to/from parameterized is disallowed
	if old.IsParameterized() && !new.IsParameterized() {
		return fmt.Errorf("cannot update non-parameterized job to being parameterized")
	}
	if new.IsParameterized() && !old.IsParameterized() {
		return fmt.Errorf("cannot update parameterized job to being non-parameterized")
	}

	return nil
}

// Dispatch a parameterized job.
func (j *Job) Dispatch(args *structs.JobDispatchRequest, reply *structs.JobDispatchResponse) error {
	if done, err := j.srv.forward("Job.Dispatch", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "dispatch"}, time.Now())

	// Lookup the parameterized job
	if args.JobID == "" {
		return fmt.Errorf("missing parameterized job ID")
	}

	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	ws := memdb.NewWatchSet()
	parameterizedJob, err := snap.JobByID(ws, args.JobID)
	if err != nil {
		return err
	}
	if parameterizedJob == nil {
		return fmt.Errorf("parameterized job not found")
	}

	if !parameterizedJob.IsParameterized() {
		return fmt.Errorf("Specified job %q is not a parameterized job", args.JobID)
	}

	if parameterizedJob.Stop {
		return fmt.Errorf("Specified job %q is stopped", args.JobID)
	}

	// Validate the arguments
	if err := validateDispatchRequest(args, parameterizedJob); err != nil {
		return err
	}

	// Derive the child job and commit it via Raft
	dispatchJob := parameterizedJob.Copy()
	dispatchJob.ParameterizedJob = nil
	dispatchJob.ID = structs.DispatchedID(parameterizedJob.ID, time.Now())
	dispatchJob.ParentID = parameterizedJob.ID
	dispatchJob.Name = dispatchJob.ID
	dispatchJob.SetSubmitTime()

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

	reply.JobCreateIndex = jobCreateIndex
	reply.DispatchedJobID = dispatchJob.ID
	reply.Index = jobCreateIndex

	// If the job is periodic, we don't create an eval.
	if !dispatchJob.IsPeriodic() {
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
		reply.Index = evalIndex
	}

	return nil
}

// validateDispatchRequest returns whether the request is valid given the
// parameterized job.
func validateDispatchRequest(req *structs.JobDispatchRequest, job *structs.Job) error {
	// Check the payload constraint is met
	hasInputData := len(req.Payload) != 0
	if job.ParameterizedJob.Payload == structs.DispatchPayloadRequired && !hasInputData {
		return fmt.Errorf("Payload is not provided but required by parameterized job")
	} else if job.ParameterizedJob.Payload == structs.DispatchPayloadForbidden && hasInputData {
		return fmt.Errorf("Payload provided but forbidden by parameterized job")
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

	required := helper.SliceStringToSet(job.ParameterizedJob.MetaRequired)
	optional := helper.SliceStringToSet(job.ParameterizedJob.MetaOptional)

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
	for _, k := range job.ParameterizedJob.MetaRequired {
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
