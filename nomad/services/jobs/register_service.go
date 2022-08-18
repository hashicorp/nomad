package jobs

import (
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/services"
	"github.com/hashicorp/nomad/nomad/structs"
	"time"
)

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

func (svc *RegisterService) Init() {
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

func (svc *RegisterService) Register(req *structs.JobRegisterRequest, reply *structs.JobRegisterResponse) error {
	defer svc.Options[Register].EmitMetrics()

	// Validate the arguments
	if req.Job == nil {
		return fmt.Errorf("missing job for registration")
	}

	// defensive check; http layer and RPC requester should ensure namespaces are set consistently
	if req.RequestNamespace() != req.Job.Namespace {
		return fmt.Errorf("mismatched request namespace in request: %q, %q", req.RequestNamespace(), req.Job.Namespace)
	}

	// Run admission controllers
	job, warnings, err := j.admissionControllers(req.Job)
	if err != nil {
		return err
	}

	req.Job = job

	// Attach the Nomad token's accessor ID so that deploymentwatcher
	// can reference the token later
	nomadACLToken, err := j.srv.ResolveSecretToken(req.AuthToken)
	if err != nil {
		return err
	}
	if nomadACLToken != nil {
		req.Job.NomadTokenID = nomadACLToken.AccessorID
	}

	// Set the warning message
	reply.Warnings = structs.MergeMultierrorWarnings(warnings...)

	// Check job submission permissions
	aclObj, err := j.srv.ResolveToken(req.AuthToken)
	if err != nil {
		return err
	} else if aclObj != nil {
		if !aclObj.AllowNsOp(req.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
			return structs.ErrPermissionDenied
		}

		// Validate Volume Permissions
		for _, tg := range req.Job.TaskGroups {
			for _, vol := range tg.Volumes {
				switch vol.Type {
				case structs.VolumeTypeCSI:
					if !allowCSIMount(aclObj, req.RequestNamespace()) {
						return structs.ErrPermissionDenied
					}
				case structs.VolumeTypeHost:
					// If a volume is readonly, then we allow access if the user has ReadOnly
					// or ReadWrite access to the volume. Otherwise we only allow access if
					// they have ReadWrite access.
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
					if !aclObj.AllowNsOp(req.RequestNamespace(), acl.NamespaceCapabilityCSIRegisterPlugin) {
						return structs.ErrPermissionDenied
					}
				}
			}
		}

		// Check if override is set and we do not have permissions
		if req.PolicyOverride {
			if !aclObj.AllowNsOp(req.RequestNamespace(), acl.NamespaceCapabilitySentinelOverride) {
				j.logger.Warn("policy override attempted without permissions for job", "job", req.Job.ID)
				return structs.ErrPermissionDenied
			}
			j.logger.Warn("policy override set for job", "job", req.Job.ID)
		}
	}

	if ok, err := registrationsAreAllowed(aclObj, j.srv.State()); !ok || err != nil {
		j.logger.Warn("job registration is currently disabled for non-management ACL")
		return structs.ErrJobRegistrationDisabled
	}

	// Lookup the job
	snap, err := j.srv.State().Snapshot()
	if err != nil {
		return err
	}
	ws := memdb.NewWatchSet()
	existingJob, err := snap.JobByID(ws, req.RequestNamespace(), req.Job.ID)
	if err != nil {
		return err
	}

	// If EnforceIndex set, check it before trying to apply
	if req.EnforceIndex {
		jmi := req.JobModifyIndex
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
	if err := validateJobUpdate(existingJob, req.Job); err != nil {
		return err
	}

	// Ensure that all scaling policies have an appropriate ID
	if err := propagateScalingPolicyIDs(existingJob, req.Job); err != nil {
		return err
	}

	// helper function that checks if the Consul token supplied with the job has
	// sufficient ACL permissions for:
	//   - registering services into namespace of each group
	//   - reading kv store of each group
	//   - establishing consul connect services
	checkConsulToken := func(usages map[string]*structs.ConsulUsage) error {
		if j.srv.config.ConsulConfig.AllowsUnauthenticated() {
			// if consul.allow_unauthenticated is enabled (which is the default)
			// just let the job through without checking anything
			return nil
		}

		ctx := context.Background()
		for namespace, usage := range usages {
			if err := j.srv.consulACLs.CheckPermissions(ctx, namespace, usage, req.Job.ConsulToken); err != nil {
				return fmt.Errorf("job-submitter consul token denied: %w", err)
			}
		}

		return nil
	}

	// Enforce the job-submitter has a Consul token with necessary ACL permissions.
	if err := checkConsulToken(req.Job.ConsulUsages()); err != nil {
		return err
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

	for ns, entries := range req.Job.ConfigEntries() {
		for service, entry := range entries.Ingress {
			if errCE := j.srv.consulConfigEntries.SetIngressCE(ctx, ns, service, entry); errCE != nil {
				return errCE
			}
		}
		for service, entry := range entries.Terminating {
			if errCE := j.srv.consulConfigEntries.SetTerminatingCE(ctx, ns, service, entry); errCE != nil {
				return errCE
			}
		}
	}

	// Enforce Sentinel policies. Pass a copy of the job to prevent
	// sentinel from altering it.
	policyWarnings, err := j.enforceSubmitJob(req.PolicyOverride, req.Job.Copy())
	if err != nil {
		return err
	}
	if policyWarnings != nil {
		warnings = append(warnings, policyWarnings)
		reply.Warnings = structs.MergeMultierrorWarnings(warnings...)
	}

	// Clear the Vault token
	req.Job.VaultToken = ""

	// Clear the Consul token
	req.Job.ConsulToken = ""

	// Preserve the existing task group counts, if so requested
	if existingJob != nil && req.PreserveCounts {
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

	// Submit a multiregion job to other regions (enterprise only).
	// The job will have its region interpolated.
	var newVersion uint64
	if existingJob != nil {
		newVersion = existingJob.Version + 1
	}
	isRunner, err := j.multiregionRegister(req, reply, newVersion)
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
		reply.EvalID = eval.ID
	}

	// Check if the job has changed at all
	if existingJob == nil || existingJob.SpecChanged(req.Job) {

		// COMPAT(1.1.0): Remove the ServerMeetMinimumVersion check to always set req.Eval
		// 0.12.1 introduced atomic eval job registration
		if eval != nil && ServersMeetMinimumVersion(j.srv.Members(), minJobRegisterAtomicEvalVersion, false) {
			req.Eval = eval
			submittedEval = true
		}

		// Commit this update via Raft
		fsmErr, index, err := j.srv.raftApply(structs.JobRegisterRequestType, req)
		if err, ok := fsmErr.(error); ok && err != nil {
			j.logger.Error("registering job failed", "error", err, "fsm", true)
			return err
		}
		if err != nil {
			j.logger.Error("registering job failed", "error", err, "raft", true)
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
	req.Job.JobModifyIndex = reply.JobModifyIndex

	if eval == nil {
		// For dispatch jobs we return early, so we need to drop regions
		// here rather than after eval for deployments is kicked off
		err = j.multiregionDrop(req, reply)
		if err != nil {
			return err
		}
		return nil
	}

	if eval != nil && !submittedEval {
		eval.JobModifyIndex = reply.JobModifyIndex
		update := &structs.EvalUpdateRequest{
			Evals:        []*structs.Evaluation{eval},
			WriteRequest: structs.WriteRequest{Region: req.Region},
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
		err = j.multiregionStart(req, reply)
		if err != nil {
			return err
		}
		// We drop any unwanted regions only once we know all jobs have
		// been registered and we've kicked off the deployment. This keeps
		// dropping regions close in semantics to dropping task groups in
		// single-region deployments
		err = j.multiregionDrop(req, reply)
		if err != nil {
			return err
		}
	}

	return nil
}
