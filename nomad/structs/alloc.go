// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"container/heap"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/lib/kheap"
)

const (
	AllocDesiredStatusRun   = "run"   // Allocation should run
	AllocDesiredStatusStop  = "stop"  // Allocation should stop
	AllocDesiredStatusEvict = "evict" // Allocation should stop, and was evicted
)

const (
	AllocClientStatusPending  = "pending"
	AllocClientStatusRunning  = "running"
	AllocClientStatusComplete = "complete"
	AllocClientStatusFailed   = "failed"
	AllocClientStatusLost     = "lost"
	AllocClientStatusUnknown  = "unknown"
)

// terminalAllocationStatuses lists allocation statutes that we consider
// terminal
var terminalAllocationStatuses = []string{
	AllocClientStatusComplete,
	AllocClientStatusFailed,
	AllocClientStatusLost,
}

// Allocation is used to allocate the placement of a task group to a node.
type Allocation struct {
	// msgpack omit empty fields during serialization
	_struct bool `codec:",omitempty"` // nolint: structcheck

	// ID of the allocation (UUID)
	ID string

	// Namespace is the namespace the allocation is created in
	Namespace string

	// ID of the evaluation that generated this allocation
	EvalID string

	// Name is a logical name of the allocation.
	Name string

	// NodeID is the node this is being placed on
	NodeID string

	// NodeName is the name of the node this is being placed on.
	NodeName string

	// Job is the parent job of the task group being allocated.
	// This is copied at allocation time to avoid issues if the job
	// definition is updated.
	JobID string
	Job   *Job

	// TaskGroup is the name of the task group that should be run
	TaskGroup string

	// COMPAT(0.11): Remove in 0.11
	// Resources is the total set of resources allocated as part
	// of this allocation of the task group. Dynamic ports will be set by
	// the scheduler.
	Resources *Resources

	// SharedResources are the resources that are shared by all the tasks in an
	// allocation
	// Deprecated: use AllocatedResources.Shared instead.
	// Keep field to allow us to handle upgrade paths from old versions
	SharedResources *Resources

	// TaskResources is the set of resources allocated to each
	// task. These should sum to the total Resources. Dynamic ports will be
	// set by the scheduler.
	// Deprecated: use AllocatedResources.Tasks instead.
	// Keep field to allow us to handle upgrade paths from old versions
	TaskResources map[string]*Resources

	// AllocatedResources is the total resources allocated for the task group.
	AllocatedResources *AllocatedResources

	// Metrics associated with this allocation
	Metrics *AllocMetric

	// Desired Status of the allocation on the client
	DesiredStatus string

	// DesiredStatusDescription is meant to provide more human useful information
	DesiredDescription string

	// DesiredTransition is used to indicate that a state transition
	// is desired for a given reason.
	DesiredTransition DesiredTransition

	// Status of the allocation on the client
	ClientStatus string

	// ClientStatusDescription is meant to provide more human useful information
	ClientDescription string

	// TaskStates stores the state of each task,
	TaskStates map[string]*TaskState

	// AllocStates track meta data associated with changes to the state of the whole allocation, like becoming lost
	AllocStates []*AllocState

	// PreviousAllocation is the allocation that this allocation is replacing
	PreviousAllocation string

	// NextAllocation is the allocation that this allocation is being replaced by
	NextAllocation string

	// DeploymentID identifies an allocation as being created from a
	// particular deployment
	DeploymentID string

	// DeploymentStatus captures the status of the allocation as part of the
	// given deployment
	DeploymentStatus *AllocDeploymentStatus

	// RescheduleTrackers captures details of previous reschedule attempts of the allocation
	RescheduleTracker *RescheduleTracker

	// NetworkStatus captures networking details of an allocation known at runtime
	NetworkStatus *AllocNetworkStatus

	// FollowupEvalID captures a follow up evaluation created to handle a failed allocation
	// that can be rescheduled in the future
	FollowupEvalID string

	// PreemptedAllocations captures IDs of any allocations that were preempted
	// in order to place this allocation
	PreemptedAllocations []string

	// PreemptedByAllocation tracks the alloc ID of the allocation that caused this allocation
	// to stop running because it got preempted
	PreemptedByAllocation string

	// SignedIdentities is a map of task names to signed identity/capability
	// claim tokens for those tasks. If needed, it is populated in the plan
	// applier.
	SignedIdentities map[string]string `json:"-"`

	// SigningKeyID is the key used to sign the SignedIdentities field.
	SigningKeyID string

	// Raft Indexes
	CreateIndex uint64
	ModifyIndex uint64

	// AllocModifyIndex is not updated when the client updates allocations. This
	// lets the client pull only the allocs updated by the server.
	AllocModifyIndex uint64

	// CreateTime is the time the allocation has finished scheduling and been
	// verified by the plan applier, stored as UnixNano.
	CreateTime int64

	// ModifyTime is the time the allocation was last updated stored as UnixNano.
	ModifyTime int64
}

// GetID implements the IDGetter interface, required for pagination.
func (a *Allocation) GetID() string {
	if a == nil {
		return ""
	}
	return a.ID
}

// Sanitize returns a copy of the allocation with the SignedIdentities field
// removed. This is useful for returning allocations to clients where the
// SignedIdentities field is not needed.
func (a *Allocation) Sanitize() *Allocation {
	if a == nil {
		return nil
	}

	if a.SignedIdentities == nil {
		return a
	}

	clean := a.Copy()
	clean.SignedIdentities = nil
	return clean
}

// GetNamespace implements the NamespaceGetter interface, required for
// pagination and filtering namespaces in endpoints that support glob namespace
// requests using tokens with limited access.
func (a *Allocation) GetNamespace() string {
	if a == nil {
		return ""
	}
	return a.Namespace
}

// GetCreateIndex implements the CreateIndexGetter interface, required for
// pagination.
func (a *Allocation) GetCreateIndex() uint64 {
	if a == nil {
		return 0
	}
	return a.CreateIndex
}

