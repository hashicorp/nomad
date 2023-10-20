// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/exp/maps"
)

const (
	attrVaultVersion      = `${attr.vault.version}`
	attrConsulVersion     = `${attr.consul.version}`
	attrNomadVersion      = `${attr.nomad.version}`
	attrNomadServiceDisco = `${attr.nomad.service_discovery}`
)

var (
	// vaultConstraint is the implicit constraint added to jobs requesting a
	// Vault token
	vaultConstraint = &structs.Constraint{
		LTarget: attrVaultVersion,
		RTarget: ">= 0.6.1",
		Operand: structs.ConstraintSemver,
	}

	// consulServiceDiscoveryConstraint is the implicit constraint added to
	// task groups which include services utilising the Consul provider. The
	// Consul version is pinned to a minimum of that which introduced the
	// namespace feature.
	consulServiceDiscoveryConstraint = &structs.Constraint{
		LTarget: attrConsulVersion,
		RTarget: ">= 1.7.0",
		Operand: structs.ConstraintSemver,
	}

	// nativeServiceDiscoveryConstraint is the constraint injected into task
	// groups that utilise Nomad's native service discovery feature. This is
	// needed, as operators can disable the client functionality, and therefore
	// we need to ensure task groups are placed where they can run
	// successfully.
	nativeServiceDiscoveryConstraint = &structs.Constraint{
		LTarget: attrNomadServiceDisco,
		RTarget: "true",
		Operand: "=",
	}

	// nativeServiceDiscoveryChecksConstraint is the constraint injected into task
	// groups that utilize Nomad's native service discovery checks feature. This
	// is needed, as operators can have versions of Nomad pre-v1.4 mixed into a
	// cluster with v1.4 servers, causing jobs to be placed on incompatible
	// clients.
	nativeServiceDiscoveryChecksConstraint = &structs.Constraint{
		LTarget: attrNomadVersion,
		RTarget: ">= 1.4.0",
		Operand: structs.ConstraintSemver,
	}

	// numaVersionConstraint is the constraint injected into task groups that
	// utilize Nomad's NUMA aware scheduling which requires Nomad 1.7 or later.
	numaVersionConstraint = &structs.Constraint{
		LTarget: "${attr.nomad.version}",
		RTarget: ">= 1.6.3-dev",
		Operand: structs.ConstraintSemver,
	}

	// numaKernelConstraint is the constraint injected into task groups that utilize
	// Nomad's NUMA aware scheduling which requires running on Linux.
	numaKernelConstraint = &structs.Constraint{
		LTarget: "${attr.kernel.name}",
		RTarget: "linux",
		Operand: "=",
	}
)

type admissionController interface {
	Name() string
}

type jobMutator interface {
	admissionController
	Mutate(*structs.Job) (out *structs.Job, warnings []error, err error)
}

type jobValidator interface {
	admissionController
	Validate(*structs.Job) (warnings []error, err error)
}

func (j *Job) admissionControllers(job *structs.Job) (out *structs.Job, warnings []error, err error) {
	// Mutators run first before validators, so validators view the final rendered job.
	// So, mutators must handle invalid jobs.
	out, warnings, err = j.admissionMutators(job)
	if err != nil {
		return nil, nil, err
	}

	validateWarnings, err := j.admissionValidators(job)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, validateWarnings...)

	return out, warnings, nil
}

// admissionMutator returns an updated job as well as warnings or an error.
func (j *Job) admissionMutators(job *structs.Job) (_ *structs.Job, warnings []error, err error) {
	var w []error
	for _, mutator := range j.mutators {
		job, w, err = mutator.Mutate(job)
		j.logger.Trace("job mutate results", "mutator", mutator.Name(), "warnings", w, "error", err)
		if err != nil {
			return nil, nil, fmt.Errorf("error in job mutator %s: %v", mutator.Name(), err)
		}
		warnings = append(warnings, w...)
	}
	return job, warnings, err
}

// admissionValidators returns a slice of validation warnings and a multierror
// of validation failures.
func (j *Job) admissionValidators(origJob *structs.Job) ([]error, error) {
	// ensure job is not mutated
	job := origJob.Copy()

	var warnings []error
	var errs error

	for _, validator := range j.validators {
		w, err := validator.Validate(job)
		j.logger.Trace("job validate results", "validator", validator.Name(), "warnings", w, "error", err)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		warnings = append(warnings, w...)
	}

	return warnings, errs

}

