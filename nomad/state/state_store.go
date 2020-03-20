package state

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/pkg/errors"
)

// Txn is a transaction against a state store.
// This can be a read or write transaction.
type Txn = *memdb.Txn

const (
	// NodeRegisterEventReregistered is the message used when the node becomes
	// reregistered.
	NodeRegisterEventRegistered = "Node registered"

	// NodeRegisterEventReregistered is the message used when the node becomes
	// reregistered.
	NodeRegisterEventReregistered = "Node re-registered"
)

// IndexEntry is used with the "index" table
// for managing the latest Raft index affecting a table.
type IndexEntry struct {
	Key   string
	Value uint64
}

// StateStoreConfig is used to configure a new state store
type StateStoreConfig struct {
	// Logger is used to output the state store's logs
	Logger log.Logger

	// Region is the region of the server embedding the state store.
	Region string
}

// The StateStore is responsible for maintaining all the Nomad
// state. It is manipulated by the FSM which maintains consistency
// through the use of Raft. The goals of the StateStore are to provide
// high concurrency for read operations without blocking writes, and
// to provide write availability in the face of reads. EVERY object
// returned as a result of a read against the state store should be
// considered a constant and NEVER modified in place.
type StateStore struct {
	logger log.Logger
	db     *memdb.MemDB

	// config is the passed in configuration
	config *StateStoreConfig

	// abandonCh is used to signal watchers that this state store has been
	// abandoned (usually during a restore). This is only ever closed.
	abandonCh chan struct{}
}

// NewStateStore is used to create a new state store
func NewStateStore(config *StateStoreConfig) (*StateStore, error) {
	// Create the MemDB
	db, err := memdb.NewMemDB(stateStoreSchema())
	if err != nil {
		return nil, fmt.Errorf("state store setup failed: %v", err)
	}

	// Create the state store
	s := &StateStore{
		logger:    config.Logger.Named("state_store"),
		db:        db,
		config:    config,
		abandonCh: make(chan struct{}),
	}
	return s, nil
}

// Config returns the state store configuration.
func (s *StateStore) Config() *StateStoreConfig {
	return s.config
}

// Snapshot is used to create a point in time snapshot. Because
// we use MemDB, we just need to snapshot the state of the underlying
// database.
func (s *StateStore) Snapshot() (*StateSnapshot, error) {
	snap := &StateSnapshot{
		StateStore: StateStore{
			logger: s.logger,
			config: s.config,
			db:     s.db.Snapshot(),
		},
	}
	return snap, nil
}

// SnapshotMinIndex is used to create a state snapshot where the index is
// guaranteed to be greater than or equal to the index parameter.
//
// Some server operations (such as scheduling) exchange objects via RPC
// concurrent with Raft log application, so they must ensure the state store
// snapshot they are operating on is at or after the index the objects
// retrieved via RPC were applied to the Raft log at.
//
// Callers should maintain their own timer metric as the time this method
// blocks indicates Raft log application latency relative to scheduling.
func (s *StateStore) SnapshotMinIndex(ctx context.Context, index uint64) (*StateSnapshot, error) {
	// Ported from work.go:waitForIndex prior to 0.9

	const backoffBase = 20 * time.Millisecond
	const backoffLimit = 1 * time.Second
	var retries uint
	var retryTimer *time.Timer

	// XXX: Potential optimization is to set up a watch on the state
	// store's index table and only unblock via a trigger rather than
	// polling.
	for {
		// Get the states current index
		snapshotIndex, err := s.LatestIndex()
		if err != nil {
			return nil, fmt.Errorf("failed to determine state store's index: %v", err)
		}

		// We only need the FSM state to be as recent as the given index
		if snapshotIndex >= index {
			return s.Snapshot()
		}

		// Exponential back off
		retries++
		if retryTimer == nil {
			// First retry, start at baseline
			retryTimer = time.NewTimer(backoffBase)
		} else {
			// Subsequent retry, reset timer
			deadline := 1 << (2 * retries) * backoffBase
			if deadline > backoffLimit {
				deadline = backoffLimit
			}
			retryTimer.Reset(deadline)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-retryTimer.C:
		}
	}
}

// Restore is used to optimize the efficiency of rebuilding
// state by minimizing the number of transactions and checking
// overhead.
func (s *StateStore) Restore() (*StateRestore, error) {
	txn := s.db.Txn(true)
	r := &StateRestore{
		txn: txn,
	}
	return r, nil
}

// AbandonCh returns a channel you can wait on to know if the state store was
// abandoned.
func (s *StateStore) AbandonCh() <-chan struct{} {
	return s.abandonCh
}

// Abandon is used to signal that the given state store has been abandoned.
// Calling this more than one time will panic.
func (s *StateStore) Abandon() {
	close(s.abandonCh)
}

// QueryFn is the definition of a function that can be used to implement a basic
// blocking query against the state store.
type QueryFn func(memdb.WatchSet, *StateStore) (resp interface{}, index uint64, err error)

// BlockingQuery takes a query function and runs the function until the minimum
// query index is met or until the passed context is cancelled.
func (s *StateStore) BlockingQuery(query QueryFn, minIndex uint64, ctx context.Context) (
	resp interface{}, index uint64, err error) {

RUN_QUERY:
	// We capture the state store and its abandon channel but pass a snapshot to
	// the blocking query function. We operate on the snapshot to allow separate
	// calls to the state store not all wrapped within the same transaction.
	abandonCh := s.AbandonCh()
	snap, _ := s.Snapshot()
	stateSnap := &snap.StateStore

	// We can skip all watch tracking if this isn't a blocking query.
	var ws memdb.WatchSet
	if minIndex > 0 {
		ws = memdb.NewWatchSet()

		// This channel will be closed if a snapshot is restored and the
		// whole state store is abandoned.
		ws.Add(abandonCh)
	}

	resp, index, err = query(ws, stateSnap)
	if err != nil {
		return nil, index, err
	}

	// We haven't reached the min-index yet.
	if minIndex > 0 && index <= minIndex {
		if err := ws.WatchCtx(ctx); err != nil {
			return nil, index, err
		}

		goto RUN_QUERY
	}

	return resp, index, nil
}

// UpsertPlanResults is used to upsert the results of a plan.
func (s *StateStore) UpsertPlanResults(index uint64, results *structs.ApplyPlanResultsRequest) error {
	snapshot, err := s.Snapshot()
	if err != nil {
		return err
	}

	allocsStopped, err := snapshot.DenormalizeAllocationDiffSlice(results.AllocsStopped)
	if err != nil {
		return err
	}

	allocsPreempted, err := snapshot.DenormalizeAllocationDiffSlice(results.AllocsPreempted)
	if err != nil {
		return err
	}

	// COMPAT 0.11: Remove this denormalization when NodePreemptions is removed
	results.NodePreemptions, err = snapshot.DenormalizeAllocationSlice(results.NodePreemptions)
	if err != nil {
		return err
	}

	txn := s.db.Txn(true)
	defer txn.Abort()

	// Upsert the newly created or updated deployment
	if results.Deployment != nil {
		if err := s.upsertDeploymentImpl(index, results.Deployment, txn); err != nil {
			return err
		}
	}

	// Update the status of deployments effected by the plan.
	if len(results.DeploymentUpdates) != 0 {
		s.upsertDeploymentUpdates(index, results.DeploymentUpdates, txn)
	}

	if results.EvalID != "" {
		// Update the modify index of the eval id
		if err := s.updateEvalModifyIndex(txn, index, results.EvalID); err != nil {
			return err
		}
	}

	numAllocs := 0
	if len(results.Alloc) > 0 || len(results.NodePreemptions) > 0 {
		// COMPAT 0.11: This branch will be removed, when Alloc is removed
		// Attach the job to all the allocations. It is pulled out in the payload to
		// avoid the redundancy of encoding, but should be denormalized prior to
		// being inserted into MemDB.
		addComputedAllocAttrs(results.Alloc, results.Job)
		numAllocs = len(results.Alloc) + len(results.NodePreemptions)
	} else {
		// Attach the job to all the allocations. It is pulled out in the payload to
		// avoid the redundancy of encoding, but should be denormalized prior to
		// being inserted into MemDB.
		addComputedAllocAttrs(results.AllocsUpdated, results.Job)
		numAllocs = len(allocsStopped) + len(results.AllocsUpdated) + len(allocsPreempted)
	}

	allocsToUpsert := make([]*structs.Allocation, 0, numAllocs)

	// COMPAT 0.11: Both these appends should be removed when Alloc and NodePreemptions are removed
	allocsToUpsert = append(allocsToUpsert, results.Alloc...)
	allocsToUpsert = append(allocsToUpsert, results.NodePreemptions...)

	allocsToUpsert = append(allocsToUpsert, allocsStopped...)
	allocsToUpsert = append(allocsToUpsert, results.AllocsUpdated...)
	allocsToUpsert = append(allocsToUpsert, allocsPreempted...)

	// handle upgrade path
	for _, alloc := range allocsToUpsert {
		alloc.Canonicalize()
	}

	if err := s.upsertAllocsImpl(index, allocsToUpsert, txn); err != nil {
		return err
	}

	// Upsert followup evals for allocs that were preempted
	for _, eval := range results.PreemptionEvals {
		if err := s.nestedUpsertEval(txn, index, eval); err != nil {
			return err
		}
	}

	txn.Commit()
	return nil
}

// addComputedAllocAttrs adds the computed/derived attributes to the allocation.
// This method is used when an allocation is being denormalized.
func addComputedAllocAttrs(allocs []*structs.Allocation, job *structs.Job) {
	structs.DenormalizeAllocationJobs(job, allocs)

	// COMPAT(0.11): Remove in 0.11
	// Calculate the total resources of allocations. It is pulled out in the
	// payload to avoid encoding something that can be computed, but should be
	// denormalized prior to being inserted into MemDB.
	for _, alloc := range allocs {
		if alloc.Resources != nil {
			continue
		}

		alloc.Resources = new(structs.Resources)
		for _, task := range alloc.TaskResources {
			alloc.Resources.Add(task)
		}

		// Add the shared resources
		alloc.Resources.Add(alloc.SharedResources)
	}
}

// upsertDeploymentUpdates updates the deployments given the passed status
// updates.
func (s *StateStore) upsertDeploymentUpdates(index uint64, updates []*structs.DeploymentStatusUpdate, txn *memdb.Txn) error {
	for _, u := range updates {
		if err := s.updateDeploymentStatusImpl(index, u, txn); err != nil {
			return err
		}
	}

	return nil
}

// UpsertJobSummary upserts a job summary into the state store.
func (s *StateStore) UpsertJobSummary(index uint64, jobSummary *structs.JobSummary) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Check if the job summary already exists
	existing, err := txn.First("job_summary", "id", jobSummary.Namespace, jobSummary.JobID)
	if err != nil {
		return fmt.Errorf("job summary lookup failed: %v", err)
	}

	// Setup the indexes correctly
	if existing != nil {
		jobSummary.CreateIndex = existing.(*structs.JobSummary).CreateIndex
		jobSummary.ModifyIndex = index
	} else {
		jobSummary.CreateIndex = index
		jobSummary.ModifyIndex = index
	}

	// Update the index
	if err := txn.Insert("job_summary", jobSummary); err != nil {
		return err
	}

	// Update the indexes table for job summary
	if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// DeleteJobSummary deletes the job summary with the given ID. This is for