// ReservedCores returns the union of reserved cores across tasks in this alloc.
func (a *Allocation) ReservedCores() *idset.Set[hw.CoreID] {
	s := idset.Empty[hw.CoreID]()
	if a == nil || a.AllocatedResources == nil {
		return s
	}
	for _, taskResources := range a.AllocatedResources.Tasks {
		if len(taskResources.Cpu.ReservedCores) > 0 {
			for _, core := range taskResources.Cpu.ReservedCores {
				s.Insert(hw.CoreID(core))
			}
		}
	}
	return s
}

// ConsulNamespace returns the Consul namespace of the task group associated
// with this allocation.
func (a *Allocation) ConsulNamespace() string {
	return a.Job.LookupTaskGroup(a.TaskGroup).Consul.GetNamespace()
}

func (a *Allocation) ConsulNamespaceForTask(taskName string) string {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	task := tg.LookupTask(taskName)
	if task.Consul != nil {
		return task.Consul.GetNamespace()
	}

	return tg.Consul.GetNamespace()
}

func (a *Allocation) JobNamespacedID() NamespacedID {
	return NewNamespacedID(a.JobID, a.Namespace)
}

// Index returns the index of the allocation. If the allocation is from a task
// group with count greater than 1, there will be multiple allocations for it.
func (a *Allocation) Index() uint {
	return AllocIndexFromName(a.Name, a.JobID, a.TaskGroup)
}

// AllocIndexFromName returns the index of an allocation given its name, the
// jobID and the task group name.
func AllocIndexFromName(allocName, jobID, taskGroup string) uint {
	l := len(allocName)
	prefix := len(jobID) + len(taskGroup) + 2
	if l <= 3 || l <= prefix {
		return uint(0)
	}

	strNum := allocName[prefix : len(allocName)-1]
	num, _ := strconv.Atoi(strNum)
	return uint(num)
}

// Copy provides a copy of the allocation and deep copies the job
func (a *Allocation) Copy() *Allocation {
	return a.copyImpl(true)
}

// CopySkipJob provides a copy of the allocation but doesn't deep copy the job
func (a *Allocation) CopySkipJob() *Allocation {
	return a.copyImpl(false)
}

// Canonicalize Allocation to ensure fields are initialized to the expectations
// of this version of Nomad. Should be called when restoring persisted
// Allocations or receiving Allocations from Nomad agents potentially on an
// older version of Nomad.
func (a *Allocation) Canonicalize() {
	if a.AllocatedResources == nil && a.TaskResources != nil {
		ar := AllocatedResources{}

		tasks := make(map[string]*AllocatedTaskResources, len(a.TaskResources))
		for name, tr := range a.TaskResources {
			atr := AllocatedTaskResources{}
			atr.Cpu.CpuShares = int64(tr.CPU)
			atr.Memory.MemoryMB = int64(tr.MemoryMB)
			atr.Networks = tr.Networks.Copy()

			tasks[name] = &atr
		}
		ar.Tasks = tasks

		if a.SharedResources != nil {
			ar.Shared.DiskMB = int64(a.SharedResources.DiskMB)
			ar.Shared.Networks = a.SharedResources.Networks.Copy()
		}

		a.AllocatedResources = &ar
	}

	a.Job.Canonicalize()
}

func (a *Allocation) copyImpl(job bool) *Allocation {
	if a == nil {
		return nil
	}
	na := new(Allocation)
	*na = *a

	if job {
		na.Job = na.Job.Copy()
	}

	na.AllocatedResources = na.AllocatedResources.Copy()
	na.Resources = na.Resources.Copy()
	na.SharedResources = na.SharedResources.Copy()

	if a.TaskResources != nil {
		tr := make(map[string]*Resources, len(na.TaskResources))
		for task, resource := range na.TaskResources {
			tr[task] = resource.Copy()
		}
		na.TaskResources = tr
	}

	na.Metrics = na.Metrics.Copy()
	na.DeploymentStatus = na.DeploymentStatus.Copy()

	if a.TaskStates != nil {
		ts := make(map[string]*TaskState, len(na.TaskStates))
		for task, state := range na.TaskStates {
			ts[task] = state.Copy()
		}
		na.TaskStates = ts
	}

	na.RescheduleTracker = a.RescheduleTracker.Copy()
	na.PreemptedAllocations = slices.Clone(a.PreemptedAllocations)
	return na
}

// TerminalStatus returns if the desired or actual status is terminal and
// will no longer transition.
func (a *Allocation) TerminalStatus() bool {
	// First check the desired state and if that isn't terminal, check client
	// state.
	return a.ServerTerminalStatus() || a.ClientTerminalStatus()
}

// ServerTerminalStatus returns true if the desired state of the allocation is terminal
func (a *Allocation) ServerTerminalStatus() bool {
	switch a.DesiredStatus {
	case AllocDesiredStatusStop, AllocDesiredStatusEvict:
		return true
	default:
		return false
	}
}

// ClientTerminalStatus returns if the client status is terminal and will no longer transition
func (a *Allocation) ClientTerminalStatus() bool {
	return slices.Contains(terminalAllocationStatuses, a.ClientStatus)
}

// ShouldReschedule returns if the allocation is eligible to be rescheduled according
// to its status and ReschedulePolicy given its failure time
func (a *Allocation) ShouldReschedule(reschedulePolicy *ReschedulePolicy, failTime time.Time) bool {
	// First check the desired state
	switch a.DesiredStatus {
	case AllocDesiredStatusStop, AllocDesiredStatusEvict:
		return false
	default:
	}
	switch a.ClientStatus {
	case AllocClientStatusFailed:
		return a.RescheduleEligible(reschedulePolicy, failTime)
	default:
		return false
	}
}

// RescheduleEligible returns if the allocation is eligible to be rescheduled according
// to its ReschedulePolicy and the current state of its reschedule trackers
func (a *Allocation) RescheduleEligible(reschedulePolicy *ReschedulePolicy, failTime time.Time) bool {
	return a.RescheduleTracker.RescheduleEligible(reschedulePolicy, failTime)
}

