// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/golang/snappy"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
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

// ErrMultipleNamespaces is send when multiple namespaces are used in the OSS setup
var ErrMultipleNamespaces = errors.New("multiple Vault namespaces requires Nomad Enterprise")

var (
	// allowRescheduleTransition is the transition that allows failed
	// allocations to be force rescheduled. We create a one off
	// variable to avoid creating a new object for every request.
	allowForceRescheduleTransition = &structs.DesiredTransition{
		ForceReschedule: pointer.Of(true),
	}
)

// Job endpoint is used for job interactions
type Job struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger

	// builtin admission controllers
	mutators   []jobMutator
	validators []jobValidator
}

// NewJobEndpoints creates a new job endpoint with builtin admission controllers
func NewJobEndpoints(s *Server, ctx *RPCContext) *Job {
	return &Job{
		srv:    s,
		ctx:    ctx,
		logger: s.logger.Named("job"),
		mutators: []jobMutator{
			&jobCanonicalizer{srv: s},
			jobVaultHook{srv: s},
			jobConsulHook{srv: s},
			jobConnectHook{},
			jobExposeCheckHook{},
			jobImpliedConstraints{},
			jobNodePoolMutatingHook{srv: s},
			jobImplicitIdentitiesHook{srv: s},
			jobNumaHook{},
		},
		validators: []jobValidator{
			jobConnectHook{},
			jobExposeCheckHook{},
			jobVaultHook{srv: s},
			jobConsulHook{srv: s},
			jobNamespaceConstraintCheckHook{srv: s},
			jobNodePoolValidatingHook{srv: s},
			&jobValidate{srv: s},
			&memoryOversubscriptionValidate{srv: s},
			jobNumaHook{},
			&jobSchedHook{},
		},
	}
}