// testing purposes only.
func (s *StateStore) DeleteJobSummary(index uint64, namespace, id string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Delete the job summary
	if _, err := txn.DeleteAll("job_summary", "id", namespace, id); err != nil {
		return fmt.Errorf("deleting job summary failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	txn.Commit()
	return nil
}

// UpsertDeployment is used to insert a new deployment. If cancelPrior is set to
// true, all prior deployments for the same job will be cancelled.
func (s *StateStore) UpsertDeployment(index uint64, deployment *structs.Deployment) error {
	txn := s.db.Txn(true)
	defer txn.Abort()
	if err := s.upsertDeploymentImpl(index, deployment, txn); err != nil {
		return err
	}
	txn.Commit()
	return nil
}

func (s *StateStore) upsertDeploymentImpl(index uint64, deployment *structs.Deployment, txn *memdb.Txn) error {
	// Check if the deployment already exists
	existing, err := txn.First("deployment", "id", deployment.ID)
	if err != nil {
		return fmt.Errorf("deployment lookup failed: %v", err)
	}

	// Setup the indexes correctly
	if existing != nil {
		deployment.CreateIndex = existing.(*structs.Deployment).CreateIndex
		deployment.ModifyIndex = index
	} else {
		deployment.CreateIndex = index
		deployment.ModifyIndex = index
	}

	// Insert the deployment
	if err := txn.Insert("deployment", deployment); err != nil {
		return err
	}

	// Update the indexes table for deployment
	if err := txn.Insert("index", &IndexEntry{"deployment", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// If the deployment is being marked as complete, set the job to stable.
	if deployment.Status == structs.DeploymentStatusSuccessful {
		if err := s.updateJobStabilityImpl(index, deployment.Namespace, deployment.JobID, deployment.JobVersion, true, txn); err != nil {
			return fmt.Errorf("failed to update job stability: %v", err)
		}
	}

	return nil
}

func (s *StateStore) Deployments(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire deployments table
	iter, err := txn.Get("deployment", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

func (s *StateStore) DeploymentsByNamespace(ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire deployments table
	iter, err := txn.Get("deployment", "namespace", namespace)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

func (s *StateStore) DeploymentsByIDPrefix(ws memdb.WatchSet, namespace, deploymentID string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire deployments table
	iter, err := txn.Get("deployment", "id_prefix", deploymentID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	// Wrap the iterator in a filter
	wrap := memdb.NewFilterIterator(iter, deploymentNamespaceFilter(namespace))
	return wrap, nil
}

// deploymentNamespaceFilter returns a filter function that filters all
// deployment not in the given namespace.
func deploymentNamespaceFilter(namespace string) func(interface{}) bool {
	return func(raw interface{}) bool {
		d, ok := raw.(*structs.Deployment)
		if !ok {
			return true
		}

		return d.Namespace != namespace
	}
}

func (s *StateStore) DeploymentByID(ws memdb.WatchSet, deploymentID string) (*structs.Deployment, error) {
	txn := s.db.Txn(false)
	return s.deploymentByIDImpl(ws, deploymentID, txn)
}

func (s *StateStore) deploymentByIDImpl(ws memdb.WatchSet, deploymentID string, txn *memdb.Txn) (*structs.Deployment, error) {
	watchCh, existing, err := txn.FirstWatch("deployment", "id", deploymentID)
	if err != nil {
		return nil, fmt.Errorf("deployment lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.Deployment), nil
	}

	return nil, nil
}

func (s *StateStore) DeploymentsByJobID(ws memdb.WatchSet, namespace, jobID string, all bool) ([]*structs.Deployment, error) {
	txn := s.db.Txn(false)

	var job *structs.Job
	// Read job from state store
	_, existing, err := txn.FirstWatch("jobs", "id", namespace, jobID)
	if err != nil {
		return nil, fmt.Errorf("job lookup failed: %v", err)
	}
	if existing != nil {
		job = existing.(*structs.Job)
	}

	// Get an iterator over the deployments
	iter, err := txn.Get("deployment", "job", namespace, jobID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var out []*structs.Deployment
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		d := raw.(*structs.Deployment)

		// If the allocation belongs to a job with the same ID but a different
		// create index and we are not getting all the allocations whose Jobs
		// matches the same Job ID then we skip it
		if !all && job != nil && d.JobCreateIndex != job.CreateIndex {
			continue
		}
		out = append(out, d)
	}

	return out, nil
}

// LatestDeploymentByJobID returns the latest deployment for the given job. The
// latest is determined strictly by CreateIndex.
func (s *StateStore) LatestDeploymentByJobID(ws memdb.WatchSet, namespace, jobID string) (*structs.Deployment, error) {
	txn := s.db.Txn(false)

	// Get an iterator over the deployments
	iter, err := txn.Get("deployment", "job", namespace, jobID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var out *structs.Deployment
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		d := raw.(*structs.Deployment)
		if out == nil || out.CreateIndex < d.CreateIndex {
			out = d
		}
	}

	return out, nil
}

// DeleteDeployment is used to delete a set of deployments by ID
func (s *StateStore) DeleteDeployment(index uint64, deploymentIDs []string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	if len(deploymentIDs) == 0 {
		return nil
	}

	for _, deploymentID := range deploymentIDs {
		// Lookup the deployment
		existing, err := txn.First("deployment", "id", deploymentID)
		if err != nil {
			return fmt.Errorf("deployment lookup failed: %v", err)
		}
		if existing == nil {
			return fmt.Errorf("deployment not found")
		}

		// Delete the deployment
		if err := txn.Delete("deployment", existing); err != nil {
			return fmt.Errorf("deployment delete failed: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"deployment", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// UpsertNode is used to register a node or update a node definition
// This is assumed to be triggered by the client, so we retain the value
// of drain/eligibility which is set by the scheduler.
func (s *StateStore) UpsertNode(index uint64, node *structs.Node) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Check if the node already exists
	existing, err := txn.First("nodes", "id", node.ID)
	if err != nil {
		return fmt.Errorf("node lookup failed: %v", err)
	}

	// Setup the indexes correctly
	if existing != nil {
		exist := existing.(*structs.Node)
		node.CreateIndex = exist.CreateIndex
		node.ModifyIndex = index

		// Retain node events that have already been set on the node
		node.Events = exist.Events

		// If we are transitioning from down, record the re-registration
		if exist.Status == structs.NodeStatusDown && node.Status != structs.NodeStatusDown {
			appendNodeEvents(index, node, []*structs.NodeEvent{
				structs.NewNodeEvent().SetSubsystem(structs.NodeEventSubsystemCluster).
					SetMessage(NodeRegisterEventReregistered).
					SetTimestamp(time.Unix(node.StatusUpdatedAt, 0))})
		}

		node.Drain = exist.Drain                                 // Retain the drain mode
		node.SchedulingEligibility = exist.SchedulingEligibility // Retain the eligibility
		node.DrainStrategy = exist.DrainStrategy                 // Retain the drain strategy
	} else {
		// Because this is the first time the node is being registered, we should
		// also create a node registration event
		nodeEvent := structs.NewNodeEvent().SetSubsystem(structs.NodeEventSubsystemCluster).
			SetMessage(NodeRegisterEventRegistered).
			SetTimestamp(time.Unix(node.StatusUpdatedAt, 0))
		node.Events = []*structs.NodeEvent{nodeEvent}
		node.CreateIndex = index
		node.ModifyIndex = index
	}

	// Insert the node
	if err := txn.Insert("nodes", node); err != nil {
		return fmt.Errorf("node insert failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// DeleteNode deregisters a batch of nodes
func (s *StateStore) DeleteNode(index uint64, nodes []string) error {
	if len(nodes) == 0 {
		return fmt.Errorf("node ids missing")
	}

	txn := s.db.Txn(true)
	defer txn.Abort()

	for _, nodeID := range nodes {
		existing, err := txn.First("nodes", "id", nodeID)
		if err != nil {
			return fmt.Errorf("node lookup failed: %s: %v", nodeID, err)
		}
		if existing == nil {
			return fmt.Errorf("node not found: %s", nodeID)
		}

		// Delete the node
		if err := txn.Delete("nodes", existing); err != nil {
			return fmt.Errorf("node delete failed: %s: %v", nodeID, err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// UpdateNodeStatus is used to update the status of a node
func (s *StateStore) UpdateNodeStatus(index uint64, nodeID, status string, updatedAt int64, event *structs.NodeEvent) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Lookup the node
	existing, err := txn.First("nodes", "id", nodeID)
	if err != nil {
		return fmt.Errorf("node lookup failed: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("node not found")
	}

	// Copy the existing node
	existingNode := existing.(*structs.Node)
	copyNode := existingNode.Copy()
	copyNode.StatusUpdatedAt = updatedAt

	// Add the event if given
	if event != nil {
		appendNodeEvents(index, copyNode, []*structs.NodeEvent{event})
	}

	// Update the status in the copy
	copyNode.Status = status
	copyNode.ModifyIndex = index

	// Insert the node
	if err := txn.Insert("nodes", copyNode); err != nil {
		return fmt.Errorf("node update failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// BatchUpdateNodeDrain is used to update the drain of a node set of nodes
func (s *StateStore) BatchUpdateNodeDrain(index uint64, updatedAt int64, updates map[string]*structs.DrainUpdate, events map[string]*structs.NodeEvent) error {
	txn := s.db.Txn(true)
	defer txn.Abort()
	for node, update := range updates {
		if err := s.updateNodeDrainImpl(txn, index, node, update.DrainStrategy, update.MarkEligible, updatedAt, events[node]); err != nil {
			return err
		}
	}
	txn.Commit()
	return nil
}

// UpdateNodeDrain is used to update the drain of a node
func (s *StateStore) UpdateNodeDrain(index uint64, nodeID string,
	drain *structs.DrainStrategy, markEligible bool, updatedAt int64, event *structs.NodeEvent) error {

	txn := s.db.Txn(true)
	defer txn.Abort()
	if err := s.updateNodeDrainImpl(txn, index, nodeID, drain, markEligible, updatedAt, event); err != nil {
		return err
	}
	txn.Commit()
	return nil
}

func (s *StateStore) updateNodeDrainImpl(txn *memdb.Txn, index uint64, nodeID string,
	drain *structs.DrainStrategy, markEligible bool, updatedAt int64, event *structs.NodeEvent) error {

	// Lookup the node
	existing, err := txn.First("nodes", "id", nodeID)
	if err != nil {
		return fmt.Errorf("node lookup failed: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("node not found")
	}

	// Copy the existing node
	existingNode := existing.(*structs.Node)
	copyNode := existingNode.Copy()
	copyNode.StatusUpdatedAt = updatedAt

	// Add the event if given
	if event != nil {
		appendNodeEvents(index, copyNode, []*structs.NodeEvent{event})
	}

	// Update the drain in the copy
	copyNode.Drain = drain != nil // COMPAT: Remove in Nomad 0.10
	copyNode.DrainStrategy = drain
	if drain != nil {
		copyNode.SchedulingEligibility = structs.NodeSchedulingIneligible
	} else if markEligible {
		copyNode.SchedulingEligibility = structs.NodeSchedulingEligible
	}

	copyNode.ModifyIndex = index

	// Insert the node
	if err := txn.Insert("nodes", copyNode); err != nil {
		return fmt.Errorf("node update failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// UpdateNodeEligibility is used to update the scheduling eligibility of a node
func (s *StateStore) UpdateNodeEligibility(index uint64, nodeID string, eligibility string, updatedAt int64, event *structs.NodeEvent) error {

	txn := s.db.Txn(true)
	defer txn.Abort()

	// Lookup the node
	existing, err := txn.First("nodes", "id", nodeID)
	if err != nil {
		return fmt.Errorf("node lookup failed: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("node not found")
	}

	// Copy the existing node
	existingNode := existing.(*structs.Node)
	copyNode := existingNode.Copy()
	copyNode.StatusUpdatedAt = updatedAt

	// Add the event if given
	if event != nil {
		appendNodeEvents(index, copyNode, []*structs.NodeEvent{event})
	}

	// Check if this is a valid action
	if copyNode.DrainStrategy != nil && eligibility == structs.NodeSchedulingEligible {
		return fmt.Errorf("can not set node's scheduling eligibility to eligible while it is draining")
	}

	// Update the eligibility in the copy
	copyNode.SchedulingEligibility = eligibility
	copyNode.ModifyIndex = index

	// Insert the node
	if err := txn.Insert("nodes", copyNode); err != nil {
		return fmt.Errorf("node update failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// UpsertNodeEvents adds the node events to the nodes, rotating events as
// necessary.
func (s *StateStore) UpsertNodeEvents(index uint64, nodeEvents map[string][]*structs.NodeEvent) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	for nodeID, events := range nodeEvents {
		if err := s.upsertNodeEvents(index, nodeID, events, txn); err != nil {
			return err
		}
	}

	txn.Commit()
	return nil
}

// upsertNodeEvent upserts a node event for a respective node. It also maintains
// that a fixed number of node events are ever stored simultaneously, deleting
// older events once this bound has been reached.
func (s *StateStore) upsertNodeEvents(index uint64, nodeID string, events []*structs.NodeEvent, txn *memdb.Txn) error {
	// Lookup the node
	existing, err := txn.First("nodes", "id", nodeID)
	if err != nil {
		return fmt.Errorf("node lookup failed: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("node not found")
	}

	// Copy the existing node
	existingNode := existing.(*structs.Node)
	copyNode := existingNode.Copy()
	appendNodeEvents(index, copyNode, events)

	// Insert the node
	if err := txn.Insert("nodes", copyNode); err != nil {
		return fmt.Errorf("node update failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// appendNodeEvents is a helper that takes a node and new events and appends
// them, pruning older events as needed.
func appendNodeEvents(index uint64, node *structs.Node, events []*structs.NodeEvent) {
	// Add the events, updating the indexes
	for _, e := range events {
		e.CreateIndex = index
		node.Events = append(node.Events, e)
	}

	// Keep node events pruned to not exceed the max allowed
	if l := len(node.Events); l > structs.MaxRetainedNodeEvents {
		delta := l - structs.MaxRetainedNodeEvents
		node.Events = node.Events[delta:]
	}
}

// NodeByID is used to lookup a node by ID
func (s *StateStore) NodeByID(ws memdb.WatchSet, nodeID string) (*structs.Node, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("nodes", "id", nodeID)
	if err != nil {
		return nil, fmt.Errorf("node lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.Node), nil
	}
	return nil, nil
}

// NodesByIDPrefix is used to lookup nodes by prefix
func (s *StateStore) NodesByIDPrefix(ws memdb.WatchSet, nodeID string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("nodes", "id_prefix", nodeID)
	if err != nil {
		return nil, fmt.Errorf("node lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// NodeBySecretID is used to lookup a node by SecretID
func (s *StateStore) NodeBySecretID(ws memdb.WatchSet, secretID string) (*structs.Node, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("nodes", "secret_id", secretID)
	if err != nil {
		return nil, fmt.Errorf("node lookup by SecretID failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.Node), nil
	}
	return nil, nil
}

// Nodes returns an iterator over all the nodes
func (s *StateStore) Nodes(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire nodes table
	iter, err := txn.Get("nodes", "id")
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// UpsertJob is used to register a job or update a job definition
func (s *StateStore) UpsertJob(index uint64, job *structs.Job) error {
	txn := s.db.Txn(true)
	defer txn.Abort()
	if err := s.upsertJobImpl(index, job, false, txn); err != nil {
		return err
	}
	txn.Commit()
	return nil
}

// UpsertJobTxn is used to register a job or update a job definition, like UpsertJob,
// but in a transaction.  Useful for when making multiple modifications atomically
func (s *StateStore) UpsertJobTxn(index uint64, job *structs.Job, txn Txn) error {
	return s.upsertJobImpl(index, job, false, txn)
}

// upsertJobImpl is the implementation for registering a job or updating a job definition
func (s *StateStore) upsertJobImpl(index uint64, job *structs.Job, keepVersion bool, txn *memdb.Txn) error {
	// Assert the namespace exists
	if exists, err := s.namespaceExists(txn, job.Namespace); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("job %q is in nonexistent namespace %q", job.ID, job.Namespace)
	}

	// Check if the job already exists
	existing, err := txn.First("jobs", "id", job.Namespace, job.ID)
	if err != nil {
		return fmt.Errorf("job lookup failed: %v", err)
	}

	// Setup the indexes correctly
	if existing != nil {
		job.CreateIndex = existing.(*structs.Job).CreateIndex
		job.ModifyIndex = index

		// Bump the version unless asked to keep it. This should only be done
		// when changing an internal field such as Stable. A spec change should
		// always come with a version bump
		if !keepVersion {
			job.JobModifyIndex = index
			job.Version = existing.(*structs.Job).Version + 1
		}

		// Compute the job status
		var err error
		job.Status, err = s.getJobStatus(txn, job, false)
		if err != nil {
			return fmt.Errorf("setting job status for %q failed: %v", job.ID, err)
		}
	} else {
		job.CreateIndex = index
		job.ModifyIndex = index
		job.JobModifyIndex = index
		job.Version = 0

		if err := s.setJobStatus(index, txn, job, false, ""); err != nil {
			return fmt.Errorf("setting job status for %q failed: %v", job.ID, err)
		}

		// Have to get the job again since it could have been updated
		updated, err := txn.First("jobs", "id", job.Namespace, job.ID)
		if err != nil {
			return fmt.Errorf("job lookup failed: %v", err)
		}
		if updated != nil {
			job = updated.(*structs.Job)
		}
	}

	if err := s.updateSummaryWithJob(index, job, txn); err != nil {
		return fmt.Errorf("unable to create job summary: %v", err)
	}

	if err := s.upsertJobVersion(index, job, txn); err != nil {
		return fmt.Errorf("unable to upsert job into job_version table: %v", err)
	}

	if err := s.updateJobScalingPolicies(index, job, txn); err != nil {
		return fmt.Errorf("unable to update job scaling policies: %v", err)
	}

	// Insert the job
	if err := txn.Insert("jobs", job); err != nil {
		return fmt.Errorf("job insert failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"jobs", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// DeleteJob is used to deregister a job
func (s *StateStore) DeleteJob(index uint64, namespace, jobID string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	err := s.DeleteJobTxn(index, namespace, jobID, txn)
	if err == nil {
		txn.Commit()
	}
	return err
}

// DeleteJobTxn is used to deregister a job, like DeleteJob,
// but in a transaction.  Useful for when making multiple modifications atomically
func (s *StateStore) DeleteJobTxn(index uint64, namespace, jobID string, txn Txn) error {
	// Lookup the node
	existing, err := txn.First("jobs", "id", namespace, jobID)
	if err != nil {
		return fmt.Errorf("job lookup failed: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("job not found")
	}

	// Check if we should update a parent job summary
	job := existing.(*structs.Job)
	if job.ParentID != "" {
		summaryRaw, err := txn.First("job_summary", "id", namespace, job.ParentID)
		if err != nil {
			return fmt.Errorf("unable to retrieve summary for parent job: %v", err)
		}

		// Only continue if the summary exists. It could not exist if the parent
		// job was removed
		if summaryRaw != nil {
			existing := summaryRaw.(*structs.JobSummary)
			pSummary := existing.Copy()
			if pSummary.Children != nil {

				modified := false
				switch job.Status {
				case structs.JobStatusPending:
					pSummary.Children.Pending--
					pSummary.Children.Dead++
					modified = true
				case structs.JobStatusRunning:
					pSummary.Children.Running--
					pSummary.Children.Dead++
					modified = true
				case structs.JobStatusDead:
				default:
					return fmt.Errorf("unknown old job status %q", job.Status)
				}

				if modified {
					// Update the modify index
					pSummary.ModifyIndex = index

					// Insert the summary
					if err := txn.Insert("job_summary", pSummary); err != nil {
						return fmt.Errorf("job summary insert failed: %v", err)
					}
					if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
						return fmt.Errorf("index update failed: %v", err)
					}
				}
			}
		}
	}

	// Delete the job
	if err := txn.Delete("jobs", existing); err != nil {
		return fmt.Errorf("job delete failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"jobs", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// Delete the job versions
	if err := s.deleteJobVersions(index, job, txn); err != nil {
		return err
	}

	// Delete the job summary
	if _, err = txn.DeleteAll("job_summary", "id", namespace, jobID); err != nil {
		return fmt.Errorf("deleting job summary failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// Delete the job scaling policies
	if _, err := txn.DeleteAll("scaling_policy", "job", namespace, jobID); err != nil {
		return fmt.Errorf("deleting job scaling policies failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"scaling_policy", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// deleteJobVersions deletes all versions of the given job.
func (s *StateStore) deleteJobVersions(index uint64, job *structs.Job, txn *memdb.Txn) error {
	iter, err := txn.Get("job_version", "id_prefix", job.Namespace, job.ID)
	if err != nil {
		return err
	}

	// Put them into a slice so there are no safety concerns while actually
	// performing the deletes
	jobs := []*structs.Job{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		// Ensure the ID is an exact match
		j := raw.(*structs.Job)
		if j.ID != job.ID {
			continue
		}

		jobs = append(jobs, j)
	}

	// Do the deletes
	for _, j := range jobs {
		if err := txn.Delete("job_version", j); err != nil {
			return fmt.Errorf("deleting job versions failed: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"job_version", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// upsertJobVersion inserts a job into its historic version table and limits the
// number of job versions that are tracked.
func (s *StateStore) upsertJobVersion(index uint64, job *structs.Job, txn *memdb.Txn) error {
	// Insert the job
	if err := txn.Insert("job_version", job); err != nil {
		return fmt.Errorf("failed to insert job into job_version table: %v", err)
	}

	if err := txn.Insert("index", &IndexEntry{"job_version", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// Get all the historic jobs for this ID
	all, err := s.jobVersionByID(txn, nil, job.Namespace, job.ID)
	if err != nil {
		return fmt.Errorf("failed to look up job versions for %q: %v", job.ID, err)
	}

	// If we are below the limit there is no GCing to be done
	if len(all) <= structs.JobTrackedVersions {
		return nil
	}

	// We have to delete a historic job to make room.
	// Find index of the highest versioned stable job
	stableIdx := -1
	for i, j := range all {
		if j.Stable {
			stableIdx = i
			break
		}
	}

	// If the stable job is the oldest version, do a swap to bring it into the
	// keep set.
	max := structs.JobTrackedVersions
	if stableIdx == max {
		all[max-1], all[max] = all[max], all[max-1]
	}

	// Delete the job outside of the set that are being kept.
	d := all[max]
	if err := txn.Delete("job_version", d); err != nil {
		return fmt.Errorf("failed to delete job %v (%d) from job_version", d.ID, d.Version)
	}

	return nil
}

// JobByID is used to lookup a job by its ID. JobByID returns the current/latest job
// version.
func (s *StateStore) JobByID(ws memdb.WatchSet, namespace, id string) (*structs.Job, error) {
	txn := s.db.Txn(false)
	return s.JobByIDTxn(ws, namespace, id, txn)
}

// JobByIDTxn is used to lookup a job by its ID, like  JobByID. JobByID returns the job version
// accessible through in the transaction
func (s *StateStore) JobByIDTxn(ws memdb.WatchSet, namespace, id string, txn Txn) (*structs.Job, error) {
	watchCh, existing, err := txn.FirstWatch("jobs", "id", namespace, id)
	if err != nil {
		return nil, fmt.Errorf("job lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.Job), nil
	}
	return nil, nil
}

// JobsByIDPrefix is used to lookup a job by prefix
func (s *StateStore) JobsByIDPrefix(ws memdb.WatchSet, namespace, id string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("jobs", "id_prefix", namespace, id)
	if err != nil {
		return nil, fmt.Errorf("job lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobVersionsByID returns all the tracked versions of a job.
func (s *StateStore) JobVersionsByID(ws memdb.WatchSet, namespace, id string) ([]*structs.Job, error) {
	txn := s.db.Txn(false)

	return s.jobVersionByID(txn, &ws, namespace, id)
}

// jobVersionByID is the underlying implementation for retrieving all tracked
// versions of a job and is called under an existing transaction. A watch set
// can optionally be passed in to add the job histories to the watch set.
func (s *StateStore) jobVersionByID(txn *memdb.Txn, ws *memdb.WatchSet, namespace, id string) ([]*structs.Job, error) {
	// Get all the historic jobs for this ID
	iter, err := txn.Get("job_version", "id_prefix", namespace, id)
	if err != nil {
		return nil, err
	}

	if ws != nil {
		ws.Add(iter.WatchCh())
	}

	var all []*structs.Job
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		// Ensure the ID is an exact match
		j := raw.(*structs.Job)
		if j.ID != id {
			continue
		}

		all = append(all, j)
	}

	// Sort in reverse order so that the highest version is first
	sort.Slice(all, func(i, j int) bool {
		return all[i].Version > all[j].Version
	})

	return all, nil
}

// JobByIDAndVersion returns the job identified by its ID and Version. The
// passed watchset may be nil.
func (s *StateStore) JobByIDAndVersion(ws memdb.WatchSet, namespace, id string, version uint64) (*structs.Job, error) {
	txn := s.db.Txn(false)
	return s.jobByIDAndVersionImpl(ws, namespace, id, version, txn)
}

// jobByIDAndVersionImpl returns the job identified by its ID and Version. The
// passed watchset may be nil.
func (s *StateStore) jobByIDAndVersionImpl(ws memdb.WatchSet, namespace, id string,
	version uint64, txn *memdb.Txn) (*structs.Job, error) {

	watchCh, existing, err := txn.FirstWatch("job_version", "id", namespace, id, version)
	if err != nil {
		return nil, err
	}

	if ws != nil {
		ws.Add(watchCh)
	}

	if existing != nil {
		job := existing.(*structs.Job)
		return job, nil
	}

	return nil, nil
}

func (s *StateStore) JobVersions(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire deployments table
	iter, err := txn.Get("job_version", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// Jobs returns an iterator over all the jobs
func (s *StateStore) Jobs(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire jobs table
	iter, err := txn.Get("jobs", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobsByNamespace returns an iterator over all the jobs for the given namespace
func (s *StateStore) JobsByNamespace(ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)
	return s.jobsByNamespaceImpl(ws, namespace, txn)
}

// jobsByNamespaceImpl returns an iterator over all the jobs for the given namespace
func (s *StateStore) jobsByNamespaceImpl(ws memdb.WatchSet, namespace string, txn *memdb.Txn) (memdb.ResultIterator, error) {
	// Walk the entire jobs table
	iter, err := txn.Get("jobs", "id_prefix", namespace, "")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobsByPeriodic returns an iterator over all the periodic or non-periodic jobs.
func (s *StateStore) JobsByPeriodic(ws memdb.WatchSet, periodic bool) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("jobs", "periodic", periodic)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobsByScheduler returns an iterator over all the jobs with the specific
// scheduler type.
func (s *StateStore) JobsByScheduler(ws memdb.WatchSet, schedulerType string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Return an iterator for jobs with the specific type.
	iter, err := txn.Get("jobs", "type", schedulerType)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobsByGC returns an iterator over all jobs eligible or uneligible for garbage
// collection.
func (s *StateStore) JobsByGC(ws memdb.WatchSet, gc bool) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("jobs", "gc", gc)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobSummary returns a job summary object which matches a specific id.
func (s *StateStore) JobSummaryByID(ws memdb.WatchSet, namespace, jobID string) (*structs.JobSummary, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("job_summary", "id", namespace, jobID)
	if err != nil {
		return nil, err
	}

	ws.Add(watchCh)

	if existing != nil {
		summary := existing.(*structs.JobSummary)
		return summary, nil
	}

	return nil, nil
}

// JobSummaries walks the entire job summary table and returns all the job
// summary objects
func (s *StateStore) JobSummaries(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("job_summary", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobSummaryByPrefix is used to look up Job Summary by id prefix
func (s *StateStore) JobSummaryByPrefix(ws memdb.WatchSet, namespace, id string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("job_summary", "id_prefix", namespace, id)
	if err != nil {
		return nil, fmt.Errorf("eval lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// UpsertPeriodicLaunch is used to register a launch or update it.
func (s *StateStore) UpsertPeriodicLaunch(index uint64, launch *structs.PeriodicLaunch) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Check if the job already exists
	existing, err := txn.First("periodic_launch", "id", launch.Namespace, launch.ID)
	if err != nil {
		return fmt.Errorf("periodic launch lookup failed: %v", err)
	}

	// Setup the indexes correctly
	if existing != nil {
		launch.CreateIndex = existing.(*structs.PeriodicLaunch).CreateIndex
		launch.ModifyIndex = index
	} else {
		launch.CreateIndex = index
		launch.ModifyIndex = index
	}

	// Insert the job
	if err := txn.Insert("periodic_launch", launch); err != nil {
		return fmt.Errorf("launch insert failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"periodic_launch", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// DeletePeriodicLaunch is used to delete the periodic launch
func (s *StateStore) DeletePeriodicLaunch(index uint64, namespace, jobID string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	err := s.DeletePeriodicLaunchTxn(index, namespace, jobID, txn)
	if err == nil {
		txn.Commit()
	}
	return err
}

// DeletePeriodicLaunchTxn is used to delete the periodic launch, like DeletePeriodicLaunch
// but in a transaction.  Useful for when making multiple modifications atomically
func (s *StateStore) DeletePeriodicLaunchTxn(index uint64, namespace, jobID string, txn Txn) error {
	// Lookup the launch
	existing, err := txn.First("periodic_launch", "id", namespace, jobID)
	if err != nil {
		return fmt.Errorf("launch lookup failed: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("launch not found")
	}

	// Delete the launch
	if err := txn.Delete("periodic_launch", existing); err != nil {
		return fmt.Errorf("launch delete failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"periodic_launch", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// PeriodicLaunchByID is used to lookup a periodic launch by the periodic job
// ID.
func (s *StateStore) PeriodicLaunchByID(ws memdb.WatchSet, namespace, id string) (*structs.PeriodicLaunch, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("periodic_launch", "id", namespace, id)
	if err != nil {
		return nil, fmt.Errorf("periodic launch lookup failed: %v", err)
	}

	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.PeriodicLaunch), nil
	}
	return nil, nil
}

// PeriodicLaunches returns an iterator over all the periodic launches
func (s *StateStore) PeriodicLaunches(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire table
	iter, err := txn.Get("periodic_launch", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// UpsertEvals is used to upsert a set of evaluations
func (s *StateStore) UpsertEvals(index uint64, evals []*structs.Evaluation) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	err := s.UpsertEvalsTxn(index, evals, txn)
	if err == nil {
		txn.Commit()
	}
	return err
}

// UpsertEvals is used to upsert a set of evaluations, like UpsertEvals
// but in a transaction.  Useful for when making multiple modifications atomically
func (s *StateStore) UpsertEvalsTxn(index uint64, evals []*structs.Evaluation, txn Txn) error {
	// Do a nested upsert
	jobs := make(map[structs.NamespacedID]string, len(evals))
	for _, eval := range evals {
		if err := s.nestedUpsertEval(txn, index, eval); err != nil {
			return err
		}

		tuple := structs.NamespacedID{
			ID:        eval.JobID,
			Namespace: eval.Namespace,
		}
		jobs[tuple] = ""
	}

	// Set the job's status
	if err := s.setJobStatuses(index, txn, jobs, false); err != nil {
		return fmt.Errorf("setting job status failed: %v", err)
	}

	return nil
}

// nestedUpsertEvaluation is used to nest an evaluation upsert within a transaction
func (s *StateStore) nestedUpsertEval(txn *memdb.Txn, index uint64, eval *structs.Evaluation) error {
	// Lookup the evaluation
	existing, err := txn.First("evals", "id", eval.ID)
	if err != nil {
		return fmt.Errorf("eval lookup failed: %v", err)
	}

	// Update the indexes
	if existing != nil {
		eval.CreateIndex = existing.(*structs.Evaluation).CreateIndex
		eval.ModifyIndex = index
	} else {
		eval.CreateIndex = index
		eval.ModifyIndex = index
	}

	// Update the job summary
	summaryRaw, err := txn.First("job_summary", "id", eval.Namespace, eval.JobID)
	if err != nil {
		return fmt.Errorf("job summary lookup failed: %v", err)
	}
	if summaryRaw != nil {
		js := summaryRaw.(*structs.JobSummary).Copy()
		hasSummaryChanged := false
		for tg, num := range eval.QueuedAllocations {
			if summary, ok := js.Summary[tg]; ok {
				if summary.Queued != num {
					summary.Queued = num
					js.Summary[tg] = summary
					hasSummaryChanged = true
				}
			} else {
				s.logger.Error("unable to update queued for job and task group", "job_id", eval.JobID, "task_group", tg, "namespace", eval.Namespace)
			}
		}

		// Insert the job summary
		if hasSummaryChanged {
			js.ModifyIndex = index
			if err := txn.Insert("job_summary", js); err != nil {
				return fmt.Errorf("job summary insert failed: %v", err)
			}
			if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
				return fmt.Errorf("index update failed: %v", err)
			}
		}
	}

	// Check if the job has any blocked evaluations and cancel them
	if eval.Status == structs.EvalStatusComplete && len(eval.FailedTGAllocs) == 0 {
		// Get the blocked evaluation for a job if it exists
		iter, err := txn.Get("evals", "job", eval.Namespace, eval.JobID, structs.EvalStatusBlocked)
		if err != nil {
			return fmt.Errorf("failed to get blocked evals for job %q in namespace %q: %v", eval.JobID, eval.Namespace, err)
		}

		var blocked []*structs.Evaluation
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			blocked = append(blocked, raw.(*structs.Evaluation))
		}

		// Go through and update the evals
		for _, eval := range blocked {
			newEval := eval.Copy()
			newEval.Status = structs.EvalStatusCancelled
			newEval.StatusDescription = fmt.Sprintf("evaluation %q successful", newEval.ID)
			newEval.ModifyIndex = index

			if err := txn.Insert("evals", newEval); err != nil {
				return fmt.Errorf("eval insert failed: %v", err)
			}
		}
	}

	// Insert the eval
	if err := txn.Insert("evals", eval); err != nil {
		return fmt.Errorf("eval insert failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"evals", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	return nil
}

// updateEvalModifyIndex is used to update the modify index of an evaluation that has been
// through a scheduler pass. This is done as part of plan apply. It ensures that when a subsequent
// scheduler workers process a re-queued evaluation it sees any partial updates from the plan apply.
func (s *StateStore) updateEvalModifyIndex(txn *memdb.Txn, index uint64, evalID string) error {
	// Lookup the evaluation
	existing, err := txn.First("evals", "id", evalID)
	if err != nil {
		return fmt.Errorf("eval lookup failed: %v", err)
	}
	if existing == nil {
		s.logger.Error("unable to find eval", "eval_id", evalID)
		return fmt.Errorf("unable to find eval id %q", evalID)
	}
	eval := existing.(*structs.Evaluation).Copy()
	// Update the indexes
	eval.ModifyIndex = index

	// Insert the eval
	if err := txn.Insert("evals", eval); err != nil {
		return fmt.Errorf("eval insert failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"evals", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	return nil
}

// DeleteEval is used to delete an evaluation
func (s *StateStore) DeleteEval(index uint64, evals []string, allocs []string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	jobs := make(map[structs.NamespacedID]string, len(evals))
	for _, eval := range evals {
		existing, err := txn.First("evals", "id", eval)
		if err != nil {
			return fmt.Errorf("eval lookup failed: %v", err)
		}
		if existing == nil {
			continue
		}
		if err := txn.Delete("evals", existing); err != nil {
			return fmt.Errorf("eval delete failed: %v", err)
		}
		eval := existing.(*structs.Evaluation)

		tuple := structs.NamespacedID{
			ID:        eval.JobID,
			Namespace: eval.Namespace,
		}
		jobs[tuple] = ""
	}

	for _, alloc := range allocs {
		raw, err := txn.First("allocs", "id", alloc)
		if err != nil {
			return fmt.Errorf("alloc lookup failed: %v", err)
		}
		if raw == nil {
			continue
		}
		if err := txn.Delete("allocs", raw); err != nil {
			return fmt.Errorf("alloc delete failed: %v", err)
		}
	}

	// Update the indexes
	if err := txn.Insert("index", &IndexEntry{"evals", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"allocs", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// Set the job's status
	if err := s.setJobStatuses(index, txn, jobs, true); err != nil {
		return fmt.Errorf("setting job status failed: %v", err)
	}

	txn.Commit()
	return nil
}

// EvalByID is used to lookup an eval by its ID
func (s *StateStore) EvalByID(ws memdb.WatchSet, id string) (*structs.Evaluation, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("evals", "id", id)
	if err != nil {
		return nil, fmt.Errorf("eval lookup failed: %v", err)
	}

	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.Evaluation), nil
	}
	return nil, nil
}

// EvalsByIDPrefix is used to lookup evaluations by prefix in a particular
// namespace
func (s *StateStore) EvalsByIDPrefix(ws memdb.WatchSet, namespace, id string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Get an iterator over all evals by the id prefix
	iter, err := txn.Get("evals", "id_prefix", id)
	if err != nil {
		return nil, fmt.Errorf("eval lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	// Wrap the iterator in a filter
	wrap := memdb.NewFilterIterator(iter, evalNamespaceFilter(namespace))
	return wrap, nil
}

// evalNamespaceFilter returns a filter function that filters all evaluations
// not in the given namespace.
func evalNamespaceFilter(namespace string) func(interface{}) bool {
	return func(raw interface{}) bool {
		eval, ok := raw.(*structs.Evaluation)
		if !ok {
			return true
		}

		return eval.Namespace != namespace
	}
}

// EvalsByJob returns all the evaluations by job id
func (s *StateStore) EvalsByJob(ws memdb.WatchSet, namespace, jobID string) ([]*structs.Evaluation, error) {
	txn := s.db.Txn(false)

	// Get an iterator over the node allocations
	iter, err := txn.Get("evals", "job_prefix", namespace, jobID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var out []*structs.Evaluation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		e := raw.(*structs.Evaluation)

		// Filter non-exact matches
		if e.JobID != jobID {
			continue
		}

		out = append(out, e)
	}
	return out, nil
}

// Evals returns an iterator over all the evaluations
func (s *StateStore) Evals(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire table
	iter, err := txn.Get("evals", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// EvalsByNamespace returns an iterator over all the evaluations in the given
// namespace
func (s *StateStore) EvalsByNamespace(ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire table
	iter, err := txn.Get("evals", "namespace", namespace)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// UpdateAllocsFromClient is used to update an allocation based on input
// from a client. While the schedulers are the authority on the allocation for
// most things, some updates are authoritative from the client. Specifically,
// the desired state comes from the schedulers, while the actual state comes
// from clients.
func (s *StateStore) UpdateAllocsFromClient(index uint64, allocs []*structs.Allocation) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Handle each of the updated allocations
	for _, alloc := range allocs {
		if err := s.nestedUpdateAllocFromClient(txn, index, alloc); err != nil {
			return err
		}
	}

	// Update the indexes
	if err := txn.Insert("index", &IndexEntry{"allocs", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// nestedUpdateAllocFromClient is used to nest an update of an allocation with client status
func (s *StateStore) nestedUpdateAllocFromClient(txn *memdb.Txn, index uint64, alloc *structs.Allocation) error {
	// Look for existing alloc
	existing, err := txn.First("allocs", "id", alloc.ID)
	if err != nil {
		return fmt.Errorf("alloc lookup failed: %v", err)
	}

	// Nothing to do if this does not exist
	if existing == nil {
		return nil
	}
	exist := existing.(*structs.Allocation)

	// Copy everything from the existing allocation
	copyAlloc := exist.Copy()

	// Pull in anything the client is the authority on
	copyAlloc.ClientStatus = alloc.ClientStatus
	copyAlloc.ClientDescription = alloc.ClientDescription
	copyAlloc.TaskStates = alloc.TaskStates

	// The client can only set its deployment health and timestamp, so just take
	// those
	if copyAlloc.DeploymentStatus != nil && alloc.DeploymentStatus != nil {
		oldHasHealthy := copyAlloc.DeploymentStatus.HasHealth()
		newHasHealthy := alloc.DeploymentStatus.HasHealth()

		// We got new health information from the client
		if newHasHealthy && (!oldHasHealthy || *copyAlloc.DeploymentStatus.Healthy != *alloc.DeploymentStatus.Healthy) {
			// Updated deployment health and timestamp
			copyAlloc.DeploymentStatus.Healthy = helper.BoolToPtr(*alloc.DeploymentStatus.Healthy)
			copyAlloc.DeploymentStatus.Timestamp = alloc.DeploymentStatus.Timestamp
			copyAlloc.DeploymentStatus.ModifyIndex = index
		}
	} else if alloc.DeploymentStatus != nil {
		// First time getting a deployment status so copy everything and just
		// set the index
		copyAlloc.DeploymentStatus = alloc.DeploymentStatus.Copy()
		copyAlloc.DeploymentStatus.ModifyIndex = index
	}

	// Update the modify index
	copyAlloc.ModifyIndex = index

	// Update the modify time
	copyAlloc.ModifyTime = alloc.ModifyTime

	if err := s.updateDeploymentWithAlloc(index, copyAlloc, exist, txn); err != nil {
		return fmt.Errorf("error updating deployment: %v", err)
	}

	if err := s.updateSummaryWithAlloc(index, copyAlloc, exist, txn); err != nil {
		return fmt.Errorf("error updating job summary: %v", err)
	}

	if err := s.updateEntWithAlloc(index, copyAlloc, exist, txn); err != nil {
		return err
	}

	// Update the allocation
	if err := txn.Insert("allocs", copyAlloc); err != nil {
		return fmt.Errorf("alloc insert failed: %v", err)
	}

	// Set the job's status
	forceStatus := ""
	if !copyAlloc.TerminalStatus() {
		forceStatus = structs.JobStatusRunning
	}

	tuple := structs.NamespacedID{
		ID:        exist.JobID,
		Namespace: exist.Namespace,
	}
	jobs := map[structs.NamespacedID]string{tuple: forceStatus}

	if err := s.setJobStatuses(index, txn, jobs, false); err != nil {
		return fmt.Errorf("setting job status failed: %v", err)
	}
	return nil
}

// UpsertAllocs is used to evict a set of allocations and allocate new ones at
// the same time.
func (s *StateStore) UpsertAllocs(index uint64, allocs []*structs.Allocation) error {
	txn := s.db.Txn(true)
	defer txn.Abort()
	if err := s.upsertAllocsImpl(index, allocs, txn); err != nil {
		return err
	}
	txn.Commit()
	return nil
}

// upsertAllocs is the actual implementation of UpsertAllocs so that it may be
// used with an existing transaction.
func (s *StateStore) upsertAllocsImpl(index uint64, allocs []*structs.Allocation, txn *memdb.Txn) error {
	// Handle the allocations
	jobs := make(map[structs.NamespacedID]string, 1)
	for _, alloc := range allocs {
		existing, err := txn.First("allocs", "id", alloc.ID)
		if err != nil {
			return fmt.Errorf("alloc lookup failed: %v", err)
		}
		exist, _ := existing.(*structs.Allocation)

		if exist == nil {
			alloc.CreateIndex = index
			alloc.ModifyIndex = index
			alloc.AllocModifyIndex = index
			if alloc.DeploymentStatus != nil {
				alloc.DeploymentStatus.ModifyIndex = index
			}

			// Issue https://github.com/hashicorp/nomad/issues/2583 uncovered
			// the a race between a forced garbage collection and the scheduler
			// marking an allocation as terminal. The issue is that the
			// allocation from the scheduler has its job normalized and the FSM
			// will only denormalize if the allocation is not terminal.  However
			// if the allocation is garbage collected, that will result in a
			// allocation being upserted for the first time without a job
			// attached. By returning an error here, it will cause the FSM to
			// error, causing the plan_apply to error and thus causing the
			// evaluation to be failed. This will force an index refresh that
			// should solve this issue.
			if alloc.Job == nil {
				return fmt.Errorf("attempting to upsert allocation %q without a job", alloc.ID)
			}
		} else {
			alloc.CreateIndex = exist.CreateIndex
			alloc.ModifyIndex = index
			alloc.AllocModifyIndex = index

			// Keep the clients task states
			alloc.TaskStates = exist.TaskStates

			// If the scheduler is marking this allocation as lost we do not
			// want to reuse the status of the existing allocation.
			if alloc.ClientStatus != structs.AllocClientStatusLost {
				alloc.ClientStatus = exist.ClientStatus
				alloc.ClientDescription = exist.ClientDescription
			}

			// The job has been denormalized so re-attach the original job
			if alloc.Job == nil {
				alloc.Job = exist.Job
			}
		}

		// OPTIMIZATION:
		// These should be given a map of new to old allocation and the updates
		// should be one on all changes. The current implementation causes O(n)
		// lookups/copies/insertions rather than O(1)
		if err := s.updateDeploymentWithAlloc(index, alloc, exist, txn); err != nil {
			return fmt.Errorf("error updating deployment: %v", err)
		}

		if err := s.updateSummaryWithAlloc(index, alloc, exist, txn); err != nil {
			return fmt.Errorf("error updating job summary: %v", err)
		}

		if err := s.updateEntWithAlloc(index, alloc, exist, txn); err != nil {
			return err
		}

		if err := txn.Insert("allocs", alloc); err != nil {
			return fmt.Errorf("alloc insert failed: %v", err)
		}

		if alloc.PreviousAllocation != "" {
			prevAlloc, err := txn.First("allocs", "id", alloc.PreviousAllocation)
			if err != nil {
				return fmt.Errorf("alloc lookup failed: %v", err)
			}
			existingPrevAlloc, _ := prevAlloc.(*structs.Allocation)
			if existingPrevAlloc != nil {
				prevAllocCopy := existingPrevAlloc.Copy()
				prevAllocCopy.NextAllocation = alloc.ID
				prevAllocCopy.ModifyIndex = index
				if err := txn.Insert("allocs", prevAllocCopy); err != nil {
					return fmt.Errorf("alloc insert failed: %v", err)
				}
			}
		}

		// If the allocation is running, force the job to running status.
		forceStatus := ""
		if !alloc.TerminalStatus() {
			forceStatus = structs.JobStatusRunning
		}

		tuple := structs.NamespacedID{
			ID:        alloc.JobID,
			Namespace: alloc.Namespace,
		}
		jobs[tuple] = forceStatus
	}

	// Update the indexes
	if err := txn.Insert("index", &IndexEntry{"allocs", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// Set the job's status
	if err := s.setJobStatuses(index, txn, jobs, false); err != nil {
		return fmt.Errorf("setting job status failed: %v", err)
	}

	return nil
}

// UpdateAllocsDesiredTransitions is used to update a set of allocations
// desired transitions.
func (s *StateStore) UpdateAllocsDesiredTransitions(index uint64, allocs map[string]*structs.DesiredTransition,
	evals []*structs.Evaluation) error {

	txn := s.db.Txn(true)
	defer txn.Abort()

	// Handle each of the updated allocations
	for id, transition := range allocs {
		if err := s.nestedUpdateAllocDesiredTransition(txn, index, id, transition); err != nil {
			return err
		}
	}

	for _, eval := range evals {
		if err := s.nestedUpsertEval(txn, index, eval); err != nil {
			return err
		}
	}

	// Update the indexes
	if err := txn.Insert("index", &IndexEntry{"allocs", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// nestedUpdateAllocDesiredTransition is used to nest an update of an
// allocations desired transition
func (s *StateStore) nestedUpdateAllocDesiredTransition(
	txn *memdb.Txn, index uint64, allocID string,
	transition *structs.DesiredTransition) error {

	// Look for existing alloc
	existing, err := txn.First("allocs", "id", allocID)
	if err != nil {
		return fmt.Errorf("alloc lookup failed: %v", err)
	}

	// Nothing to do if this does not exist
	if existing == nil {
		return nil
	}
	exist := existing.(*structs.Allocation)

	// Copy everything from the existing allocation
	copyAlloc := exist.Copy()

	// Merge the desired transitions
	copyAlloc.DesiredTransition.Merge(transition)

	// Update the modify index
	copyAlloc.ModifyIndex = index

	// Update the allocation
	if err := txn.Insert("allocs", copyAlloc); err != nil {
		return fmt.Errorf("alloc insert failed: %v", err)
	}

	return nil
}

// AllocByID is used to lookup an allocation by its ID
func (s *StateStore) AllocByID(ws memdb.WatchSet, id string) (*structs.Allocation, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("allocs", "id", id)
	if err != nil {
		return nil, fmt.Errorf("alloc lookup failed: %v", err)
	}

	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.Allocation), nil
	}
	return nil, nil
}

// AllocsByIDPrefix is used to lookup allocs by prefix
func (s *StateStore) AllocsByIDPrefix(ws memdb.WatchSet, namespace, id string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("allocs", "id_prefix", id)
	if err != nil {
		return nil, fmt.Errorf("alloc lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	// Wrap the iterator in a filter
	wrap := memdb.NewFilterIterator(iter, allocNamespaceFilter(namespace))
	return wrap, nil
}

// allocNamespaceFilter returns a filter function that filters all allocations
// not in the given namespace.
func allocNamespaceFilter(namespace string) func(interface{}) bool {
	return func(raw interface{}) bool {
		alloc, ok := raw.(*structs.Allocation)
		if !ok {
			return true
		}

		return alloc.Namespace != namespace
	}
}

// AllocsByNode returns all the allocations by node
func (s *StateStore) AllocsByNode(ws memdb.WatchSet, node string) ([]*structs.Allocation, error) {
	txn := s.db.Txn(false)

	// Get an iterator over the node allocations, using only the
	// node prefix which ignores the terminal status
	iter, err := txn.Get("allocs", "node_prefix", node)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var out []*structs.Allocation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Allocation))
	}
	return out, nil
}

// AllocsByNode returns all the allocations by node and terminal status
func (s *StateStore) AllocsByNodeTerminal(ws memdb.WatchSet, node string, terminal bool) ([]*structs.Allocation, error) {
	txn := s.db.Txn(false)

	// Get an iterator over the node allocations
	iter, err := txn.Get("allocs", "node", node, terminal)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var out []*structs.Allocation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Allocation))
	}
	return out, nil
}

// AllocsByJob returns all the allocations by job id
func (s *StateStore) AllocsByJob(ws memdb.WatchSet, namespace, jobID string, all bool) ([]*structs.Allocation, error) {
	txn := s.db.Txn(false)

	// Get the job
	var job *structs.Job
	rawJob, err := txn.First("jobs", "id", namespace, jobID)
	if err != nil {
		return nil, err
	}
	if rawJob != nil {
		job = rawJob.(*structs.Job)
	}

	// Get an iterator over the node allocations
	iter, err := txn.Get("allocs", "job", namespace, jobID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var out []*structs.Allocation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		alloc := raw.(*structs.Allocation)
		// If the allocation belongs to a job with the same ID but a different
		// create index and we are not getting all the allocations whose Jobs
		// matches the same Job ID then we skip it
		if !all && job != nil && alloc.Job.CreateIndex != job.CreateIndex {
			continue
		}
		out = append(out, raw.(*structs.Allocation))
	}
	return out, nil
}

// AllocsByEval returns all the allocations by eval id
func (s *StateStore) AllocsByEval(ws memdb.WatchSet, evalID string) ([]*structs.Allocation, error) {
	txn := s.db.Txn(false)

	// Get an iterator over the eval allocations
	iter, err := txn.Get("allocs", "eval", evalID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var out []*structs.Allocation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Allocation))
	}
	return out, nil
}

// AllocsByDeployment returns all the allocations by deployment id
func (s *StateStore) AllocsByDeployment(ws memdb.WatchSet, deploymentID string) ([]*structs.Allocation, error) {
	txn := s.db.Txn(false)

	// Get an iterator over the deployments allocations
	iter, err := txn.Get("allocs", "deployment", deploymentID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var out []*structs.Allocation
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.Allocation))
	}
	return out, nil
}

// Allocs returns an iterator over all the evaluations
func (s *StateStore) Allocs(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire table
	iter, err := txn.Get("allocs", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// AllocsByNamespace returns an iterator over all the allocations in the
// namespace
func (s *StateStore) AllocsByNamespace(ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)
	return s.allocsByNamespaceImpl(ws, txn, namespace)
}

// allocsByNamespaceImpl returns an iterator over all the allocations in the
// namespace
func (s *StateStore) allocsByNamespaceImpl(ws memdb.WatchSet, txn *memdb.Txn, namespace string) (memdb.ResultIterator, error) {
	// Walk the entire table
	iter, err := txn.Get("allocs", "namespace", namespace)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// UpsertVaultAccessors is used to register a set of Vault Accessors
func (s *StateStore) UpsertVaultAccessor(index uint64, accessors []*structs.VaultAccessor) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	for _, accessor := range accessors {
		// Set the create index
		accessor.CreateIndex = index

		// Insert the accessor
		if err := txn.Insert("vault_accessors", accessor); err != nil {
			return fmt.Errorf("accessor insert failed: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"vault_accessors", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// DeleteVaultAccessors is used to delete a set of Vault Accessors
func (s *StateStore) DeleteVaultAccessors(index uint64, accessors []*structs.VaultAccessor) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Lookup the accessor
	for _, accessor := range accessors {
		// Delete the accessor
		if err := txn.Delete("vault_accessors", accessor); err != nil {
			return fmt.Errorf("accessor delete failed: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"vault_accessors", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// VaultAccessor returns the given Vault accessor
func (s *StateStore) VaultAccessor(ws memdb.WatchSet, accessor string) (*structs.VaultAccessor, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("vault_accessors", "id", accessor)
	if err != nil {
		return nil, fmt.Errorf("accessor lookup failed: %v", err)
	}

	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.VaultAccessor), nil
	}

	return nil, nil
}

// VaultAccessors returns an iterator of Vault accessors.
func (s *StateStore) VaultAccessors(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("vault_accessors", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// VaultAccessorsByAlloc returns all the Vault accessors by alloc id
func (s *StateStore) VaultAccessorsByAlloc(ws memdb.WatchSet, allocID string) ([]*structs.VaultAccessor, error) {
	txn := s.db.Txn(false)

	// Get an iterator over the accessors
	iter, err := txn.Get("vault_accessors", "alloc_id", allocID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var out []*structs.VaultAccessor
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.VaultAccessor))
	}
	return out, nil
}

// VaultAccessorsByNode returns all the Vault accessors by node id
func (s *StateStore) VaultAccessorsByNode(ws memdb.WatchSet, nodeID string) ([]*structs.VaultAccessor, error) {
	txn := s.db.Txn(false)

	// Get an iterator over the accessors
	iter, err := txn.Get("vault_accessors", "node_id", nodeID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var out []*structs.VaultAccessor
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		out = append(out, raw.(*structs.VaultAccessor))
	}
	return out, nil
}

func indexEntry(table string, index uint64) *IndexEntry {
	return &IndexEntry{
		Key:   table,
		Value: index,
	}
}

const siTokenAccessorTable = "si_token_accessors"

// UpsertSITokenAccessors is used to register a set of Service Identity token accessors.
func (s *StateStore) UpsertSITokenAccessors(index uint64, accessors []*structs.SITokenAccessor) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	for _, accessor := range accessors {
		// set the create index
		accessor.CreateIndex = index

		// insert the accessor
		if err := txn.Insert(siTokenAccessorTable, accessor); err != nil {
			return errors.Wrap(err, "accessor insert failed")
		}
	}

	// update the index for this table
	if err := txn.Insert("index", indexEntry(siTokenAccessorTable, index)); err != nil {
		return errors.Wrap(err, "index update failed")
	}

	txn.Commit()
	return nil
}

// DeleteSITokenAccessors is used to delete a set of Service Identity token accessors.
func (s *StateStore) DeleteSITokenAccessors(index uint64, accessors []*structs.SITokenAccessor) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Lookup each accessor
	for _, accessor := range accessors {
		// Delete the accessor
		if err := txn.Delete(siTokenAccessorTable, accessor); err != nil {
			return errors.Wrap(err, "accessor delete failed")
		}
	}

	// update the index for this table
	if err := txn.Insert("index", indexEntry(siTokenAccessorTable, index)); err != nil {
		return errors.Wrap(err, "index update failed")
	}

	txn.Commit()
	return nil
}

// SITokenAccessor returns the given Service Identity token accessor.
func (s *StateStore) SITokenAccessor(ws memdb.WatchSet, accessorID string) (*structs.SITokenAccessor, error) {
	txn := s.db.Txn(false)
	defer txn.Abort()

	watchCh, existing, err := txn.FirstWatch(siTokenAccessorTable, "id", accessorID)
	if err != nil {
		return nil, errors.Wrap(err, "accessor lookup failed")
	}

	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.SITokenAccessor), nil
	}

	return nil, nil
}

// SITokenAccessors returns an iterator of Service Identity token accessors.
func (s *StateStore) SITokenAccessors(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)
	defer txn.Abort()

	iter, err := txn.Get(siTokenAccessorTable, "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// SITokenAccessorsByAlloc returns all the Service Identity token accessors by alloc ID.
func (s *StateStore) SITokenAccessorsByAlloc(ws memdb.WatchSet, allocID string) ([]*structs.SITokenAccessor, error) {
	txn := s.db.Txn(false)
	defer txn.Abort()

	// Get an iterator over the accessors
	iter, err := txn.Get(siTokenAccessorTable, "alloc_id", allocID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var result []*structs.SITokenAccessor
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		result = append(result, raw.(*structs.SITokenAccessor))
	}

	return result, nil
}

// SITokenAccessorsByNode returns all the Service Identity token accessors by node ID.
func (s *StateStore) SITokenAccessorsByNode(ws memdb.WatchSet, nodeID string) ([]*structs.SITokenAccessor, error) {
	txn := s.db.Txn(false)
	defer txn.Abort()

	// Get an iterator over the accessors
	iter, err := txn.Get(siTokenAccessorTable, "node_id", nodeID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	var result []*structs.SITokenAccessor
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		result = append(result, raw.(*structs.SITokenAccessor))
	}

	return result, nil
}

// UpdateDeploymentStatus is used to make deployment status updates and
// potentially make a evaluation
func (s *StateStore) UpdateDeploymentStatus(index uint64, req *structs.DeploymentStatusUpdateRequest) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	if err := s.updateDeploymentStatusImpl(index, req.DeploymentUpdate, txn); err != nil {
		return err
	}

	// Upsert the job if necessary
	if req.Job != nil {
		if err := s.upsertJobImpl(index, req.Job, false, txn); err != nil {
			return err
		}
	}

	// Upsert the optional eval
	if req.Eval != nil {
		if err := s.nestedUpsertEval(txn, index, req.Eval); err != nil {
			return err
		}
	}

	txn.Commit()
	return nil
}

// updateDeploymentStatusImpl is used to make deployment status updates
func (s *StateStore) updateDeploymentStatusImpl(index uint64, u *structs.DeploymentStatusUpdate, txn *memdb.Txn) error {
	// Retrieve deployment
	ws := memdb.NewWatchSet()
	deployment, err := s.deploymentByIDImpl(ws, u.DeploymentID, txn)
	if err != nil {
		return err
	} else if deployment == nil {
		return fmt.Errorf("Deployment ID %q couldn't be updated as it does not exist", u.DeploymentID)
	} else if !deployment.Active() {
		return fmt.Errorf("Deployment %q has terminal status %q:", deployment.ID, deployment.Status)
	}

	// Apply the new status
	copy := deployment.Copy()
	copy.Status = u.Status
	copy.StatusDescription = u.StatusDescription
	copy.ModifyIndex = index

	// Insert the deployment
	if err := txn.Insert("deployment", copy); err != nil {
		return err
	}

	// Update the index
	if err := txn.Insert("index", &IndexEntry{"deployment", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// If the deployment is being marked as complete, set the job to stable.
	if copy.Status == structs.DeploymentStatusSuccessful {
		if err := s.updateJobStabilityImpl(index, copy.Namespace, copy.JobID, copy.JobVersion, true, txn); err != nil {
			return fmt.Errorf("failed to update job stability: %v", err)
		}
	}

	return nil
}

// UpdateJobStability updates the stability of the given job and version to the
// desired status.
func (s *StateStore) UpdateJobStability(index uint64, namespace, jobID string, jobVersion uint64, stable bool) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	if err := s.updateJobStabilityImpl(index, namespace, jobID, jobVersion, stable, txn); err != nil {
		return err
	}

	txn.Commit()
	return nil
}

// updateJobStabilityImpl updates the stability of the given job and version
func (s *StateStore) updateJobStabilityImpl(index uint64, namespace, jobID string, jobVersion uint64, stable bool, txn *memdb.Txn) error {
	// Get the job that is referenced
	job, err := s.jobByIDAndVersionImpl(nil, namespace, jobID, jobVersion, txn)
	if err != nil {
		return err
	}

	// Has already been cleared, nothing to do
	if job == nil {
		return nil
	}

	// If the job already has the desired stability, nothing to do
	if job.Stable == stable {
		return nil
	}

	copy := job.Copy()
	copy.Stable = stable
	return s.upsertJobImpl(index, copy, true, txn)
}

// UpdateDeploymentPromotion is used to promote canaries in a deployment and
// potentially make a evaluation
func (s *StateStore) UpdateDeploymentPromotion(index uint64, req *structs.ApplyDeploymentPromoteRequest) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Retrieve deployment and ensure it is not terminal and is active
	ws := memdb.NewWatchSet()
	deployment, err := s.deploymentByIDImpl(ws, req.DeploymentID, txn)
	if err != nil {
		return err
	} else if deployment == nil {
		return fmt.Errorf("Deployment ID %q couldn't be updated as it does not exist", req.DeploymentID)
	} else if !deployment.Active() {
		return fmt.Errorf("Deployment %q has terminal status %q:", deployment.ID, deployment.Status)
	}

	// Retrieve effected allocations
	iter, err := txn.Get("allocs", "deployment", req.DeploymentID)
	if err != nil {
		return err
	}

	// groupIndex is a map of groups being promoted
	groupIndex := make(map[string]struct{}, len(req.Groups))
	for _, g := range req.Groups {
		groupIndex[g] = struct{}{}
	}

	// canaryIndex is the set of placed canaries in the deployment
	canaryIndex := make(map[string]struct{}, len(deployment.TaskGroups))
	for _, state := range deployment.TaskGroups {
		for _, c := range state.PlacedCanaries {
			canaryIndex[c] = struct{}{}
		}
	}

	// healthyCounts is a mapping of group to the number of healthy canaries
	healthyCounts := make(map[string]int, len(deployment.TaskGroups))

	// promotable is the set of allocations that we can move from canary to
	// non-canary
	var promotable []*structs.Allocation

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		alloc := raw.(*structs.Allocation)

		// Check that the alloc is a canary
		if _, ok := canaryIndex[alloc.ID]; !ok {
			continue
		}

		// Check that the canary is part of a group being promoted
		if _, ok := groupIndex[alloc.TaskGroup]; !req.All && !ok {
			continue
		}

		// Ensure the canaries are healthy
		if alloc.TerminalStatus() || !alloc.DeploymentStatus.IsHealthy() {
			continue
		}

		healthyCounts[alloc.TaskGroup]++
		promotable = append(promotable, alloc)
	}

	// Determine if we have enough healthy allocations
	var unhealthyErr multierror.Error
	for tg, state := range deployment.TaskGroups {
		if _, ok := groupIndex[tg]; !req.All && !ok {
			continue
		}

		need := state.DesiredCanaries
		if need == 0 {
			continue
		}

		if have := healthyCounts[tg]; have < need {
			multierror.Append(&unhealthyErr, fmt.Errorf("Task group %q has %d/%d healthy allocations", tg, have, need))
		}
	}

	if err := unhealthyErr.ErrorOrNil(); err != nil {
		return err
	}

	// Update deployment
	copy := deployment.Copy()
	copy.ModifyIndex = index
	for tg, status := range copy.TaskGroups {
		_, ok := groupIndex[tg]
		if !req.All && !ok {
			continue
		}

		status.Promoted = true
	}

	// If the deployment no longer needs promotion, update its status
	if !copy.RequiresPromotion() && copy.Status == structs.DeploymentStatusRunning {
		copy.StatusDescription = structs.DeploymentStatusDescriptionRunning
	}

	// Insert the deployment
	if err := s.upsertDeploymentImpl(index, copy, txn); err != nil {
		return err
	}

	// Upsert the optional eval
	if req.Eval != nil {
		if err := s.nestedUpsertEval(txn, index, req.Eval); err != nil {
			return err
		}
	}

	// For each promotable allocation remove the canary field
	for _, alloc := range promotable {
		promoted := alloc.Copy()
		promoted.DeploymentStatus.Canary = false
		promoted.DeploymentStatus.ModifyIndex = index
		promoted.ModifyIndex = index
		promoted.AllocModifyIndex = index

		if err := txn.Insert("allocs", promoted); err != nil {
			return fmt.Errorf("alloc insert failed: %v", err)
		}
	}

	// Update the alloc index
	if err := txn.Insert("index", &IndexEntry{"allocs", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// UpdateDeploymentAllocHealth is used to update the health of allocations as
// part of the deployment and potentially make a evaluation
func (s *StateStore) UpdateDeploymentAllocHealth(index uint64, req *structs.ApplyDeploymentAllocHealthRequest) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Retrieve deployment and ensure it is not terminal and is active
	ws := memdb.NewWatchSet()
	deployment, err := s.deploymentByIDImpl(ws, req.DeploymentID, txn)
	if err != nil {
		return err
	} else if deployment == nil {
		return fmt.Errorf("Deployment ID %q couldn't be updated as it does not exist", req.DeploymentID)
	} else if !deployment.Active() {
		return fmt.Errorf("Deployment %q has terminal status %q:", deployment.ID, deployment.Status)
	}

	// Update the health status of each allocation
	if total := len(req.HealthyAllocationIDs) + len(req.UnhealthyAllocationIDs); total != 0 {
		setAllocHealth := func(id string, healthy bool, ts time.Time) error {
			existing, err := txn.First("allocs", "id", id)
			if err != nil {
				return fmt.Errorf("alloc %q lookup failed: %v", id, err)
			}
			if existing == nil {
				return fmt.Errorf("unknown alloc %q", id)
			}

			old := existing.(*structs.Allocation)
			if old.DeploymentID != req.DeploymentID {
				return fmt.Errorf("alloc %q is not part of deployment %q", id, req.DeploymentID)
			}

			// Set the health
			copy := old.Copy()
			if copy.DeploymentStatus == nil {
				copy.DeploymentStatus = &structs.AllocDeploymentStatus{}
			}
			copy.DeploymentStatus.Healthy = helper.BoolToPtr(healthy)
			copy.DeploymentStatus.Timestamp = ts
			copy.DeploymentStatus.ModifyIndex = index
			copy.ModifyIndex = index

			if err := s.updateDeploymentWithAlloc(index, copy, old, txn); err != nil {
				return fmt.Errorf("error updating deployment: %v", err)
			}

			if err := txn.Insert("allocs", copy); err != nil {
				return fmt.Errorf("alloc insert failed: %v", err)
			}

			return nil
		}

		for _, id := range req.HealthyAllocationIDs {
			if err := setAllocHealth(id, true, req.Timestamp); err != nil {
				return err
			}
		}
		for _, id := range req.UnhealthyAllocationIDs {
			if err := setAllocHealth(id, false, req.Timestamp); err != nil {
				return err
			}
		}

		// Update the indexes
		if err := txn.Insert("index", &IndexEntry{"allocs", index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
	}

	// Update the deployment status as needed.
	if req.DeploymentUpdate != nil {
		if err := s.updateDeploymentStatusImpl(index, req.DeploymentUpdate, txn); err != nil {
			return err
		}
	}

	// Upsert the job if necessary
	if req.Job != nil {
		if err := s.upsertJobImpl(index, req.Job, false, txn); err != nil {
			return err
		}
	}

	// Upsert the optional eval
	if req.Eval != nil {
		if err := s.nestedUpsertEval(txn, index, req.Eval); err != nil {
			return err
		}
	}

	txn.Commit()
	return nil
}

// LastIndex returns the greatest index value for all indexes
func (s *StateStore) LatestIndex() (uint64, error) {
	indexes, err := s.Indexes()
	if err != nil {
		return 0, err
	}

	var max uint64 = 0
	for {
		raw := indexes.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		idx := raw.(*IndexEntry)

		// Determine the max
		if idx.Value > max {
			max = idx.Value
		}
	}

	return max, nil
}

// Index finds the matching index value
func (s *StateStore) Index(name string) (uint64, error) {
	txn := s.db.Txn(false)

	// Lookup the first matching index
	out, err := txn.First("index", "id", name)
	if err != nil {
		return 0, err
	}
	if out == nil {
		return 0, nil
	}
	return out.(*IndexEntry).Value, nil
}

// RemoveIndex is a helper method to remove an index for testing purposes
func (s *StateStore) RemoveIndex(name string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	if _, err := txn.DeleteAll("index", "id", name); err != nil {
		return err
	}

	txn.Commit()
	return nil
}

// Indexes returns an iterator over all the indexes
func (s *StateStore) Indexes() (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire nodes table
	iter, err := txn.Get("index", "id")
	if err != nil {
		return nil, err
	}
	return iter, nil
}

// ReconcileJobSummaries re-creates summaries for all jobs present in the state
// store
func (s *StateStore) ReconcileJobSummaries(index uint64) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Get all the jobs
	iter, err := txn.Get("jobs", "id")
	if err != nil {
		return err
	}
	// COMPAT: Remove after 0.11
	// Iterate over jobs to build a list of parent jobs and their children
	parentMap := make(map[string][]*structs.Job)
	for {
		rawJob := iter.Next()
		if rawJob == nil {
			break
		}
		job := rawJob.(*structs.Job)
		if job.ParentID != "" {
			children := parentMap[job.ParentID]
			children = append(children, job)
			parentMap[job.ParentID] = children
		}
	}

	// Get all the jobs again
	iter, err = txn.Get("jobs", "id")
	if err != nil {
		return err
	}

	for {
		rawJob := iter.Next()
		if rawJob == nil {
			break
		}
		job := rawJob.(*structs.Job)

		if job.IsParameterized() || job.IsPeriodic() {
			// COMPAT: Remove after 0.11

			// The following block of code fixes incorrect child summaries due to a bug
			// See https://github.com/hashicorp/nomad/issues/3886 for details
			rawSummary, err := txn.First("job_summary", "id", job.Namespace, job.ID)
			if err != nil {
				return err
			}
			if rawSummary == nil {
				continue
			}

			oldSummary := rawSummary.(*structs.JobSummary)

			// Create an empty summary
			summary := &structs.JobSummary{
				JobID:     job.ID,
				Namespace: job.Namespace,
				Summary:   make(map[string]structs.TaskGroupSummary),
				Children:  &structs.JobChildrenSummary{},
			}

			// Iterate over children of this job if any to fix summary counts
			children := parentMap[job.ID]
			for _, childJob := range children {
				switch childJob.Status {
				case structs.JobStatusPending:
					summary.Children.Pending++
				case structs.JobStatusDead:
					summary.Children.Dead++
				case structs.JobStatusRunning:
					summary.Children.Running++
				}
			}

			// Insert the job summary if its different
			if !reflect.DeepEqual(summary, oldSummary) {
				// Set the create index of the summary same as the job's create index
				// and the modify index to the current index
				summary.CreateIndex = job.CreateIndex
				summary.ModifyIndex = index

				if err := txn.Insert("job_summary", summary); err != nil {
					return fmt.Errorf("error inserting job summary: %v", err)
				}
			}

			// Done with handling a parent job, continue to next
			continue
		}

		// Create a job summary for the job
		summary := &structs.JobSummary{
			JobID:     job.ID,
			Namespace: job.Namespace,
			Summary:   make(map[string]structs.TaskGroupSummary),
		}
		for _, tg := range job.TaskGroups {
			summary.Summary[tg.Name] = structs.TaskGroupSummary{}
		}

		// Find all the allocations for the jobs
		iterAllocs, err := txn.Get("allocs", "job", job.Namespace, job.ID)
		if err != nil {
			return err
		}

		// Calculate the summary for the job
		for {
			rawAlloc := iterAllocs.Next()
			if rawAlloc == nil {
				break
			}
			alloc := rawAlloc.(*structs.Allocation)

			// Ignore the allocation if it doesn't belong to the currently
			// registered job. The allocation is checked because of issue #2304
			if alloc.Job == nil || alloc.Job.CreateIndex != job.CreateIndex {
				continue
			}

			tg := summary.Summary[alloc.TaskGroup]
			switch alloc.ClientStatus {
			case structs.AllocClientStatusFailed:
				tg.Failed += 1
			case structs.AllocClientStatusLost:
				tg.Lost += 1
			case structs.AllocClientStatusComplete:
				tg.Complete += 1
			case structs.AllocClientStatusRunning:
				tg.Running += 1
			case structs.AllocClientStatusPending:
				tg.Starting += 1
			default:
				s.logger.Error("invalid client status set on allocation", "client_status", alloc.ClientStatus, "alloc_id", alloc.ID)
			}
			summary.Summary[alloc.TaskGroup] = tg
		}

		// Set the create index of the summary same as the job's create index
		// and the modify index to the current index
		summary.CreateIndex = job.CreateIndex
		summary.ModifyIndex = index

		// Insert the job summary
		if err := txn.Insert("job_summary", summary); err != nil {
			return fmt.Errorf("error inserting job summary: %v", err)
		}
	}

	// Update the indexes table for job summary
	if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	txn.Commit()
	return nil
}

// setJobStatuses is a helper for calling setJobStatus on multiple jobs by ID.
// It takes a map of job IDs to an optional forceStatus string. It returns an
// error if the job doesn't exist or setJobStatus fails.
func (s *StateStore) setJobStatuses(index uint64, txn *memdb.Txn,
	jobs map[structs.NamespacedID]string, evalDelete bool) error {
	for tuple, forceStatus := range jobs {

		existing, err := txn.First("jobs", "id", tuple.Namespace, tuple.ID)
		if err != nil {
			return fmt.Errorf("job lookup failed: %v", err)
		}

		if existing == nil {
			continue
		}

		if err := s.setJobStatus(index, txn, existing.(*structs.Job), evalDelete, forceStatus); err != nil {
			return err
		}
	}

	return nil
}

// setJobStatus sets the status of the job by looking up associated evaluations
// and allocations. evalDelete should be set to true if setJobStatus is being
// called because an evaluation is being deleted (potentially because of garbage
// collection). If forceStatus is non-empty, the job's status will be set to the
// passed status.
func (s *StateStore) setJobStatus(index uint64, txn *memdb.Txn,
	job *structs.Job, evalDelete bool, forceStatus string) error {

	// Capture the current status so we can check if there is a change
	oldStatus := job.Status
	if index == job.CreateIndex {
		oldStatus = ""
	}
	newStatus := forceStatus

	// If forceStatus is not set, compute the jobs status.
	if forceStatus == "" {
		var err error
		newStatus, err = s.getJobStatus(txn, job, evalDelete)
		if err != nil {
			return err
		}
	}

	// Fast-path if nothing has changed.
	if oldStatus == newStatus {
		return nil
	}

	// Copy and update the existing job
	updated := job.Copy()
	updated.Status = newStatus
	updated.ModifyIndex = index

	// Insert the job
	if err := txn.Insert("jobs", updated); err != nil {
		return fmt.Errorf("job insert failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"jobs", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// Update the children summary
	if updated.ParentID != "" {
		// Try to update the summary of the parent job summary
		summaryRaw, err := txn.First("job_summary", "id", updated.Namespace, updated.ParentID)
		if err != nil {
			return fmt.Errorf("unable to retrieve summary for parent job: %v", err)
		}

		// Only continue if the summary exists. It could not exist if the parent
		// job was removed
		if summaryRaw != nil {
			existing := summaryRaw.(*structs.JobSummary)
			pSummary := existing.Copy()
			if pSummary.Children == nil {
				pSummary.Children = new(structs.JobChildrenSummary)
			}

			// Determine the transition and update the correct fields
			children := pSummary.Children

			// Decrement old status
			if oldStatus != "" {
				switch oldStatus {
				case structs.JobStatusPending:
					children.Pending--
				case structs.JobStatusRunning:
					children.Running--
				case structs.JobStatusDead:
					children.Dead--
				default:
					return fmt.Errorf("unknown old job status %q", oldStatus)
				}
			}

			// Increment new status
			switch newStatus {
			case structs.JobStatusPending:
				children.Pending++
			case structs.JobStatusRunning:
				children.Running++
			case structs.JobStatusDead:
				children.Dead++
			default:
				return fmt.Errorf("unknown new job status %q", newStatus)
			}

			// Update the index
			pSummary.ModifyIndex = index

			// Insert the summary
			if err := txn.Insert("job_summary", pSummary); err != nil {
				return fmt.Errorf("job summary insert failed: %v", err)
			}
			if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
				return fmt.Errorf("index update failed: %v", err)
			}
		}
	}

	return nil
}

func (s *StateStore) getJobStatus(txn *memdb.Txn, job *structs.Job, evalDelete bool) (string, error) {
	// System, Periodic and Parameterized jobs are running until explicitly
	// stopped
	if job.Type == structs.JobTypeSystem || job.IsParameterized() || job.IsPeriodic() {
		if job.Stop {
			return structs.JobStatusDead, nil
		}

		return structs.JobStatusRunning, nil
	}

	allocs, err := txn.Get("allocs", "job", job.Namespace, job.ID)
	if err != nil {
		return "", err
	}

	// If there is a non-terminal allocation, the job is running.
	hasAlloc := false
	for alloc := allocs.Next(); alloc != nil; alloc = allocs.Next() {
		hasAlloc = true
		if !alloc.(*structs.Allocation).TerminalStatus() {
			return structs.JobStatusRunning, nil
		}
	}

	evals, err := txn.Get("evals", "job_prefix", job.Namespace, job.ID)
	if err != nil {
		return "", err
	}

	hasEval := false
	for raw := evals.Next(); raw != nil; raw = evals.Next() {
		e := raw.(*structs.Evaluation)

		// Filter non-exact matches
		if e.JobID != job.ID {
			continue
		}

		hasEval = true
		if !e.TerminalStatus() {
			return structs.JobStatusPending, nil
		}
	}

	// The job is dead if all the allocations and evals are terminal or if there
	// are no evals because of garbage collection.
	if evalDelete || hasEval || hasAlloc {
		return structs.JobStatusDead, nil
	}

	return structs.JobStatusPending, nil
}

// updateSummaryWithJob creates or updates job summaries when new jobs are
// upserted or existing ones are updated
func (s *StateStore) updateSummaryWithJob(index uint64, job *structs.Job,
	txn *memdb.Txn) error {

	// Update the job summary
	summaryRaw, err := txn.First("job_summary", "id", job.Namespace, job.ID)
	if err != nil {
		return fmt.Errorf("job summary lookup failed: %v", err)
	}

	// Get the summary or create if necessary
	var summary *structs.JobSummary
	hasSummaryChanged := false
	if summaryRaw != nil {
		summary = summaryRaw.(*structs.JobSummary).Copy()
	} else {
		summary = &structs.JobSummary{
			JobID:       job.ID,
			Namespace:   job.Namespace,
			Summary:     make(map[string]structs.TaskGroupSummary),
			Children:    new(structs.JobChildrenSummary),
			CreateIndex: index,
		}
		hasSummaryChanged = true
	}

	for _, tg := range job.TaskGroups {
		if _, ok := summary.Summary[tg.Name]; !ok {
			newSummary := structs.TaskGroupSummary{
				Complete: 0,
				Failed:   0,
				Running:  0,
				Starting: 0,
			}
			summary.Summary[tg.Name] = newSummary
			hasSummaryChanged = true
		}
	}

	// The job summary has changed, so update the modify index.
	if hasSummaryChanged {
		summary.ModifyIndex = index

		// Update the indexes table for job summary
		if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
		if err := txn.Insert("job_summary", summary); err != nil {
			return err
		}
	}

	return nil
}

// updateJobScalingPolicies upserts any scaling policies contained in the job and removes
// any previous scaling policies that were removed from the job
func (s *StateStore) updateJobScalingPolicies(index uint64, job *structs.Job, txn *memdb.Txn) error {

	ws := memdb.NewWatchSet()

	scalingPolicies := job.GetScalingPolicies()
	newTargets := map[string]struct{}{}
	for _, p := range scalingPolicies {
		newTargets[p.Target] = struct{}{}
	}
	// find existing policies that need to be deleted
	deletedPolicies := []string{}
	iter, err := s.ScalingPoliciesByJobTxn(ws, job.Namespace, job.ID, txn)
	if err != nil {
		return fmt.Errorf("ScalingPoliciesByJob lookup failed: %v", err)
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		oldPolicy := raw.(*structs.ScalingPolicy)
		if _, ok := newTargets[oldPolicy.Target]; !ok {
			deletedPolicies = append(deletedPolicies, oldPolicy.ID)
		}
	}
	err = s.DeleteScalingPoliciesTxn(index, deletedPolicies, txn)
	if err != nil {
		return fmt.Errorf("DeleteScalingPolicies of removed policies failed: %v", err)
	}

	err = s.UpsertScalingPoliciesTxn(index, scalingPolicies, txn)
	if err != nil {
		return fmt.Errorf("UpsertScalingPolicies of policies failed: %v", err)
	}

	return nil
}

// updateDeploymentWithAlloc is used to update the deployment state associated
// with the given allocation. The passed alloc may be updated if the deployment
// status has changed to capture the modify index at which it has changed.
func (s *StateStore) updateDeploymentWithAlloc(index uint64, alloc, existing *structs.Allocation, txn *memdb.Txn) error {
	// Nothing to do if the allocation is not associated with a deployment
	if alloc.DeploymentID == "" {
		return nil
	}

	// Get the deployment
	ws := memdb.NewWatchSet()
	deployment, err := s.deploymentByIDImpl(ws, alloc.DeploymentID, txn)
	if err != nil {
		return err
	}
	if deployment == nil {
		return nil
	}

	// Retrieve the deployment state object
	_, ok := deployment.TaskGroups[alloc.TaskGroup]
	if !ok {
		// If the task group isn't part of the deployment, the task group wasn't
		// part of a rolling update so nothing to do
		return nil
	}

	// Do not modify in-place. Instead keep track of what must be done
	placed := 0
	healthy := 0
	unhealthy := 0

	// If there was no existing allocation, this is a placement and we increment
	// the placement
	existingHealthSet := existing != nil && existing.DeploymentStatus.HasHealth()
	allocHealthSet := alloc.DeploymentStatus.HasHealth()
	if existing == nil || existing.DeploymentID != alloc.DeploymentID {
		placed++
	} else if !existingHealthSet && allocHealthSet {
		if *alloc.DeploymentStatus.Healthy {
			healthy++
		} else {
			unhealthy++
		}
	} else if existingHealthSet && allocHealthSet {
		// See if it has gone from healthy to unhealthy
		if *existing.DeploymentStatus.Healthy && !*alloc.DeploymentStatus.Healthy {
			healthy--
			unhealthy++
		}
	}

	// Nothing to do
	if placed == 0 && healthy == 0 && unhealthy == 0 {
		return nil
	}

	// Update the allocation's deployment status modify index
	if alloc.DeploymentStatus != nil && healthy+unhealthy != 0 {
		alloc.DeploymentStatus.ModifyIndex = index
	}

	// Create a copy of the deployment object
	deploymentCopy := deployment.Copy()
	deploymentCopy.ModifyIndex = index

	state := deploymentCopy.TaskGroups[alloc.TaskGroup]
	state.PlacedAllocs += placed
	state.HealthyAllocs += healthy
	state.UnhealthyAllocs += unhealthy

	// Ensure PlacedCanaries accurately reflects the alloc canary status
	if alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Canary {
		found := false
		for _, canary := range state.PlacedCanaries {
			if alloc.ID == canary {
				found = true
				break
			}
		}
		if !found {
			state.PlacedCanaries = append(state.PlacedCanaries, alloc.ID)
		}
	}

	// Update the progress deadline
	if pd := state.ProgressDeadline; pd != 0 {
		// If we are the first placed allocation for the deployment start the progress deadline.
		if placed != 0 && state.RequireProgressBy.IsZero() {
			// Use modify time instead of create time because we may in-place
			// update the allocation to be part of a new deployment.
			state.RequireProgressBy = time.Unix(0, alloc.ModifyTime).Add(pd)
		} else if healthy != 0 {
			if d := alloc.DeploymentStatus.Timestamp.Add(pd); d.After(state.RequireProgressBy) {
				state.RequireProgressBy = d
			}
		}
	}

	// Upsert the deployment
	if err := s.upsertDeploymentImpl(index, deploymentCopy, txn); err != nil {
		return err
	}

	return nil
}

// updateSummaryWithAlloc updates the job summary when allocations are updated
// or inserted
func (s *StateStore) updateSummaryWithAlloc(index uint64, alloc *structs.Allocation,
	existingAlloc *structs.Allocation, txn *memdb.Txn) error {

	// We don't have to update the summary if the job is missing
	if alloc.Job == nil {
		return nil
	}

	summaryRaw, err := txn.First("job_summary", "id", alloc.Namespace, alloc.JobID)
	if err != nil {
		return fmt.Errorf("unable to lookup job summary for job id %q in namespace %q: %v", alloc.JobID, alloc.Namespace, err)
	}

	if summaryRaw == nil {
		// Check if the job is de-registered
		rawJob, err := txn.First("jobs", "id", alloc.Namespace, alloc.JobID)
		if err != nil {
			return fmt.Errorf("unable to query job: %v", err)
		}

		// If the job is de-registered then we skip updating it's summary
		if rawJob == nil {
			return nil
		}

		return fmt.Errorf("job summary for job %q in namespace %q is not present", alloc.JobID, alloc.Namespace)
	}

	// Get a copy of the existing summary
	jobSummary := summaryRaw.(*structs.JobSummary).Copy()

	// Not updating the job summary because the allocation doesn't belong to the
	// currently registered job
	if jobSummary.CreateIndex != alloc.Job.CreateIndex {
		return nil
	}

	tgSummary, ok := jobSummary.Summary[alloc.TaskGroup]
	if !ok {
		return fmt.Errorf("unable to find task group in the job summary: %v", alloc.TaskGroup)
	}

	summaryChanged := false
	if existingAlloc == nil {
		switch alloc.DesiredStatus {
		case structs.AllocDesiredStatusStop, structs.AllocDesiredStatusEvict:
			s.logger.Error("new allocation inserted into state store with bad desired status",
				"alloc_id", alloc.ID, "desired_status", alloc.DesiredStatus)
		}
		switch alloc.ClientStatus {
		case structs.AllocClientStatusPending:
			tgSummary.Starting += 1
			if tgSummary.Queued > 0 {
				tgSummary.Queued -= 1
			}
			summaryChanged = true
		case structs.AllocClientStatusRunning, structs.AllocClientStatusFailed,
			structs.AllocClientStatusComplete:
			s.logger.Error("new allocation inserted into state store with bad client status",
				"alloc_id", alloc.ID, "client_status", alloc.ClientStatus)
		}
	} else if existingAlloc.ClientStatus != alloc.ClientStatus {
		// Incrementing the client of the bin of the current state
		switch alloc.ClientStatus {
		case structs.AllocClientStatusRunning:
			tgSummary.Running += 1
		case structs.AllocClientStatusFailed:
			tgSummary.Failed += 1
		case structs.AllocClientStatusPending:
			tgSummary.Starting += 1
		case structs.AllocClientStatusComplete:
			tgSummary.Complete += 1
		case structs.AllocClientStatusLost:
			tgSummary.Lost += 1
		}

		// Decrementing the count of the bin of the last state
		switch existingAlloc.ClientStatus {
		case structs.AllocClientStatusRunning:
			if tgSummary.Running > 0 {
				tgSummary.Running -= 1
			}
		case structs.AllocClientStatusPending:
			if tgSummary.Starting > 0 {
				tgSummary.Starting -= 1
			}
		case structs.AllocClientStatusLost:
			if tgSummary.Lost > 0 {
				tgSummary.Lost -= 1
			}
		case structs.AllocClientStatusFailed, structs.AllocClientStatusComplete:
		default:
			s.logger.Error("invalid old client status for allocatio",
				"alloc_id", existingAlloc.ID, "client_status", existingAlloc.ClientStatus)
		}
		summaryChanged = true
	}
	jobSummary.Summary[alloc.TaskGroup] = tgSummary

	if summaryChanged {
		jobSummary.ModifyIndex = index

		// Update the indexes table for job summary
		if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}

		if err := txn.Insert("job_summary", jobSummary); err != nil {
			return fmt.Errorf("updating job summary failed: %v", err)
		}
	}

	return nil
}

// UpsertACLPolicies is used to create or update a set of ACL policies
func (s *StateStore) UpsertACLPolicies(index uint64, policies []*structs.ACLPolicy) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	for _, policy := range policies {
		// Ensure the policy hash is non-nil. This should be done outside the state store
		// for performance reasons, but we check here for defense in depth.
		if len(policy.Hash) == 0 {
			policy.SetHash()
		}

		// Check if the policy already exists
		existing, err := txn.First("acl_policy", "id", policy.Name)
		if err != nil {
			return fmt.Errorf("policy lookup failed: %v", err)
		}

		// Update all the indexes
		if existing != nil {
			policy.CreateIndex = existing.(*structs.ACLPolicy).CreateIndex
			policy.ModifyIndex = index
		} else {
			policy.CreateIndex = index
			policy.ModifyIndex = index
		}

		// Update the policy
		if err := txn.Insert("acl_policy", policy); err != nil {
			return fmt.Errorf("upserting policy failed: %v", err)
		}
	}

	// Update the indexes tabl
	if err := txn.Insert("index", &IndexEntry{"acl_policy", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// DeleteACLPolicies deletes the policies with the given names
func (s *StateStore) DeleteACLPolicies(index uint64, names []string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Delete the policy
	for _, name := range names {
		if _, err := txn.DeleteAll("acl_policy", "id", name); err != nil {
			return fmt.Errorf("deleting acl policy failed: %v", err)
		}
	}
	if err := txn.Insert("index", &IndexEntry{"acl_policy", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	txn.Commit()
	return nil
}

// ACLPolicyByName is used to lookup a policy by name
func (s *StateStore) ACLPolicyByName(ws memdb.WatchSet, name string) (*structs.ACLPolicy, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("acl_policy", "id", name)
	if err != nil {
		return nil, fmt.Errorf("acl policy lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.ACLPolicy), nil
	}
	return nil, nil
}

// ACLPolicyByNamePrefix is used to lookup policies by prefix
func (s *StateStore) ACLPolicyByNamePrefix(ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("acl_policy", "id_prefix", prefix)
	if err != nil {
		return nil, fmt.Errorf("acl policy lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// ACLPolicies returns an iterator over all the acl policies
func (s *StateStore) ACLPolicies(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire table
	iter, err := txn.Get("acl_policy", "id")
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// UpsertACLTokens is used to create or update a set of ACL tokens
func (s *StateStore) UpsertACLTokens(index uint64, tokens []*structs.ACLToken) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	for _, token := range tokens {
		// Ensure the policy hash is non-nil. This should be done outside the state store
		// for performance reasons, but we check here for defense in depth.
		if len(token.Hash) == 0 {
			token.SetHash()
		}

		// Check if the token already exists
		existing, err := txn.First("acl_token", "id", token.AccessorID)
		if err != nil {
			return fmt.Errorf("token lookup failed: %v", err)
		}

		// Update all the indexes
		if existing != nil {
			existTK := existing.(*structs.ACLToken)
			token.CreateIndex = existTK.CreateIndex
			token.ModifyIndex = index

			// Do not allow SecretID or create time to change
			token.SecretID = existTK.SecretID
			token.CreateTime = existTK.CreateTime

		} else {
			token.CreateIndex = index
			token.ModifyIndex = index
		}

		// Update the token
		if err := txn.Insert("acl_token", token); err != nil {
			return fmt.Errorf("upserting token failed: %v", err)
		}
	}

	// Update the indexes table
	if err := txn.Insert("index", &IndexEntry{"acl_token", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	txn.Commit()
	return nil
}

// DeleteACLTokens deletes the tokens with the given accessor ids
func (s *StateStore) DeleteACLTokens(index uint64, ids []string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Delete the tokens
	for _, id := range ids {
		if _, err := txn.DeleteAll("acl_token", "id", id); err != nil {
			return fmt.Errorf("deleting acl token failed: %v", err)
		}
	}
	if err := txn.Insert("index", &IndexEntry{"acl_token", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	txn.Commit()
	return nil
}

// ACLTokenByAccessorID is used to lookup a token by accessor ID
func (s *StateStore) ACLTokenByAccessorID(ws memdb.WatchSet, id string) (*structs.ACLToken, error) {
	if id == "" {
		return nil, fmt.Errorf("acl token lookup failed: missing accessor id")
	}

	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("acl_token", "id", id)
	if err != nil {
		return nil, fmt.Errorf("acl token lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.ACLToken), nil
	}
	return nil, nil
}

// ACLTokenBySecretID is used to lookup a token by secret ID
func (s *StateStore) ACLTokenBySecretID(ws memdb.WatchSet, secretID string) (*structs.ACLToken, error) {
	if secretID == "" {
		return nil, fmt.Errorf("acl token lookup failed: missing secret id")
	}

	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("acl_token", "secret", secretID)
	if err != nil {
		return nil, fmt.Errorf("acl token lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.ACLToken), nil
	}
	return nil, nil
}

// ACLTokenByAccessorIDPrefix is used to lookup tokens by prefix
func (s *StateStore) ACLTokenByAccessorIDPrefix(ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("acl_token", "id_prefix", prefix)
	if err != nil {
		return nil, fmt.Errorf("acl token lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// ACLTokens returns an iterator over all the tokens
func (s *StateStore) ACLTokens(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire table
	iter, err := txn.Get("acl_token", "id")
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// ACLTokensByGlobal returns an iterator over all the tokens filtered by global value
func (s *StateStore) ACLTokensByGlobal(ws memdb.WatchSet, globalVal bool) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire table
	iter, err := txn.Get("acl_token", "global", globalVal)
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// CanBootstrapACLToken checks if bootstrapping is possible and returns the reset index
func (s *StateStore) CanBootstrapACLToken() (bool, uint64, error) {
	txn := s.db.Txn(false)

	// Lookup the bootstrap sentinel
	out, err := txn.First("index", "id", "acl_token_bootstrap")
	if err != nil {
		return false, 0, err
	}

	// No entry, we haven't bootstrapped yet
	if out == nil {
		return true, 0, nil
	}

	// Return the reset index if we've already bootstrapped
	return false, out.(*IndexEntry).Value, nil
}

// BootstrapACLToken is used to create an initial ACL token
func (s *StateStore) BootstrapACLTokens(index, resetIndex uint64, token *structs.ACLToken) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Check if we have already done a bootstrap
	existing, err := txn.First("index", "id", "acl_token_bootstrap")
	if err != nil {
		return fmt.Errorf("bootstrap check failed: %v", err)
	}
	if existing != nil {
		if resetIndex == 0 {
			return fmt.Errorf("ACL bootstrap already done")
		} else if resetIndex != existing.(*IndexEntry).Value {
			return fmt.Errorf("Invalid reset index for ACL bootstrap")
		}
	}

	// Update the Create/Modify time
	token.CreateIndex = index
	token.ModifyIndex = index

	// Insert the token
	if err := txn.Insert("acl_token", token); err != nil {
		return fmt.Errorf("upserting token failed: %v", err)
	}

	// Update the indexes table, prevents future bootstrap until reset
	if err := txn.Insert("index", &IndexEntry{"acl_token", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"acl_token_bootstrap", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	txn.Commit()
	return nil
}

// SchedulerConfig is used to get the current Scheduler configuration.
func (s *StateStore) SchedulerConfig() (uint64, *structs.SchedulerConfiguration, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// Get the scheduler config
	c, err := tx.First("scheduler_config", "id")
	if err != nil {
		return 0, nil, fmt.Errorf("failed scheduler config lookup: %s", err)
	}

	config, ok := c.(*structs.SchedulerConfiguration)
	if !ok {
		return 0, nil, nil
	}

	return config.ModifyIndex, config, nil
}

// SchedulerSetConfig is used to set the current Scheduler configuration.
func (s *StateStore) SchedulerSetConfig(idx uint64, config *structs.SchedulerConfiguration) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	s.schedulerSetConfigTxn(idx, tx, config)

	tx.Commit()
	return nil
}

func (s *StateStore) ClusterMetadata() (*structs.ClusterMetadata, error) {
	txn := s.db.Txn(false)
	defer txn.Abort()

	// Get the cluster metadata
	m, err := txn.First("cluster_meta", "id")
	if err != nil {
		return nil, errors.Wrap(err, "failed cluster metadata lookup")
	}

	if m != nil {
		return m.(*structs.ClusterMetadata), nil
	}

	return nil, nil
}

func (s *StateStore) ClusterSetMetadata(index uint64, meta *structs.ClusterMetadata) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	if err := s.setClusterMetadata(txn, meta); err != nil {
		return errors.Wrap(err, "set cluster metadata failed")
	}

	txn.Commit()
	return nil
}

// WithWriteTransaction executes the passed function within a write transaction,
// and returns its result.  If the invocation returns no error, the transaction
// is committed; otherwise, it's aborted.
func (s *StateStore) WithWriteTransaction(fn func(Txn) error) error {
	tx := s.db.Txn(true)
	defer tx.Abort()

	err := fn(tx)
	if err == nil {
		tx.Commit()
	}
	return err
}

// SchedulerCASConfig is used to update the scheduler configuration with a
// given Raft index. If the CAS index specified is not equal to the last observed index
// for the config, then the call is a noop.
func (s *StateStore) SchedulerCASConfig(idx, cidx uint64, config *structs.SchedulerConfiguration) (bool, error) {
	tx := s.db.Txn(true)
	defer tx.Abort()

	// Check for an existing config
	existing, err := tx.First("scheduler_config", "id")
	if err != nil {
		return false, fmt.Errorf("failed scheduler config lookup: %s", err)
	}

	// If the existing index does not match the provided CAS
	// index arg, then we shouldn't update anything and can safely
	// return early here.
	e, ok := existing.(*structs.SchedulerConfiguration)
	if !ok || (e != nil && e.ModifyIndex != cidx) {
		return false, nil
	}

	s.schedulerSetConfigTxn(idx, tx, config)

	tx.Commit()
	return true, nil
}

func (s *StateStore) schedulerSetConfigTxn(idx uint64, tx *memdb.Txn, config *structs.SchedulerConfiguration) error {
	// Check for an existing config
	existing, err := tx.First("scheduler_config", "id")
	if err != nil {
		return fmt.Errorf("failed scheduler config lookup: %s", err)
	}

	// Set the indexes.
	if existing != nil {
		config.CreateIndex = existing.(*structs.SchedulerConfiguration).CreateIndex
	} else {
		config.CreateIndex = idx
	}
	config.ModifyIndex = idx

	if err := tx.Insert("scheduler_config", config); err != nil {
		return fmt.Errorf("failed updating scheduler config: %s", err)
	}
	return nil
}

func (s *StateStore) setClusterMetadata(txn *memdb.Txn, meta *structs.ClusterMetadata) error {
	// Check for an existing config, if it exists, sanity check the cluster ID matches
	existing, err := txn.First("cluster_meta", "id")
	if err != nil {
		return fmt.Errorf("failed cluster meta lookup: %v", err)
	}

	if existing != nil {
		existingClusterID := existing.(*structs.ClusterMetadata).ClusterID
		if meta.ClusterID != existingClusterID {
			// there is a bug in cluster ID detection
			return fmt.Errorf("refusing to set new cluster id, previous: %s, new: %s", existingClusterID, meta.ClusterID)
		}
	}

	// update is technically a noop, unless someday we add more / mutable fields
	if err := txn.Insert("cluster_meta", meta); err != nil {
		return fmt.Errorf("set cluster metadata failed: %v", err)
	}

	return nil
}

// UpsertScalingPolicy is used to insert a new scaling policy.
func (s *StateStore) UpsertScalingPolicies(index uint64, scalingPolicies []*structs.ScalingPolicy) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	if err := s.UpsertScalingPoliciesTxn(index, scalingPolicies, txn); err != nil {
		return err
	}

	txn.Commit()
	return nil
}

// upsertScalingPolicy is used to insert a new scaling policy.
func (s *StateStore) UpsertScalingPoliciesTxn(index uint64, scalingPolicies []*structs.ScalingPolicy,
	txn *memdb.Txn) error {

	for _, scalingPolicy := range scalingPolicies {
		// Check if the scaling policy already exists
		existing, err := txn.First("scaling_policy", "target", scalingPolicy.Namespace, scalingPolicy.Target)
		if err != nil {
			return fmt.Errorf("scaling policy lookup failed: %v", err)
		}

		// Setup the indexes correctly
		if existing != nil {
			scalingPolicy.CreateIndex = existing.(*structs.ScalingPolicy).CreateIndex
			scalingPolicy.ModifyIndex = index
			scalingPolicy.ID = existing.(*structs.ScalingPolicy).ID
		} else {
			scalingPolicy.CreateIndex = index
			scalingPolicy.ModifyIndex = index
			if scalingPolicy.ID == "" {
				scalingPolicy.ID = uuid.Generate()
			}
		}

		// Insert the scaling policy
		if err := txn.Insert("scaling_policy", scalingPolicy); err != nil {
			return err
		}
	}

	// Update the indexes table for scaling policy
	if err := txn.Insert("index", &IndexEntry{"scaling_policy", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

func (s *StateStore) DeleteScalingPolicies(index uint64, ids []string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	err := s.DeleteScalingPoliciesTxn(index, ids, txn)
	if err == nil {
		txn.Commit()
	}

	return err
}

// DeleteScalingPolicies is used to delete a set of scaling policies by ID
func (s *StateStore) DeleteScalingPoliciesTxn(index uint64, ids []string, txn *memdb.Txn) error {
	if len(ids) == 0 {
		return nil
	}

	for _, id := range ids {
		// Lookup the scaling policy
		existing, err := txn.First("scaling_policy", "id", id)
		if err != nil {
			return fmt.Errorf("scaling policy lookup failed: %v", err)
		}
		if existing == nil {
			return fmt.Errorf("scaling policy not found")
		}

		// Delete the scaling policy
		if err := txn.Delete("scaling_policy", existing); err != nil {
			return fmt.Errorf("scaling policy delete failed: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"scaling_policy", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

func (s *StateStore) ScalingPoliciesByNamespace(ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("scaling_policy", "target_prefix", namespace)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

func (s *StateStore) ScalingPoliciesByJob(ws memdb.WatchSet, namespace, jobID string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)
	return s.ScalingPoliciesByJobTxn(ws, namespace, jobID, txn)
}

func (s *StateStore) ScalingPoliciesByJobTxn(ws memdb.WatchSet, namespace, jobID string,
	txn *memdb.Txn) (memdb.ResultIterator, error) {

	iter, err := txn.Get("scaling_policy", "job", namespace, jobID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

func (s *StateStore) ScalingPolicyByID(ws memdb.WatchSet, id string) (*structs.ScalingPolicy, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("scaling_policy", "id", id)
	if err != nil {
		return nil, fmt.Errorf("scaling_policy lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.ScalingPolicy), nil
	}

	return nil, nil
}

func (s *StateStore) ScalingPolicyByTarget(ws memdb.WatchSet, namespace, target string) (*structs.ScalingPolicy, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("scaling_policy", "target", namespace, target)
	if err != nil {
		return nil, fmt.Errorf("scaling_policy lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.ScalingPolicy), nil
	}

	return nil, nil
}

// StateSnapshot is used to provide a point-in-time snapshot
type StateSnapshot struct {
	StateStore
}

// DenormalizeAllocationsMap takes in a map of nodes to allocations, and queries the
// Allocation for each of the Allocation diffs and merges the updated attributes with
// the existing Allocation, and attaches the Job provided
func (s *StateSnapshot) DenormalizeAllocationsMap(nodeAllocations map[string][]*structs.Allocation) error {
	for nodeID, allocs := range nodeAllocations {
		denormalizedAllocs, err := s.DenormalizeAllocationSlice(allocs)
		if err != nil {
			return err
		}

		nodeAllocations[nodeID] = denormalizedAllocs
	}
	return nil
}

// DenormalizeAllocationSlice queries the Allocation for each allocation diff
// represented as an Allocation and merges the updated attributes with the existing
// Allocation, and attaches the Job provided.
func (s *StateSnapshot) DenormalizeAllocationSlice(allocs []*structs.Allocation) ([]*structs.Allocation, error) {
	allocDiffs := make([]*structs.AllocationDiff, len(allocs))
	for i, alloc := range allocs {
		allocDiffs[i] = alloc.AllocationDiff()
	}

	return s.DenormalizeAllocationDiffSlice(allocDiffs)
}

// DenormalizeAllocationDiffSlice queries the Allocation for each AllocationDiff and merges
// the updated attributes with the existing Allocation, and attaches the Job provided.
//
// This should only be called on terminal alloc, particularly stopped or preempted allocs
func (s *StateSnapshot) DenormalizeAllocationDiffSlice(allocDiffs []*structs.AllocationDiff) ([]*structs.Allocation, error) {
	// Output index for denormalized Allocations
	j := 0

	denormalizedAllocs := make([]*structs.Allocation, len(allocDiffs))
	for _, allocDiff := range allocDiffs {
		alloc, err := s.AllocByID(nil, allocDiff.ID)
		if err != nil {
			return nil, fmt.Errorf("alloc lookup failed: %v", err)
		}
		if alloc == nil {
			return nil, fmt.Errorf("alloc %v doesn't exist", allocDiff.ID)
		}

		// Merge the updates to the Allocation.  Don't update alloc.Job for terminal allocs
		// so alloc refers to the latest Job view before destruction and to ease handler implementations
		allocCopy := alloc.Copy()

		if allocDiff.PreemptedByAllocation != "" {
			allocCopy.PreemptedByAllocation = allocDiff.PreemptedByAllocation
			allocCopy.DesiredDescription = getPreemptedAllocDesiredDescription(allocDiff.PreemptedByAllocation)
			allocCopy.DesiredStatus = structs.AllocDesiredStatusEvict
		} else {
			// If alloc is a stopped alloc
			allocCopy.DesiredDescription = allocDiff.DesiredDescription
			allocCopy.DesiredStatus = structs.AllocDesiredStatusStop
			if allocDiff.ClientStatus != "" {
				allocCopy.ClientStatus = allocDiff.ClientStatus
			}
		}
		if allocDiff.ModifyTime != 0 {
			allocCopy.ModifyTime = allocDiff.ModifyTime
		}

		// Update the allocDiff in the slice to equal the denormalized alloc
		denormalizedAllocs[j] = allocCopy
		j++
	}
	// Retain only the denormalized Allocations in the slice
	denormalizedAllocs = denormalizedAllocs[:j]
	return denormalizedAllocs, nil
}

func getPreemptedAllocDesiredDescription(PreemptedByAllocID string) string {
	return fmt.Sprintf("Preempted by alloc ID %v", PreemptedByAllocID)
}

// StateRestore is used to optimize the performance when
// restoring state by only using a single large transaction
// instead of thousands of sub transactions
type StateRestore struct {
	txn *memdb.Txn
}

// Abort is used to abort the restore operation
func (s *StateRestore) Abort() {
	s.txn.Abort()
}

// Commit is used to commit the restore operation
func (s *StateRestore) Commit() {
	s.txn.Commit()
}

// NodeRestore is used to restore a node
func (r *StateRestore) NodeRestore(node *structs.Node) error {
	if err := r.txn.Insert("nodes", node); err != nil {
		return fmt.Errorf("node insert failed: %v", err)
	}
	return nil
}

// JobRestore is used to restore a job
func (r *StateRestore) JobRestore(job *structs.Job) error {
	if err := r.txn.Insert("jobs", job); err != nil {
		return fmt.Errorf("job insert failed: %v", err)
	}
	return nil
}

// EvalRestore is used to restore an evaluation
func (r *StateRestore) EvalRestore(eval *structs.Evaluation) error {
	if err := r.txn.Insert("evals", eval); err != nil {
		return fmt.Errorf("eval insert failed: %v", err)
	}
	return nil
}

// AllocRestore is used to restore an allocation
func (r *StateRestore) AllocRestore(alloc *structs.Allocation) error {
	if err := r.txn.Insert("allocs", alloc); err != nil {
		return fmt.Errorf("alloc insert failed: %v", err)
	}
	return nil
}

// IndexRestore is used to restore an index
func (r *StateRestore) IndexRestore(idx *IndexEntry) error {
	if err := r.txn.Insert("index", idx); err != nil {
		return fmt.Errorf("index insert failed: %v", err)
	}
	return nil
}

// PeriodicLaunchRestore is used to restore a periodic launch.
func (r *StateRestore) PeriodicLaunchRestore(launch *structs.PeriodicLaunch) error {
	if err := r.txn.Insert("periodic_launch", launch); err != nil {
		return fmt.Errorf("periodic launch insert failed: %v", err)
	}
	return nil
}

// JobSummaryRestore is used to restore a job summary
func (r *StateRestore) JobSummaryRestore(jobSummary *structs.JobSummary) error {
	if err := r.txn.Insert("job_summary", jobSummary); err != nil {
		return fmt.Errorf("job summary insert failed: %v", err)
	}
	return nil
}

// JobVersionRestore is used to restore a job version
func (r *StateRestore) JobVersionRestore(version *structs.Job) error {
	if err := r.txn.Insert("job_version", version); err != nil {
		return fmt.Errorf("job version insert failed: %v", err)
	}
	return nil
}

// DeploymentRestore is used to restore a deployment
func (r *StateRestore) DeploymentRestore(deployment *structs.Deployment) error {
	if err := r.txn.Insert("deployment", deployment); err != nil {
		return fmt.Errorf("deployment insert failed: %v", err)
	}
	return nil
}

// VaultAccessorRestore is used to restore a vault accessor
func (r *StateRestore) VaultAccessorRestore(accessor *structs.VaultAccessor) error {
	if err := r.txn.Insert("vault_accessors", accessor); err != nil {
		return fmt.Errorf("vault accessor insert failed: %v", err)
	}
	return nil
}

// SITokenAccessorRestore is used to restore an SI token accessor
func (r *StateRestore) SITokenAccessorRestore(accessor *structs.SITokenAccessor) error {
	if err := r.txn.Insert(siTokenAccessorTable, accessor); err != nil {
		return errors.Wrap(err, "si token accessor insert failed")
	}
	return nil
}

// ACLPolicyRestore is used to restore an ACL policy
func (r *StateRestore) ACLPolicyRestore(policy *structs.ACLPolicy) error {
	if err := r.txn.Insert("acl_policy", policy); err != nil {
		return fmt.Errorf("inserting acl policy failed: %v", err)
	}
	return nil
}

// ACLTokenRestore is used to restore an ACL token
func (r *StateRestore) ACLTokenRestore(token *structs.ACLToken) error {
	if err := r.txn.Insert("acl_token", token); err != nil {
		return fmt.Errorf("inserting acl token failed: %v", err)
	}
	return nil
}

func (r *StateRestore) SchedulerConfigRestore(schedConfig *structs.SchedulerConfiguration) error {
	if err := r.txn.Insert("scheduler_config", schedConfig); err != nil {
		return fmt.Errorf("inserting scheduler config failed: %s", err)
	}
	return nil
}

func (r *StateRestore) ClusterMetadataRestore(meta *structs.ClusterMetadata) error {
	if err := r.txn.Insert("cluster_meta", meta); err != nil {
		return fmt.Errorf("inserting cluster meta failed: %v", err)
	}
	return nil
}