func (a *Allocation) RescheduleInfo() (int, int) {
	return a.RescheduleTracker.rescheduleInfo(a.ReschedulePolicy(), a.LastEventTime())
}

// LastEventTime is the time of the last task event in the allocation.
// It is used to determine allocation failure time. If the FinishedAt field
// is not set, the alloc's modify time is used
func (a *Allocation) LastEventTime() time.Time {
	var lastEventTime time.Time
	if a.TaskStates != nil {
		for _, s := range a.TaskStates {
			if lastEventTime.IsZero() || s.FinishedAt.After(lastEventTime) {
				lastEventTime = s.FinishedAt
			}
		}
	}

	if lastEventTime.IsZero() {
		return time.Unix(0, a.ModifyTime).UTC()
	}
	return lastEventTime
}

// ReschedulePolicy returns the reschedule policy based on the task group
func (a *Allocation) ReschedulePolicy() *ReschedulePolicy {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	if tg == nil {
		return nil
	}
	return tg.ReschedulePolicy
}

// MigrateStrategy returns the migrate strategy based on the task group
func (a *Allocation) MigrateStrategy() *MigrateStrategy {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	if tg == nil {
		return nil
	}
	return tg.Migrate
}

// NextRescheduleTime returns a time on or after which the allocation is eligible to be rescheduled,
// and whether the next reschedule time is within policy's interval if the policy doesn't allow unlimited reschedules
func (a *Allocation) NextRescheduleTime() (time.Time, bool) {
	failTime := a.LastEventTime()
	reschedulePolicy := a.ReschedulePolicy()
	isRescheduledBatch := a.Job.Type == JobTypeBatch && a.DesiredTransition.ShouldReschedule()

	// If reschedule is disabled, return early
	if reschedulePolicy == nil || (reschedulePolicy.Attempts == 0 && !reschedulePolicy.Unlimited) {
		return time.Time{}, false
	}

	if (a.DesiredStatus == AllocDesiredStatusStop && !a.LastRescheduleFailed()) ||
		(!isRescheduledBatch && a.ClientStatus != AllocClientStatusFailed && a.ClientStatus != AllocClientStatusLost) ||
		failTime.IsZero() {
		return time.Time{}, false
	}

	return a.nextRescheduleTime(failTime, reschedulePolicy)
}

func (a *Allocation) nextRescheduleTime(failTime time.Time, reschedulePolicy *ReschedulePolicy) (time.Time, bool) {
	nextDelay := a.NextDelay()
	nextRescheduleTime := failTime.Add(nextDelay)
	rescheduleEligible := reschedulePolicy.Unlimited || (reschedulePolicy.Attempts > 0 && a.RescheduleTracker == nil)
	if reschedulePolicy.Attempts > 0 && a.RescheduleTracker != nil && a.RescheduleTracker.Events != nil {
		// Check for eligibility based on the interval if max attempts is set
		attempted, attempts := a.RescheduleTracker.rescheduleInfo(reschedulePolicy, failTime)
		rescheduleEligible = attempted < attempts && nextDelay < reschedulePolicy.Interval
	}

	return nextRescheduleTime, rescheduleEligible
}

// NextRescheduleTimeByTime works like NextRescheduleTime but allows callers
// specify a failure time. Useful for things like determining whether to reschedule
// an alloc on a disconnected node.
func (a *Allocation) NextRescheduleTimeByTime(t time.Time) (time.Time, bool) {
	reschedulePolicy := a.ReschedulePolicy()
	if reschedulePolicy == nil {
		return time.Time{}, false
	}

	return a.nextRescheduleTime(t, reschedulePolicy)
}

// ShouldClientStop tests an alloc for StopAfterClient on the Disconnect configuration
func (a *Allocation) ShouldClientStop() bool {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	timeout := tg.GetDisconnectStopTimeout()

	if tg == nil ||
		timeout == nil ||
		*timeout == 0*time.Nanosecond {
		return false
	}
	return true
}

// WaitClientStop uses the reschedule delay mechanism to block rescheduling until
// disconnect.stop_on_client_after's interval passes
func (a *Allocation) WaitClientStop() time.Time {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)

	// An alloc can only be marked lost once, so use the first lost transition
	var t time.Time
	for _, s := range a.AllocStates {
		if s.Field == AllocStateFieldClientStatus &&
			s.Value == AllocClientStatusLost {
			t = s.Time
			break
		}
	}

	// On the first pass, the alloc hasn't been marked lost yet, and so we start
	// counting from now
	if t.IsZero() {
		t = time.Now().UTC()
	}

	// Find the max kill timeout
	kill := DefaultKillTimeout
	for _, t := range tg.Tasks {
		if t.KillTimeout > kill {
			kill = t.KillTimeout
		}
	}

	return t.Add(*tg.GetDisconnectStopTimeout() + kill)
}

// DisconnectTimeout uses the Disconnect.LostAfter to compute when the allocation
// should transition to lost.
func (a *Allocation) DisconnectTimeout(now time.Time) time.Time {
	if a == nil || a.Job == nil {
		return now
	}

	tg := a.Job.LookupTaskGroup(a.TaskGroup)

	timeout := tg.GetDisconnectLostAfter()
	if timeout == 0 {
		return now
	}

	return now.Add(timeout)
}

// SupportsDisconnectedClients determines whether both the server and the task group
// are configured to allow the allocation to reconnect after network connectivity
// has been lost and then restored.
func (a *Allocation) SupportsDisconnectedClients(serverSupportsDisconnectedClients bool) bool {
	if !serverSupportsDisconnectedClients {
		return false
	}

	if a.Job != nil {
		tg := a.Job.LookupTaskGroup(a.TaskGroup)
		if tg != nil {
			return tg.GetDisconnectLostAfter() != 0
		}
	}

	return false
}

// ReplaceOnDisconnect determines if an alloc can be replaced
// when Disconnected.
func (a *Allocation) ReplaceOnDisconnect() bool {
	if a.Job != nil {
		tg := a.Job.LookupTaskGroup(a.TaskGroup)
		if tg != nil {
			return tg.Replace()
		}
	}

	return true
}

