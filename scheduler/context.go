// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"regexp"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Context is used to track contextual information used for placement
type Context interface {
	// State is used to inspect the current global state
	State() State

	// Plan returns the current plan
	Plan() *structs.Plan

	// Logger provides a way to log
	Logger() log.Logger

	// Metrics returns the current metrics
	Metrics() *structs.AllocMetric

	// Reset is invoked after making a placement
	Reset()

	// ProposedAllocs returns the proposed allocations for a node which are
	// the existing allocations, removing evictions, and adding any planned
	// placements.
	ProposedAllocs(nodeID string) ([]*structs.Allocation, error)

	// RegexpCache is a cache of regular expressions
	RegexpCache() map[string]*regexp.Regexp

	// VersionConstraintCache is a cache of version constraints
	VersionConstraintCache() map[string]VerConstraints

	// SemverConstraintCache is a cache of semver constraints
	SemverConstraintCache() map[string]VerConstraints

	// Eligibility returns a tracker for node eligibility in the context of the
	// eval.
	Eligibility() *EvalEligibility

	// SendEvent provides best-effort delivery of scheduling and placement
	// events.
	SendEvent(event interface{})
}

// EvalCache is used to cache certain things during an evaluation
type EvalCache struct {
	reCache      map[string]*regexp.Regexp
	versionCache map[string]VerConstraints
	semverCache  map[string]VerConstraints
}

func (e *EvalCache) RegexpCache() map[string]*regexp.Regexp {
	if e.reCache == nil {
		e.reCache = make(map[string]*regexp.Regexp)
	}
	return e.reCache
}

func (e *EvalCache) VersionConstraintCache() map[string]VerConstraints {
	if e.versionCache == nil {
		e.versionCache = make(map[string]VerConstraints)
	}
	return e.versionCache
}

func (e *EvalCache) SemverConstraintCache() map[string]VerConstraints {
	if e.semverCache == nil {
		e.semverCache = make(map[string]VerConstraints)
	}
	return e.semverCache
}

// PortCollisionEvent is an event that can happen during scheduling when
// an unexpected port collision is detected.
type PortCollisionEvent struct {
	Reason      string
	Node        *structs.Node
	Allocations []*structs.Allocation

	// TODO: this is a large struct, but may be required to debug unexpected
	// port collisions. Re-evaluate its need in the future if the bug is fixed
	// or not caused by this field.
	NetIndex *structs.NetworkIndex
}

func (ev *PortCollisionEvent) Copy() *PortCollisionEvent {
	if ev == nil {
		return nil
	}
	c := new(PortCollisionEvent)
	*c = *ev
	c.Node = ev.Node.Copy()
	if len(ev.Allocations) > 0 {
		for i, a := range ev.Allocations {
			c.Allocations[i] = a.Copy()
		}

	}
	c.NetIndex = ev.NetIndex.Copy()
	return c
}

func (ev *PortCollisionEvent) Sanitize() *PortCollisionEvent {
	if ev == nil {
		return nil
	}
	clean := ev.Copy()

	clean.Node = ev.Node.Sanitize()
	clean.Node.Meta = make(map[string]string)

	for i, alloc := range ev.Allocations {
		clean.Allocations[i] = alloc.CopySkipJob()
		clean.Allocations[i].Job = nil
	}

	return clean
}

// EvalContext is a Context used during an Evaluation
type EvalContext struct {
	EvalCache
	eventsCh    chan<- interface{}
	state       State
	plan        *structs.Plan
	logger      log.Logger
	metrics     *structs.AllocMetric
	eligibility *EvalEligibility
}

// NewEvalContext constructs a new EvalContext
func NewEvalContext(eventsCh chan<- interface{}, s State, p *structs.Plan, log log.Logger) *EvalContext {
	ctx := &EvalContext{
		eventsCh: eventsCh,
		state:    s,
		plan:     p,
		logger:   log,
		metrics:  new(structs.AllocMetric),
	}
	return ctx
}

func (e *EvalContext) State() State {
	return e.state
}

