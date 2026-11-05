// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"errors"
	"time"
)

const (
	ConstraintDistinctProperty  = "distinct_property"
	ConstraintDistinctHosts     = "distinct_hosts"
	ConstraintRegex             = "regexp"
	ConstraintVersion           = "version"
	ConstraintSemver            = "semver"
	ConstraintSetContains       = "set_contains"
	ConstraintSetContainsAll    = "set_contains_all"
	ConstraintSetContainsAny    = "set_contains_any"
	ConstraintAttributeIsSet    = "is_set"
	ConstraintAttributeIsNotSet = "is_not_set"

	JobDependencyComplete   = "jobComplete"   // All expected allocations are complete
	JobDependencyRunning    = "jobRunning"    // All expected allocations are running
	JobDependencyRecovering = "jobRecovering" // Some allocations are pending
	JobDependencyLost       = "jobLost"       // All allocations are unknown
	JobDependencyFailed     = "jobFailed"     // All allocations are failed, lost, or unplaced

	JobDependencyTimeoutDefault = 1 * time.Hour
	JobDependencyStatusDefault  = JobDependencyComplete
)

// Constraint is used to serialize a job placement constraint.
type Constraint struct {
	LTarget string `hcl:"attribute,optional"`
	RTarget string `hcl:"value,optional"`
	Operand string `hcl:"operator,optional"`
}

// NewConstraint generates a new job placement constraint.
func NewConstraint(left, operand, right string) *Constraint {
	return &Constraint{
		LTarget: left,
		RTarget: right,
		Operand: operand,
	}
}

type JobDependency struct {
	Name   string `hcl:"name"`
	Status string `hcl:"status,optional"`
}

func NewJobDependency(name, status string) *JobDependency {
	return &JobDependency{
		Name:   name,
		Status: status,
	}
}

func (d *JobDependency) Canonicalize() {
	if d.Status == "" {
		d.Status = JobDependencyStatusDefault
	}
}

func (d *JobDependency) Copy() *JobDependency {
	if d == nil {
		return nil
	}

	copy := *d
	return &copy
}

func (d *JobDependency) Validate() error {
	if d.Name == "" {
		return errors.New("dependency job name is required")
	}

	switch d.Status {
	case JobDependencyComplete, JobDependencyRunning, JobDependencyRecovering,
		JobDependencyLost, JobDependencyFailed:

	default:
		return errors.New("invalid state for dependency job")
	}

	return nil
}

// JobDependencies is used to serialize a job placement dependency.
// A value of 0 on the timeout means no timeout.
type JobDependencies struct {
	Timeout *time.Duration   `hcl:"timeout,optional"`
	Jobs    []*JobDependency `hcl:"job,block"`
}

func NewJobDependencies(timeout, actionOnTimeout string, jobs ...*JobDependency) *JobDependencies {
	copyJobs := make([]*JobDependency, 0, len(jobs))
	for _, job := range jobs {
		copyJobs = append(copyJobs, job.Copy())
	}

	duration, _ := time.ParseDuration(timeout)
	return &JobDependencies{
		Timeout: &duration,
		Jobs:    copyJobs,
	}
}

func (d *JobDependencies) Canonicalize() {
	if d.Timeout == nil {
		d.Timeout = new(JobDependencyTimeoutDefault)
	}
	for _, job := range d.Jobs {
		job.Canonicalize()
	}
}

func (d *JobDependencies) Copy() *JobDependencies {
	if d == nil {
		return nil
	}

	jobs := make([]*JobDependency, 0, len(d.Jobs))
	for _, job := range d.Jobs {
		jobs = append(jobs, job.Copy())
	}

	return &JobDependencies{
		Timeout: d.Timeout,
		Jobs:    jobs,
	}
}

func (d *JobDependencies) Validate() error {
	if d == nil {
		return nil
	}

	if d.Timeout == nil {
		return errors.New("dependency timeout is required")
	}

	if len(d.Jobs) == 0 {
		return errors.New("dependency requires at least one job block")
	}

	// Should we check that each dependency is unique??
	for _, job := range d.Jobs {
		if err := job.Validate(); err != nil {
			return err
		}
	}

	return nil
}
