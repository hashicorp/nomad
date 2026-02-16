// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"slices"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
)

const (
	// DeploymentStatuses are the various states a deployment can be be in
	DeploymentStatusRunning      = "running"
	DeploymentStatusPaused       = "paused"
	DeploymentStatusFailed       = "failed"
	DeploymentStatusSuccessful   = "successful"
	DeploymentStatusCancelled    = "cancelled"
	DeploymentStatusInitializing = "initializing"
	DeploymentStatusPending      = "pending"
	DeploymentStatusBlocked      = "blocked"
	DeploymentStatusUnblocking   = "unblocking"

	// TODO Statuses and Descriptions do not match 1:1 and we sometimes use the Description as a status flag

	// DeploymentStatusDescriptions are the various descriptions of the states a
	// deployment can be in.
	DeploymentStatusDescriptionRunning               = "Deployment is running"
	DeploymentStatusDescriptionRunningNeedsPromotion = "Deployment is running but requires manual promotion"
	DeploymentStatusDescriptionRunningAutoPromotion  = "Deployment is running pending automatic promotion"
	DeploymentStatusDescriptionPaused                = "Deployment is paused"
	DeploymentStatusDescriptionSuccessful            = "Deployment completed successfully"
	DeploymentStatusDescriptionStoppedJob            = "Cancelled because job is stopped"
	DeploymentStatusDescriptionNewerJob              = "Cancelled due to newer version of job"
	DeploymentStatusDescriptionFailedAllocations     = "Failed due to unhealthy allocations"
	DeploymentStatusDescriptionProgressDeadline      = "Failed due to progress deadline"
	DeploymentStatusDescriptionFailedByUser          = "Deployment marked as failed"

	// used only in multiregion deployments
	DeploymentStatusDescriptionFailedByPeer   = "Failed because of an error in peer region"
	DeploymentStatusDescriptionBlocked        = "Deployment is complete but waiting for peer region"
	DeploymentStatusDescriptionUnblocking     = "Deployment is unblocking remaining regions"
	DeploymentStatusDescriptionPendingForPeer = "Deployment is pending, waiting for peer region"
)

// DeploymentStatusDescriptionRollback is used to get the status description of
// a deployment when rolling back to an older job.
func DeploymentStatusDescriptionRollback(baseDescription string, jobVersion uint64) string {
	return fmt.Sprintf("%s - rolling back to job version %d", baseDescription, jobVersion)
}

// DeploymentStatusDescriptionRollbackNoop is used to get the status description of
// a deployment when rolling back is not possible because it has the same specification
func DeploymentStatusDescriptionRollbackNoop(baseDescription string, jobVersion uint64) string {
	return fmt.Sprintf("%s - not rolling back to stable job version %d as current job has same specification", baseDescription, jobVersion)
}

// DeploymentStatusDescriptionNoRollbackTarget is used to get the status description of
// a deployment when there is no target to rollback to but autorevert is desired.
func DeploymentStatusDescriptionNoRollbackTarget(baseDescription string) string {
	return fmt.Sprintf("%s - no stable job version to auto revert to", baseDescription)
}

// Deployment is the object that represents a job deployment which is used to
// transition a job between versions.
type Deployment struct {
	// ID is a generated UUID for the deployment
	ID string

	// Namespace is the namespace the deployment is created in
	Namespace string

	// JobID is the job the deployment is created for
	JobID string

	// JobVersion is the version of the job at which the deployment is tracking
	JobVersion uint64

	// JobModifyIndex is the ModifyIndex of the job which the deployment is
	// tracking.
	JobModifyIndex uint64

	// JobSpecModifyIndex is the JobModifyIndex of the job which the
	// deployment is tracking.
	JobSpecModifyIndex uint64

	// JobCreateIndex is the create index of the job which the deployment is
	// tracking. It is needed so that if the job gets stopped and reran we can
	// present the correct list of deployments for the job and not old ones.
	JobCreateIndex uint64

	// Multiregion specifies if deployment is part of multiregion deployment
	IsMultiregion bool

	// TaskGroups is the set of task groups effected by the deployment and their
	// current deployment status.
	TaskGroups map[string]*DeploymentState

	// The status of the deployment
	Status string

	// StatusDescription allows a human readable description of the deployment
	// status.
	StatusDescription string

	// EvalPriority tracks the priority of the evaluation which lead to the
	// creation of this Deployment object. Any additional evaluations created
	// as a result of this deployment can therefore inherit this value, which
	// is not guaranteed to be that of the job priority parameter.
	EvalPriority int

	CreateIndex uint64
	ModifyIndex uint64

	// Creation and modification times, stored as UnixNano
	CreateTime int64
	ModifyTime int64
}

