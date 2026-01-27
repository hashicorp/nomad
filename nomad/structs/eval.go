// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
)

const (
	EvalTriggerJobRegister          = "job-register"
	EvalTriggerJobDeregister        = "job-deregister"
	EvalTriggerPeriodicJob          = "periodic-job"
	EvalTriggerNodeDrain            = "node-drain"
	EvalTriggerNodeUpdate           = "node-update"
	EvalTriggerAllocStop            = "alloc-stop"
	EvalTriggerScheduled            = "scheduled"
	EvalTriggerRollingUpdate        = "rolling-update"
	EvalTriggerDeploymentWatcher    = "deployment-watcher"
	EvalTriggerFailedFollowUp       = "failed-follow-up"
	EvalTriggerMaxPlans             = "max-plan-attempts"
	EvalTriggerRetryFailedAlloc     = "alloc-failure"
	EvalTriggerQueuedAllocs         = "queued-allocs"
	EvalTriggerPreemption           = "preemption"
	EvalTriggerScaling              = "job-scaling"
	EvalTriggerMaxDisconnectTimeout = "max-disconnect-timeout"
	EvalTriggerReconnect            = "reconnect"
	EvalTriggerAllocReschedule      = "alloc-reschedule"

	EvalStatusBlocked   = "blocked"
	EvalStatusPending   = "pending"
	EvalStatusComplete  = "complete"
	EvalStatusFailed    = "failed"
	EvalStatusCancelled = "canceled"

	// EvalDeleteRPCMethod is the RPC method for batch deleting evaluations
	// using their IDs.
	//
	// Args: EvalDeleteRequest
	// Reply: EvalDeleteResponse
	EvalDeleteRPCMethod = "Eval.Delete"

	// CoreJobEvalGC is used for the garbage collection of evaluations
	// and allocations. We periodically scan evaluations in a terminal state,
	// in which all the corresponding allocations are also terminal. We
	// delete these out of the system to bound the state.
	CoreJobEvalGC = "eval-gc"

	// CoreJobNodeGC is used for the garbage collection of failed nodes.
	// We periodically scan nodes in a terminal state, and if they have no
	// corresponding allocations we delete these out of the system.
	CoreJobNodeGC = "node-gc"

	// CoreJobJobGC is used for the garbage collection of eligible jobs. We
	// periodically scan garbage collectible jobs and check if both their
	// evaluations and allocations are terminal. If so, we delete these out of
	// the system.
	CoreJobJobGC = "job-gc"

	// CoreJobDeploymentGC is used for the garbage collection of eligible
	// deployments. We periodically scan garbage collectible deployments and
	// check if they are terminal. If so, we delete these out of the system.
	CoreJobDeploymentGC = "deployment-gc"

	// CoreJobCSIVolumeClaimGC is use for the garbage collection of CSI
	// volume claims. We periodically scan volumes to see if no allocs are
	// claiming them. If so, we unclaim the volume.
	CoreJobCSIVolumeClaimGC = "csi-volume-claim-gc"

	// CoreJobCSIPluginGC is use for the garbage collection of CSI plugins.
	// We periodically scan plugins to see if they have no associated volumes
	// or allocs running them. If so, we delete the plugin.
	CoreJobCSIPluginGC = "csi-plugin-gc"

	// CoreJobOneTimeTokenGC is use for the garbage collection of one-time
	// tokens. We periodically scan for expired tokens and delete them.
	CoreJobOneTimeTokenGC = "one-time-token-gc"

	// CoreJobLocalTokenExpiredGC is used for the garbage collection of
	// expired local ACL tokens. We periodically scan for expired tokens and
	// delete them.
	CoreJobLocalTokenExpiredGC = "local-token-expired-gc"

	// CoreJobGlobalTokenExpiredGC is used for the garbage collection of
	// expired global ACL tokens. We periodically scan for expired tokens and
	// delete them.
	CoreJobGlobalTokenExpiredGC = "global-token-expired-gc"

	// CoreJobRootKeyRotateGC is used for periodic key rotation and
	// garbage collection of unused encryption keys.
	CoreJobRootKeyRotateOrGC = "root-key-rotate-gc"

	// CoreJobVariablesRekey is used to fully rotate the encryption keys for
	// variables by decrypting all variables and re-encrypting them with the
	// active key
	CoreJobVariablesRekey = "variables-rekey"

	// CoreJobForceGC is used to force garbage collection of all GCable objects.
	CoreJobForceGC = "force-gc"
)

