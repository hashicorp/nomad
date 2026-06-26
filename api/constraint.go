// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"errors"
	"fmt"
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
	Name   string `hcl:"name,optional"`
	Status string `hcl:"status,optional"`
}

// JobDepdendency is kept as an alias for compatibility with callers using the
// legacy misspelled type name.
type JobDepdendency = JobDependency

func NewJobDependency(name, status string) *JobDependency {
	return &JobDependency{
		Name:   name,
		Status: status,
	}
}

func (d *JobDependency) Canonicalize() {
	if d.Status == "" {
		d.Status = "completed"
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

	if d.Status == "" {
		return errors.New("dependency job status is required")
	}

	return nil
}

// Dependency is used to serialize a job placement dependency.
type Dependency struct {
	Timeout         string           `hcl:"timeout,optional"`
	ActionOnTimeout string           `hcl:"action_on_timeout,optional"`
	Jobs            []*JobDependency `hcl:"job,block"`
}

func NewDependency(timeout, actionOnTimeout string, jobs ...JobDepdendency) *Dependency {
	copyJobs := make([]*JobDependency, 0, len(jobs))
	for _, job := range jobs {
		copyJobs = append(copyJobs, (&job).Copy())
	}

	return &Dependency{
		Timeout:         timeout,
		Jobs:            copyJobs,
		ActionOnTimeout: actionOnTimeout,
	}
}

func (d *Dependency) Canonicalize() {
	if d.ActionOnTimeout == "" {
		d.ActionOnTimeout = "reject"
	}

	for _, job := range d.Jobs {
		job.Canonicalize()
	}
}

func (d *Dependency) Copy() *Dependency {
	if d == nil {
		return nil
	}

	jobs := make([]*JobDependency, 0, len(d.Jobs))
	for _, job := range d.Jobs {
		jobs = append(jobs, job.Copy())
	}

	return &Dependency{
		Timeout:         d.Timeout,
		ActionOnTimeout: d.ActionOnTimeout,
		Jobs:            jobs,
	}
}

func (d *Dependency) Validate() error {
	if d == nil {
		return nil
	}

	if d.Timeout == "" {
		return errors.New("dependency timeout is required")
	}

	if d.ActionOnTimeout == "" {
		return errors.New("dependency action_on_timeout is required")
	}

	if d.ActionOnTimeout != "reject" {
		return fmt.Errorf("invalid dependency action_on_timeout %q", d.ActionOnTimeout)
	}

	if len(d.Jobs) == 0 {
		return errors.New("dependency requires at least one job block")
	}

	for _, job := range d.Jobs {
		if err := job.Validate(); err != nil {
			return err
		}
	}

	return nil
}