// Register is used to upsert a job for scheduling
func (j *Job) Register(args *structs.JobRegisterRequest, reply *structs.JobRegisterResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Register", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "register"}, time.Now())

	aclObj, err := j.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if ok, err := registrationsAreAllowed(aclObj, j.srv.State()); !ok || err != nil {
		j.logger.Warn("job registration is currently disabled for non-management ACL")
		return structs.ErrJobRegistrationDisabled
	}

	// Validate the arguments
	if args.Job == nil {
		return fmt.Errorf("missing job for registration")
	}

	// defensive check; http layer and RPC requester should ensure namespaces are set consistently
	if args.RequestNamespace() != args.Job.Namespace {
		return fmt.Errorf("mismatched request namespace in request: %q, %q", args.RequestNamespace(), args.Job.Namespace)
	}

	// Run admission controllers
	job, warnings, err := j.admissionControllers(args.Job)
	if err != nil {
		return err
	}
	args.Job = job

	// Run the submission controller
	warnings = append(warnings, j.submissionController(args))

	// Attach the user token's accessor ID so that deploymentwatcher can
	// reference the token later in multiregion deployments. We can't auth once
	// and then use the leader ACL because the leader ACLs aren't shared across
	// regions. Note this implies WIs can't be used to register multi-region
	// jobs b/c their identities are only valid in a single region.
	if args.GetIdentity().ACLToken != nil &&
		args.GetIdentity().ACLToken.AccessorID != "acls-disabled" {
		args.Job.NomadTokenID = args.GetIdentity().ACLToken.AccessorID
	}

	// Set the warning message
	reply.Warnings = helper.MergeMultierrorWarnings(warnings...)

	// Check job submission permissions
	if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	// Validate Volume Permissions
	for _, tg := range args.Job.TaskGroups {
		for _, vol := range tg.Volumes {
			switch vol.Type {
			case structs.VolumeTypeCSI:
				if !allowCSIMount(aclObj, args.RequestNamespace()) {
					return structs.ErrPermissionDenied
				}
			case structs.VolumeTypeHost:
				// If a volume is readonly, then we allow access if the user has
				// ReadOnly or ReadWrite access to the volume. Otherwise we only
				// allow access if they have ReadWrite access.
				if vol.ReadOnly {
					if !aclObj.AllowHostVolumeOperation(vol.Source, acl.HostVolumeCapabilityMountReadOnly) &&
						!aclObj.AllowHostVolumeOperation(vol.Source, acl.HostVolumeCapabilityMountReadWrite) {
						return structs.ErrPermissionDenied
					}
				} else {
					if !aclObj.AllowHostVolumeOperation(vol.Source, acl.HostVolumeCapabilityMountReadWrite) {
						return structs.ErrPermissionDenied
					}
				}
			default:
				return structs.ErrPermissionDenied
			}
		}

		for _, t := range tg.Tasks {
			for _, vm := range t.VolumeMounts {
				vol := tg.Volumes[vm.Volume]
				if vm.PropagationMode == structs.VolumeMountPropagationBidirectional &&
					!aclObj.AllowHostVolumeOperation(vol.Source, acl.HostVolumeCapabilityMountReadWrite) {
					return structs.ErrPermissionDenied
				}
			}

			if t.CSIPluginConfig != nil {
				if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityCSIRegisterPlugin) {
					return structs.ErrPermissionDenied
				}
			}
		}

		// Check if override is set and we do not have permissions
		if args.PolicyOverride {
			if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySentinelOverride) {
				j.logger.Warn("policy override attempted without permissions for job", "job", args.Job.ID)
				return structs.ErrPermissionDenied
			}
			j.logger.Warn("policy override set for job", "job", args.Job.ID)
		}
	}

	// Lookup the job
	snap, err := j.srv.State().Snapshot()
	if err != nil {
		return err
	}
	ws := memdb.NewWatchSet()
	existingJob, err := snap.JobByID(ws, args.RequestNamespace(), args.Job.ID)
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
	if err := validateJobUpdate(existingJob, args.Job); err != nil {
		return err
	}

	// Ensure that all scaling policies have an appropriate ID
	if err := propagateScalingPolicyIDs(existingJob, args.Job); err != nil {
		return err
	}

	// helper function that checks if the Consul token supplied with the job has
	// sufficient ACL permissions for:
	//   - registering services into namespace of each group
	//   - reading kv store of each group
	//   - establishing consul connect services
	checkConsulToken := func(usages map[string]*structs.ConsulUsage) error {
		if j.srv.config.GetDefaultConsul().AllowsUnauthenticated() {
			// if consul.allow_unauthenticated is enabled (which is the default)
			// just let the job through without checking anything
			return nil
		}

		ctx := context.Background()
		for namespace, usage := range usages {
			if err := j.srv.consulACLs.CheckPermissions(ctx, namespace, usage, args.Job.ConsulToken); err != nil {
				return fmt.Errorf("job-submitter consul token denied: %w", err)
			}
		}

		return nil
	}

	// Enforce the job-submitter has a Consul token with necessary ACL permissions.
	if err := checkConsulToken(args.Job.ConsulUsages()); err != nil {
		return err
	}

	// Enforce Sentinel policies. Pass a copy of the job to prevent
	// sentinel from altering it.
	ns, err := snap.NamespaceByName(nil, args.RequestNamespace())
	if err != nil {
		return err
	}

	policyWarnings, err := j.enforceSubmitJob(args.PolicyOverride, args.Job.Copy(),
		existingJob, args.GetIdentity().GetACLToken(), ns)
	if err != nil {
		return err
	}
	if policyWarnings != nil {
		warnings = append(warnings, policyWarnings)
		reply.Warnings = helper.MergeMultierrorWarnings(warnings...)
	}

	// Create or Update Consul Configuration Entries defined in the job. For now
	// Nomad only supports Configuration Entries types
	// - "ingress-gateway" for managing Ingress Gateways
	// - "terminating-gateway" for managing Terminating Gateways
	//
	// This is done as a blocking operation that prevents the job from being
	// submitted if the configuration entries cannot be set in Consul.
	//
	// Every job update will re-write the Configuration Entry into Consul.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for ns, entries := range args.Job.ConfigEntries() {
		for service, entry := range entries.Ingress {
			if errCE := j.srv.consulConfigEntries.SetIngressCE(
				ctx, ns, service, entries.Cluster, entries.Partition, entry); errCE != nil {
				return errCE
			}
		}
		for service, entry := range entries.Terminating {
			if errCE := j.srv.consulConfigEntries.SetTerminatingCE(
				ctx, ns, service, entries.Cluster, entries.Partition, entry); errCE != nil {
				return errCE
			}
		}
	}

	// Clear the Vault token
	args.Job.VaultToken = ""

	// Clear the Consul token
	args.Job.ConsulToken = ""

	// Preserve the existing task group counts, if so requested
	if existingJob != nil && args.PreserveCounts {
		prevCounts := make(map[string]int)
		for _, tg := range existingJob.TaskGroups {
			prevCounts[tg.Name] = tg.Count
		}
		for _, tg := range args.Job.TaskGroups {
			if count, ok := prevCounts[tg.Name]; ok {
				tg.Count = count
			}
		}
	}

	// Submit a multiregion job to other regions (enterprise only).
	// The job will have its region interpolated.
	var newVersion uint64
	if existingJob != nil {
		newVersion = existingJob.Version + 1
	}
	isRunner, err := j.multiregionRegister(args, reply, newVersion)
	if err != nil {
		return err
	}

	// Create a new evaluation
	now := time.Now().UnixNano()
	submittedEval := false
	var eval *structs.Evaluation

	// Set the submit time
	args.Job.SubmitTime = now

	// If the job is periodic or parameterized, we don't create an eval.
	if !(args.Job.IsPeriodic() || args.Job.IsParameterized()) {

		// Initially set the eval priority to that of the job priority. If the
		// user supplied an eval priority override, we subsequently use this.
		evalPriority := args.Job.Priority
		if args.EvalPriority > 0 {
			evalPriority = args.EvalPriority
		}

		eval = &structs.Evaluation{
			ID:          uuid.Generate(),
			Namespace:   args.RequestNamespace(),
			Priority:    evalPriority,
			Type:        args.Job.Type,
			TriggeredBy: structs.EvalTriggerJobRegister,
			JobID:       args.Job.ID,
			Status:      structs.EvalStatusPending,
			CreateTime:  now,
			ModifyTime:  now,
		}
		reply.EvalID = eval.ID
	}

	// Check if the job has changed at all
	specChanged, err := j.multiregionSpecChanged(existingJob, args)
	if err != nil {
		return err
	}

	if existingJob == nil || specChanged {

		if eval != nil {
			args.Eval = eval
			submittedEval = true
		}

		// Pre-register a deployment if necessary.
		args.Deployment = j.multiregionCreateDeployment(job, eval)

		// Commit this update via Raft
		_, index, err := j.srv.raftApply(structs.JobRegisterRequestType, args)
		if err != nil {
			j.logger.Error("registering job failed", "error", err)
			return err
		}

		// Populate the reply with job information
		reply.JobModifyIndex = index
		reply.Index = index

		if submittedEval {
			reply.EvalCreateIndex = index
		}

	} else {
		reply.JobModifyIndex = existingJob.JobModifyIndex
	}

	// used for multiregion start
	args.Job.JobModifyIndex = reply.JobModifyIndex

	if eval == nil {
		// For dispatch jobs we return early, so we need to drop regions
		// here rather than after eval for deployments is kicked off
		err = j.multiregionDrop(args, reply)
		if err != nil {
			return err
		}
		return nil
	}

	if eval != nil && !submittedEval {
		eval.JobModifyIndex = reply.JobModifyIndex
		update := &structs.EvalUpdateRequest{
			Evals:        []*structs.Evaluation{eval},
			WriteRequest: structs.WriteRequest{Region: args.Region},
		}

		// Commit this evaluation via Raft
		// There is a risk of partial failure where the JobRegister succeeds
		// but that the EvalUpdate does not, before 0.12.1
		_, evalIndex, err := j.srv.raftApply(structs.EvalUpdateRequestType, update)
		if err != nil {
			j.logger.Error("eval create failed", "error", err, "method", "register")
			return err
		}

		reply.EvalCreateIndex = evalIndex
		reply.Index = evalIndex
	}

	// Kick off a multiregion deployment (enterprise only).
	if isRunner {
		err = j.multiregionStart(args, reply)
		if err != nil {
			return err
		}
		// We drop any unwanted regions only once we know all jobs have
		// been registered and we've kicked off the deployment. This keeps
		// dropping regions close in semantics to dropping task groups in
		// single-region deployments
		err = j.multiregionDrop(args, reply)
		if err != nil {
			return err
		}
	}

	return nil
}

// propagateScalingPolicyIDs propagates scaling policy IDs from existing job
// to updated job, or generates random IDs in new job
func propagateScalingPolicyIDs(old, new *structs.Job) error {

	oldIDs := make(map[string]string)
	if old != nil {
		// use the job-scoped key (includes type, group, and task) to uniquely
		// identify policies in a job
		for _, p := range old.GetScalingPolicies() {
			oldIDs[p.JobKey()] = p.ID
		}
	}

	// ignore any existing ID in the policy, they should be empty
	for _, p := range new.GetScalingPolicies() {
		if id, ok := oldIDs[p.JobKey()]; ok {
			p.ID = id
		} else {
			p.ID = uuid.Generate()
		}
	}

	return nil
}