// Evaluation is used anytime we need to apply business logic as a result
// of a change to our desired state (job specification) or the emergent state
// (registered nodes). When the inputs change, we need to "evaluate" them,
// potentially taking action (allocation of work) or doing nothing if the state
// of the world does not require it.
type Evaluation struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	// ID is a randomly generated UUID used for this evaluation. This
	// is assigned upon the creation of the evaluation.
	ID string

	// Namespace is the namespace the evaluation is created in
	Namespace string

	// Priority is used to control scheduling importance and if this job
	// can preempt other jobs.
	Priority int

	// Type is used to control which schedulers are available to handle
	// this evaluation.
	Type string

	// TriggeredBy is used to give some insight into why this Eval
	// was created. (Job change, node failure, alloc failure, etc).
	TriggeredBy string

	// JobID is the job this evaluation is scoped to. Evaluations cannot
	// be run in parallel for a given JobID, so we serialize on this.
	JobID string

	// JobModifyIndex is the modify index of the job at the time
	// the evaluation was created
	JobModifyIndex uint64

	// NodeID is the node that was affected triggering the evaluation.
	NodeID string

	// NodeModifyIndex is the modify index of the node at the time
	// the evaluation was created
	NodeModifyIndex uint64

	// DeploymentID is the ID of the deployment that triggered the evaluation.
	DeploymentID string

	// Status of the evaluation
	Status string

	// StatusDescription is meant to provide more human useful information
	StatusDescription string

	// Wait is a minimum wait time for running the eval. This is used to
	// support a rolling upgrade in versions prior to 0.7.0
	// Deprecated
	Wait time.Duration

	// WaitUntil is the time when this eval should be run. This is used to
	// supported delayed rescheduling of failed allocations, and delayed
	// stopping of allocations that are configured with max_client_disconnect.
	WaitUntil time.Time

	// NextEval is the evaluation ID for the eval created to do a followup.
	// This is used to support rolling upgrades and failed-follow-up evals, where
	// we need a chain of evaluations.
	NextEval string

	// PreviousEval is the evaluation ID for the eval creating this one to do a followup.
	// This is used to support rolling upgrades and failed-follow-up evals, where
	// we need a chain of evaluations.
	PreviousEval string

	// BlockedEval is the evaluation ID for a created blocked eval. A
	// blocked eval will be created if all allocations could not be placed due
	// to constraints or lacking resources.
	BlockedEval string

	// RelatedEvals is a list of all the evaluations that are related (next,
	// previous, or blocked) to this one. It may be nil if not requested.
	RelatedEvals []*EvaluationStub

	// FailedTGAllocs are task groups which have allocations that could not be
	// made, but the metrics are persisted so that the user can use the feedback
	// to determine the cause.
	FailedTGAllocs map[string]*AllocMetric

	// PlanAnnotations represents the output of the reconciliation step.
	PlanAnnotations *PlanAnnotations

	// ClassEligibility tracks computed node classes that have been explicitly
	// marked as eligible or ineligible.
	ClassEligibility map[string]bool

	// QuotaLimitReached marks whether a quota limit was reached for the
	// evaluation.
	QuotaLimitReached string

	// EscapedComputedClass marks whether the job has constraints that are not
	// captured by computed node classes.
	EscapedComputedClass bool

	// AnnotatePlan triggers the scheduler to provide additional annotations
	// during the evaluation. This should not be set during normal operations.
	AnnotatePlan bool

	// QueuedAllocations is the number of unplaced allocations at the time the
	// evaluation was processed. The map is keyed by Task Group names.
	QueuedAllocations map[string]int

	// LeaderACL provides the ACL token to when issuing RPCs back to the
	// leader. This will be a valid management token as long as the leader is
	// active. This should not ever be exposed via the API.
	LeaderACL string

	// SnapshotIndex is the Raft index of the snapshot used to process the
	// evaluation. The index will either be set when it has gone through the
	// scheduler or if a blocked evaluation is being created. The index is set
	// in this case so we can determine if an early unblocking is required since
	// capacity has changed since the evaluation was created. This can result in
	// the SnapshotIndex being less than the CreateIndex.
	SnapshotIndex uint64

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64

	// Creation and modification times stored as UnixNano
	CreateTime int64
	ModifyTime int64
}