// NextDelay returns a duration after which the allocation can be rescheduled.
// It is calculated according to the delay function and previous reschedule attempts.
func (a *Allocation) NextDelay() time.Duration {
	policy := a.ReschedulePolicy()
	// Can be nil if the task group was updated to remove its reschedule policy
	if policy == nil {
		return 0
	}
	delayDur := policy.Delay
	if a.RescheduleTracker == nil || a.RescheduleTracker.Events == nil || len(a.RescheduleTracker.Events) == 0 {
		return delayDur
	}
	events := a.RescheduleTracker.Events
	switch policy.DelayFunction {
	case "exponential":
		delayDur = a.RescheduleTracker.Events[len(a.RescheduleTracker.Events)-1].Delay * 2
	case "fibonacci":
		if len(events) >= 2 {
			fibN1Delay := events[len(events)-1].Delay
			fibN2Delay := events[len(events)-2].Delay
			// Handle reset of delay ceiling which should cause
			// a new series to start
			if fibN2Delay == policy.MaxDelay && fibN1Delay == policy.Delay {
				delayDur = fibN1Delay
			} else {
				delayDur = fibN1Delay + fibN2Delay
			}
		}
	default:
		return delayDur
	}
	if policy.MaxDelay > 0 && delayDur > policy.MaxDelay {
		delayDur = policy.MaxDelay
		// check if delay needs to be reset

		lastRescheduleEvent := a.RescheduleTracker.Events[len(a.RescheduleTracker.Events)-1]
		timeDiff := a.LastEventTime().UTC().UnixNano() - lastRescheduleEvent.RescheduleTime
		if timeDiff > delayDur.Nanoseconds() {
			delayDur = policy.Delay
		}

	}

	return delayDur
}

// Terminated returns if the allocation is in a terminal state on a client.
func (a *Allocation) Terminated() bool {
	if a.ClientStatus == AllocClientStatusFailed ||
		a.ClientStatus == AllocClientStatusComplete ||
		a.ClientStatus == AllocClientStatusLost {
		return true
	}
	return false
}

// SetStop updates the allocation in place to a DesiredStatus stop, with the ClientStatus
func (a *Allocation) SetStop(clientStatus, clientDesc string) {
	a.DesiredStatus = AllocDesiredStatusStop
	a.ClientStatus = clientStatus
	a.ClientDescription = clientDesc
	a.AppendState(AllocStateFieldClientStatus, clientStatus)
}

// AppendState creates and appends an AllocState entry recording the time of the state
// transition. Used to mark the transition to lost
func (a *Allocation) AppendState(field AllocStateField, value string) {
	a.AllocStates = append(a.AllocStates, &AllocState{
		Field: field,
		Value: value,
		Time:  time.Now().UTC(),
	})
}

// RanSuccessfully returns whether the client has ran the allocation and all
// tasks finished successfully. Critically this function returns whether the
// allocation has ran to completion and not just that the alloc has converged to
// its desired state. That is to say that a batch allocation must have finished
// with exit code 0 on all task groups. This doesn't really have meaning on a
// non-batch allocation because a service and system allocation should not
// finish.
func (a *Allocation) RanSuccessfully() bool {
	// Handle the case the client hasn't started the allocation.
	if len(a.TaskStates) == 0 {
		return false
	}

	// Check to see if all the tasks finished successfully in the allocation
	allSuccess := true
	for _, state := range a.TaskStates {
		allSuccess = allSuccess && state.Successful()
	}

	return allSuccess
}

// ShouldMigrate returns if the allocation needs data migration
func (a *Allocation) ShouldMigrate() bool {
	if a.PreviousAllocation == "" {
		return false
	}

	if a.DesiredStatus == AllocDesiredStatusStop || a.DesiredStatus == AllocDesiredStatusEvict {
		return false
	}

	tg := a.Job.LookupTaskGroup(a.TaskGroup)

	// if the task group is nil or the ephemeral disk block isn't present then
	// we won't migrate
	if tg == nil || tg.EphemeralDisk == nil {
		return false
	}

	// We won't migrate any data if the user hasn't enabled migration
	return tg.EphemeralDisk.Migrate
}

// SetEventDisplayMessages populates the display message if its not already set,
// a temporary fix to handle old allocations that don't have it.
// This method will be removed in a future release.
func (a *Allocation) SetEventDisplayMessages() {
	setDisplayMsg(a.TaskStates)
}

// LookupTask by name from the Allocation. Returns nil if the Job is not set, the
// TaskGroup does not exist, or the task name cannot be found.
func (a *Allocation) LookupTask(name string) *Task {
	if a.Job == nil {
		return nil
	}

	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	if tg == nil {
		return nil
	}

	return tg.LookupTask(name)
}

// Stub returns a list stub for the allocation
func (a *Allocation) Stub(fields *AllocStubFields) *AllocListStub {
	s := &AllocListStub{
		ID:                    a.ID,
		EvalID:                a.EvalID,
		Name:                  a.Name,
		Namespace:             a.Namespace,
		NodeID:                a.NodeID,
		NodeName:              a.NodeName,
		JobID:                 a.JobID,
		JobType:               a.Job.Type,
		JobVersion:            a.Job.Version,
		TaskGroup:             a.TaskGroup,
		DesiredStatus:         a.DesiredStatus,
		DesiredDescription:    a.DesiredDescription,
		ClientStatus:          a.ClientStatus,
		ClientDescription:     a.ClientDescription,
		DesiredTransition:     a.DesiredTransition,
		TaskStates:            a.TaskStates,
		DeploymentStatus:      a.DeploymentStatus,
		FollowupEvalID:        a.FollowupEvalID,
		NextAllocation:        a.NextAllocation,
		RescheduleTracker:     a.RescheduleTracker,
		PreemptedAllocations:  a.PreemptedAllocations,
		PreemptedByAllocation: a.PreemptedByAllocation,
		CreateIndex:           a.CreateIndex,
		ModifyIndex:           a.ModifyIndex,
		CreateTime:            a.CreateTime,
		ModifyTime:            a.ModifyTime,
	}

	if fields != nil {
		if fields.Resources {
			s.AllocatedResources = a.AllocatedResources
		}
		if !fields.TaskStates {
			s.TaskStates = nil
		}
	}

	return s
}