// jobCanonicalizer calls job.Canonicalize (sets defaults and initializes
// fields) and returns any errors as warnings.
type jobCanonicalizer struct {
	srv *Server
}

func (c *jobCanonicalizer) Name() string {
	return "canonicalize"
}

func (c *jobCanonicalizer) Mutate(job *structs.Job) (*structs.Job, []error, error) {
	job.Canonicalize()

	// If the job priority is not set, we fallback on the defaults specified in the server config
	if job.Priority == 0 {
		job.Priority = c.srv.GetConfig().JobDefaultPriority
	}

	return job, nil, nil
}

// jobImpliedConstraints adds constraints to a job implied by other job fields
// and blocks.
type jobImpliedConstraints struct{}

func (jobImpliedConstraints) Name() string {
	return "constraints"
}

func (jobImpliedConstraints) Mutate(j *structs.Job) (*structs.Job, []error, error) {
	// Get the Vault blocks in the job
	vaultBlocks := j.Vault()

	// Get the required signals
	signals := j.RequiredSignals()

	// Identify which task groups are utilising Nomad native service discovery.
	nativeServiceDisco := j.RequiredNativeServiceDiscovery()

	// Identify which task groups are utilising Consul service discovery.
	consulServiceDisco := j.RequiredConsulServiceDiscovery()

	// Identify which task groups are utilizing NUMA resources.
	numaTaskGroups := j.RequiredNUMA()

	// Hot path where none of our things require constraints.
	//
	// [UPDATE THIS] if you are adding a new constraint thing!
	if len(signals) == 0 && len(vaultBlocks) == 0 &&
		nativeServiceDisco.Empty() && len(consulServiceDisco) == 0 &&
		numaTaskGroups.Empty() {
		return j, nil, nil
	}

	// Iterate through all the task groups within the job and add any required
	// constraints. When adding new implicit constraints, they should go inside
	// this single loop, with a new constraintMatcher if needed.
	for _, tg := range j.TaskGroups {
		// If the task group utilises Vault, run the mutator.
		vaultTasks := maps.Keys(vaultBlocks[tg.Name])
		sort.Strings(vaultTasks)
		for _, vaultTask := range vaultTasks {
			vaultBlock := vaultBlocks[tg.Name][vaultTask]
			mutateConstraint(constraintMatcherLeft, tg, vaultConstraintFn(vaultBlock))
		}

		// If the task group utilizes NUMA resources, run the mutator.
		if numaTaskGroups.Contains(tg.Name) {
			mutateConstraint(constraintMatcherFull, tg, numaVersionConstraint)
			mutateConstraint(constraintMatcherFull, tg, numaKernelConstraint)
		}

		// Check whether the task group is using signals. In the case that it
		// is, we flatten the signals and build a constraint, then run the
		// mutator.
		if tgSignals, ok := signals[tg.Name]; ok {
			required := helper.UniqueMapSliceValues(tgSignals)
			sigConstraint := getSignalConstraint(required)
			mutateConstraint(constraintMatcherFull, tg, sigConstraint)
		}

		// If the task group utilises Nomad service discovery, run the mutator.
		if nativeServiceDisco.Basic.Contains(tg.Name) {
			mutateConstraint(constraintMatcherFull, tg, nativeServiceDiscoveryConstraint)
		}

		// If the task group utilizes NSD checks, run the mutator.
		if nativeServiceDisco.Checks.Contains(tg.Name) {
			mutateConstraint(constraintMatcherFull, tg, nativeServiceDiscoveryChecksConstraint)
		}

		// If the task group utilises Consul service discovery, run the mutator.
		if consulServiceDisco[tg.Name] {
			for _, service := range tg.Services {
				if service.IsConsul() {
					mutateConstraint(constraintMatcherLeft, tg, consulConstraintFn(service))
				}

				for _, task := range tg.Tasks {
					for _, service := range task.Services {
						if service.IsConsul() {
							mutateConstraint(constraintMatcherLeft, tg, consulConstraintFn(service))
						}
					}
				}
			}
		}
	}

	return j, nil, nil
}