type EvaluationStub struct {
	ID                string
	Namespace         string
	Priority          int
	Type              string
	TriggeredBy       string
	JobID             string
	NodeID            string
	DeploymentID      string
	Status            string
	StatusDescription string
	WaitUntil         time.Time
	NextEval          string
	PreviousEval      string
	BlockedEval       string
	CreateIndex       uint64
	ModifyIndex       uint64
	CreateTime        int64
	ModifyTime        int64
}

// GetID implements the IDGetter interface, required for pagination.
func (e *Evaluation) GetID() string {
	if e == nil {
		return ""
	}
	return e.ID
}

// GetNamespace implements the NamespaceGetter interface, required for pagination.
func (e *Evaluation) GetNamespace() string {
	if e == nil {
		return ""
	}
	return e.Namespace
}

// GetCreateIndex implements the CreateIndexGetter interface, required for
// pagination.
func (e *Evaluation) GetCreateIndex() uint64 {
	if e == nil {
		return 0
	}
	return e.CreateIndex
}

// TerminalStatus returns if the current status is terminal and
// will no longer transition.
func (e *Evaluation) TerminalStatus() bool {
	switch e.Status {
	case EvalStatusComplete, EvalStatusFailed, EvalStatusCancelled:
		return true
	default:
		return false
	}
}

func (e *Evaluation) GoString() string {
	return fmt.Sprintf("<Eval %q JobID: %q Namespace: %q>", e.ID, e.JobID, e.Namespace)
}

func (e *Evaluation) RelatedIDs() []string {
	if e == nil {
		return nil
	}

	ids := []string{e.NextEval, e.PreviousEval, e.BlockedEval}
	related := make([]string, 0, len(ids))

	for _, id := range ids {
		if id != "" {
			related = append(related, id)
		}
	}

	return related
}

func (e *Evaluation) Stub() *EvaluationStub {
	if e == nil {
		return nil
	}

	return &EvaluationStub{
		ID:                e.ID,
		Namespace:         e.Namespace,
		Priority:          e.Priority,
		Type:              e.Type,
		TriggeredBy:       e.TriggeredBy,
		JobID:             e.JobID,
		NodeID:            e.NodeID,
		DeploymentID:      e.DeploymentID,
		Status:            e.Status,
		StatusDescription: e.StatusDescription,
		WaitUntil:         e.WaitUntil,
		NextEval:          e.NextEval,
		PreviousEval:      e.PreviousEval,
		BlockedEval:       e.BlockedEval,
		CreateIndex:       e.CreateIndex,
		ModifyIndex:       e.ModifyIndex,
		CreateTime:        e.CreateTime,
		ModifyTime:        e.ModifyTime,
	}
}

func (e *Evaluation) Copy() *Evaluation {
	if e == nil {
		return nil
	}
	ne := new(Evaluation)
	*ne = *e

	// Copy ClassEligibility
	if e.ClassEligibility != nil {
		classes := make(map[string]bool, len(e.ClassEligibility))
		for class, elig := range e.ClassEligibility {
			classes[class] = elig
		}
		ne.ClassEligibility = classes
	}

	// Copy FailedTGAllocs
	if e.FailedTGAllocs != nil {
		failedTGs := make(map[string]*AllocMetric, len(e.FailedTGAllocs))
		for tg, metric := range e.FailedTGAllocs {
			failedTGs[tg] = metric.Copy()
		}
		ne.FailedTGAllocs = failedTGs
	}

	// Copy queued allocations
	if e.QueuedAllocations != nil {
		queuedAllocations := make(map[string]int, len(e.QueuedAllocations))
		for tg, num := range e.QueuedAllocations {
			queuedAllocations[tg] = num
		}
		ne.QueuedAllocations = queuedAllocations
	}

	return ne
}

// ShouldEnqueue checks if a given evaluation should be enqueued into the
// eval_broker
func (e *Evaluation) ShouldEnqueue() bool {
	switch e.Status {
	case EvalStatusPending:
		return true
	case EvalStatusComplete, EvalStatusFailed, EvalStatusBlocked, EvalStatusCancelled:
		return false
	default:
		panic(fmt.Sprintf("unhandled evaluation (%s) status %s", e.ID, e.Status))
	}
}

// ShouldBlock checks if a given evaluation should be entered into the blocked
// eval tracker.
func (e *Evaluation) ShouldBlock() bool {
	switch e.Status {
	case EvalStatusBlocked:
		return true
	case EvalStatusComplete, EvalStatusFailed, EvalStatusPending, EvalStatusCancelled:
		return false
	default:
		panic(fmt.Sprintf("unhandled evaluation (%s) status %s", e.ID, e.Status))
	}
}

