// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
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

	// Hot path
	if len(signals) == 0 && len(vaultBlocks) == 0 &&
		nativeServiceDisco.Empty() && len(consulServiceDisco) == 0 {
		return j, nil, nil
	}

	// Iterate through all the task groups within the job and add any required
	// constraints. When adding new implicit constraints, they should go inside
	// this single loop, with a new constraintMatcher if needed.
	for _, tg := range j.TaskGroups {

		// If the task group utilises Vault, run the mutator.
		if _, ok := vaultBlocks[tg.Name]; ok {
			mutateConstraint(constraintMatcherLeft, tg, vaultConstraint)
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
		if ok := consulServiceDisco[tg.Name]; ok {
			mutateConstraint(constraintMatcherLeft, tg, consulServiceDiscoveryConstraint)
		}
	}

	return j, nil, nil
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
			if s.Identity != nil && s.Identity.Name == "" {
				multierror.Append(validationErrors, fmt.Errorf("service identity name cannot be empty"))
			}
		}
	}

	return warnings, validationErrors.ErrorOrNil()
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

// jobIdentityCreator adds implicit `identity` blocks for external services,
// like Consul and Vault.
type jobIdentityCreator struct {
	srv *Server
}

func (jobIdentityCreator) Name() string {
	return "identityBlockCreator"
}

func (jobIdentityCreator) Mutate(job *structs.Job) (*structs.Job, []error, error) {
	for _, tg := range job.TaskGroups {
		for _, s := range tg.Services {
			if s.Provider != "consul" {
				continue
			}

			identityName := fmt.Sprintf("consul-service/%s", s.Name)

			// if there's an identity block present, overwrite its name and ServiceName, but
			// keep the rest. Users can set custom aud, file and env properties to account
			// for the specifics of their environments, but Name and ServiceName must always
			// conform to a convention.
			if s.Identity != nil {
				s.Identity.Name = identityName
				s.Identity.ServiceName = s.Name
			} else {
				s.Identity = &structs.WorkloadIdentity{
					Name:        identityName,
					Audience:    []string{"consul.io"},
					ServiceName: s.Name,
				}
			}
		}
	}

	return job, nil, nil
}
