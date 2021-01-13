package nomad

import (
	"fmt"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// vaultConstraintLTarget is the lefthand side of the Vault constraint
	// injected when Vault policies are used. If an existing constraint
	// with this target exists it overrides the injected constraint.
	vaultConstraintLTarget = "${attr.vault.version}"
)

var (
	// vaultConstraint is the implicit constraint added to jobs requesting a
	// Vault token
	vaultConstraint = &structs.Constraint{
		LTarget: vaultConstraintLTarget,
		RTarget: ">= 0.6.1",
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
type jobCanonicalizer struct{}

func (jobCanonicalizer) Name() string {
	return "canonicalize"
}

func (jobCanonicalizer) Mutate(job *structs.Job) (*structs.Job, []error, error) {
	job.Canonicalize()
	return job, nil, nil
}

// jobImpliedConstraints adds constraints to a job implied by other job fields
// and stanzas.
type jobImpliedConstraints struct{}

func (jobImpliedConstraints) Name() string {
	return "constraints"
}

func (jobImpliedConstraints) Mutate(j *structs.Job) (*structs.Job, []error, error) {
	// Get the required Vault Policies
	policies := j.VaultPolicies()

	// Get the required signals
	signals := j.RequiredSignals()

	// Hot path
	if len(signals) == 0 && len(policies) == 0 {
		return j, nil, nil
	}

	// Add Vault constraints if no Vault constraint exists
	for _, tg := range j.TaskGroups {
		_, ok := policies[tg.Name]
		if !ok {
			// Not requesting Vault
			continue
		}

		found := false
		for _, c := range tg.Constraints {
			if c.LTarget == vaultConstraintLTarget {
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
			if c.Equals(sigConstraint) {
				found = true
				break
			}
		}

		if !found {
			tg.Constraints = append(tg.Constraints, sigConstraint)
		}
	}

	return j, nil, nil
}

// jobValidate validates a Job and task drivers and returns an error if there is
// a validation problem or if the Job is of a type a user is not allowed to
// submit.
type jobValidate struct{}

func (jobValidate) Name() string {
	return "validate"
}

func (jobValidate) Validate(job *structs.Job) (warnings []error, err error) {
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

	return warnings, validationErrors.ErrorOrNil()
}
