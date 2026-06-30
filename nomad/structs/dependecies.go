// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
)

const (
	DependencyActionReject   = "reject"
	DependencyActionDispatch = "dispatch"
)

type JobDependency struct {
	Name   string
	Status string
}

func (d *JobDependency) Equal(o *JobDependency) bool {
	if d == nil || o == nil {
		return d == o
	}

	return d == o ||
		d.Name == o.Name &&
			d.Status == o.Status
}

func (d *JobDependency) Validate() error {
	if d == nil {
		return errors.New("dependency job block is required")
	}

	if d.Name == "" {
		return errors.New("dependency job name is mandatory")
	}

	if d.Status == "" {
		return errors.New("dependency job status is mandatory")
	}

	return nil
}

func (d *JobDependency) Canonicalize() {
	if d == nil {
		return
	}

	if d.Status == "" {
		d.Status = "completed"
	}
}

func (d *JobDependency) String() string {
	if d == nil {
		return ""
	}

	return fmt.Sprintf("%s: %s", d.Name, d.Status)
}

// A Dependency is used to restrict placement options.
type Dependency struct {
	Timeout         time.Duration
	ActionOnTimeout string
	Jobs            []*JobDependency
}

// Equal checks if two dependencies are equal.
func (d *Dependency) Equal(o *Dependency) bool {
	if d == nil || o == nil {
		return d == o
	}

	if len(d.Jobs) != len(o.Jobs) {
		return false
	}

	jEqual := true
	for i := range d.Jobs {
		jEqual = jEqual && d.Jobs[i].Equal(o.Jobs[i])
	}

	return d == o ||
		d.Timeout == o.Timeout &&
			d.ActionOnTimeout == o.ActionOnTimeout &&
			jEqual
}

func (d *Dependency) Copy() *Dependency {
	if d == nil {
		return nil
	}

	jobs := make([]*JobDependency, 0, len(d.Jobs))
	for _, job := range d.Jobs {
		if job == nil {
			jobs = append(jobs, nil)
			continue
		}

		copy := *job
		jobs = append(jobs, &copy)
	}

	return &Dependency{
		Timeout:         d.Timeout,
		ActionOnTimeout: d.ActionOnTimeout,
		Jobs:            jobs,
	}
}

func (d *Dependency) String() string {
	jobs := make([]string, 0, len(d.Jobs))
	for _, j := range d.Jobs {
		jobs = append(jobs, j.String())
	}

	return fmt.Sprintf("%s %s: %s", d.Timeout, d.ActionOnTimeout, strings.Join(jobs, ", "))
}

func (d *Dependency) Validate() error {
	var mErr multierror.Error
	if d == nil {
		return nil
	}

	if d.ActionOnTimeout != DependencyActionReject && d.ActionOnTimeout != DependencyActionDispatch {
		mErr.Errors = append(mErr.Errors, errors.New("Invalid action on timeout in dependency, must be 'reject' or 'dispatch'"))
	}

	if len(d.Jobs) == 0 {
		mErr.Errors = append(mErr.Errors, errors.New("Missing job in dependency"))
	}

	for idx, job := range d.Jobs {
		if err := job.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Dependency job %d validation failed: %v", idx+1, err))
		}
	}

	return mErr.ErrorOrNil()
}

func (d *Dependency) Canonicalize() {
	if d == nil {
		return
	}

	if d.ActionOnTimeout == "" {
		d.ActionOnTimeout = "reject"
	}

	for _, job := range d.Jobs {
		job.Canonicalize()
	}
}

// DiffID fulfills the DiffableWithID interface.
func (d *Dependency) DiffID() string {
	return d.String()
}