func (e *EvalContext) Plan() *structs.Plan {
	return e.plan
}

func (e *EvalContext) Logger() log.Logger {
	return e.logger
}

func (e *EvalContext) Metrics() *structs.AllocMetric {
	return e.metrics
}

func (e *EvalContext) SetState(s State) {
	e.state = s
}

func (e *EvalContext) Reset() {
	e.metrics = new(structs.AllocMetric)
}

func (e *EvalContext) ProposedAllocs(nodeID string) ([]*structs.Allocation, error) {
	// Get the existing allocations that are non-terminal
	ws := memdb.NewWatchSet()
	proposed, err := e.state.AllocsByNode(ws, nodeID)
	if err != nil {
		return nil, err
	}

	// Determine the proposed allocation by first removing allocations
	// that are planned evictions and adding the new allocations.
	if update := e.plan.NodeUpdate[nodeID]; len(update) > 0 {
		proposed = structs.RemoveAllocs(proposed, update)
	}

	// Remove any allocs that are being preempted
	nodePreemptedAllocs := e.plan.NodePreemptions[nodeID]
	if len(nodePreemptedAllocs) > 0 {
		proposed = structs.RemoveAllocs(proposed, nodePreemptedAllocs)
	}

	// We create an index of the existing allocations so that if an inplace
	// update occurs, we do not double count and we override the old allocation.
	proposedIDs := make(map[string]*structs.Allocation, len(proposed))
	for _, alloc := range proposed {
		if alloc.ClientTerminalStatus() {
			continue
		}

		proposedIDs[alloc.ID] = alloc
	}
	for _, alloc := range e.plan.NodeAllocation[nodeID] {
		proposedIDs[alloc.ID] = alloc
	}

	// Materialize the proposed slice
	proposed = make([]*structs.Allocation, 0, len(proposedIDs))
	for _, alloc := range proposedIDs {
		proposed = append(proposed, alloc)
	}

	return proposed, nil
}

func (e *EvalContext) Eligibility() *EvalEligibility {
	if e.eligibility == nil {
		e.eligibility = NewEvalEligibility()
	}

	return e.eligibility
}

func (e *EvalContext) SendEvent(event interface{}) {
	if e == nil || e.eventsCh == nil {
		return
	}

	select {
	case e.eventsCh <- event:
	default:
	}
}

type ComputedClassFeasibility byte

const (
	// EvalComputedClassUnknown is the initial state until the eligibility has
	// been explicitly marked to eligible/ineligible or escaped.
	EvalComputedClassUnknown ComputedClassFeasibility = iota

	// EvalComputedClassIneligible is used to mark the computed class as
	// ineligible for the evaluation.
	EvalComputedClassIneligible

	// EvalComputedClassIneligible is used to mark the computed class as
	// eligible for the evaluation.
	EvalComputedClassEligible

	// EvalComputedClassEscaped signals that computed class can not determine
	// eligibility because a constraint exists that is not captured by computed
	// node classes.
	EvalComputedClassEscaped
)

// EvalEligibility tracks eligibility of nodes by computed node class over the
// course of an evaluation.
type EvalEligibility struct {
	// job tracks the eligibility at the job level per computed node class.
	job map[string]ComputedClassFeasibility

	// jobEscaped marks whether constraints have escaped at the job level.
	jobEscaped bool

	// taskGroups tracks the eligibility at the task group level per computed
	// node class.
	taskGroups map[string]map[string]ComputedClassFeasibility

	// tgEscapedConstraints is a map of task groups to whether constraints have
	// escaped.
	tgEscapedConstraints map[string]bool

	// quotaReached marks that the quota limit has been reached for the given
	// quota
	quotaReached string
}

// NewEvalEligibility returns an eligibility tracker for the context of an evaluation.
func NewEvalEligibility() *EvalEligibility {
	return &EvalEligibility{
		job:                  make(map[string]ComputedClassFeasibility),
		taskGroups:           make(map[string]map[string]ComputedClassFeasibility),
		tgEscapedConstraints: make(map[string]bool),
	}
}

