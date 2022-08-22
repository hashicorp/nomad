package jobs

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/services"
	"github.com/hashicorp/nomad/nomad/structs"
	"time"
)

// TODO: Find how to register with msgpack and then other solutions.
const (
	JobPrefix = "Job."
	Register  = "Register"
)

type RegisterService struct {
	services.ServiceBase
	srv    *nomad.Server
	logger hclog.Logger

	// builtin admission controllers
	mutators   []nomad.JobMutator
	validators []nomad.JobValidator
}

// TODO: Find a way to replace this with Factory method and/or DI
func (svc *RegisterService) Init() {
	svc.mutators = []nomad.JobMutator{
		nomad.JobCanonicalizer{},
		nomad.JobConnectHookController{},
		nomad.JobCheckHookController{},
		nomad.JobImpliedConstraintsMutator{},
	}

	svc.validators = []nomad.JobValidator{
		nomad.JobConnectHookController{},
		nomad.JobCheckHookController{},
		nomad.JobVaultHookValidator{SRV: svc.srv},
		nomad.JobNamespaceConstraintCheckHookValidator{SRV: svc.srv},
		nomad.JobConfigValidator{},
		&nomad.MemoryOversubscriptionValidator{SRV: svc.srv},
	}

	svc.Options = map[string]*services.Options{
		Register: {
			MetricKeys:           []string{"nomad", "job", "register"},
			RequiredCapabilities: []string{acl.NamespaceCapabilitySubmitJob},
		},
	}
}

func (svc *RegisterService) Copy() *RegisterService {
	c := new(RegisterService)
	*c = *svc
	return c
}