// getSignalConstraint builds a suitable constraint based on the required
// signals
func getSignalConstraint(signals []string) *structs.Constraint {
	sort.Strings(signals)
	return &structs.Constraint{
		Operand: structs.ConstraintSetContains,
		LTarget: "${attr.os.signals}",
		RTarget: strings.Join(signals, ","),
	}
}

// Summary retrieves the summary of a job.
func (j *Job) Summary(args *structs.JobSummaryRequest, reply *structs.JobSummaryResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Summary", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job_summary", "get_job_summary"}, time.Now())

	// Check for read-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for job summary
			out, err := state.JobSummaryByID(ws, args.RequestNamespace(), args.JobID)
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

// Validate validates a job.
//
// Must forward to the leader, because only the leader will have a live Vault
// client with which to validate vault tokens.
func (j *Job) Validate(args *structs.JobValidateRequest, reply *structs.JobValidateResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Validate", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "validate"}, time.Now())

	// defensive check; http layer and RPC requester should ensure namespaces are set consistently
	if args.RequestNamespace() != args.Job.Namespace {
		return fmt.Errorf("mismatched request namespace in request: %q, %q", args.RequestNamespace(), args.Job.Namespace)
	}

	job, mutateWarnings, err := j.admissionMutators(args.Job)
	if err != nil {
		return err
	}
	args.Job = job

	// Check for read-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Validate the job and capture any warnings
	validateWarnings, err := j.admissionValidators(args.Job)
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

	validateWarnings = append(validateWarnings, mutateWarnings...)

	// Set the warning message
	reply.Warnings = helper.MergeMultierrorWarnings(validateWarnings...)
	reply.DriverConfigValidated = true
	return nil
}

