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

func NewJobDependency(name, status string) *JobDependency {
	return &JobDependency{
		Name:   name,
		Status: status,
	}
}

func (d *JobDependency) Canonicalize() {
	if d.Status == "" {
		d.Status = "dead"
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
	return nil
}

// Dependency is used to serialize a job placement dependency.
type Dependency struct {
	Timeout *time.Duration   `hcl:"timeout,optional"`
	Jobs    []*JobDependency `hcl:"job,block"`
}

func NewDependency(timeout string, jobs ...*JobDependency) *Dependency {
	copyJobs := make([]*JobDependency, 0, len(jobs))
	for _, job := range jobs {
		copyJobs = append(copyJobs, job.Copy())
	}

	duration, _ := time.ParseDuration(timeout)
	return &Dependency{
		Timeout: &duration,
		Jobs:    copyJobs,
	}
}

func (d *Dependency) Canonicalize() {
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
		Timeout: d.Timeout,
		Jobs:    jobs,
	}
}

func (d *Dependency) Validate() error {
	if d == nil {
		return nil
	}

	if d.Timeout == nil || *d.Timeout == 0 {
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
