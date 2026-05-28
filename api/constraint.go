// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

package api

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

// Dependency is used to serialize a job placement dependency.
type Dependency struct {
	Name   string `hcl:"name,label"`
	Output string `hcl:"output,optional"`
	Job    string `hcl:"job"`
}

func NewDependency(name, job, output string) *Dependency {
	return &Dependency{
		Name:   name,
		Job:    job,
		Output: output,
	}
}

func (d *Dependency) Canonicalize() {
	if d.Name == "" {
		d.Name = d.Job
	}

	if d.Output == "" {
		d.Output = "dead"
	}
}

func (d *Dependency) Copy() *Dependency {
	if d == nil {
		return nil
	}
	return &Dependency{
		Job:    d.Job,
		Output: d.Output,
		Name:   d.Name,
	}
}