// SetJob takes the job being evaluated and calculates the escaped constraints
// at the job and task group level.
func (e *EvalEligibility) SetJob(job *structs.Job) {
	// Determine whether the job has escaped constraints.
	e.jobEscaped = len(structs.EscapedConstraints(job.Constraints)) != 0

	// Determine the escaped constraints per task group.
	for _, tg := range job.TaskGroups {
		constraints := tg.Constraints
		for _, task := range tg.Tasks {
			constraints = append(constraints, task.Constraints...)
		}

		e.tgEscapedConstraints[tg.Name] = len(structs.EscapedConstraints(constraints)) != 0
	}
}

// HasEscaped returns whether any of the constraints in the passed job have
// escaped computed node classes.
func (e *EvalEligibility) HasEscaped() bool {
	if e.jobEscaped {
		return true
	}

	for _, escaped := range e.tgEscapedConstraints {
		if escaped {
			return true
		}
	}

	return false
}

// GetClasses returns the tracked classes to their eligibility, across the job
// and task groups.
func (e *EvalEligibility) GetClasses() map[string]bool {
	elig := make(map[string]bool)

	// Go through the task groups.
	for _, classes := range e.taskGroups {
		for class, feas := range classes {
			switch feas {
			case EvalComputedClassEligible:
				elig[class] = true
			case EvalComputedClassIneligible:
				// Only mark as ineligible if it hasn't been marked before. This
				// prevents one task group marking a class as ineligible when it
				// is eligible on another task group.
				if _, ok := elig[class]; !ok {
					elig[class] = false
				}
			}
		}
	}

	// Go through the job.
	for class, feas := range e.job {
		switch feas {
		case EvalComputedClassEligible:
			// Only mark as eligible if it hasn't been marked before. This
			// prevents the job marking a class as eligible when it is ineligible
			// to all the task groups.
			if _, ok := elig[class]; !ok {
				elig[class] = true
			}
		case EvalComputedClassIneligible:
			elig[class] = false
		}
	}

	return elig
}

// JobStatus returns the eligibility status of the job.
func (e *EvalEligibility) JobStatus(class string) ComputedClassFeasibility {
	if e.jobEscaped {
		return EvalComputedClassEscaped
	}

	if status, ok := e.job[class]; ok {
		return status
	}
	return EvalComputedClassUnknown
}

// SetJobEligibility sets the eligibility status of the job for the computed
// node class.
func (e *EvalEligibility) SetJobEligibility(eligible bool, class string) {
	if eligible {
		e.job[class] = EvalComputedClassEligible
	} else {
		e.job[class] = EvalComputedClassIneligible
	}
}

// TaskGroupStatus returns the eligibility status of the task group.
func (e *EvalEligibility) TaskGroupStatus(tg, class string) ComputedClassFeasibility {
	if escaped, ok := e.tgEscapedConstraints[tg]; ok {
		if escaped {
			return EvalComputedClassEscaped
		}
	}

	if classes, ok := e.taskGroups[tg]; ok {
		if status, ok := classes[class]; ok {
			return status
		}
	}
	return EvalComputedClassUnknown
}

// SetTaskGroupEligibility sets the eligibility status of the task group for the
// computed node class.
func (e *EvalEligibility) SetTaskGroupEligibility(eligible bool, tg, class string) {
	var eligibility ComputedClassFeasibility
	if eligible {
		eligibility = EvalComputedClassEligible
	} else {
		eligibility = EvalComputedClassIneligible
	}

	if classes, ok := e.taskGroups[tg]; ok {
		classes[class] = eligibility
	} else {
		e.taskGroups[tg] = map[string]ComputedClassFeasibility{class: eligibility}
	}
}

// SetQuotaLimitReached marks that the quota limit has been reached for the
// given quota
func (e *EvalEligibility) SetQuotaLimitReached(quota string) {
	e.quotaReached = quota
}

// QuotaLimitReached returns the quota name if the quota limit has been reached.
func (e *EvalEligibility) QuotaLimitReached() string {
	return e.quotaReached
}