// AllocationDiff converts an Allocation type to an AllocationDiff type
// If at any time, modification are made to AllocationDiff so that an
// Allocation can no longer be safely converted to AllocationDiff,
// this method should be changed accordingly.
func (a *Allocation) AllocationDiff() *AllocationDiff {
	return (*AllocationDiff)(a)
}

// Expired determines whether an allocation has exceeded its Disconnect.LostAfter
// duration relative to the passed time stamp.
func (a *Allocation) Expired(now time.Time) bool {
	if a == nil || a.Job == nil {
		return false
	}

	// If alloc is not Unknown it cannot be expired.
	if a.ClientStatus != AllocClientStatusUnknown {
		return false
	}

	lastUnknown := a.LastUnknown()
	if lastUnknown.IsZero() {
		return false
	}

	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	if tg == nil {
		return false
	}

	expiry := lastUnknown.Add(tg.GetDisconnectLostAfter())
	return expiry.Sub(now) <= 0
}

// LastUnknown returns the timestamp for the last time the allocation
// transitioned into the unknown client status.
func (a *Allocation) LastUnknown() time.Time {
	var lastUnknown time.Time
	foundUnknown := false

	// Traverse backwards
	for i := len(a.AllocStates) - 1; i >= 0; i-- {
		s := a.AllocStates[i]
		if s.Field == AllocStateFieldClientStatus {
			if s.Value == AllocClientStatusUnknown {
				lastUnknown = s.Time
				foundUnknown = true
			} else if foundUnknown {
				// We found the transition from non-unknown to unknown
				break
			}
		}
	}

	return lastUnknown.UTC()
}

// NeedsToReconnect returns true if the last known ClientStatus value is
// "unknown" and so the allocation did not reconnect yet.
func (a *Allocation) NeedsToReconnect() bool {
	disconnected := false

	// AllocStates are appended to the list and we only need the latest
	// ClientStatus transition, so traverse from the end until we find one.
	for i := len(a.AllocStates) - 1; i >= 0; i-- {
		s := a.AllocStates[i]
		if s.Field != AllocStateFieldClientStatus {
			continue
		}

		disconnected = s.Value == AllocClientStatusUnknown
		break
	}

	return disconnected
}

// FollowupEvalForReconnect returns the ID of the allocation's follow-up eval if
// the allocation is waiting to reconnect and the clientUpdate indicates that
// the client has reconnected.
func (a *Allocation) FollowupEvalForReconnect(clientUpdate *Allocation) (string, bool) {
	if !a.NeedsToReconnect() || a.FollowupEvalID == "" {
		return "", false
	}

	switch clientUpdate.ClientStatus {
	case AllocClientStatusRunning, AllocClientStatusComplete, AllocClientStatusFailed:
		return a.FollowupEvalID, true
	}

	return "", false
}

// LastStartOfTask returns the time of the last start event for the given task
// using the allocations TaskStates. If the task has not started, the zero time
// will be returned.
func (a *Allocation) LastStartOfTask(taskName string) time.Time {
	task := a.TaskStates[taskName]
	if task == nil {
		return time.Time{}
	}

	if task.Restarts > 0 {
		return task.LastRestart
	}

	return task.StartedAt
}

// HasAnyPausedTasks returns true if any of the TaskStates on the alloc
// are Paused (Enterprise feature) either due to a schedule or being forced.
func (a *Allocation) HasAnyPausedTasks() bool {
	if a == nil {
		return false
	}
	for _, ts := range a.TaskStates {
		if ts == nil {
			continue
		}
		if ts.Paused.Stop() {
			return true
		}
	}
	return false
}

// LastRescheduleFailed returns whether the scheduler previously attempted to
// reschedule this allocation but failed to find a placement
func (a *Allocation) LastRescheduleFailed() bool {
	if a.RescheduleTracker == nil {
		return false
	}
	return a.RescheduleTracker.LastReschedule != "" &&
		a.RescheduleTracker.LastReschedule != LastRescheduleSuccess
}

// AllocationDiff is another named type for Allocation (to use the same fields),
// which is used to represent the delta for an Allocation. If you need a method
// defined on the al
type AllocationDiff Allocation

// AllocListStub is used to return a subset of alloc information
type AllocListStub struct {
	ID                    string
	EvalID                string
	Name                  string
	Namespace             string
	NodeID                string
	NodeName              string
	JobID                 string
	JobType               string
	JobVersion            uint64
	TaskGroup             string
	AllocatedResources    *AllocatedResources `json:",omitempty"`
	DesiredStatus         string
	DesiredDescription    string
	ClientStatus          string
	ClientDescription     string
	DesiredTransition     DesiredTransition
	TaskStates            map[string]*TaskState
	DeploymentStatus      *AllocDeploymentStatus
	FollowupEvalID        string
	NextAllocation        string
	RescheduleTracker     *RescheduleTracker
	PreemptedAllocations  []string
	PreemptedByAllocation string
	CreateIndex           uint64
	ModifyIndex           uint64
	CreateTime            int64
	ModifyTime            int64
}

// SetEventDisplayMessages populates the display message if its not already
// set, a temporary fix to handle old allocations that don't have it. This
// method will be removed in a future release.
func (a *AllocListStub) SetEventDisplayMessages() {
	setDisplayMsg(a.TaskStates)
}

// RescheduleEligible returns if the allocation is eligible to be rescheduled according
// to its ReschedulePolicy and the current state of its reschedule trackers
func (a *AllocListStub) RescheduleEligible(reschedulePolicy *ReschedulePolicy, failTime time.Time) bool {
	return a.RescheduleTracker.RescheduleEligible(reschedulePolicy, failTime)
}

// ClientTerminalStatus returns if the client status is terminal and will no longer transition
func (a *AllocListStub) ClientTerminalStatus() bool {
	return slices.Contains(terminalAllocationStatuses, a.ClientStatus)
}