// NewDeployment creates a new deployment given the job.
func NewDeployment(job *Job, evalPriority int, now int64) *Deployment {
	return &Deployment{
		ID:                 uuid.Generate(),
		Namespace:          job.Namespace,
		JobID:              job.ID,
		JobVersion:         job.Version,
		JobModifyIndex:     job.ModifyIndex,
		JobSpecModifyIndex: job.JobModifyIndex,
		JobCreateIndex:     job.CreateIndex,
		IsMultiregion:      job.IsMultiregion(),
		Status:             DeploymentStatusRunning,
		StatusDescription:  DeploymentStatusDescriptionRunning,
		TaskGroups:         make(map[string]*DeploymentState, len(job.TaskGroups)),
		EvalPriority:       evalPriority,
		CreateTime:         now,
	}
}

func (d *Deployment) Copy() *Deployment {
	if d == nil {
		return nil
	}

	c := &Deployment{}
	*c = *d

	c.TaskGroups = nil
	if l := len(d.TaskGroups); d.TaskGroups != nil {
		c.TaskGroups = make(map[string]*DeploymentState, l)
		for tg, s := range d.TaskGroups {
			c.TaskGroups[tg] = s.Copy()
		}
	}

	return c
}

// Stub implements support for pagination
func (d *Deployment) Stub() (*Deployment, error) {
	return d, nil
}

// Active returns whether the deployment is active or terminal.
func (d *Deployment) Active() bool {
	switch d.Status {
	case DeploymentStatusRunning, DeploymentStatusPaused, DeploymentStatusBlocked,
		DeploymentStatusUnblocking, DeploymentStatusInitializing, DeploymentStatusPending:
		return true
	default:
		return false
	}
}

// GetID is a helper for getting the ID when the object may be nil
func (d *Deployment) GetID() string {
	if d == nil {
		return ""
	}
	return d.ID
}

// GetCreateIndex implements the CreateIndexGetter interface, required for
// pagination.
func (d *Deployment) GetCreateIndex() uint64 {
	if d == nil {
		return 0
	}
	return d.CreateIndex
}

// HasPlacedCanaries returns whether the deployment has placed canaries
func (d *Deployment) HasPlacedCanaries() bool {
	if d == nil || len(d.TaskGroups) == 0 {
		return false
	}
	for _, group := range d.TaskGroups {
		if len(group.PlacedCanaries) != 0 {
			return true
		}
	}
	return false
}

// RequiresPromotion returns whether the deployment requires promotion to
// continue
func (d *Deployment) RequiresPromotion() bool {
	if d == nil || len(d.TaskGroups) == 0 || d.Status != DeploymentStatusRunning {
		return false
	}
	for _, group := range d.TaskGroups {
		if group.DesiredCanaries > 0 && !group.Promoted {
			return true
		}
	}
	return false
}

// HasAutoPromote determines if all taskgroups are marked auto_promote
func (d *Deployment) HasAutoPromote() bool {
	if d == nil || len(d.TaskGroups) == 0 || d.Status != DeploymentStatusRunning {
		return false
	}
	for _, group := range d.TaskGroups {
		if group.DesiredCanaries > 0 && !group.AutoPromote {
			return false
		}
	}
	return true
}