// Revert is used to revert the job to a prior version
func (j *Job) Revert(args *structs.JobRevertRequest, reply *structs.JobRegisterResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Revert", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "revert"}, time.Now())

	// Check for submit-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

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
	cur, err := snap.JobByID(ws, args.RequestNamespace(), args.JobID)
	if err != nil {
		return err
	}
	if cur == nil {
		return fmt.Errorf("job %q not found", args.JobID)
	}
	if args.JobVersion == cur.Version {
		return fmt.Errorf("can't revert to current version")
	}

	jobV, err := snap.JobByIDAndVersion(ws, args.RequestNamespace(), args.JobID, args.JobVersion)
	if err != nil {
		return err
	}
	if jobV == nil {
		return fmt.Errorf("job %q in namespace %q at version %d not found", args.JobID, args.RequestNamespace(), args.JobVersion)
	}

	// Build the register request
	revJob := jobV.Copy()
	revJob.VaultToken = args.VaultToken   // use vault token from revert to perform (re)registration
	revJob.ConsulToken = args.ConsulToken // use consul token from revert to perform (re)registration

	// Clear out the VersionTag to prevent tag duplication
	revJob.VersionTag = nil

	reg := &structs.JobRegisterRequest{
		Job:          revJob,
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
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Stable", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "stable"}, time.Now())

	// Check for read-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

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
	jobV, err := snap.JobByIDAndVersion(ws, args.RequestNamespace(), args.JobID, args.JobVersion)
	if err != nil {
		return err
	}
	if jobV == nil {
		return fmt.Errorf("job %q in namespace %q at version %d not found", args.JobID, args.RequestNamespace(), args.JobVersion)
	}

	// Commit this stability request via Raft
	_, modifyIndex, err := j.srv.raftApply(structs.JobStabilityRequestType, args)
	if err != nil {
		j.logger.Error("submitting job stability request failed", "error", err)
		return err
	}

	// Setup the reply
	reply.Index = modifyIndex
	return nil
}

// Evaluate is used to force a job for re-evaluation
func (j *Job) Evaluate(args *structs.JobEvaluateRequest, reply *structs.JobRegisterResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Evaluate", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "evaluate"}, time.Now())

	// Check for submit-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

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
	job, err := snap.JobByID(ws, args.RequestNamespace(), args.JobID)
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

	forceRescheduleAllocs := make(map[string]*structs.DesiredTransition)

	if args.EvalOptions.ForceReschedule {
		// Find any failed allocs that could be force rescheduled
		allocs, err := snap.AllocsByJob(ws, args.RequestNamespace(), args.JobID, false)
		if err != nil {
			return err
		}

		for _, alloc := range allocs {
			taskGroup := job.LookupTaskGroup(alloc.TaskGroup)
			// Forcing rescheduling is only allowed if task group has rescheduling enabled
			if taskGroup == nil || !taskGroup.ReschedulePolicy.Enabled() {
				continue
			}

			if alloc.NextAllocation == "" && alloc.ClientStatus == structs.AllocClientStatusFailed && !alloc.DesiredTransition.ShouldForceReschedule() {
				forceRescheduleAllocs[alloc.ID] = allowForceRescheduleTransition
			}
		}
	}

	// Create a new evaluation
	now := time.Now().UnixNano()
	eval := &structs.Evaluation{
		ID:             uuid.Generate(),
		Namespace:      args.RequestNamespace(),
		Priority:       job.Priority,
		Type:           job.Type,
		TriggeredBy:    structs.EvalTriggerJobRegister,
		JobID:          job.ID,
		JobModifyIndex: job.ModifyIndex,
		Status:         structs.EvalStatusPending,
		CreateTime:     now,
		ModifyTime:     now,
	}

	// Create a AllocUpdateDesiredTransitionRequest request with the eval and any forced rescheduled allocs
	updateTransitionReq := &structs.AllocUpdateDesiredTransitionRequest{
		Allocs: forceRescheduleAllocs,
		Evals:  []*structs.Evaluation{eval},
	}
	_, evalIndex, err := j.srv.raftApply(structs.AllocUpdateDesiredTransitionRequestType, updateTransitionReq)

	if err != nil {
		j.logger.Error("eval create failed", "error", err, "method", "evaluate")
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
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Deregister", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "deregister"}, time.Now())

	// Check for submit-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

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
	job, err := snap.JobByID(ws, args.RequestNamespace(), args.JobID)
	if err != nil {
		return err
	}
	if job == nil {
		return nil
	}

	var eval *structs.Evaluation

	// The job priority / type is strange for this, since it's not a high
	// priority even if the job was.
	now := time.Now().UnixNano()

	// If the job is periodic or parameterized, we don't create an eval.
	if !(job.IsPeriodic() || job.IsParameterized()) {

		// The evaluation priority is determined by several factors. It
		// defaults to the job default priority and is overridden by the
		// priority set on the job specification.
		//
		// If the user supplied an eval priority override, we subsequently
		// use this.
		priority := structs.JobDefaultPriority
		if job.Priority > 0 {
			priority = job.Priority
		}
		if args.EvalPriority > 0 {
			priority = args.EvalPriority
		}

		eval = &structs.Evaluation{
			ID:          uuid.Generate(),
			Namespace:   args.RequestNamespace(),
			Priority:    priority,
			Type:        structs.JobTypeService,
			TriggeredBy: structs.EvalTriggerJobDeregister,
			JobID:       args.JobID,
			Status:      structs.EvalStatusPending,
			CreateTime:  now,
			ModifyTime:  now,
		}
		reply.EvalID = eval.ID
	}

	args.SubmitTime = now
	args.Eval = eval

	// Commit the job update via Raft
	_, index, err := j.srv.raftApply(structs.JobDeregisterRequestType, args)
	if err != nil {
		j.logger.Error("deregister failed", "error", err)
		return err
	}

	// Populate the reply with job information
	reply.JobModifyIndex = index
	reply.EvalCreateIndex = index
	reply.Index = index

	err = j.multiregionStop(job, args, reply)
	if err != nil {
		return err
	}

	return nil
}

// BatchDeregister is used to remove a set of jobs from the cluster. This
// endpoint is used for garbage collection purposes only and is not exposed via
// the HTTP API. It is the responsibility of the caller to ensure the jobs are
// eligible for GC and that they have no running or pending allocs or evals.
func (j *Job) BatchDeregister(args *structs.JobBatchDeregisterRequest, reply *structs.JobBatchDeregisterResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward(structs.JobBatchDeregisterRPCMethod, args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "batch_deregister"}, time.Now())

	aclObj, err := j.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	// This RPC endpoint is only called from a Nomad server using the leader
	// ACL when performing garbage collection, therefore we can get away with a
	// simple management check to ensure the caller can trigger this
	// functionality as the leader token is always a management token.
	if !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Validate the arguments
	if len(args.Jobs) == 0 {
		return fmt.Errorf("given no jobs to deregister")
	}

	// Commit this update via Raft
	_, index, err := j.srv.raftApply(structs.JobBatchDeregisterRequestType, args)
	if err != nil {
		j.logger.Error("batch deregister failed", "error", err)
		return err
	}

	reply.Index = index
	return nil
}

// Scale is used to modify one of the scaling targets in the job
func (j *Job) Scale(args *structs.JobScaleRequest, reply *structs.JobRegisterResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Scale", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "scale"}, time.Now())

	namespace := args.RequestNamespace()

	aclObj, err := j.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	hasScaleJob := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityScaleJob)
	hasSubmitJob := aclObj.AllowNsOp(namespace, acl.NamespaceCapabilitySubmitJob)
	if !(hasScaleJob || hasSubmitJob) {
		return structs.ErrPermissionDenied
	}

	if ok, err := registrationsAreAllowed(aclObj, j.srv.State()); !ok || err != nil {
		j.logger.Warn("job scaling is currently disabled for non-management ACL")
		return structs.ErrJobRegistrationDisabled
	}

	// Validate args
	err = args.Validate()
	if err != nil {
		return err
	}

	// Find job
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	job, err := snap.JobByID(ws, namespace, args.JobID)
	if err != nil {
		j.logger.Error("unable to lookup job", "error", err)
		return err
	}

	// Perform validation on the job to ensure we have something that can
	// actually be scaled. This logic can only exist here, as we need access
	// to the job object.
	if job == nil {
		return structs.NewErrRPCCoded(404, fmt.Sprintf("job %q not found", args.JobID))
	}

	if job.Type == structs.JobTypeSystem && *args.Count > 1 {
		return structs.NewErrRPCCoded(http.StatusBadRequest, `jobs of type "system" can only be scaled to 1 or 0`)
	}

	// Since job is going to be mutated we must copy it since state store methods
	// return a shared pointer.
	job = job.Copy()

	// Find target group in job TaskGroups
	groupName := args.Target[structs.ScalingTargetGroup]
	var group *structs.TaskGroup
	for _, tg := range job.TaskGroups {
		if tg.Name == groupName {
			group = tg
			break
		}
	}

	if group == nil {
		return structs.NewErrRPCCoded(400,
			fmt.Sprintf("task group %q specified for scaling does not exist in job", groupName))
	}

	now := time.Now().UnixNano()
	prevCount := int64(group.Count)

	event := &structs.ScalingEventRequest{
		Namespace: job.Namespace,
		JobID:     job.ID,
		TaskGroup: groupName,
		ScalingEvent: &structs.ScalingEvent{
			Time:          now,
			PreviousCount: prevCount,
			Count:         args.Count,
			Message:       args.Message,
			Error:         args.Error,
			Meta:          args.Meta,
		},
	}

	if args.Count != nil {
		// Further validation for count-based scaling event
		if group.Scaling != nil {
			if *args.Count < group.Scaling.Min {
				return structs.NewErrRPCCoded(400,
					fmt.Sprintf("group count was less than scaling policy minimum: %d < %d",
						*args.Count, group.Scaling.Min))
			}
			if group.Scaling.Max < *args.Count {
				return structs.NewErrRPCCoded(400,
					fmt.Sprintf("group count was greater than scaling policy maximum: %d > %d",
						*args.Count, group.Scaling.Max))
			}
		}

		// Update group count
		group.Count = int(*args.Count)
		job.SubmitTime = now

		// Block scaling event if there's an active deployment
		deployment, err := snap.LatestDeploymentByJobID(ws, namespace, args.JobID)
		if err != nil {
			j.logger.Error("unable to lookup latest deployment", "error", err)
			return err
		}

		if deployment != nil && deployment.Active() && deployment.JobCreateIndex == job.CreateIndex {
			return structs.NewErrRPCCoded(400, "job scaling blocked due to active deployment")
		}

		// If JobModifyIndex set, check it before trying to apply
		if args.JobModifyIndex > 0 {
			if args.JobModifyIndex != job.JobModifyIndex {
				return fmt.Errorf("%s %d: job exists with conflicting job modify index: %d",
					RegisterEnforceIndexErrPrefix, args.JobModifyIndex, job.JobModifyIndex)
			}
		}

		// Commit the job update
		_, jobModifyIndex, err := j.srv.raftApply(
			structs.JobRegisterRequestType,
			structs.JobRegisterRequest{
				Job:            job,
				EnforceIndex:   true,
				JobModifyIndex: job.ModifyIndex,
				PolicyOverride: args.PolicyOverride,
				WriteRequest:   args.WriteRequest,
			},
		)
		if err != nil {
			j.logger.Error("job register for scale failed", "error", err)
			return err
		}
		reply.JobModifyIndex = jobModifyIndex

		// Create an eval for non-dispatch jobs
		if !(job.IsPeriodic() || job.IsParameterized()) {
			eval := &structs.Evaluation{
				ID:             uuid.Generate(),
				Namespace:      namespace,
				Priority:       job.Priority, // Safe as nil check performed above.
				Type:           job.Type,
				TriggeredBy:    structs.EvalTriggerScaling,
				JobID:          args.JobID,
				JobModifyIndex: reply.JobModifyIndex,
				Status:         structs.EvalStatusPending,
				CreateTime:     now,
				ModifyTime:     now,
			}

			_, evalIndex, err := j.srv.raftApply(
				structs.EvalUpdateRequestType,
				&structs.EvalUpdateRequest{
					Evals:        []*structs.Evaluation{eval},
					WriteRequest: structs.WriteRequest{Region: args.Region},
				},
			)
			if err != nil {
				j.logger.Error("eval create failed", "error", err, "method", "scale")
				return err
			}

			reply.EvalID = eval.ID
			reply.EvalCreateIndex = evalIndex
			event.ScalingEvent.EvalID = &reply.EvalID
		}
	} else {
		reply.JobModifyIndex = job.ModifyIndex
	}

	_, eventIndex, err := j.srv.raftApply(structs.ScalingEventRegisterRequestType, event)
	if err != nil {
		j.logger.Error("scaling event create failed", "error", err)
		return err
	}

	reply.Index = eventIndex

	j.srv.setQueryMeta(&reply.QueryMeta)

	return nil
}