// MakePlan is used to make a plan from the given evaluation
// for a given Job
func (e *Evaluation) MakePlan(j *Job) *Plan {
	p := &Plan{
		EvalID:   e.ID,
		Priority: e.Priority,
		JobTuple: &PlanJobTuple{
			Namespace: j.Namespace,
			ID:        j.ID,
			Version:   j.Version,
		},
		NodeUpdate:      make(map[string][]*Allocation),
		NodeAllocation:  make(map[string][]*Allocation),
		NodePreemptions: make(map[string][]*Allocation),
	}
	if j != nil {
		p.AllAtOnce = j.AllAtOnce
	}
	return p
}

// NextRollingEval creates an evaluation to followup this eval for rolling updates
func (e *Evaluation) NextRollingEval(wait time.Duration) *Evaluation {
	now := time.Now().UTC().UnixNano()
	return &Evaluation{
		ID:             uuid.Generate(),
		Namespace:      e.Namespace,
		Priority:       e.Priority,
		Type:           e.Type,
		TriggeredBy:    EvalTriggerRollingUpdate,
		JobID:          e.JobID,
		JobModifyIndex: e.JobModifyIndex,
		Status:         EvalStatusPending,
		Wait:           wait,
		PreviousEval:   e.ID,
		CreateTime:     now,
		ModifyTime:     now,
	}
}

// CreateBlockedEval creates a blocked evaluation to followup this eval to place any
// failed allocations. It takes the classes marked explicitly eligible or
// ineligible, whether the job has escaped computed node classes and whether the
// quota limit was reached.
func (e *Evaluation) CreateBlockedEval(classEligibility map[string]bool,
	escaped bool, quotaReached string, failedTGAllocs map[string]*AllocMetric) *Evaluation {
	now := time.Now().UTC().UnixNano()
	return &Evaluation{
		ID:                   uuid.Generate(),
		Namespace:            e.Namespace,
		Priority:             e.Priority,
		Type:                 e.Type,
		TriggeredBy:          EvalTriggerQueuedAllocs,
		JobID:                e.JobID,
		JobModifyIndex:       e.JobModifyIndex,
		Status:               EvalStatusBlocked,
		PreviousEval:         e.ID,
		FailedTGAllocs:       failedTGAllocs,
		ClassEligibility:     classEligibility,
		EscapedComputedClass: escaped,
		QuotaLimitReached:    quotaReached,
		CreateTime:           now,
		ModifyTime:           now,
	}
}

// CreateFailedFollowUpEval creates a follow up evaluation when the current one
// has been marked as failed because it has hit the delivery limit and will not
// be retried by the eval_broker. Callers should copy the created eval's ID to
// into the old eval's NextEval field.
func (e *Evaluation) CreateFailedFollowUpEval(wait time.Duration) *Evaluation {
	now := time.Now().UTC().UnixNano()
	return &Evaluation{
		ID:             uuid.Generate(),
		Namespace:      e.Namespace,
		Priority:       e.Priority,
		Type:           e.Type,
		TriggeredBy:    EvalTriggerFailedFollowUp,
		JobID:          e.JobID,
		JobModifyIndex: e.JobModifyIndex,
		Status:         EvalStatusPending,
		Wait:           wait,
		PreviousEval:   e.ID,
		CreateTime:     now,
		ModifyTime:     now,
	}
}

// UpdateModifyTime takes into account that clocks on different servers may be
// slightly out of sync. Even in case of a leader change, this method will
// guarantee that ModifyTime will always be after CreateTime.
func (e *Evaluation) UpdateModifyTime() {
	now := time.Now().UTC().UnixNano()
	if now <= e.CreateTime {
		e.ModifyTime = e.CreateTime + 1
	} else {
		e.ModifyTime = now
	}
}

// EvalDeleteRequest is the request object used when operators are manually
// deleting evaluations. The number of evaluation IDs within the request must
// not be greater than MaxEvalIDsPerDeleteRequest.
type EvalDeleteRequest struct {
	EvalIDs []string

	// Filter specifies the go-bexpr filter expression to be used for deleting a
	// set of evaluations that matches the filter
	Filter string

	WriteRequest
}

// EvalDeleteResponse is the response object when one or more evaluation are
// deleted manually by an operator.
type EvalDeleteResponse struct {
	Count int // how many Evaluations were safe to delete and/or matched the filter
	WriteMeta
}