// vaultConstraintFn returns a constraint that matches the fingerprint of the
// requested Vault cluster. This is to support Nomad Enterprise but neither the
// fingerprint or non-default cluster are allowed well before we get here, so no
// need to split out the behavior to ENT-specific code.
func vaultConstraintFn(vault *structs.Vault) *structs.Constraint {
	if vault.Cluster != structs.VaultDefaultCluster && vault.Cluster != "" {
		return &structs.Constraint{
			LTarget: fmt.Sprintf("${attr.vault.%s.version}", vault.Cluster),
			RTarget: ">= 0.6.1",
			Operand: structs.ConstraintSemver,
		}
	}
	return vaultConstraint
}

// consulConstraintFn returns a service discovery constraint that matches the
// fingerprint of the requested Consul cluster. This is to support Nomad
// Enterprise but neither the fingerprint or non-default cluster are allowed
// well before we get here, so no need to split out the behavior to ENT-specific
// code.
func consulConstraintFn(service *structs.Service) *structs.Constraint {
	if service.Cluster != structs.ConsulDefaultCluster && service.Cluster != "" {
		return &structs.Constraint{
			LTarget: fmt.Sprintf("${attr.consul.%s.version}", service.Cluster),
			RTarget: ">= 1.7.0",
			Operand: structs.ConstraintSemver,
		}
	}
	return consulServiceDiscoveryConstraint
}

// constraintMatcher is a custom type which helps control how constraints are
// identified as being present within a task group.
type constraintMatcher uint

const (
	// constraintMatcherFull ensures that a constraint is only considered found
	// when they match totally. This check is performed using the
	// structs.Constraint Equal function.
	constraintMatcherFull constraintMatcher = iota

	// constraintMatcherLeft ensure that a constraint is considered found if
	// the constraints LTarget is matched only. This allows an existing
	// constraint to override the proposed implicit one.
	constraintMatcherLeft
)

// mutateConstraint is a generic mutator used to set implicit constraints
// within the task group if they are needed.
func mutateConstraint(matcher constraintMatcher, taskGroup *structs.TaskGroup, constraint *structs.Constraint) {

	var found bool

	// It's possible to switch on the matcher within the constraint loop to
	// reduce repetition. This, however, means switching per constraint,
	// therefore we do it here.
	switch matcher {
	case constraintMatcherFull:
		for _, c := range taskGroup.Constraints {
			if c.Equal(constraint) {
				found = true
				break
			}
		}
	case constraintMatcherLeft:
		for _, c := range taskGroup.Constraints {
			if c.LTarget == constraint.LTarget {
				found = true
				break
			}
		}
	}

	// If we didn't find a suitable constraint match, add one.
	if !found {
		taskGroup.Constraints = append(taskGroup.Constraints, constraint)
	}
}

// jobValidate validates a Job and task drivers and returns an error if there is
// a validation problem or if the Job is of a type a user is not allowed to
// submit.
type jobValidate struct {
	srv *Server
}

func (*jobValidate) Name() string {
	return "validate"
}

func (v *jobValidate) Validate(job *structs.Job) (warnings []error, err error) {
	validationErrors := new(multierror.Error)
	if err := job.Validate(); err != nil {
		multierror.Append(validationErrors, err)
	}

	// Get any warnings
	jobWarnings := job.Warnings()
	if jobWarnings != nil {
		if multi, ok := jobWarnings.(*multierror.Error); ok {
			// Unpack multiple warnings
			warnings = append(warnings, multi.Errors...)
		} else {
			warnings = append(warnings, jobWarnings)
		}
	}

	// TODO: Validate the driver configurations. These had to be removed in 0.9
	//       to support driver plugins, but see issue: #XXXX for more info.

	if job.Type == structs.JobTypeCore {
		multierror.Append(validationErrors, fmt.Errorf("job type cannot be core"))
	}

	if len(job.Payload) != 0 {
		multierror.Append(validationErrors, fmt.Errorf("job can't be submitted with a payload, only dispatched"))
	}

	if job.Priority < structs.JobMinPriority || job.Priority > v.srv.config.JobMaxPriority {
		multierror.Append(validationErrors, fmt.Errorf("job priority must be between [%d, %d]", structs.JobMinPriority, v.srv.config.JobMaxPriority))
	}

	for _, tg := range job.TaskGroups {
		for _, s := range tg.Services {
			serviceErrs := v.validateServiceIdentity(s, fmt.Sprintf("task group %s", tg.Name))
			multierror.Append(validationErrors, serviceErrs)
		}

		for _, t := range tg.Tasks {
			for _, s := range t.Services {
				serviceErrs := v.validateServiceIdentity(s, fmt.Sprintf("task %s", t.Name))
				multierror.Append(validationErrors, serviceErrs)
			}

			vaultWarns, vaultErrs := v.validateVaultIdentity(t)
			multierror.Append(validationErrors, vaultErrs)
			warnings = append(warnings, vaultWarns...)
		}
	}

	return warnings, validationErrors.ErrorOrNil()
}

