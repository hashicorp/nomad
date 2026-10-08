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
	JobDependencyComplete   = "jobComplete"   // All expected allocations are complete
	JobDependencyRunning    = "jobRunning"    // All expected allocations are running
	JobDependencyRecovering = "jobRecovering" // Some allocations are pending
	JobDependencyLost       = "jobLost"       // All allocations are unknown
	JobDependencyFailed     = "jobFailed"     // All allocations are failed, lost, or unplaced

	JobDependencyTimeoutDefault = 1 * time.Hour
	JobDependencyStatusDefault  = JobDependencyComplete
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

	switch d.Status {
	case JobDependencyComplete, JobDependencyRunning, JobDependencyRecovering,
		JobDependencyLost, JobDependencyFailed:

	default:
		return errors.New("invalid state for dependency job")
	}

	return nil
}

func (d *JobDependency) Canonicalize() {
	if d == nil {
		return
	}

	if d.Status == "" {
		d.Status = JobDependencyComplete
	}
}

func (d *JobDependency) String() string {
	if d == nil {
		return ""
	}

	return fmt.Sprintf("%s: %s", d.Name, d.Status)
}

// JobDependencies is used to restrict placement options.
type JobDependencies struct {
	Timeout *time.Duration
	Jobs    []*JobDependency
}

func (d *JobDependencies) Empty() bool {
	if d != nil && len(d.Jobs) > 0 {
		return false
	}

	return true
}

// Equal checks if two dependencies are equal.
func (d *JobDependencies) Equal(o *JobDependencies) bool {
	if d == nil || o == nil {
		return d == o
	}

	if d.Timeout != o.Timeout {
		return false
	}

	if len(d.Jobs) != len(o.Jobs) {
		return false
	}

	for i, job := range d.Jobs {
		if !job.Equal(o.Jobs[i]) {
			return false
		}
	}

	return true
}

func (d *JobDependencies) Copy() *JobDependencies {
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

	return &JobDependencies{
		Timeout: d.Timeout,
		Jobs:    jobs,
	}
}

func (d *JobDependencies) String() string {
	jobs := make([]string, 0, len(d.Jobs))
	for _, j := range d.Jobs {
		jobs = append(jobs, j.String())
	}

	return fmt.Sprintf("%s: %s", d.Timeout, strings.Join(jobs, ", "))
}

func (d *JobDependencies) Validate() error {
	var mErr multierror.Error
	if d == nil {
		return nil
	}

	if len(d.Jobs) == 0 {
		mErr.Errors = append(mErr.Errors, errors.New("Missing job in dependency"))
		return mErr.ErrorOrNil()
	}

	for idx, job := range d.Jobs {
		if err := job.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("Dependency job %d validation failed: %v", idx+1, err))
		}
	}

	return mErr.ErrorOrNil()
}

func (d *JobDependencies) Canonicalize() {
	if d == nil {
		return
	}

	if d.Timeout == nil {
		d.Timeout = new(JobDependencyTimeoutDefault)
	}

	for _, job := range d.Jobs {
		job.Canonicalize()
	}
}

// DiffID fulfills the DiffableWithID interface.
func (d *JobDependencies) DiffID() string {
	return d.String()
}
