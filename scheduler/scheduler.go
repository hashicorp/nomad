// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"fmt"

	log "github.com/hashicorp/go-hclog"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// SchedulerVersion is the version of the scheduler. Changes to the
	// scheduler that are incompatible with prior schedulers will increment this
	// version. It is used to disallow dequeueing when the versions do not match
	// across the leader and the dequeueing scheduler.
	SchedulerVersion uint16 = 1
)

// BuiltinSchedulers contains the built in registered schedulers
// which are available
var BuiltinSchedulers = map[string]Factory{
	"service":  NewServiceScheduler,
	"batch":    NewBatchScheduler,
	"system":   NewSystemScheduler,
	"sysbatch": NewSysBatchScheduler,
}

// NewScheduler is used to instantiate and return a new scheduler
// given the scheduler name, initial state, and planner.
func NewScheduler(name string, logger log.Logger, eventsCh chan<- interface{}, state State, planner Planner) (Scheduler, error) {
	// Lookup the factory function
	factory, ok := BuiltinSchedulers[name]
	if !ok {
		return nil, fmt.Errorf("unknown scheduler '%s'", name)
	}

	// Instantiate the scheduler
	sched := factory(logger, eventsCh, state, planner)
	return sched, nil
}

// Factory is used to instantiate a new Scheduler
type Factory func(log.Logger, chan<- interface{}, State, Planner) Scheduler

// Scheduler is the top level instance for a scheduler. A scheduler is
// meant to only encapsulate business logic, pushing the various plumbing
// into Nomad itself. They are invoked to process a single evaluation at
// a time. The evaluation may result in task allocations which are computed
// optimistically, as there are many concurrent evaluations being processed.
// The task allocations are submitted as a plan, and the current leader will
// coordinate the commits to prevent oversubscription or improper allocations
// based on stale state.
type Scheduler interface {
	// Process is used to handle a new evaluation. The scheduler is free to
	// apply any logic necessary to make the task placements. The state and
	// planner will be provided prior to any invocations of process.
	Process(*structs.Evaluation) error
}

// State is an immutable view of the global state. This allows schedulers
// to make intelligent decisions based on allocations of other schedulers
// and to enforce complex constraints that require more information than
// is available to a local state scheduler.
type State interface {
	// Config returns the configuration of the state store
	Config() *state.StateStoreConfig

	// Nodes returns an iterator over all the nodes.
	// The type of each result is *structs.Node
	Nodes(ws memdb.WatchSet) (memdb.ResultIterator, error)

	// NodesByNodePool returns an iterator over all nodes in the node pool
	NodesByNodePool(ws memdb.WatchSet, poolName string) (memdb.ResultIterator, error)

	// NodePoolByName is used to lookup a node by ID.
	NodePoolByName(ws memdb.WatchSet, poolName string) (*structs.NodePool, error)

	// AllocsByJob returns the allocations by JobID
	AllocsByJob(ws memdb.WatchSet, namespace, jobID string, all bool) ([]*structs.Allocation, error)

	// AllocsByNode returns all the allocations by node
	AllocsByNode(ws memdb.WatchSet, node string) ([]*structs.Allocation, error)

	// AllocByID returns the allocation
	AllocByID(ws memdb.WatchSet, allocID string) (*structs.Allocation, error)

	// AllocsByNodeTerminal returns all the allocations by node filtering by terminal status
	AllocsByNodeTerminal(ws memdb.WatchSet, node string, terminal bool) ([]*structs.Allocation, error)

	// NodeByID is used to lookup a node by ID
	NodeByID(ws memdb.WatchSet, nodeID string) (*structs.Node, error)

	// GetJobByID is used to lookup a job by ID
	JobByID(ws memdb.WatchSet, namespace, id string) (*structs.Job, error)

	// DeploymentsByJobID returns the deployments associated with the job
	DeploymentsByJobID(ws memdb.WatchSet, namespace, jobID string, all bool) ([]*structs.Deployment, error)

	// JobByIDAndVersion returns the job associated with id and specific version
	JobByIDAndVersion(ws memdb.WatchSet, namespace, id string, version uint64) (*structs.Job, error)

	// LatestDeploymentByJobID returns the latest deployment matching the given
	// job ID
	LatestDeploymentByJobID(ws memdb.WatchSet, namespace, jobID string) (*structs.Deployment, error)

	// SchedulerConfig returns config options for the scheduler
	SchedulerConfig() (uint64, *structs.SchedulerConfiguration, error)

	// CSIVolumeByID fetch CSI volumes, containing controller jobs
	CSIVolumeByID(memdb.WatchSet, string, string) (*structs.CSIVolume, error)

	// CSIVolumeByID fetch CSI volumes, containing controller jobs
	CSIVolumesByNodeID(memdb.WatchSet, string, string) (memdb.ResultIterator, error)

	// LatestIndex returns the greatest index value for all indexes.
	LatestIndex() (uint64, error)
}

// Planner interface is used to submit a task allocation plan.
type Planner interface {
	// SubmitPlan is used to submit a plan for consideration.
	// This will return a PlanResult or an error. It is possible
	// that this will result in a state refresh as well.
	SubmitPlan(*structs.Plan) (*structs.PlanResult, State, error)

	// UpdateEval is used to update an evaluation. This should update
	// a copy of the input evaluation since that should be immutable.
	UpdateEval(*structs.Evaluation) error

	// CreateEval is used to create an evaluation. This should set the
	// PreviousEval to that of the current evaluation.
	CreateEval(*structs.Evaluation) error

	// ReblockEval takes a blocked evaluation and re-inserts it into the blocked
	// evaluation tracker. This update occurs only in-memory on the leader. The
	// evaluation must exist in a blocked state prior to this being called such
	// that on leader changes, the evaluation will be reblocked properly.
	ReblockEval(*structs.Evaluation) error

	// ServersMeetMinimumVersion returns whether the Nomad servers in the
	// worker's region are at least on the given Nomad version. The
	// checkFailedServers parameter specifies whether version for the failed
	// servers should be verified.
	ServersMeetMinimumVersion(minVersion *version.Version, checkFailedServers bool) bool
}