func (j *Job) GetJobSubmission(args *structs.JobSubmissionRequest, reply *structs.JobSubmissionResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.GetJobSubmission", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job_submission", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "get_job_submission"}, time.Now())

	// Check for read-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for the submission
			out, err := state.JobSubmission(ws, args.RequestNamespace(), args.JobID, args.Version)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Submission = out
			if out != nil {
				// associate with the index of the job this submission originates from
				reply.Index = out.JobModifyIndex
			} else {
				// if there is no submission context, associate with no index
				reply.Index = 0
			}

			// Set the query response
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return j.srv.blockingRPC(&opts)
}

// GetJob is used to request information about a specific job
func (j *Job) GetJob(args *structs.JobSpecificRequest,
	reply *structs.SingleJobResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.GetJob", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "get_job"}, time.Now())

	// Check for read-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for the job
			out, err := state.JobByID(ws, args.RequestNamespace(), args.JobID)
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
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.GetJobVersions", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "get_job_versions"}, time.Now())

	// Check for read-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for the job
			out, err := state.JobVersionsByID(ws, args.RequestNamespace(), args.JobID)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Versions = out
			if len(out) != 0 {

				var compareVersionNumber uint64
				var compareVersion *structs.Job
				var compareSpecificVersion bool

				if args.Diffs {
					if args.DiffTagName != "" {
						compareSpecificVersion = true
						compareVersion, err = state.JobVersionByTagName(ws, args.RequestNamespace(), args.JobID, args.DiffTagName)
						if err != nil {
							return fmt.Errorf("error looking up job version by tag: %v", err)
						}
						if compareVersion == nil {
							return fmt.Errorf("tag %q not found", args.DiffTagName)
						}
						compareVersionNumber = compareVersion.Version
					} else if args.DiffVersion != nil {
						compareSpecificVersion = true
						compareVersionNumber = *args.DiffVersion
					}
				}

				// Note: a previous assumption here was that the 0th job was the latest, and that we don't modify "old" versions.
				// Adding version tags breaks this assumption (you can tag an old version, which should unblock /versions queries) so we now look for the highest ModifyIndex.
				var maxModifyIndex uint64
				for _, job := range out {
					if job.ModifyIndex > maxModifyIndex {
						maxModifyIndex = job.ModifyIndex
					}
					if compareSpecificVersion && job.Version == compareVersionNumber {
						compareVersion = job
					}
				}
				reply.Index = maxModifyIndex

				if compareSpecificVersion && compareVersion == nil {
					if args.DiffTagName != "" {
						return fmt.Errorf("tag %q not found", args.DiffTagName)
					}
					return fmt.Errorf("version %d not found", *args.DiffVersion)
				}

				// Compute the diffs

				if args.Diffs {
					for i := 0; i < len(out); i++ {
						var old, new *structs.Job
						new = out[i]

						if compareSpecificVersion {
							old = compareVersion
						} else {
							if i == len(out)-1 {
								// Skip the last version if not comparing to a specific version
								break
							}
							old = out[i+1]
						}

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

// allowedNSes returns a set (as map of ns->true) of the namespaces a token has access to.
// Returns `nil` set if the token has access to all namespaces
// and ErrPermissionDenied if the token has no capabilities on any namespace.
func allowedNSes(aclObj *acl.ACL, state *state.StateStore, allow func(ns string) bool) (map[string]bool, error) {
	if aclObj.IsManagement() {
		return nil, nil
	}

	// namespaces
	nses, err := state.NamespaceNames()
	if err != nil {
		return nil, err
	}

	r := make(map[string]bool, len(nses))

	for _, ns := range nses {
		if allow(ns) {
			r[ns] = true
		}
	}

	if len(r) == 0 {
		return nil, structs.ErrPermissionDenied
	}

	return r, nil
}

// registrationsAreAllowed checks that the scheduler is not in
// RejectJobRegistration mode for load-shedding.
func registrationsAreAllowed(aclObj *acl.ACL, state *state.StateStore) (bool, error) {
	_, cfg, err := state.SchedulerConfig()
	if err != nil {
		return false, err
	}
	if cfg != nil && !cfg.RejectJobRegistration {
		return true, nil
	}
	if aclObj.IsManagement() {
		return true, nil
	}
	return false, nil
}

// List is used to list the jobs registered in the system
func (j *Job) List(args *structs.JobListRequest, reply *structs.JobListResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.List", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "list"}, time.Now())

	namespace := args.RequestNamespace()

	// Check for list-job permissions
	aclObj, err := j.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityListJobs) {
		return structs.ErrPermissionDenied
	}
	allow := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityListJobs)

	sort := state.QueryOptionSort(args.QueryOptions)

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture all the jobs
			var err error
			var iter memdb.ResultIterator

			// Get the namespaces the user is allowed to access.
			allowableNamespaces, err := allowedNSes(aclObj, state, allow)
			if err == structs.ErrPermissionDenied {
				// return empty jobs if token isn't authorized for any
				// namespace, matching other endpoints
				reply.Jobs = make([]*structs.JobListStub, 0)
			} else if err != nil {
				return err
			} else {
				if prefix := args.QueryOptions.Prefix; prefix != "" {
					iter, err = state.JobsByIDPrefix(ws, namespace, prefix, sort)
				} else if namespace != structs.AllNamespacesSentinel {
					iter, err = state.JobsByNamespace(ws, namespace, sort)
				} else {
					iter, err = state.Jobs(ws, sort)
				}
				if err != nil {
					return err
				}

				tokenizer := paginator.NewStructsTokenizer(
					iter,
					paginator.StructsTokenizerOptions{
						WithNamespace: true,
						WithID:        true,
					},
				)
				filters := []paginator.Filter{
					paginator.NamespaceFilter{
						AllowableNamespaces: allowableNamespaces,
					},
				}

				var jobs []*structs.JobListStub
				paginator, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
					func(raw interface{}) error {
						job := raw.(*structs.Job)
						summary, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
						if err != nil || summary == nil {
							return fmt.Errorf("unable to look up summary for job: %v", job.ID)
						}
						jobs = append(jobs, job.Stub(summary, args.Fields))
						return nil
					})
				if err != nil {
					return structs.NewErrRPCCodedf(
						http.StatusBadRequest, "failed to create result paginator: %v", err)
				}

				nextToken, err := paginator.Page()
				if err != nil {
					return structs.NewErrRPCCodedf(
						http.StatusBadRequest, "failed to read result page: %v", err)
				}

				reply.QueryMeta.NextToken = nextToken
				reply.Jobs = jobs
			}

			// Use the last index that affected the jobs table or summary
			jindex, err := state.Index("jobs")
			if err != nil {
				return err
			}
			sindex, err := state.Index("job_summary")
			if err != nil {
				return err
			}
			reply.Index = max(jindex, sindex)

			// Set the query response
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return j.srv.blockingRPC(&opts)
}

// Allocations is used to list the allocations for a job
func (j *Job) Allocations(args *structs.JobSpecificRequest,
	reply *structs.JobAllocationsResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Allocations", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "allocations"}, time.Now())

	// Check for read-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Ensure JobID is set otherwise everything works and never returns
	// allocations which can hide bugs in request code.
	if args.JobID == "" {
		return fmt.Errorf("missing job ID")
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture the allocations
			allocs, err := state.AllocsByJob(ws, args.RequestNamespace(), args.JobID, args.All)
			if err != nil {
				return err
			}

			// Convert to stubs
			if len(allocs) > 0 {
				reply.Allocations = make([]*structs.AllocListStub, 0, len(allocs))
				for _, alloc := range allocs {
					reply.Allocations = append(reply.Allocations, alloc.Stub(nil))
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
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Evaluations", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "evaluations"}, time.Now())

	// Check for read-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture the evals
			var err error
			reply.Evaluations, err = state.EvalsByJob(ws, args.RequestNamespace(), args.JobID)
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
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Deployments", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "deployments"}, time.Now())

	// Check for read-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture the deployments
			deploys, err := state.DeploymentsByJobID(ws, args.RequestNamespace(), args.JobID, args.All)
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
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.LatestDeployment", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "latest_deployment"}, time.Now())

	// Check for read-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture the deployments
			deploys, err := state.DeploymentsByJobID(ws, args.RequestNamespace(), args.JobID, args.All)
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

// GetActions is used to iterate through a job's taskgroups' tasks and
// aggregate their actions, flattened.
func (j *Job) GetActions(args *structs.JobActionListRequest, reply *structs.JobActionListResponse) error {
	// authenticate, measure, and forward
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward(structs.JobGetActionsRPCMethod, args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "get_actions"}, time.Now())

	// Check for read-job permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Validate the arguments
	if args.JobID == "" {
		return fmt.Errorf("JobID required for actions")
	}

	// Grab the job
	job, err := j.srv.fsm.State().JobByID(nil, args.RequestNamespace(), args.JobID)
	if err != nil {
		return err
	}
	if job == nil {
		return structs.NewErrUnknownJob(args.JobID)
	}

	// Get its task groups' tasks' actions
	jobActions := make([]*structs.JobAction, 0)
	for _, tg := range job.TaskGroups {
		for _, task := range tg.Tasks {
			for _, action := range task.Actions {
				jobAction := &structs.JobAction{
					Action:        *action,
					TaskName:      task.Name,
					TaskGroupName: tg.Name,
				}
				jobActions = append(jobActions, jobAction)
			}
		}
	}

	reply.Actions = jobActions
	reply.Index = job.ModifyIndex

	j.srv.setQueryMeta(&reply.QueryMeta)

	return nil
}

// Plan is used to cause a dry-run evaluation of the Job and return the results
// with a potential diff containing annotations.
func (j *Job) Plan(args *structs.JobPlanRequest, reply *structs.JobPlanResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Plan", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "plan"}, time.Now())

	// Validate the arguments
	if args.Job == nil {
		return fmt.Errorf("Job required for plan")
	}

	// Run admission controllers
	job, warnings, err := j.admissionControllers(args.Job)
	if err != nil {
		return err
	}
	args.Job = job

	// Set the warning message
	reply.Warnings = helper.MergeMultierrorWarnings(warnings...)

	// Check job submission permissions, which we assume is the same for plan
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else {
		if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
			return structs.ErrPermissionDenied
		}
		// Check if override is set and we do not have permissions
		if args.PolicyOverride {
			if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySentinelOverride) {
				return structs.ErrPermissionDenied
			}
		}
	}

	// Acquire a snapshot of the state
	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	// Enforce Sentinel policies
	nomadACLToken, err := snap.ACLTokenBySecretID(nil, args.AuthToken)
	if err != nil && !strings.Contains(err.Error(), "missing secret id") {
		return err
	}
	ns, err := snap.NamespaceByName(nil, args.RequestNamespace())
	if err != nil {
		return err
	}

	// Get the original job
	ws := memdb.NewWatchSet()
	existingJob, err := snap.JobByID(ws, args.RequestNamespace(), args.Job.ID)
	if err != nil {
		return err
	}

	policyWarnings, err := j.enforceSubmitJob(args.PolicyOverride, args.Job, existingJob, nomadACLToken, ns)
	if err != nil {
		return err
	}
	if policyWarnings != nil {
		warnings = append(warnings, policyWarnings)
		reply.Warnings = helper.MergeMultierrorWarnings(warnings...)
	}

	// Interpolate the job for this region
	err = j.interpolateMultiregionFields(args)
	if err != nil {
		return err
	}

	// Ensure that all scaling policies have an appropriate ID
	if err := propagateScalingPolicyIDs(existingJob, args.Job); err != nil {
		return err
	}

	var index uint64
	var updatedIndex uint64

	if existingJob != nil {
		index = existingJob.JobModifyIndex

		// We want to reuse deployments where possible, so only insert the job if
		// it has changed or the job didn't exist
		if existingJob.SpecChanged(args.Job) {
			// Insert the updated Job into the snapshot
			updatedIndex = existingJob.JobModifyIndex + 1
			if err := snap.UpsertJob(structs.IgnoreUnknownTypeFlag, updatedIndex, nil, args.Job); err != nil {
				return err
			}
		}
	} else if existingJob == nil {
		// Insert the updated Job into the snapshot
		err := snap.UpsertJob(structs.IgnoreUnknownTypeFlag, 100, nil, args.Job)
		if err != nil {
			return err
		}
	}

	// Create an eval and mark it as requiring annotations and insert that as well
	now := time.Now().UnixNano()
	eval := &structs.Evaluation{
		ID:             uuid.Generate(),
		Namespace:      args.RequestNamespace(),
		Priority:       args.Job.Priority,
		Type:           args.Job.Type,
		TriggeredBy:    structs.EvalTriggerJobRegister,
		JobID:          args.Job.ID,
		JobModifyIndex: updatedIndex,
		Status:         structs.EvalStatusPending,
		AnnotatePlan:   true,
		// Timestamps are added for consistency but this eval is never persisted
		CreateTime: now,
		ModifyTime: now,
	}

	// Ignore eval event creation during snapshot eval creation
	snap.UpsertEvals(structs.IgnoreUnknownTypeFlag, 100, []*structs.Evaluation{eval})

	// Create an in-memory Planner that returns no errors and stores the
	// submitted plan and created evals.
	planner := &scheduler.Harness{
		State: &snap.StateStore,
	}

	// Create the scheduler and run it
	sched, err := scheduler.NewScheduler(eval.Type, j.logger, j.srv.workersEventCh, snap, planner)
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
		jobDiff, err := existingJob.Diff(args.Job, true)
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
		reply.NextPeriodicLaunch, err = args.Job.Periodic.Next(time.Now().In(args.Job.Periodic.GetLocation()))
		if err != nil {
			return fmt.Errorf("Failed to parse cron expression: %v", err)
		}
	}

	reply.FailedTGAllocs = updatedEval.FailedTGAllocs
	reply.JobModifyIndex = index
	reply.Annotations = annotations
	reply.CreatedEvals = planner.CreateEvals
	reply.Index = index
	return nil
}

// validateJobUpdate ensures updates to a job are valid.
func validateJobUpdate(old, new *structs.Job) error {
	// Validate Dispatch not set on new Jobs
	if old == nil {
		if new.Dispatched {
			return fmt.Errorf("job can't be submitted with 'Dispatched' set")
		}
		return nil
	}

	// Type transitions are disallowed
	if old.Type != new.Type {
		return fmt.Errorf("cannot update job from type %q to %q", old.Type, new.Type)
	}

	// Transitioning to/from periodic is disallowed
	if old.IsPeriodic() && !new.IsPeriodic() {
		return fmt.Errorf("cannot update periodic job to being non-periodic")
	}
	if new.IsPeriodic() && !old.IsPeriodic() {
		return fmt.Errorf("cannot update non-periodic job to being periodic")
	}

	// Transitioning to/from parameterized is disallowed
	if old.IsParameterized() && !new.IsParameterized() {
		return fmt.Errorf("cannot update parameterized job to being non-parameterized")
	}
	if new.IsParameterized() && !old.IsParameterized() {
		return fmt.Errorf("cannot update non-parameterized job to being parameterized")
	}

	if old.Dispatched != new.Dispatched {
		return fmt.Errorf("field 'Dispatched' is read-only")
	}

	return nil
}

// Dispatch a parameterized job.
func (j *Job) Dispatch(args *structs.JobDispatchRequest, reply *structs.JobDispatchResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.Dispatch", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "dispatch"}, time.Now())

	// Check for submit-job permissions
	aclObj, err := j.srv.ResolveACL(args)
	if err != nil {
		return err
	} else if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityDispatchJob) {
		return structs.ErrPermissionDenied
	}

	if ok, err := registrationsAreAllowed(aclObj, j.srv.State()); !ok || err != nil {
		j.logger.Warn("job dispatch is currently disabled for non-management ACL")
		return structs.ErrJobRegistrationDisabled
	}

	// Lookup the parameterized job
	if args.JobID == "" {
		return fmt.Errorf("missing parameterized job ID")
	}

	snap, err := j.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	ws := memdb.NewWatchSet()
	parameterizedJob, err := snap.JobByID(ws, args.RequestNamespace(), args.JobID)
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

	// Avoid creating new dispatched jobs for retry requests, by using the idempotency token
	if args.IdempotencyToken != "" {
		// Fetch all jobs that match the parameterized job ID prefix
		iter, err := snap.JobsByIDPrefix(ws, parameterizedJob.Namespace, parameterizedJob.ID, state.SortDefault)
		if err != nil {
			const errMsg = "failed to retrieve jobs for idempotency check"
			j.logger.Error(errMsg, "error", err)
			return fmt.Errorf(errMsg)
		}

		// Iterate
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}

			// Ensure the parent ID is an exact match
			existingJob := raw.(*structs.Job)
			if existingJob.ParentID != parameterizedJob.ID {
				continue
			}

			// Idempotency tokens match
			if existingJob.DispatchIdempotencyToken == args.IdempotencyToken {
				// The existing job has not yet been garbage collected.
				// Registering a new job would violate the idempotency token.
				// Return the existing job.
				reply.JobCreateIndex = existingJob.CreateIndex
				reply.DispatchedJobID = existingJob.ID
				reply.Index = existingJob.ModifyIndex

				return nil
			}
		}
	}

	// Derive the child job and commit it via Raft - with initial status
	dispatchJob := parameterizedJob.Copy()
	dispatchJob.ID = structs.DispatchedID(parameterizedJob.ID, args.IdPrefixTemplate, time.Now())
	dispatchJob.ParentID = parameterizedJob.ID
	dispatchJob.Name = dispatchJob.ID
	dispatchJob.SetSubmitTime()
	dispatchJob.Dispatched = true
	dispatchJob.Status = ""
	dispatchJob.StatusDescription = ""
	dispatchJob.DispatchIdempotencyToken = args.IdempotencyToken

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
		j.logger.Error("dispatched job register failed", "error")
		return err
	}

	reply.JobCreateIndex = jobCreateIndex
	reply.DispatchedJobID = dispatchJob.ID
	reply.Index = jobCreateIndex

	// If the job is periodic, we don't create an eval.
	if !dispatchJob.IsPeriodic() {
		// Create a new evaluation
		now := time.Now().UnixNano()
		eval := &structs.Evaluation{
			ID:             uuid.Generate(),
			Namespace:      args.RequestNamespace(),
			Priority:       dispatchJob.Priority,
			Type:           dispatchJob.Type,
			TriggeredBy:    structs.EvalTriggerJobRegister,
			JobID:          dispatchJob.ID,
			JobModifyIndex: jobCreateIndex,
			Status:         structs.EvalStatusPending,
			CreateTime:     now,
			ModifyTime:     now,
		}
		update := &structs.EvalUpdateRequest{
			Evals:        []*structs.Evaluation{eval},
			WriteRequest: structs.WriteRequest{Region: args.Region},
		}

		// Commit this evaluation via Raft
		_, evalIndex, err := j.srv.raftApply(structs.EvalUpdateRequestType, update)
		if err != nil {
			j.logger.Error("eval create failed", "error", err, "method", "dispatch")
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
	for k := range req.Meta {
		if _, ok := keys[k]; ok {
			return fmt.Errorf("Duplicate key %q in passed metadata", k)
		}
		keys[k] = struct{}{}
	}

	required := set.From(job.ParameterizedJob.MetaRequired)
	optional := set.From(job.ParameterizedJob.MetaOptional)

	// Check the metadata key constraints are met
	unpermitted := make(map[string]struct{})
	for k := range req.Meta {
		req := required.Contains(k)
		opt := optional.Contains(k)
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

// ScaleStatus retrieves the scaling status for a job
func (j *Job) ScaleStatus(args *structs.JobScaleStatusRequest,
	reply *structs.JobScaleStatusResponse) error {

	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.ScaleStatus", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "scale_status"}, time.Now())

	// Check for autoscaler permissions
	if aclObj, err := j.srv.ResolveACL(args); err != nil {
		return err
	} else {
		hasReadJob := aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob)
		hasReadJobScaling := aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJobScaling)
		if !(hasReadJob || hasReadJobScaling) {
			return structs.ErrPermissionDenied
		}
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			// We need the job and the job summary
			job, err := state.JobByID(ws, args.RequestNamespace(), args.JobID)
			if err != nil {
				return err
			}
			if job == nil {
				// HTTPServer.jobScaleStatus() will 404 if this is nil
				reply.JobScaleStatus = nil

				// reply with latest index, since if the job does get created,
				// it must necessarily be later than current latest.
				reply.Index, err = state.LatestIndex()
				return err
			}

			events, eventsIndex, err := state.ScalingEventsByJob(ws, args.RequestNamespace(), args.JobID)
			if err != nil {
				return err
			}
			if events == nil {
				events = make(map[string][]*structs.ScalingEvent)
			}

			var allocs []*structs.Allocation
			var allocsIndex uint64
			allocs, err = state.AllocsByJob(ws, job.Namespace, job.ID, false)
			if err != nil {
				return err
			}

			// Setup the output
			reply.JobScaleStatus = &structs.JobScaleStatus{
				JobID:          job.ID,
				Namespace:      job.Namespace,
				JobCreateIndex: job.CreateIndex,
				JobModifyIndex: job.ModifyIndex,
				JobStopped:     job.Stop,
				TaskGroups:     make(map[string]*structs.TaskGroupScaleStatus),
			}

			for _, tg := range job.TaskGroups {
				tgScale := &structs.TaskGroupScaleStatus{
					Desired: tg.Count,
				}
				tgScale.Events = events[tg.Name]
				reply.JobScaleStatus.TaskGroups[tg.Name] = tgScale
			}

			for _, alloc := range allocs {
				// TODO: ignore canaries until we figure out what we should do with canaries
				if alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Canary {
					continue
				}
				if alloc.TerminalStatus() {
					continue
				}
				tgScale, ok := reply.JobScaleStatus.TaskGroups[alloc.TaskGroup]
				if !ok || tgScale == nil {
					continue
				}
				tgScale.Placed++
				if alloc.ClientStatus == structs.AllocClientStatusRunning {
					tgScale.Running++
				}
				if alloc.DeploymentStatus != nil && alloc.DeploymentStatus.HasHealth() {
					if alloc.DeploymentStatus.IsHealthy() {
						tgScale.Healthy++
					} else if alloc.DeploymentStatus.IsUnhealthy() {
						tgScale.Unhealthy++
					}
				}
				if alloc.ModifyIndex > allocsIndex {
					allocsIndex = alloc.ModifyIndex
				}
			}

			maxIndex := job.ModifyIndex
			if eventsIndex > maxIndex {
				maxIndex = eventsIndex
			}
			if allocsIndex > maxIndex {
				maxIndex = allocsIndex
			}
			reply.Index = maxIndex

			// Set the query response
			j.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return j.srv.blockingRPC(&opts)
}

// GetServiceRegistrations returns a list of service registrations which belong
// to the passed job ID.
func (j *Job) GetServiceRegistrations(
	args *structs.JobServiceRegistrationsRequest,
	reply *structs.JobServiceRegistrationsResponse) error {

	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward(structs.JobServiceRegistrationsRPCMethod, args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "get_service_registrations"}, time.Now())

	// Check for read-job namespace capability.
	aclObj, err := j.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Set up the blocking query.
	return j.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			job, err := stateStore.JobByID(ws, args.RequestNamespace(), args.JobID)
			if err != nil {
				return err
			}

			// Guard against the job not-existing. Do not create an empty list
			// to allow the API to determine whether the job was found or not.
			if job == nil {
				return nil
			}

			// Perform the state query to get an iterator.
			iter, err := stateStore.GetServiceRegistrationsByJobID(ws, args.RequestNamespace(), args.JobID)
			if err != nil {
				return err
			}

			// Set up our output after we have checked the error.
			services := make([]*structs.ServiceRegistration, 0)

			// Iterate the iterator, appending all service registrations
			// returned to the reply.
			for raw := iter.Next(); raw != nil; raw = iter.Next() {
				services = append(services, raw.(*structs.ServiceRegistration))
			}
			reply.Services = services

			// Use the index table to populate the query meta as we have no way
			// of tracking the max index on deletes.
			return j.srv.setReplyQueryMeta(stateStore, state.TableServiceRegistrations, &reply.QueryMeta)
		},
	})
}

func (j *Job) TagVersion(args *structs.JobApplyTagRequest, reply *structs.JobTagResponse) error {
	authErr := j.srv.Authenticate(j.ctx, args)
	if done, err := j.srv.forward("Job.TagVersion", args, args, reply); done {
		return err
	}
	j.srv.MeasureRPCRate("job", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "job", "tag_version"}, time.Now())

	aclObj, err := j.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	if args.Tag != nil {
		args.Tag.TaggedTime = time.Now().UnixNano()
	}

	_, index, err := j.srv.raftApply(structs.JobVersionTagRequestType, args)
	if err != nil {
		j.logger.Error("tagging version failed", "error", err)
		return err
	}

	reply.Index = index
	if args.Tag != nil {
		reply.Name = args.Tag.Name
		reply.Description = args.Tag.Description
		reply.TaggedTime = args.Tag.TaggedTime
	}

	return nil
}