func setDisplayMsg(taskStates map[string]*TaskState) {
	for _, taskState := range taskStates {
		for _, event := range taskState.Events {
			event.PopulateEventDisplayMessage()
		}
	}
}

// AllocStubFields defines which fields are included in the AllocListStub.
type AllocStubFields struct {
	// Resources includes resource-related fields if true.
	Resources bool

	// TaskStates removes the TaskStates field if false (default is to
	// include TaskStates).
	TaskStates bool
}

func NewAllocStubFields() *AllocStubFields {
	return &AllocStubFields{
		// Maintain backward compatibility by retaining task states by
		// default.
		TaskStates: true,
	}
}

// AllocMetric is used to track various metrics while attempting
// to make an allocation. These are used to debug a job, or to better
// understand the pressure within the system.
type AllocMetric struct {
	// NodesEvaluated is the number of nodes that were evaluated
	NodesEvaluated int

	// NodesFiltered is the number of nodes filtered due to a constraint
	NodesFiltered int

	// NodesInPool is the number of nodes in the node pool used by the job.
	NodesInPool int

	// NodePool is the node pool the node belongs to.
	NodePool string

	// NodesAvailable is the number of nodes available for evaluation per DC.
	NodesAvailable map[string]int

	// ClassFiltered is the number of nodes filtered by class
	ClassFiltered map[string]int

	// ConstraintFiltered is the number of failures caused by constraint
	ConstraintFiltered map[string]int

	// NodesExhausted is the number of nodes skipped due to being
	// exhausted of at least one resource
	NodesExhausted int

	// ClassExhausted is the number of nodes exhausted by class
	ClassExhausted map[string]int

	// DimensionExhausted provides the count by dimension or reason
	DimensionExhausted map[string]int

	// QuotaExhausted provides the exhausted dimensions
	QuotaExhausted []string

	// ResourcesExhausted provides the amount of resources exhausted by task
	// during the allocation placement
	ResourcesExhausted map[string]*Resources

	// Scores is the scores of the final few nodes remaining
	// for placement. The top score is typically selected.
	// Deprecated: Replaced by ScoreMetaData in Nomad 0.9
	Scores map[string]float64

	// ScoreMetaData is a slice of top scoring nodes displayed in the CLI
	ScoreMetaData []*NodeScoreMeta

	// nodeScoreMeta is used to keep scores for a single node id. It is cleared out after
	// we receive normalized score during the last step of the scoring stack.
	nodeScoreMeta *NodeScoreMeta

	// topScores is used to maintain a heap of the top K nodes with
	// the highest normalized score
	topScores *kheap.ScoreHeap

	// AllocationTime is a measure of how long the allocation
	// attempt took. This can affect performance and SLAs.
	AllocationTime time.Duration

	// CoalescedFailures indicates the number of other
	// allocations that were coalesced into this failed allocation.
	// This is to prevent creating many failed allocations for a
	// single task group.
	CoalescedFailures int
}

func (a *AllocMetric) Copy() *AllocMetric {
	if a == nil {
		return nil
	}
	na := new(AllocMetric)
	*na = *a
	na.NodesAvailable = maps.Clone(na.NodesAvailable)
	na.ClassFiltered = maps.Clone(na.ClassFiltered)
	na.ConstraintFiltered = maps.Clone(na.ConstraintFiltered)
	na.ClassExhausted = maps.Clone(na.ClassExhausted)
	na.DimensionExhausted = maps.Clone(na.DimensionExhausted)
	na.QuotaExhausted = slices.Clone(na.QuotaExhausted)
	na.Scores = maps.Clone(na.Scores)
	na.ScoreMetaData = CopySliceNodeScoreMeta(na.ScoreMetaData)
	return na
}

func (a *AllocMetric) EvaluateNode() {
	a.NodesEvaluated += 1
}

func (a *AllocMetric) FilterNode(node *Node, constraint string) {
	a.NodesFiltered += 1
	if node != nil && node.NodeClass != "" {
		if a.ClassFiltered == nil {
			a.ClassFiltered = make(map[string]int)
		}
		a.ClassFiltered[node.NodeClass] += 1
	}
	if constraint != "" {
		if a.ConstraintFiltered == nil {
			a.ConstraintFiltered = make(map[string]int)
		}
		a.ConstraintFiltered[constraint] += 1
	}
}

func (a *AllocMetric) ExhaustedNode(node *Node, dimension string) {
	a.NodesExhausted += 1
	if node != nil && node.NodeClass != "" {
		if a.ClassExhausted == nil {
			a.ClassExhausted = make(map[string]int)
		}
		a.ClassExhausted[node.NodeClass] += 1
	}
	if dimension != "" {
		if a.DimensionExhausted == nil {
			a.DimensionExhausted = make(map[string]int)
		}
		a.DimensionExhausted[dimension] += 1
	}
}

func (a *AllocMetric) ExhaustQuota(dimensions []string) {
	if a.QuotaExhausted == nil {
		a.QuotaExhausted = make([]string, 0, len(dimensions))
	}

	a.QuotaExhausted = append(a.QuotaExhausted, dimensions...)
}

// ExhaustResources updates the amount of resources exhausted for the
// allocation because of the given task group.
func (a *AllocMetric) ExhaustResources(tg *TaskGroup) {
	if a.DimensionExhausted == nil {
		return
	}

	if a.ResourcesExhausted == nil {
		a.ResourcesExhausted = make(map[string]*Resources)
	}

	for _, t := range tg.Tasks {
		exhaustedResources := a.ResourcesExhausted[t.Name]
		if exhaustedResources == nil {
			exhaustedResources = &Resources{}
		}

		if a.DimensionExhausted["memory"] > 0 {
			exhaustedResources.MemoryMB += t.Resources.MemoryMB
		}

		if a.DimensionExhausted["cpu"] > 0 {
			exhaustedResources.CPU += t.Resources.CPU
		}

		a.ResourcesExhausted[t.Name] = exhaustedResources
	}
}