func (d *Deployment) GoString() string {
	base := fmt.Sprintf("Deployment ID %q for job %q has status %q (%v):", d.ID, d.JobID, d.Status, d.StatusDescription)
	for group, state := range d.TaskGroups {
		base += fmt.Sprintf("\nTask Group %q has state:\n%#v", group, state)
	}
	return base
}

// GetNamespace implements the NamespaceGetter interface, required for pagination.
func (d *Deployment) GetNamespace() string {
	if d == nil {
		return ""
	}
	return d.Namespace
}

// DeploymentState tracks the state of a deployment for a given task group.
type DeploymentState struct {
	// AutoRevert marks whether the task group has indicated the job should be
	// reverted on failure
	AutoRevert bool

	// AutoPromote marks promotion triggered automatically by healthy canaries
	// copied from TaskGroup UpdateStrategy in scheduler.reconcile
	AutoPromote bool

	// ProgressDeadline is the deadline by which an allocation must transition
	// to healthy before the deployment is considered failed. This value is set
	// by the jobspec `update.progress_deadline` field.
	ProgressDeadline time.Duration

	// RequireProgressBy is the time by which an allocation must transition to
	// healthy before the deployment is considered failed. This value is reset
	// to "now" + ProgressDeadline when an allocation updates the deployment.
	RequireProgressBy time.Time

	// Promoted marks whether the canaries have been promoted
	Promoted bool

	// PlacedCanaries is the set of placed canary allocations
	PlacedCanaries []string

	// DesiredCanaries is the number of canaries that should be created.
	DesiredCanaries int

	// DesiredTotal is the total number of allocations that should be created as
	// part of the deployment.
	DesiredTotal int

	// PlacedAllocs is the number of allocations that have been placed
	PlacedAllocs int

	// HealthyAllocs is the number of allocations that have been marked healthy.
	HealthyAllocs int

	// UnhealthyAllocs are allocations that have been marked as unhealthy.
	UnhealthyAllocs int
}

func (d *DeploymentState) GoString() string {
	base := fmt.Sprintf("\tDesired Total: %d", d.DesiredTotal)
	base += fmt.Sprintf("\n\tDesired Canaries: %d", d.DesiredCanaries)
	base += fmt.Sprintf("\n\tPlaced Canaries: %#v", d.PlacedCanaries)
	base += fmt.Sprintf("\n\tPromoted: %v", d.Promoted)
	base += fmt.Sprintf("\n\tPlaced: %d", d.PlacedAllocs)
	base += fmt.Sprintf("\n\tHealthy: %d", d.HealthyAllocs)
	base += fmt.Sprintf("\n\tUnhealthy: %d", d.UnhealthyAllocs)
	base += fmt.Sprintf("\n\tAutoRevert: %v", d.AutoRevert)
	base += fmt.Sprintf("\n\tAutoPromote: %v", d.AutoPromote)
	return base
}

func (d *DeploymentState) Copy() *DeploymentState {
	if d == nil {
		return nil
	}
	c := &DeploymentState{}
	*c = *d
	c.PlacedCanaries = slices.Clone(d.PlacedCanaries)
	return c
}

func (d *DeploymentState) MergeClientValues(c *DeploymentState) {
	d.HealthyAllocs = c.HealthyAllocs
	d.UnhealthyAllocs = c.UnhealthyAllocs
	d.PlacedAllocs = c.PlacedAllocs
	d.RequireProgressBy = c.RequireProgressBy
}

// DeploymentStatusUpdate is used to update the status of a given deployment
type DeploymentStatusUpdate struct {
	// DeploymentID is the ID of the deployment to update
	DeploymentID string

	// Status is the new status of the deployment.
	Status string

	// StatusDescription is the new status description of the deployment.
	StatusDescription string

	// UpdatedAt is the time of the update, stored as UnixNano
	UpdatedAt int64
}