func (svc *RegisterService) Register(req *structs.JobRegisterRequest, resp *structs.JobRegisterResponse) (err error) {
	defer svc.Options[Register].EmitMetrics()

	// TODO: Request Validation belongs at the RPC layer and allows for templated codegen
	err = svc.ValidateRequest(req)
	if err != nil {
		return err
	}

	// Run admission controllers
	job, warnings, err := svc.admissionControllers(req.Job)
	if err != nil {
		return err
	}

	// Set any warnings
	resp.Warnings = structs.MergeMultierrorWarnings(warnings...)

	// Update with mutated job
	req.Job = job

	// Attach the Nomad token's accessor ID so that deploymentwatcher
	// can reference the token later
	err = svc.resolveNomadACLToken(req)
	if err != nil {
		return err
	}

	// Check job submission permissions
	aclProvider, err := svc.srv.ResolveToken(req.AuthToken)
	if err != nil {
		return err
	}

	err = svc.applyACLs(req, aclProvider)
	if err != nil {
		return err
	}

	// Lookup the job
	snap, err := svc.srv.State().Snapshot()
	if err != nil {
		return err
	}

	// TODO: Verify deleting this watchSet is ok
	existingJob, err := snap.JobByID(nil, req.RequestNamespace(), req.Job.ID)
	if err != nil {
		return err
	}

	err = svc.checkJobModifyIndex(req, existingJob)
	if err != nil {
		return err
	}

	// Validate job transitions if its an update
	if err = svc.validateJobUpdate(existingJob, req.Job); err != nil {
		return err
	}

	// Ensure that all scaling policies have an appropriate ID
	if err = svc.propagateScalingPolicyIDs(existingJob, req.Job); err != nil {
		return err
	}

	// Enforce the job-submitter has a Consul token with necessary ACL permissions.
	if err = svc.checkConsulToken(req); err != nil {
		return err
	}

	err = svc.createOrUpdateConsulConfigEntries(req)
	if err != nil {
		return err
	}

	err = svc.enforceSentinelPolicies(req, resp, warnings)
	if err != nil {
		return err
	}

	// Clear the Vault token
	req.Job.VaultToken = ""

	// Clear the Consul token
	req.Job.ConsulToken = ""

	// Preserve the existing task group counts, if so requested
	svc.preserveJobCounts(req, existingJob)

	// Submit a multiregion job to other regions (enterprise only).
	// The job will have its region interpolated.
	// TODO: Pass existingJob to MultiRegionRegister and have it calculate version.
	var newVersion uint64
	if existingJob != nil {
		newVersion = existingJob.Version + 1
	}
	isRunner, err := svc.multiregionRegister(req, resp, newVersion)
	if err != nil {
		return err
	}

	// Create a new evaluation
	now := time.Now().UnixNano()
	submittedEval := false
	var eval *structs.Evaluation

	// Set the submit time
	req.Job.SubmitTime = now

	// If the job is periodic or parameterized, we don't create an eval.
	if !(req.Job.IsPeriodic() || req.Job.IsParameterized()) {

		// Initially set the eval priority to that of the job priority. If the
		// user supplied an eval priority override, we subsequently use this.
		evalPriority := req.Job.Priority
		if req.EvalPriority > 0 {
			evalPriority = req.EvalPriority
		}

		eval = &structs.Evaluation{
			ID:          uuid.Generate(),
			Namespace:   req.RequestNamespace(),
			Priority:    evalPriority,
			Type:        req.Job.Type,
			TriggeredBy: structs.EvalTriggerJobRegister,
			JobID:       req.Job.ID,
			Status:      structs.EvalStatusPending,
			CreateTime:  now,
			ModifyTime:  now,
		}
		resp.EvalID = eval.ID
	}

	// Check if the job has changed at all
	if existingJob == nil || existingJob.SpecChanged(req.Job) {

		// COMPAT(1.1.0): Remove the ServerMeetMinimumVersion check to always set req.Eval
		// 0.12.1 introduced atomic eval job registration
		if eval != nil && nomad.ServersMeetMinimumVersion(svc.srv.Members(), nomad.MinJobRegisterAtomicEvalVersion, false) {
			req.Eval = eval
			submittedEval = true
		}

		// Commit this update via Raft
		var index uint64
		var errType string
		index, errType, err = svc.srv.RaftApply(structs.JobRegisterRequestType, req)
		if err != nil {
			svc.logger.Error("registering job failed", "error", err, errType, true)
			return err
		}

		// Populate the resp with job information
		resp.JobModifyIndex = index
		resp.Index = index

		if submittedEval {
			resp.EvalCreateIndex = index
		}

	} else {
		resp.JobModifyIndex = existingJob.JobModifyIndex
	}

	// used for multiregion start
	req.Job.JobModifyIndex = resp.JobModifyIndex

	if eval == nil {
		// For dispatch jobs we return early, so we need to drop regions
		// here rather than after eval for deployments is kicked off
		err = svc.multiregionDrop(req, resp)
		if err != nil {
			return err
		}
		return nil
	}

	if eval != nil && !submittedEval {
		eval.JobModifyIndex = resp.JobModifyIndex
		update := &structs.EvalUpdateRequest{
			Evals:        []*structs.Evaluation{eval},
			WriteRequest: structs.WriteRequest{Region: req.Region},
		}

		// Commit this evaluation via Raft
		// There is a risk of partial failure where the JobRegister succeeds
		// but that the EvalUpdate does not, before 0.12.1
		_, evalIndex, err := svc.srv.raftApply(structs.EvalUpdateRequestType, update)
		if err != nil {
			svc.logger.Error("eval create failed", "error", err, "method", "register")
			return err
		}

		resp.EvalCreateIndex = evalIndex
		resp.Index = evalIndex
	}

	// Kick off a multiregion deployment (enterprise only).
	if isRunner {
		err = svc.multiregionStart(req, resp)
		if err != nil {
			return err
		}
		// We drop any unwanted regions only once we know all jobs have
		// been registered and we've kicked off the deployment. This keeps
		// dropping regions close in semantics to dropping task groups in
		// single-region deployments
		err = svc.multiregionDrop(req, resp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (svc *RegisterService) ValidateRequest(req *structs.JobRegisterRequest) error {
	// Validate the arguments
	if req.Job == nil {
		return fmt.Errorf("missing job for registration")
	}

	// defensive check; http layer and RPC requester should ensure namespaces are set consistently
	if req.RequestNamespace() != req.Job.Namespace {
		return fmt.Errorf("mismatched request namespace in request: %q, %q", req.RequestNamespace(), req.Job.Namespace)
	}

	return nil
}

func (svc *RegisterService) admissionControllers(job *structs.Job) (out *structs.Job, warnings []error, err error) {
	// Mutators run first before validators, so validators view the final rendered job.
	// So, mutators must handle invalid jobs.
	out, warnings, err = svc.admissionMutators(job)
	if err != nil {
		return nil, nil, err
	}

	validateWarnings, err := svc.admissionValidators(job)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, validateWarnings...)

	return out, warnings, nil
}

// admissionMutator returns an updated job as well as warnings or an error.
func (svc *RegisterService) admissionMutators(job *structs.Job) (_ *structs.Job, warnings []error, err error) {
	var w []error
	for _, mutator := range svc.mutators {
		job, w, err = mutator.Mutate(job)
		svc.logger.Trace("job mutate results", "mutator", mutator.Name(), "warnings", w, "error", err)
		if err != nil {
			return nil, nil, fmt.Errorf("error in job mutator %s: %v", mutator.Name(), err)
		}
		warnings = append(warnings, w...)
	}
	return job, warnings, err
}

// admissionValidators returns a slice of validation warnings and a multierror
// of validation failures.
func (svc *RegisterService) admissionValidators(origJob *structs.Job) ([]error, error) {
	// ensure job is not mutated
	job := origJob.Copy()

	var warnings []error
	var errs error

	for _, validator := range svc.validators {
		w, err := validator.Validate(job)
		svc.logger.Trace("job validate results", "validator", validator.Name(), "warnings", w, "error", err)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		warnings = append(warnings, w...)
	}

	return warnings, errs

}

func (svc *RegisterService) resolveNomadACLToken(req *structs.JobRegisterRequest) error {
	nomadACLToken, err := svc.srv.ResolveSecretToken(req.AuthToken)
	if err != nil {
		return err
	}

	if nomadACLToken != nil {
		req.Job.NomadTokenID = nomadACLToken.AccessorID
	}

	return nil
}

func (svc *RegisterService) applyACLs(req *structs.JobRegisterRequest, aclProvider *acl.ACL) error {
	if aclProvider == nil {
		return nil
	}

	if !aclProvider.AllowNsOp(req.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	// Enforce Volume Permissions
	err := svc.enforceVolumePermissions(aclProvider, req)
	if err != nil {
		return err
	}

	// Check if override is set and we do not have permissions
	err = svc.checkPolicyOverride(aclProvider, req)
	if err != nil {
		return err
	}

	err = svc.registrationsAreAllowed(aclProvider, svc.srv.State())
	if err != nil {
		svc.logger.Warn("job registration is currently disabled for non-management ACL")
		return structs.ErrJobRegistrationDisabled
	}

	return nil
}

func (svc *RegisterService) enforceVolumePermissions(req *structs.JobRegisterRequest, aclProvider *acl.ACL) error {
	for _, tg := range req.Job.TaskGroups {
		for _, vol := range tg.Volumes {
			switch vol.Type {
			case structs.VolumeTypeCSI:
				if !nomad.AllowCSIMount(aclProvider, req.RequestNamespace()) {
					return structs.ErrPermissionDenied
				}
			case structs.VolumeTypeHost:
				// If a volume is readonly, then we allow access if the user has ReadOnly
				// or ReadWrite access to the volume. Otherwise we only allow access if
				// they have ReadWrite access.
				if vol.ReadOnly {
					if !aclProvider.AllowHostVolumeOperation(vol.Source, acl.HostVolumeCapabilityMountReadOnly) &&
						!aclProvider.AllowHostVolumeOperation(vol.Source, acl.HostVolumeCapabilityMountReadWrite) {
						return structs.ErrPermissionDenied
					}
				} else {
					if !aclProvider.AllowHostVolumeOperation(vol.Source, acl.HostVolumeCapabilityMountReadWrite) {
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
					!aclProvider.AllowHostVolumeOperation(vol.Source, acl.HostVolumeCapabilityMountReadWrite) {
					return structs.ErrPermissionDenied
				}
			}

			if t.CSIPluginConfig != nil {
				if !aclProvider.AllowNsOp(req.RequestNamespace(), acl.NamespaceCapabilityCSIRegisterPlugin) {
					return structs.ErrPermissionDenied
				}
			}
		}
	}

	return nil
}

func (svc *RegisterService) checkPolicyOverride(req *structs.JobRegisterRequest, aclProvider *acl.ACL) error {
	if req.PolicyOverride {
		if !aclProvider.AllowNsOp(req.RequestNamespace(), acl.NamespaceCapabilitySentinelOverride) {
			svc.logger.Warn("policy override attempted without permissions for job", "job", req.Job.ID)
			return structs.ErrPermissionDenied
		}
		svc.logger.Warn("policy override set for job", "job", req.Job.ID)
	}

	return nil
}

// TODO: Fixed a bug where getting ScheduleConfig from state would
// TODO: always result in ErrJobRegistrationDeisabled. May break tests.
// registrationsAreAllowed checks that the scheduler is not in
// RejectJobRegistration mode for load-shedding.
func (svc *RegisterService) registrationsAreAllowed(aclProvider *acl.ACL) error {
	_, cfg, err := svc.srv.State().SchedulerConfig()
	if err != nil {
		return err
	}

	if cfg != nil && !cfg.RejectJobRegistration {
		return nil
	}

	if aclProvider != nil && aclProvider.IsManagement() {
		return nil
	}

	svc.logger.Warn("job registration is currently disabled for non-management ACL")
	return structs.ErrJobRegistrationDisabled
}

// checkJobModifyIndex checks if EnforceIndex set and checks it before trying to apply.
func (svc *RegisterService) checkJobModifyIndex(req *structs.JobRegisterRequest, existingJob *structs.Job) error {
	if !req.EnforceIndex {
		return nil
	}

	if existingJob == nil {
		if req.JobModifyIndex != 0 {
			return fmt.Errorf("%s %d: job does not exist", nomad.RegisterEnforceIndexErrPrefix, req.JobModifyIndex)
		}
		return nil
	}

	// Job exists so check index
	if req.JobModifyIndex == 0 {
		return fmt.Errorf("%s 0: job already exists", nomad.RegisterEnforceIndexErrPrefix)
	} else if req.JobModifyIndex != existingJob.JobModifyIndex {
		return fmt.Errorf("%s %d: job exists with conflicting job modify index: %d",
			nomad.RegisterEnforceIndexErrPrefix, req.JobModifyIndex, existingJob.JobModifyIndex)
	}

	return nil
}

// validateJobUpdate ensures updates to a job are valid.
func (svc *RegisterService) validateJobUpdate(old, new *structs.Job) error {
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

// propagateScalingPolicyIDs propagates scaling policy IDs from existing job
// to updated job, or generates random IDs in new job
func (svc *RegisterService) propagateScalingPolicyIDs(old, new *structs.Job) error {
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

// checkConsulToken is a helper function that checks if the Consul token supplied with the job has
// sufficient ACL permissions for:
//   - registering services into namespace of each group
//   - reading kv store of each group
//   - establishing consul connect services
func (svc *RegisterService) checkConsulToken(req *structs.JobRegisterRequest) error {
	if svc.srv.GetConfig().ConsulConfig.AllowsUnauthenticated() {
		// if consul.allow_unauthenticated is enabled (which is the default)
		// just let the job through without checking anything
		return nil
	}

	ctx := context.Background()
	for namespace, usage := range req.Job.ConsulUsages() {
		if err := svc.srv.CheckConsulTokenPermissions(ctx, namespace, req.Job.ConsulToken, usage); err != nil {
			return fmt.Errorf("job-submitter consul token denied: %w", err)
		}
	}

	return nil
}

// createOrUpdateConsulConfigEntries Creates or Update Consul Configuration Entries defined in the job.
// For now Nomad only supports Configuration Entries types
// - "ingress-gateway" for managing Ingress Gateways
// - "terminating-gateway" for managing Terminating Gateways
//
// This is done as a blocking operation that prevents the job from being
// submitted if the configuration entries cannot be set in Consul.
//
// Every job update will re-write the Configuration Entry into Consul.
func (svc *RegisterService) createOrUpdateConsulConfigEntries(req *structs.JobRegisterRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for ns, entries := range req.Job.ConfigEntries() {
		for service, entry := range entries.Ingress {
			if errCE := svc.srv.SetConsulIngressConfigEntry(ctx, ns, service, entry); errCE != nil {
				return errCE
			}
		}
		for service, entry := range entries.Terminating {
			if errCE := svc.srv.SetConsulTerminatingConfigEntry(ctx, ns, service, entry); errCE != nil {
				return errCE
			}
		}
	}

	return nil
}

func (svc *RegisterService) enforceSentinelPolicies(req *structs.JobRegisterRequest, resp *structs.JobRegisterResponse, warnings []error) error {
	// Pass a copy of the job to prevent sentinel from altering it.
	policyWarnings, err := svc.enforceSubmitJob(req.PolicyOverride, req.Job.Copy())
	if err != nil {
		return err
	}

	if policyWarnings != nil {
		warnings = append(warnings, policyWarnings)
		resp.Warnings = structs.MergeMultierrorWarnings(warnings...)
	}

	return nil
}

func (svc *RegisterService) preserveJobCounts(req *structs.JobRegisterRequest, existingJob *structs.Job) {
	if !req.PreserveCounts || existingJob == nil {
		return
	}

	prevCounts := make(map[string]int)
	for _, tg := range existingJob.TaskGroups {
		prevCounts[tg.Name] = tg.Count
	}
	for _, tg := range req.Job.TaskGroups {
		if count, ok := prevCounts[tg.Name]; ok {
			tg.Count = count
		}
	}
}