// ScoreNode is used to gather top K scoring nodes in a heap
func (a *AllocMetric) ScoreNode(node *Node, name string, score float64) {
	// Create nodeScoreMeta lazily if its the first time or if its a new node
	if a.nodeScoreMeta == nil || a.nodeScoreMeta.NodeID != node.ID {
		a.nodeScoreMeta = &NodeScoreMeta{
			NodeID: node.ID,
			Scores: make(map[string]float64),
		}
	}
	if name == NormScorerName {
		a.nodeScoreMeta.NormScore = score
		// Once we have the normalized score we can push to the heap
		// that tracks top K by normalized score

		// Create the heap if its not there already
		if a.topScores == nil {
			a.topScores = kheap.NewScoreHeap(MaxRetainedNodeScores)
		}
		heap.Push(a.topScores, a.nodeScoreMeta)

		// Clear out this entry because its now in the heap
		a.nodeScoreMeta = nil
	} else {
		a.nodeScoreMeta.Scores[name] = score
	}
}

// PopulateScoreMetaData populates a map of scorer to scoring metadata
// The map is populated by popping elements from a heap of top K scores
// maintained per scorer
func (a *AllocMetric) PopulateScoreMetaData() {
	if a.topScores == nil {
		return
	}

	if a.ScoreMetaData == nil {
		a.ScoreMetaData = make([]*NodeScoreMeta, a.topScores.Len())
	}
	heapItems := a.topScores.GetItemsReverse()
	for i, item := range heapItems {
		a.ScoreMetaData[i] = item.(*NodeScoreMeta)
	}
}

// MaxNormScore returns the ScoreMetaData entry with the highest normalized
// score.
func (a *AllocMetric) MaxNormScore() *NodeScoreMeta {
	if a == nil || len(a.ScoreMetaData) == 0 {
		return nil
	}
	return a.ScoreMetaData[0]
}

const (
	// AllocServiceRegistrationsRPCMethod is the RPC method for listing all
	// service registrations assigned to a specific allocation.
	//
	// Args: AllocServiceRegistrationsRequest
	// Reply: AllocServiceRegistrationsResponse
	AllocServiceRegistrationsRPCMethod = "Alloc.GetServiceRegistrations"
)

// AllocServiceRegistrationsRequest is the request object used to list all
// service registrations belonging to the specified Allocation.ID.
type AllocServiceRegistrationsRequest struct {
	AllocID string
	QueryOptions
}

// AllocServiceRegistrationsResponse is the response object when performing a
// listing of services belonging to an allocation.
type AllocServiceRegistrationsResponse struct {
	Services []*ServiceRegistration
	QueryMeta
}

// ServiceProviderNamespace returns the namespace within which the allocations
// services should be registered. This takes into account the different
// providers that can provide service registrations. In the event no services
// are found, the function will return the Consul namespace which allows hooks
// to work as they did before Nomad native services.
//
// It currently assumes that all services within an allocation use the same
// provider and therefore the same namespace, which is enforced at job submit
// time.
func (a *Allocation) ServiceProviderNamespace() string {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)

	if len(tg.Services) > 0 {
		switch tg.Services[0].Provider {
		case ServiceProviderNomad:
			return a.Job.Namespace
		default:
			return tg.Consul.GetNamespace()
		}
	}

	for _, task := range tg.Tasks {
		if len(task.Services) > 0 {
			switch task.Services[0].Provider {
			case ServiceProviderNomad:
				return a.Job.Namespace
			default:
				return a.ConsulNamespaceForTask(task.Name)
			}
		}
	}

	return tg.Consul.GetNamespace()
}

// ServiceProviderNamespaceForTask returns the namespace within which a given
// tasks's services should be registered. This takes into account the different
// providers that can provide service registrations. In the event no services
// are found, the function will return the Consul namespace which allows hooks
// to work as they did before Nomad native services.
//
// It currently assumes that all services within a task use the same provider
// and therefore the same namespace, which is enforced at job submit time.
func (a *Allocation) ServiceProviderNamespaceForTask(taskName string) string {
	tg := a.Job.LookupTaskGroup(a.TaskGroup)

	for _, task := range tg.Tasks {
		if task.Name == taskName {
			for _, service := range task.Services {
				switch service.Provider {
				case ServiceProviderNomad:
					return a.Job.Namespace
				default:
					return a.ConsulNamespaceForTask(taskName)
				}
			}
		}
	}

	return a.ConsulNamespaceForTask(taskName)
}

type AllocInfo struct {
	AllocID string

	// Group in which the service belongs for a group-level service, or the
	// group in which task belongs for a task-level service.
	Group string

	// Task in which the service belongs for task-level service. Will be empty
	// for a group-level service.
	Task string

	// JobID provides additional context for providers regarding which job
	// caused this registration.
	JobID string

	// NomadNamespace provides additional context for providers regarding which
	// nomad namespace caused this registration.
	Namespace string
}

// AllocNetworkStatus captures the status of an allocation's network during runtime.
// Depending on the network mode, an allocation's address may need to be known to other
// systems in Nomad such as service registration.
type AllocNetworkStatus struct {
	InterfaceName string
	Address       string
	AddressIPv6   string
	DNS           *DNSConfig
}

func (a *AllocNetworkStatus) Copy() *AllocNetworkStatus {
	if a == nil {
		return nil
	}
	return &AllocNetworkStatus{
		InterfaceName: a.InterfaceName,
		Address:       a.Address,
		AddressIPv6:   a.AddressIPv6,
		DNS:           a.DNS.Copy(),
	}
}

func (a *AllocNetworkStatus) Equal(o *AllocNetworkStatus) bool {
	// note: this accounts for when DNSConfig is non-nil but empty
	switch {
	case a == nil && o.IsZero():
		return true
	case o == nil && a.IsZero():
		return true
	case a == nil || o == nil:
		return a == o
	}

	switch {
	case a.InterfaceName != o.InterfaceName:
		return false
	case a.Address != o.Address:
		return false
	case a.AddressIPv6 != o.AddressIPv6:
		return false
	case !a.DNS.Equal(o.DNS):
		return false
	}
	return true
}