func (v *jobValidate) validateServiceIdentity(s *structs.Service, parent string) error {
	if s.Identity != nil && s.Identity.Name == "" {
		return fmt.Errorf("Service %s in %s has an identity with an empty name", s.Name, parent)
	}

	return nil
}

func (v *jobValidate) validateVaultIdentity(t *structs.Task) ([]error, error) {
	var mErr *multierror.Error
	var warnings []error

	vaultWIDs := make([]string, 0, len(t.Identities))
	for _, wid := range t.Identities {
		if strings.HasPrefix(wid.Name, structs.WorkloadIdentityVaultPrefix) {
			vaultWIDs = append(vaultWIDs, wid.Name)
		}
	}

	if t.Vault == nil {
		for _, wid := range vaultWIDs {
			warnings = append(warnings, fmt.Errorf("Task %s has an identity called %s but no vault block", t.Name, wid))
		}
		return warnings, nil
	}

	hasTaskWID := len(vaultWIDs) > 0
	hasDefaultWID := v.srv.config.VaultIdentityConfig(t.Vault.Cluster) != nil

	if hasTaskWID || hasDefaultWID {
		if len(t.Vault.Policies) > 0 {
			warnings = append(warnings, fmt.Errorf(
				"Task %s has a Vault block with policies but uses workload identity to authenticate with Vault, policies will be ignored",
				t.Name,
			))
		}
		return warnings, nil
	}

	// At this point Nomad will use the legacy token-based flow, so keep the
	// existing validations.
	if len(t.Vault.Policies) == 0 {
		mErr = multierror.Append(mErr, fmt.Errorf("Task %s has a Vault block with an empty list of policies", t.Name))
	}

	return warnings, mErr.ErrorOrNil()
}

type memoryOversubscriptionValidate struct {
	srv *Server
}

func (*memoryOversubscriptionValidate) Name() string {
	return "memory_oversubscription"
}

func (v *memoryOversubscriptionValidate) Validate(job *structs.Job) (warnings []error, err error) {
	_, c, err := v.srv.State().SchedulerConfig()
	if err != nil {
		return nil, err
	}

	pool, err := v.srv.State().NodePoolByName(nil, job.NodePool)
	if err != nil {
		return nil, err
	}

	if pool.MemoryOversubscriptionEnabled(c) {
		return nil, nil
	}

	for _, tg := range job.TaskGroups {
		for _, t := range tg.Tasks {
			if t.Resources != nil && t.Resources.MemoryMaxMB != 0 {
				warnings = append(warnings, fmt.Errorf("Memory oversubscription is not enabled; Task \"%v.%v\" memory_max value will be ignored. Update the Scheduler Configuration to allow oversubscription.", tg.Name, t.Name))
			}
		}
	}

	return warnings, err
}

// submissionController is used to protect against job source sizes that exceed
// the maximum as set in server config as job_max_source_size
//
// Such jobs will have their source discarded and emit a warning, but the job
// itself will still continue with being registered.
func (j *Job) submissionController(args *structs.JobRegisterRequest) error {
	if args.Submission == nil {
		return nil
	}
	maxSize := j.srv.GetConfig().JobMaxSourceSize
	submission := args.Submission
	// discard the submission if the source + variables is larger than the maximum
	// allowable size as set by client config
	totalSize := len(submission.Source)
	totalSize += len(submission.Variables)
	for key, value := range submission.VariableFlags {
		totalSize += len(key)
		totalSize += len(value)
	}
	if totalSize > maxSize {
		args.Submission = nil
		totalSizeHuman := humanize.Bytes(uint64(totalSize))
		maxSizeHuman := humanize.Bytes(uint64(maxSize))
		return fmt.Errorf("job source size of %s exceeds maximum of %s and will be discarded", totalSizeHuman, maxSizeHuman)
	}
	return nil
}