func (a *AllocNetworkStatus) IsZero() bool {
	if a == nil {
		return true
	}
	if a.InterfaceName != "" || a.Address != "" || a.AddressIPv6 != "" {
		return false
	}
	if !a.DNS.IsZero() {
		return false
	}
	return true
}

// NetworkStatus is an interface satisfied by alloc runner, for acquiring the
// network status of an allocation.
type NetworkStatus interface {
	NetworkStatus() *AllocNetworkStatus
}

// AllocDeploymentStatus captures the status of the allocation as part of the
// deployment. This can include things like if the allocation has been marked as
// healthy.
type AllocDeploymentStatus struct {
	// Healthy marks whether the allocation has been marked healthy or unhealthy
	// as part of a deployment. It can be unset if it has neither been marked
	// healthy or unhealthy.
	Healthy *bool

	// Timestamp is the time at which the health status was set.
	Timestamp time.Time

	// Canary marks whether the allocation is a canary or not. A canary that has
	// been promoted will have this field set to false.
	Canary bool

	// ModifyIndex is the raft index in which the deployment status was last
	// changed.
	ModifyIndex uint64
}

// HasHealth returns true if the allocation has its health set.
func (a *AllocDeploymentStatus) HasHealth() bool {
	return a != nil && a.Healthy != nil
}

// IsHealthy returns if the allocation is marked as healthy as part of a
// deployment
func (a *AllocDeploymentStatus) IsHealthy() bool {
	if a == nil {
		return false
	}

	return a.Healthy != nil && *a.Healthy
}

// IsUnhealthy returns if the allocation is marked as unhealthy as part of a
// deployment
func (a *AllocDeploymentStatus) IsUnhealthy() bool {
	if a == nil {
		return false
	}

	return a.Healthy != nil && !*a.Healthy
}

// IsCanary returns if the allocation is marked as a canary
func (a *AllocDeploymentStatus) IsCanary() bool {
	if a == nil {
		return false
	}

	return a.Canary
}

func (a *AllocDeploymentStatus) Copy() *AllocDeploymentStatus {
	if a == nil {
		return nil
	}

	c := new(AllocDeploymentStatus)
	*c = *a

	if a.Healthy != nil {
		c.Healthy = pointer.Of(*a.Healthy)
	}

	return c
}

func (a *AllocDeploymentStatus) Equal(o *AllocDeploymentStatus) bool {
	if a == nil || o == nil {
		return a == o
	}

	switch {
	case !pointer.Eq(a.Healthy, o.Healthy):
		return false
	case a.Timestamp != o.Timestamp:
		return false
	case a.Canary != o.Canary:
		return false
	case a.ModifyIndex != o.ModifyIndex:
		return false
	}
	return true
}

// DesiredTransition is used to mark an allocation as having a desired state
// transition. This information can be used by the scheduler to make the
// correct decision.
type DesiredTransition struct {
	// Migrate is used to indicate that this allocation should be stopped and
	// migrated to another node.
	Migrate *bool

	// Reschedule is used to indicate that this allocation is eligible to be
	// rescheduled. Most allocations are automatically eligible for
	// rescheduling, so this field is only required when an allocation is not
	// automatically eligible. An example is an allocation that is part of a
	// deployment.
	Reschedule *bool

	// ForceReschedule is used to indicate that this allocation must be rescheduled.
	// This field is only used when operators want to force a placement even if
	// a failed allocation is not eligible to be rescheduled
	ForceReschedule *bool

	// NoShutdownDelay, if set to true, will override the group and
	// task shutdown_delay configuration and ignore the delay for any
	// allocations stopped as a result of this Deregister call.
	NoShutdownDelay *bool

	// MigrateDisablePlacement is used to disable the placement of the allocation
	// when Migrate is set. This field is used to prevent batch job allocations
	// from being placed after being stopped.
	MigrateDisablePlacement *bool
}

// Merge merges the two desired transitions, preferring the values from the
// passed in object.
func (d *DesiredTransition) Merge(o *DesiredTransition) {
	if o.Migrate != nil {
		d.Migrate = o.Migrate
	}

	if o.MigrateDisablePlacement != nil {
		d.MigrateDisablePlacement = o.MigrateDisablePlacement
	}

	if o.Reschedule != nil {
		d.Reschedule = o.Reschedule
	}

	if o.ForceReschedule != nil {
		d.ForceReschedule = o.ForceReschedule
	}

	if o.NoShutdownDelay != nil {
		d.NoShutdownDelay = o.NoShutdownDelay
	}
}

// ShouldMigrate returns whether the transition object dictates a migration.
func (d *DesiredTransition) ShouldMigrate() bool {
	if d == nil {
		return false
	}
	return d.Migrate != nil && *d.Migrate
}

// ShouldReschedule returns whether the transition object dictates a
// rescheduling.
func (d *DesiredTransition) ShouldReschedule() bool {
	if d == nil {
		return false
	}
	return d.Reschedule != nil && *d.Reschedule
}

// ShouldForceReschedule returns whether the transition object dictates a
// forced rescheduling.
func (d *DesiredTransition) ShouldForceReschedule() bool {
	if d == nil {
		return false
	}
	return d.ForceReschedule != nil && *d.ForceReschedule
}

// ShouldIgnoreShutdownDelay returns whether the transition object dictates
// that shutdown skip any shutdown delays.
func (d *DesiredTransition) ShouldIgnoreShutdownDelay() bool {
	if d == nil {
		return false
	}
	return d.NoShutdownDelay != nil && *d.NoShutdownDelay
}

// ShouldDisableMigrationPlacement returns whether the transition object dictates
// that the migration should place allocation.
func (d *DesiredTransition) ShouldDisableMigrationPlacement() bool {
	if d == nil {
		return false
	}
	return d.MigrateDisablePlacement != nil && *d.MigrateDisablePlacement
}

// AllocStateField records a single event that changes the state of the whole allocation
type AllocStateField uint8

const (
	AllocStateFieldClientStatus AllocStateField = iota
)

type AllocState struct {
	Field AllocStateField
	Value string
	Time  time.Time
}
