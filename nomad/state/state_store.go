// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/lib/lang"
	"github.com/hashicorp/nomad/nomad/stream"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Txn is a transaction against a state store.
// This can be a read or write transaction.
type Txn = *txn

// NodeUpsertOption represents options to configure a NodeUpsert operation.
type NodeUpsertOption uint8

const (
	// NodeUpsertWithNodePool indicates that the node pool in the node should
	// be created if it doesn't exist.
	NodeUpsertWithNodePool NodeUpsertOption = iota
)

const (
	// NodeEligibilityEventPlanRejectThreshold is the message used when the node
	// is set to ineligible due to multiple plan failures.
	// This is a preventive measure to signal scheduler workers to not consider
	// the node for future placements.
	// Plan rejections for a node are expected due to the optimistic and
	// concurrent nature of the scheduling process, but repeated failures for
	// the same node may indicate an underlying issue not detected by Nomad.
	// The plan applier keeps track of plan rejection history and will mark
	// nodes as ineligible if they cross a given threshold.
	NodeEligibilityEventPlanRejectThreshold = "Node marked as ineligible for scheduling due to multiple plan rejections, refer to https://developer.hashicorp.com/nomad/s/port-plan-failure for more information"

	// NodeRegisterEventRegistered is the message used when the node becomes
	// registered.
	NodeRegisterEventRegistered = "Node registered"

	// NodeRegisterEventReregistered is the message used when the node becomes
	// re-registered.
	NodeRegisterEventReregistered = "Node re-registered"
)

// terminate appends the go-memdb terminator character to s.
//
// We can then use the result for exact matches during prefix
// scans over compound indexes that start with s.
func terminate(s string) string {
	return s + "\x00"
}

// IndexEntry is used with the "index" table
// for managing the latest Raft index affecting a table.
type IndexEntry struct {
	Key   string
	Value uint64
}

// StateStoreConfig is used to configure a new state store
type StateStoreConfig struct {
	// Logger is used to output the state store's logs
	Logger hclog.Logger

	// Region is the region of the server embedding the state store.
	Region string

	// EnablePublisher is used to enable or disable the event publisher
	EnablePublisher bool

	// EventBufferSize configures the amount of events to hold in memory
	EventBufferSize int64

	// JobTrackedVersions is the number of historic job versions that are kept.
	JobTrackedVersions int
}

func (c *StateStoreConfig) Validate() error {
	if c.JobTrackedVersions <= 0 {
		return fmt.Errorf("JobTrackedVersions must be positive; got: %d", c.JobTrackedVersions)
	}
	return nil
}

// The StateStore is responsible for maintaining all the Nomad
// state. It is manipulated by the FSM which maintains consistency
// through the use of Raft. The goals of the StateStore are to provide
// high concurrency for read operations without blocking writes, and
// to provide write availability in the face of reads. EVERY object
// returned as a result of a read against the state store should be
// considered a constant and NEVER modified in place.
type StateStore struct {
	logger hclog.Logger
	db     *changeTrackerDB

	// config is the passed in configuration
	config *StateStoreConfig

	// abandonCh is used to signal watchers that this state store has been
	// abandoned (usually during a restore). This is only ever closed.
	abandonCh chan struct{}

	// TODO: refactor abandonCh to use a context so that both can use the same
	// cancel mechanism.
	stopEventBroker func()
}

type streamACLDelegate struct {
	s *StateStore
}

func (a *streamACLDelegate) TokenProvider() stream.ACLTokenProvider {
	resolver, _ := a.s.Snapshot()
	return resolver
}

// NewStateStore is used to create a new state store
func NewStateStore(config *StateStoreConfig) (*StateStore, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create the MemDB
	db, err := memdb.NewMemDB(stateStoreSchema())
	if err != nil {
		return nil, fmt.Errorf("state store setup failed: %v", err)
	}

	// Create the state store
	ctx, cancel := context.WithCancel(context.TODO())
	s := &StateStore{
		logger:          config.Logger.Named("state_store"),
		config:          config,
		abandonCh:       make(chan struct{}),
		stopEventBroker: cancel,
	}

	if config.EnablePublisher {
		// Create new event publisher using provided config
		broker, err := stream.NewEventBroker(ctx, &streamACLDelegate{s}, stream.EventBrokerCfg{
			EventBufferSize: config.EventBufferSize,
			Logger:          config.Logger,
		})
		if err != nil {
			return nil, fmt.Errorf("creating state store event broker %w", err)
		}
		s.db = NewChangeTrackerDB(db, broker, eventsFromChanges)
	} else {
		s.db = NewChangeTrackerDB(db, nil, noOpProcessChanges)
	}

	// Initialize the state store with the default namespace and built-in node
	// pools.
	if err := s.namespaceInit(); err != nil {
		return nil, fmt.Errorf("namespace state store initialization failed: %v", err)
	}
	if err := s.nodePoolInit(); err != nil {
		return nil, fmt.Errorf("node pool state store initialization failed: %w", err)
	}

	return s, nil
}

// NewWatchSet returns a new memdb.WatchSet that adds the state stores abandonCh
// as a watcher. This is important in that it will notify when this specific
// state store is no longer valid, usually due to a new snapshot being loaded
func (s *StateStore) NewWatchSet() memdb.WatchSet {
	ws := memdb.NewWatchSet()
	ws.Add(s.AbandonCh())
	return ws
}

func (s *StateStore) EventBroker() (*stream.EventBroker, error) {
	if s.db.publisher == nil {
		return nil, fmt.Errorf("EventBroker not configured")
	}
	return s.db.publisher, nil
}

// namespaceInit ensures the default namespace exists.
func (s *StateStore) namespaceInit() error {
	// Create the default namespace. This is safe to do every time we create the
	// state store. There are two main cases, a brand new cluster in which case
	// each server will have the same default namespace object, or a new cluster
	// in which case if the default namespace has been modified, it will be
	// overridden by the restore code path.
	defaultNs := &structs.Namespace{
		Name:        structs.DefaultNamespace,
		Description: structs.DefaultNamespaceDescription,
	}

	if err := s.UpsertNamespaces(1, []*structs.Namespace{defaultNs}); err != nil {
		return fmt.Errorf("inserting default namespace failed: %v", err)
	}

	return nil
}

// Config returns the state store configuration.
func (s *StateStore) Config() *StateStoreConfig {
	return s.config
}

// Snapshot is used to create a point in time snapshot. Because
// we use MemDB, we just need to snapshot the state of the underlying
// database.
func (s *StateStore) Snapshot() (*StateSnapshot, error) {
	memDBSnap := s.db.memdb.Snapshot()

	store := StateStore{
		logger: s.logger,
		config: s.config,
	}

	// Create a new change tracker DB that does not publish or track changes
	store.db = NewChangeTrackerDB(memDBSnap, nil, noOpProcessChanges)

	snap := &StateSnapshot{
		StateStore: store,
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
	var retries uint64
	var retryTimer *time.Timer
	var deadline time.Duration

	// XXX: Potential optimization is to set up a watch on the state
	// store's index table and only unblock via a trigger rather than
	// polling.
	for {
		// Get the states current index
		snapshotIndex, err := s.LatestIndex()
		if err != nil {
			return nil, fmt.Errorf("failed to determine state store's index: %w", err)
		}

		// We only need the FSM state to be as recent as the given index
		if snapshotIndex >= index {
			return s.Snapshot()
		}

		// Exponential back off
		if retryTimer == nil {
			// First retry, start at baseline
			retryTimer = time.NewTimer(backoffBase)
		} else {
			// Subsequent retry, reset timer
			deadline = helper.Backoff(backoffBase, backoffLimit, retries)
			retries++
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
	txn := s.db.WriteTxnRestore()
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
	s.StopEventBroker()
	close(s.abandonCh)
}

// StopEventBroker calls the cancel func for the state stores event
// publisher. It should be called during server shutdown.
func (s *StateStore) StopEventBroker() {
	s.stopEventBroker()
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
func (s *StateStore) UpsertPlanResults(msgType structs.MessageType, index uint64, results *structs.ApplyPlanResultsRequest) error {
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

	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	// Mark nodes as ineligible.
	for _, nodeID := range results.IneligibleNodes {
		s.logger.Warn("marking node as ineligible due to multiple plan rejections, refer to https://developer.hashicorp.com/nomad/s/port-plan-failure for more information", "node_id", nodeID)

		nodeEvent := structs.NewNodeEvent().
			SetSubsystem(structs.NodeEventSubsystemScheduler).
			SetMessage(NodeEligibilityEventPlanRejectThreshold)

		err := s.updateNodeEligibilityImpl(index, nodeID,
			structs.NodeSchedulingIneligible, results.UpdatedAt, nodeEvent, txn)
		if err != nil {
			return err
		}
	}

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
		alloc = alloc.Copy()
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

	return txn.Commit()
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

		// While we still rely on alloc.Resources field for quotas, we have to add
		// device info from AllocatedResources to alloc.Resources
		for _, resources := range alloc.AllocatedResources.Tasks {
			for _, d := range resources.Devices {
				name := d.ID().String()
				count := len(d.DeviceIDs)

				if count > 0 {
					if alloc.Resources.Devices == nil {
						alloc.Resources.Devices = make(structs.ResourceDevices, 0)
					}
					alloc.Resources.Devices = append(
						alloc.Resources.Devices, &structs.RequestedDevice{Name: name, Count: uint64(count)},
					)
				}
			}
		}

		// Add the shared resources
		alloc.Resources.Add(alloc.SharedResources)
	}
}

// upsertDeploymentUpdates updates the deployments given the passed status
// updates.
func (s *StateStore) upsertDeploymentUpdates(index uint64, updates []*structs.DeploymentStatusUpdate, txn *txn) error {
	for _, u := range updates {
		if err := s.updateDeploymentStatusImpl(index, u, txn); err != nil {
			return err
		}
	}

	return nil
}

// UpsertJobSummary upserts a job summary into the state store.
func (s *StateStore) UpsertJobSummary(index uint64, jobSummary *structs.JobSummary) error {
	txn := s.db.WriteTxn(index)
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

	return txn.Commit()
}

// DeleteJobSummary deletes the job summary with the given ID. This is for
// testing purposes only.
func (s *StateStore) DeleteJobSummary(index uint64, namespace, id string) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// Delete the job summary
	if _, err := txn.DeleteAll("job_summary", "id", namespace, id); err != nil {
		return fmt.Errorf("deleting job summary failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	return txn.Commit()
}

// UpsertDeployment is used to insert or update a new deployment.
func (s *StateStore) UpsertDeployment(index uint64, deployment *structs.Deployment) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()
	if err := s.upsertDeploymentImpl(index, deployment, txn); err != nil {
		return err
	}
	return txn.Commit()
}

func (s *StateStore) upsertDeploymentImpl(index uint64, deployment *structs.Deployment, txn *txn) error {
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

func (s *StateStore) Deployments(ws memdb.WatchSet, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var it memdb.ResultIterator
	var err error

	switch sort {
	case SortReverse:
		it, err = txn.GetReverse("deployment", "create")
	default:
		it, err = txn.Get("deployment", "create")
	}

	if err != nil {
		return nil, err
	}

	ws.Add(it.WatchCh())

	return it, nil
}

func (s *StateStore) DeploymentsByNamespace(ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire deployments table
	iter, err := txn.Get("deployment", "namespace", namespace)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

func (s *StateStore) DeploymentsByNamespaceOrdered(ws memdb.WatchSet, namespace string, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var (
		it    memdb.ResultIterator
		err   error
		exact = terminate(namespace)
	)

	switch sort {
	case SortReverse:
		it, err = txn.GetReverse("deployment", "namespace_create_prefix", exact)
	default:
		it, err = txn.Get("deployment", "namespace_create_prefix", exact)
	}

	if err != nil {
		return nil, err
	}

	ws.Add(it.WatchCh())

	return it, nil
}

func (s *StateStore) DeploymentsByIDPrefix(ws memdb.WatchSet, namespace, deploymentID string, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var iter memdb.ResultIterator
	var err error

	// Walk the entire deployments table
	switch sort {
	case SortReverse:
		iter, err = txn.GetReverse("deployment", "id_prefix", deploymentID)
	default:
		iter, err = txn.Get("deployment", "id_prefix", deploymentID)
	}
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

		return namespace != structs.AllNamespacesSentinel &&
			d.Namespace != namespace
	}
}

func (s *StateStore) DeploymentByID(ws memdb.WatchSet, deploymentID string) (*structs.Deployment, error) {
	txn := s.db.ReadTxn()
	return s.deploymentByIDImpl(ws, deploymentID, txn)
}

func (s *StateStore) deploymentByIDImpl(ws memdb.WatchSet, deploymentID string, txn *txn) (*structs.Deployment, error) {
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
	txn := s.db.ReadTxn()

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
	txn := s.db.ReadTxn()

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
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	err := s.DeleteDeploymentTxn(index, deploymentIDs, txn)
	if err == nil {
		return txn.Commit()
	}

	return err
}

// DeleteDeploymentTxn is used to delete a set of deployments by ID, like
// DeleteDeployment but in a transaction. Useful when making multiple
// modifications atomically.
func (s *StateStore) DeleteDeploymentTxn(index uint64, deploymentIDs []string, txn Txn) error {
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
			continue
		}

		// Delete the deployment
		if err := txn.Delete("deployment", existing); err != nil {
			return fmt.Errorf("deployment delete failed: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"deployment", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// deleteAllocsForJobTxn deletes all the allocations for a given job, ensuring
// that any associated server-side resources like quotas are also cleaned up,
// but not client-side resources like CSI volumes, which are resolved by the
// client
func (s *StateStore) deleteAllocsForJobTxn(txn Txn, index uint64, namespace, jobID string) error {

	allocs, err := s.AllocsByJob(nil, namespace, jobID, true)
	if err != nil {
		return fmt.Errorf("alloc lookup for job %s failed: %w", jobID, err)
	}

	for _, existing := range allocs {
		if !existing.ClientTerminalStatus() {
			stopped := existing.Copy()
			stopped.ClientStatus = structs.AllocClientStatusComplete
			s.updateEntWithAlloc(index, stopped, existing, txn)
		}
		if err := txn.Delete("allocs", existing); err != nil {
			return fmt.Errorf("alloc delete failed: %w", err)
		}
	}
	if err := txn.Insert("index", &IndexEntry{"allocs", index}); err != nil {
		return fmt.Errorf("index update failed: %w", err)
	}

	return nil
}

// UpsertScalingEvent is used to insert a new scaling event.
// Only the most recent JobTrackedScalingEvents will be kept.
func (s *StateStore) UpsertScalingEvent(index uint64, req *structs.ScalingEventRequest) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// Get the existing events
	existing, err := txn.First("scaling_event", "id", req.Namespace, req.JobID)
	if err != nil {
		return fmt.Errorf("scaling event lookup failed: %v", err)
	}

	var jobEvents *structs.JobScalingEvents
	if existing != nil {
		jobEvents = existing.(*structs.JobScalingEvents).Copy()
	} else {
		jobEvents = &structs.JobScalingEvents{
			Namespace:     req.Namespace,
			JobID:         req.JobID,
			ScalingEvents: make(map[string][]*structs.ScalingEvent),
		}
	}

	jobEvents.ModifyIndex = index
	req.ScalingEvent.CreateIndex = index

	events := jobEvents.ScalingEvents[req.TaskGroup]
	// Prepend this latest event
	events = append(
		[]*structs.ScalingEvent{req.ScalingEvent},
		events...,
	)
	// Truncate older events
	if len(events) > structs.JobTrackedScalingEvents {
		events = events[0:structs.JobTrackedScalingEvents]
	}
	jobEvents.ScalingEvents[req.TaskGroup] = events

	// Insert the new event
	if err := txn.Insert("scaling_event", jobEvents); err != nil {
		return fmt.Errorf("scaling event insert failed: %v", err)
	}

	// Update the indexes table for scaling_event
	if err := txn.Insert("index", &IndexEntry{"scaling_event", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// ScalingEvents returns an iterator over all the job scaling events
func (s *StateStore) ScalingEvents(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire scaling_event table
	iter, err := txn.Get("scaling_event", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

func (s *StateStore) ScalingEventsByJob(ws memdb.WatchSet, namespace, jobID string) (map[string][]*structs.ScalingEvent, uint64, error) {
	txn := s.db.ReadTxn()

	watchCh, existing, err := txn.FirstWatch("scaling_event", "id", namespace, jobID)
	if err != nil {
		return nil, 0, fmt.Errorf("job scaling events lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		events := existing.(*structs.JobScalingEvents)
		return events.ScalingEvents, events.ModifyIndex, nil
	}
	return nil, 0, nil
}

// UpsertNode is used to register a node or update a node definition
// This is assumed to be triggered by the client, so we retain the value
// of drain/eligibility which is set by the scheduler.
func (s *StateStore) UpsertNode(msgType structs.MessageType, index uint64, node *structs.Node, opts ...NodeUpsertOption) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	for _, opt := range opts {
		// Create node pool if necessary.
		if opt == NodeUpsertWithNodePool && node.NodePool != "" {
			_, err := s.fetchOrCreateNodePoolTxn(txn, index, node.NodePool)
			if err != nil {
				return err
			}
		}
	}

	err := upsertNodeTxn(txn, index, node)
	if err != nil {
		return nil
	}
	return txn.Commit()
}

func upsertNodeTxn(txn *txn, index uint64, node *structs.Node) error {
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

		// Update last missed heartbeat if the node became unresponsive.
		if !exist.UnresponsiveStatus() && node.UnresponsiveStatus() {
			node.LastMissedHeartbeatIndex = index
		}

		// Retain node events that have already been set on the node
		node.Events = exist.Events

		// If we are transitioning from down, record the re-registration
		if exist.Status == structs.NodeStatusDown && node.Status != structs.NodeStatusDown {
			appendNodeEvents(index, node, []*structs.NodeEvent{
				structs.NewNodeEvent().SetSubsystem(structs.NodeEventSubsystemCluster).
					SetMessage(NodeRegisterEventReregistered).
					SetTimestamp(time.Unix(node.StatusUpdatedAt, 0))})
		}

		node.SchedulingEligibility = exist.SchedulingEligibility // Retain the eligibility
		node.DrainStrategy = exist.DrainStrategy                 // Retain the drain strategy
		node.LastDrain = exist.LastDrain                         // Retain the drain metadata

		// Retain the last index the node missed a heartbeat.
		if node.LastMissedHeartbeatIndex < exist.LastMissedHeartbeatIndex {
			node.LastMissedHeartbeatIndex = exist.LastMissedHeartbeatIndex
		}

		// Retain the last index the node updated its allocs.
		if node.LastAllocUpdateIndex < exist.LastAllocUpdateIndex {
			node.LastAllocUpdateIndex = exist.LastAllocUpdateIndex
		}
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
	if err := upsertCSIPluginsForNode(txn, node, index); err != nil {
		return fmt.Errorf("csi plugin update failed: %v", err)
	}

	return nil
}

// DeleteNode deregisters a batch of nodes
func (s *StateStore) DeleteNode(msgType structs.MessageType, index uint64, nodes []string) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	err := deleteNodeTxn(txn, index, nodes)
	if err != nil {
		return nil
	}
	return txn.Commit()
}

func deleteNodeTxn(txn *txn, index uint64, nodes []string) error {
	if len(nodes) == 0 {
		return fmt.Errorf("node ids missing")
	}

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

		node := existing.(*structs.Node)
		if err := deleteNodeCSIPlugins(txn, node, index); err != nil {
			return fmt.Errorf("csi plugin delete failed: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// UpdateNodeStatus is used to update the status of a node
func (s *StateStore) UpdateNodeStatus(msgType structs.MessageType, index uint64, nodeID, status string, updatedAt int64, event *structs.NodeEvent) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	if err := s.updateNodeStatusTxn(txn, nodeID, status, updatedAt, event); err != nil {
		return err
	}

	return txn.Commit()
}

func (s *StateStore) updateNodeStatusTxn(txn *txn, nodeID, status string, updatedAt int64, event *structs.NodeEvent) error {

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
		appendNodeEvents(txn.Index, copyNode, []*structs.NodeEvent{event})
	}

	// Update the status in the copy
	copyNode.Status = status
	copyNode.ModifyIndex = txn.Index

	// Update last missed heartbeat if the node became unresponsive or reset it
	// zero if the node became ready.
	if !existingNode.UnresponsiveStatus() && copyNode.UnresponsiveStatus() {
		copyNode.LastMissedHeartbeatIndex = txn.Index
	} else if existingNode.Status != structs.NodeStatusReady &&
		copyNode.Status == structs.NodeStatusReady {
		copyNode.LastMissedHeartbeatIndex = 0
	}

	// Insert the node
	if err := txn.Insert("nodes", copyNode); err != nil {
		return fmt.Errorf("node update failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", txn.Index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// Deregister any services on the node in the same transaction
	if copyNode.Status == structs.NodeStatusDown {
		s.deleteServiceRegistrationByNodeIDTxn(txn, txn.Index, copyNode.ID)
	}

	return nil
}

// BatchUpdateNodeDrain is used to update the drain of a node set of nodes.
// This is currently only called when node drain is completed by the drainer.
func (s *StateStore) BatchUpdateNodeDrain(msgType structs.MessageType, index uint64, updatedAt int64,
	updates map[string]*structs.DrainUpdate, events map[string]*structs.NodeEvent) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()
	for node, update := range updates {
		if err := s.updateNodeDrainImpl(txn, index, node, update.DrainStrategy, update.MarkEligible, updatedAt,
			events[node], nil, "", true); err != nil {
			return err
		}
	}
	return txn.Commit()
}

// UpdateNodeDrain is used to update the drain of a node
func (s *StateStore) UpdateNodeDrain(msgType structs.MessageType, index uint64, nodeID string,
	drain *structs.DrainStrategy, markEligible bool, updatedAt int64,
	event *structs.NodeEvent, drainMeta map[string]string, accessorId string) error {

	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()
	if err := s.updateNodeDrainImpl(txn, index, nodeID, drain, markEligible, updatedAt, event,
		drainMeta, accessorId, false); err != nil {

		return err
	}
	return txn.Commit()
}

func (s *StateStore) updateNodeDrainImpl(txn *txn, index uint64, nodeID string,
	drain *structs.DrainStrategy, markEligible bool, updatedAt int64,
	event *structs.NodeEvent, drainMeta map[string]string, accessorId string,
	drainCompleted bool) error {

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
	updatedNode := existingNode.Copy()
	updatedNode.StatusUpdatedAt = updatedAt

	// Add the event if given
	if event != nil {
		appendNodeEvents(index, updatedNode, []*structs.NodeEvent{event})
	}

	// Update the drain in the copy
	updatedNode.DrainStrategy = drain
	if drain != nil {
		updatedNode.SchedulingEligibility = structs.NodeSchedulingIneligible
	} else if markEligible {
		updatedNode.SchedulingEligibility = structs.NodeSchedulingEligible
	}

	// Update LastDrain
	updateTime := time.Unix(updatedAt, 0)

	// if drain strategy isn't set before or after, this wasn't a drain operation
	// in that case, we don't care about .LastDrain
	drainNoop := existingNode.DrainStrategy == nil && updatedNode.DrainStrategy == nil
	// otherwise, when done with this method, updatedNode.LastDrain should be set
	// if starting a new drain operation, create a new LastDrain. otherwise, update the existing one.
	startedDraining := existingNode.DrainStrategy == nil && updatedNode.DrainStrategy != nil
	if !drainNoop {
		if startedDraining {
			updatedNode.LastDrain = &structs.DrainMetadata{
				StartedAt: updateTime,
				Meta:      drainMeta,
			}
		} else if updatedNode.LastDrain == nil {
			// if already draining and LastDrain doesn't exist, we need to create a new one
			// this could happen if we upgraded to 1.1.x during a drain
			updatedNode.LastDrain = &structs.DrainMetadata{
				// we don't have sub-second accuracy on these fields, so truncate this
				StartedAt: time.Unix(existingNode.DrainStrategy.StartedAt.Unix(), 0),
				Meta:      drainMeta,
			}
		}

		updatedNode.LastDrain.UpdatedAt = updateTime

		// won't have new metadata on drain complete; keep the existing operator-provided metadata
		// also, keep existing if they didn't provide it
		if len(drainMeta) != 0 {
			updatedNode.LastDrain.Meta = drainMeta
		}

		// we won't have an accessor ID on drain complete, so don't overwrite the existing one
		if accessorId != "" {
			updatedNode.LastDrain.AccessorID = accessorId
		}

		if updatedNode.DrainStrategy != nil {
			updatedNode.LastDrain.Status = structs.DrainStatusDraining
		} else if drainCompleted {
			updatedNode.LastDrain.Status = structs.DrainStatusComplete
		} else {
			updatedNode.LastDrain.Status = structs.DrainStatusCanceled
		}
	}

	updatedNode.ModifyIndex = index

	// Insert the node
	if err := txn.Insert("nodes", updatedNode); err != nil {
		return fmt.Errorf("node update failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// UpdateNodeEligibility is used to update the scheduling eligibility of a node
func (s *StateStore) UpdateNodeEligibility(msgType structs.MessageType, index uint64, nodeID string, eligibility string, updatedAt int64, event *structs.NodeEvent) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()
	if err := s.updateNodeEligibilityImpl(index, nodeID, eligibility, updatedAt, event, txn); err != nil {
		return err
	}
	return txn.Commit()
}

func (s *StateStore) updateNodeEligibilityImpl(index uint64, nodeID string, eligibility string, updatedAt int64, event *structs.NodeEvent, txn *txn) error {
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

	return nil
}

// UpsertNodeEvents adds the node events to the nodes, rotating events as
// necessary.
func (s *StateStore) UpsertNodeEvents(msgType structs.MessageType, index uint64, nodeEvents map[string][]*structs.NodeEvent) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	for nodeID, events := range nodeEvents {
		if err := s.upsertNodeEvents(index, nodeID, events, txn); err != nil {
			return err
		}
	}

	return txn.Commit()
}

// upsertNodeEvent upserts a node event for a respective node. It also maintains
// that a fixed number of node events are ever stored simultaneously, deleting
// older events once this bound has been reached.
func (s *StateStore) upsertNodeEvents(index uint64, nodeID string, events []*structs.NodeEvent, txn *txn) error {
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

// upsertCSIPluginsForNode indexes csi plugins for volume retrieval, with health. It's called
// on upsertNodeEvents, so that event driven health changes are updated
func upsertCSIPluginsForNode(txn *txn, node *structs.Node, index uint64) error {

	upsertFn := func(info *structs.CSIInfo) error {
		raw, err := txn.First("csi_plugins", "id", info.PluginID)
		if err != nil {
			return fmt.Errorf("csi_plugin lookup error: %s %v", info.PluginID, err)
		}

		var plug *structs.CSIPlugin
		if raw != nil {
			plug = raw.(*structs.CSIPlugin).Copy()
		} else {
			if !info.Healthy {
				// we don't want to create new plugins for unhealthy
				// allocs, otherwise we'd recreate the plugin when we
				// get the update for the alloc becoming terminal
				return nil
			}
			plug = structs.NewCSIPlugin(info.PluginID, index)
		}

		// the plugin may have been created by the job being updated, in which case
		// this data will not be configured, it's only available to the fingerprint
		// system
		plug.Provider = info.Provider
		plug.Version = info.ProviderVersion

		err = plug.AddPlugin(node.ID, info)
		if err != nil {
			return err
		}

		plug.ModifyIndex = index

		err = txn.Insert("csi_plugins", plug)
		if err != nil {
			return fmt.Errorf("csi_plugins insert error: %v", err)
		}

		return nil
	}

	inUseController := map[string]struct{}{}
	inUseNode := map[string]struct{}{}

	for _, info := range node.CSIControllerPlugins {
		err := upsertFn(info)
		if err != nil {
			return err
		}
		inUseController[info.PluginID] = struct{}{}
	}

	for _, info := range node.CSINodePlugins {
		err := upsertFn(info)
		if err != nil {
			return err
		}
		inUseNode[info.PluginID] = struct{}{}
	}

	// remove the client node from any plugin that's not
	// running on it.
	iter, err := txn.Get("csi_plugins", "id")
	if err != nil {
		return fmt.Errorf("csi_plugins lookup failed: %v", err)
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		plug, ok := raw.(*structs.CSIPlugin)
		if !ok {
			continue
		}
		plug = plug.Copy()

		var hadDelete bool
		if _, ok := inUseController[plug.ID]; !ok {
			if _, asController := plug.Controllers[node.ID]; asController {
				err := plug.DeleteNodeForType(node.ID, structs.CSIPluginTypeController)
				if err != nil {
					return err
				}
				hadDelete = true
			}
		}
		if _, ok := inUseNode[plug.ID]; !ok {
			if _, asNode := plug.Nodes[node.ID]; asNode {
				err := plug.DeleteNodeForType(node.ID, structs.CSIPluginTypeNode)
				if err != nil {
					return err
				}
				hadDelete = true
			}
		}
		// we check this flag both for performance and to make sure we
		// don't delete a plugin when registering a node plugin but
		// no controller
		if hadDelete {
			err = updateOrGCPlugin(index, txn, plug)
			if err != nil {
				return err
			}
		}
	}

	if err := txn.Insert("index", &IndexEntry{"csi_plugins", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// deleteNodeCSIPlugins cleans up CSIInfo node health status, called in DeleteNode
func deleteNodeCSIPlugins(txn *txn, node *structs.Node, index uint64) error {
	if len(node.CSIControllerPlugins) == 0 && len(node.CSINodePlugins) == 0 {
		return nil
	}

	names := map[string]struct{}{}
	for _, info := range node.CSIControllerPlugins {
		names[info.PluginID] = struct{}{}
	}
	for _, info := range node.CSINodePlugins {
		names[info.PluginID] = struct{}{}
	}

	for id := range names {
		raw, err := txn.First("csi_plugins", "id", id)
		if err != nil {
			return fmt.Errorf("csi_plugins lookup error %s: %v", id, err)
		}
		if raw == nil {
			// plugin may have been deregistered but we didn't
			// update the fingerprint yet
			continue
		}

		plug := raw.(*structs.CSIPlugin).Copy()
		err = plug.DeleteNode(node.ID)
		if err != nil {
			return err
		}
		err = updateOrGCPlugin(index, txn, plug)
		if err != nil {
			return err
		}
	}

	if err := txn.Insert("index", &IndexEntry{"csi_plugins", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// updateOrGCPlugin updates a plugin but will delete it if the plugin is empty
func updateOrGCPlugin(index uint64, txn Txn, plug *structs.CSIPlugin) error {
	if plug.IsEmpty() {
		err := txn.Delete("csi_plugins", plug)
		if err != nil {
			return fmt.Errorf("csi_plugins delete error: %v", err)
		}
	} else {
		plug.ModifyIndex = index
		err := txn.Insert("csi_plugins", plug)
		if err != nil {
			return fmt.Errorf("csi_plugins update error %s: %v", plug.ID, err)
		}
	}
	return nil
}

// deleteJobFromPlugins removes the allocations of this job from any plugins the job is
// running, possibly deleting the plugin if it's no longer in use. It's called in DeleteJobTxn
func (s *StateStore) deleteJobFromPlugins(index uint64, txn Txn, job *structs.Job) error {
	ws := memdb.NewWatchSet()
	summary, err := s.JobSummaryByID(ws, job.Namespace, job.ID)
	if err != nil {
		return fmt.Errorf("error getting job summary: %v", err)
	}

	allocs, err := s.AllocsByJob(ws, job.Namespace, job.ID, false)
	if err != nil {
		return fmt.Errorf("error getting allocations: %v", err)
	}

	type pair struct {
		pluginID string
		alloc    *structs.Allocation
	}

	plugAllocs := []*pair{}
	found := map[string]struct{}{}

	// Find plugins for allocs that belong to this job
	for _, a := range allocs {
		tg := a.Job.LookupTaskGroup(a.TaskGroup)
		found[tg.Name] = struct{}{}
		for _, t := range tg.Tasks {
			if t.CSIPluginConfig == nil {
				continue
			}
			plugAllocs = append(plugAllocs, &pair{
				pluginID: t.CSIPluginConfig.ID,
				alloc:    a,
			})
		}
	}

	// Find any plugins that do not yet have allocs for this job
	for _, tg := range job.TaskGroups {
		if _, ok := found[tg.Name]; ok {
			continue
		}

		for _, t := range tg.Tasks {
			if t.CSIPluginConfig == nil {
				continue
			}
			plugAllocs = append(plugAllocs, &pair{
				pluginID: t.CSIPluginConfig.ID,
			})
		}
	}

	plugins := map[string]*structs.CSIPlugin{}

	for _, x := range plugAllocs {
		plug, ok := plugins[x.pluginID]

		if !ok {
			plug, err = s.CSIPluginByIDTxn(txn, nil, x.pluginID)
			if err != nil {
				return fmt.Errorf("error getting plugin: %s, %v", x.pluginID, err)
			}
			if plug == nil {
				// plugin was never successfully registered or has been
				// GC'd out from under us
				continue
			}
			// only copy once, so we update the same plugin on each alloc
			plugins[x.pluginID] = plug.Copy()
			plug = plugins[x.pluginID]
		}

		if x.alloc == nil {
			continue
		}
		err := plug.DeleteAlloc(x.alloc.ID, x.alloc.NodeID)
		if err != nil {
			return err
		}
	}

	for _, plug := range plugins {
		plug.DeleteJob(job, summary)
		err = updateOrGCPlugin(index, txn, plug)
		if err != nil {
			return err
		}
	}

	if len(plugins) > 0 {
		if err = txn.Insert("index", &IndexEntry{"csi_plugins", index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
	}

	return nil
}

// NodeByID is used to lookup a node by ID
func (s *StateStore) NodeByID(ws memdb.WatchSet, nodeID string) (*structs.Node, error) {
	txn := s.db.ReadTxn()

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
	txn := s.db.ReadTxn()

	iter, err := txn.Get("nodes", "id_prefix", nodeID)
	if err != nil {
		return nil, fmt.Errorf("node lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// NodeBySecretID is used to lookup a node by SecretID
func (s *StateStore) NodeBySecretID(ws memdb.WatchSet, secretID string) (*structs.Node, error) {
	txn := s.db.ReadTxn()

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

// NodesByNodePool returns an iterator over all nodes that are part of the
// given node pool.
func (s *StateStore) NodesByNodePool(ws memdb.WatchSet, pool string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get("nodes", "node_pool", pool)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// Nodes returns an iterator over all the nodes
func (s *StateStore) Nodes(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire nodes table
	iter, err := txn.Get("nodes", "id")
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// UpsertJob is used to register a job or update a job definition
func (s *StateStore) UpsertJob(msgType structs.MessageType, index uint64, sub *structs.JobSubmission, job *structs.Job) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()
	if err := s.upsertJobImpl(index, sub, job, false, txn); err != nil {
		return err
	}
	return txn.Commit()
}

// UpsertJobTxn is used to register a job or update a job definition, like UpsertJob,
// but in a transaction.  Useful for when making multiple modifications atomically
func (s *StateStore) UpsertJobTxn(index uint64, sub *structs.JobSubmission, job *structs.Job, txn Txn) error {
	return s.upsertJobImpl(index, sub, job, false, txn)
}

// upsertJobImpl is the implementation for registering a job or updating a job definition
func (s *StateStore) upsertJobImpl(index uint64, sub *structs.JobSubmission, job *structs.Job, keepVersion bool, txn *txn) error {
	// Assert the namespace exists
	if exists, err := s.namespaceExists(txn, job.Namespace); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("job %q is in nonexistent namespace %q", job.ID, job.Namespace)
	}

	// Upgrade path.
	// Assert the node pool is set and exists.
	if job.NodePool == "" {
		job.NodePool = structs.NodePoolDefault
	}
	if exists, err := s.nodePoolExists(txn, job.NodePool); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("job %q is in nonexistent node pool %q", job.ID, job.NodePool)
	}

	// Check if the job already exists
	existing, err := txn.First("jobs", "id", job.Namespace, job.ID)
	var existingJob *structs.Job
	if err != nil {
		return fmt.Errorf("job lookup failed: %v", err)
	}

	// Setup the indexes correctly
	if existing != nil {
		job.CreateIndex = existing.(*structs.Job).CreateIndex
		job.ModifyIndex = index

		existingJob = existing.(*structs.Job)

		// Bump the version unless asked to keep it. This should only be done
		// when changing an internal field such as Stable. A spec change should
		// always come with a version bump
		if !keepVersion {
			job.JobModifyIndex = index
			if job.Version <= existingJob.Version {
				if sub == nil {
					// in the reversion case we must set the submission to be
					// that of the job version we are reverting to
					sub, _ = s.jobSubmission(nil, job.Namespace, job.ID, job.Version, txn)
				}
				job.Version = existingJob.Version + 1
			}
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

	if err := s.updateJobRecommendations(index, txn, existingJob, job); err != nil {
		return fmt.Errorf("unable to update job recommendations: %v", err)
	}

	if err := s.updateJobCSIPlugins(index, job, existingJob, txn); err != nil {
		return fmt.Errorf("unable to update job csi plugins: %v", err)
	}

	if err := s.updateJobSubmission(index, sub, job.Namespace, job.ID, job.Version, txn); err != nil {
		return fmt.Errorf("unable to update job submission: %v", err)
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
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	err := s.DeleteJobTxn(index, namespace, jobID, txn)
	if err == nil {
		return txn.Commit()
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

	// Delete job deployments
	deployments, err := s.DeploymentsByJobID(nil, namespace, job.ID, true)
	if err != nil {
		return fmt.Errorf("deployment lookup for job %s failed: %v", job.ID, err)
	}

	deploymentIDs := []string{}
	for _, d := range deployments {
		deploymentIDs = append(deploymentIDs, d.ID)
	}

	if err := s.DeleteDeploymentTxn(index, deploymentIDs, txn); err != nil {
		return err
	}

	// Mark all "pending" evals for this job as "complete"
	evals, err := s.EvalsByJob(nil, namespace, job.ID)
	if err != nil {
		return fmt.Errorf("eval lookup for job %s failed: %v", job.ID, err)
	}

	for _, eval := range evals {
		existing, err := txn.First("evals", "id", eval.ID)
		if err != nil {
			return fmt.Errorf("eval lookup failed: %v", err)
		}
		if existing == nil {
			continue
		}

		if existing.(*structs.Evaluation).Status != structs.EvalStatusPending {
			continue
		}

		eval := existing.(*structs.Evaluation).Copy()
		eval.Status = structs.EvalStatusComplete
		eval.StatusDescription = fmt.Sprintf("job %s deleted", job.ID)

		// Insert the eval
		if err := txn.Insert("evals", eval); err != nil {
			return fmt.Errorf("eval insert failed: %v", err)
		}
		if err := txn.Insert("index", &IndexEntry{"evals", index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
	}

	// Delete allocs associated with the job
	if err := s.deleteAllocsForJobTxn(txn, index, namespace, job.ID); err != nil {
		return err
	}

	// Cleanup plugins registered by this job, before we delete the summary
	err = s.deleteJobFromPlugins(index, txn, job)
	if err != nil {
		return fmt.Errorf("deleting job from plugin: %v", err)
	}

	// Delete the job summary
	if _, err = txn.DeleteAll("job_summary", "id", namespace, jobID); err != nil {
		return fmt.Errorf("deleting job summary failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// Delete the job submission
	if err := s.deleteJobSubmission(job, txn); err != nil {
		return fmt.Errorf("deleting job submission failed: %v", err)
	}

	// Delete any remaining job scaling policies
	if err := s.deleteJobScalingPolicies(index, job, txn); err != nil {
		return fmt.Errorf("deleting job scaling policies failed: %v", err)
	}

	// Delete any job recommendations
	if err := s.deleteRecommendationsByJob(index, txn, job); err != nil {
		return fmt.Errorf("deleting job recommendatons failed: %v", err)
	}

	// Delete the scaling events
	if _, err = txn.DeleteAll("scaling_event", "id", namespace, jobID); err != nil {
		return fmt.Errorf("deleting job scaling events failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"scaling_event", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// deleteJobScalingPolicies deletes any scaling policies associated with the job
func (s *StateStore) deleteJobScalingPolicies(index uint64, job *structs.Job, txn *txn) error {
	iter, err := s.ScalingPoliciesByJobTxn(nil, job.Namespace, job.ID, txn)
	if err != nil {
		return fmt.Errorf("getting job scaling policies for deletion failed: %v", err)
	}

	// Put them into a slice so there are no safety concerns while actually
	// performing the deletes
	policies := []interface{}{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policies = append(policies, raw)
	}

	// Do the deletes
	for _, p := range policies {
		if err := txn.Delete("scaling_policy", p); err != nil {
			return fmt.Errorf("deleting scaling policy failed: %v", err)
		}
	}

	if len(policies) > 0 {
		if err := txn.Insert("index", &IndexEntry{"scaling_policy", index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
	}
	return nil
}

func (s *StateStore) deleteJobSubmission(job *structs.Job, txn *txn) error {
	// find submissions associated with job
	remove := *set.NewHashSet[*structs.JobSubmission, string](s.config.JobTrackedVersions)

	iter, err := txn.Get("job_submission", "id_prefix", job.Namespace, job.ID)
	if err != nil {
		return err
	}

	for {
		obj := iter.Next()
		if obj == nil {
			break
		}
		sub := obj.(*structs.JobSubmission)

		// iterating by prefix; ensure we have an exact match
		if sub.Namespace == job.Namespace && sub.JobID == job.ID {
			remove.Insert(sub)
		}
	}

	// now delete the submissions we found associated with the job
	for sub := range remove.Items() {
		err := txn.Delete("job_submission", sub)
		if err != nil {
			return err
		}
	}

	return nil
}

// deleteJobVersions deletes all versions of the given job.
func (s *StateStore) deleteJobVersions(index uint64, job *structs.Job, txn *txn) error {
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
func (s *StateStore) upsertJobVersion(index uint64, job *structs.Job, txn *txn) error {
	// JobTrackedVersions really must not be zero here
	if err := s.config.Validate(); err != nil {
		return err
	}

	// Insert the job
	if err := txn.Insert("job_version", job); err != nil {
		return fmt.Errorf("failed to insert job into job_version table: %v", err)
	}

	if err := txn.Insert("index", &IndexEntry{"job_version", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// Get all the historic jobs for this ID, except those with a VersionTag,
	// as they should always be kept. They are in Version order, high to low.
	all, err := s.jobVersionByID(txn, nil, job.Namespace, job.ID, false)
	if err != nil {
		return fmt.Errorf("failed to look up job versions for %q: %v", job.ID, err)
	}

	// If we are below the limit there is no GCing to be done
	if len(all) <= s.config.JobTrackedVersions {
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
	max := s.config.JobTrackedVersions
	if stableIdx == max {
		all[max-1], all[max] = all[max], all[max-1]
	}

	// Delete the oldest one
	d := all[max]
	if err := txn.Delete("job_version", d); err != nil {
		return fmt.Errorf("failed to delete job %v (%d) from job_version", d.ID, d.Version)
	}

	return nil
}

// GetJobSubmissions returns an iterator that contains all job submissions
// stored within state. This is not currently exposed via RPC and is only used
// for snapshot persist and restore functionality.
func (s *StateStore) GetJobSubmissions(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire table to get all job submissions.
	iter, err := txn.Get(TableJobSubmission, indexID)
	if err != nil {
		return nil, fmt.Errorf("job submissions lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobSubmission returns the original HCL/Variables context of a job, if available.
//
// Note: it is a normal case for the submission context to be unavailable, in which case
// nil is returned with no error.
func (s *StateStore) JobSubmission(ws memdb.WatchSet, namespace, jobName string, version uint64) (*structs.JobSubmission, error) {
	txn := s.db.ReadTxn()
	return s.jobSubmission(ws, namespace, jobName, version, txn)
}

func (s *StateStore) jobSubmission(ws memdb.WatchSet, namespace, jobName string, version uint64, txn Txn) (*structs.JobSubmission, error) {
	watchCh, existing, err := txn.FirstWatch("job_submission", "id", namespace, jobName, version)
	if err != nil {
		return nil, fmt.Errorf("job submission lookup failed: %v", err)
	}
	ws.Add(watchCh)
	if existing != nil {
		return existing.(*structs.JobSubmission), nil
	}
	return nil, nil
}

// JobByID is used to lookup a job by its ID. JobByID returns the current/latest job
// version.
func (s *StateStore) JobByID(ws memdb.WatchSet, namespace, id string) (*structs.Job, error) {
	txn := s.db.ReadTxn()
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

// JobsByIDPrefix is used to lookup a job by prefix. If querying all namespaces
// the prefix will not be filtered by an index.
func (s *StateStore) JobsByIDPrefix(ws memdb.WatchSet, namespace, id string, sort SortOption) (memdb.ResultIterator, error) {
	if namespace == structs.AllNamespacesSentinel {
		return s.jobsByIDPrefixAllNamespaces(ws, id)
	}

	txn := s.db.ReadTxn()

	iter, err := getSorted(txn, sort, "jobs", "id_prefix", namespace, id)
	if err != nil {
		return nil, fmt.Errorf("job lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

func (s *StateStore) jobsByIDPrefixAllNamespaces(ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire jobs table
	iter, err := txn.Get("jobs", "id")

	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	// Filter the iterator by ID prefix
	f := func(raw interface{}) bool {
		job, ok := raw.(*structs.Job)
		if !ok {
			return true
		}
		return !strings.HasPrefix(job.ID, prefix)
	}
	wrap := memdb.NewFilterIterator(iter, f)
	return wrap, nil
}

// JobVersionsByID returns all the tracked versions of a job, sorted in from highest version to lowest.
func (s *StateStore) JobVersionsByID(ws memdb.WatchSet, namespace, id string) ([]*structs.Job, error) {
	txn := s.db.ReadTxn()

	return s.jobVersionByID(txn, ws, namespace, id, true)
}

// JobVersionByTagName returns a Job if it has a Tag with the passed name
func (s *StateStore) JobVersionByTagName(ws memdb.WatchSet, namespace, id string, tagName string) (*structs.Job, error) {
	// First get all versions of the job
	versions, err := s.JobVersionsByID(ws, namespace, id)
	if err != nil {
		return nil, err
	}
	for _, j := range versions {
		if j.VersionTag != nil && j.VersionTag.Name == tagName {
			return j, nil
		}
	}
	return nil, nil
}

// jobVersionByID is the underlying implementation for retrieving all tracked
// versions of a job and is called under an existing transaction. A watch set
// can optionally be passed in to add the job histories to the watch set.
func (s *StateStore) jobVersionByID(txn *txn, ws memdb.WatchSet, namespace, id string, includeTagged bool) ([]*structs.Job, error) {
	// Get all the historic jobs for this ID
	iter, err := txn.Get("job_version", "id_prefix", namespace, id)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

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

		if !includeTagged && j.VersionTag != nil {
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
	txn := s.db.ReadTxn()
	return s.jobByIDAndVersionImpl(ws, namespace, id, version, txn)
}

// jobByIDAndVersionImpl returns the job identified by its ID and Version. The
// passed watchset may be nil.
func (s *StateStore) jobByIDAndVersionImpl(ws memdb.WatchSet, namespace, id string,
	version uint64, txn *txn) (*structs.Job, error) {

	watchCh, existing, err := txn.FirstWatch("job_version", "id", namespace, id, version)
	if err != nil {
		return nil, err
	}

	ws.Add(watchCh)

	if existing != nil {
		job := existing.(*structs.Job)
		return job, nil
	}

	return nil, nil
}

func (s *StateStore) JobVersions(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire deployments table
	iter, err := txn.Get("job_version", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// Jobs returns an iterator over all the jobs
func (s *StateStore) Jobs(ws memdb.WatchSet, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire jobs table
	iter, err := getSorted(txn, sort, "jobs", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobsByNamespace returns an iterator over all the jobs for the given namespace
func (s *StateStore) JobsByNamespace(ws memdb.WatchSet, namespace string, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()
	return s.jobsByNamespaceImpl(ws, namespace, txn, sort)
}

// jobsByNamespaceImpl returns an iterator over all the jobs for the given namespace
func (s *StateStore) jobsByNamespaceImpl(ws memdb.WatchSet, namespace string, txn *txn, sort SortOption) (memdb.ResultIterator, error) {
	iter, err := getSorted(txn, sort, "jobs", "id_prefix", namespace, "")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobsByPeriodic returns an iterator over all the periodic or non-periodic jobs.
func (s *StateStore) JobsByPeriodic(ws memdb.WatchSet, periodic bool) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

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
	txn := s.db.ReadTxn()

	// Return an iterator for jobs with the specific type.
	iter, err := txn.Get("jobs", "type", schedulerType)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobsByGC returns an iterator over all jobs eligible or ineligible for garbage
// collection.
func (s *StateStore) JobsByGC(ws memdb.WatchSet, gc bool) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get("jobs", "gc", gc)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobsByPool returns an iterator over all jobs in a given node pool.
func (s *StateStore) JobsByPool(ws memdb.WatchSet, pool string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get("jobs", "pool", pool)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobsByModifyIndex returns an iterator over all jobs, sorted by ModifyIndex.
func (s *StateStore) JobsByModifyIndex(ws memdb.WatchSet, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := getSorted(txn, sort, "jobs", "modify_index")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobSummaryByID returns a job summary object which matches a specific id.
func (s *StateStore) JobSummaryByID(ws memdb.WatchSet, namespace, jobID string) (*structs.JobSummary, error) {
	txn := s.db.ReadTxn()

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
	txn := s.db.ReadTxn()

	iter, err := txn.Get("job_summary", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobSummaryByPrefix is used to look up Job Summary by id prefix
func (s *StateStore) JobSummaryByPrefix(ws memdb.WatchSet, namespace, id string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get("job_summary", "id_prefix", namespace, id)
	if err != nil {
		return nil, fmt.Errorf("job_summary lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// UpsertCSIVolume inserts a volume in the state store.
func (s *StateStore) UpsertCSIVolume(index uint64, volumes []*structs.CSIVolume) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	for _, v := range volumes {
		if exists, err := s.namespaceExists(txn, v.Namespace); err != nil {
			return err
		} else if !exists {
			return fmt.Errorf("volume %s is in nonexistent namespace %s", v.ID, v.Namespace)
		}

		obj, err := txn.First("csi_volumes", "id", v.Namespace, v.ID)
		if err != nil {
			return fmt.Errorf("volume existence check error: %v", err)
		}
		if obj != nil {
			// Allow some properties of a volume to be updated in place, but
			// prevent accidentally overwriting important properties.
			old := obj.(*structs.CSIVolume)
			if old.ExternalID != v.ExternalID ||
				old.PluginID != v.PluginID ||
				old.Provider != v.Provider {
				return fmt.Errorf("volume identity cannot be updated: %s", v.ID)
			}
		} else {
			v.CreateIndex = index
		}
		v.ModifyIndex = index

		// Allocations are copy on write, so we want to keep the Allocation ID
		// but we need to clear the pointer so that we don't store it when we
		// write the volume to the state store. We'll get it from the db in
		// denormalize.
		for allocID := range v.ReadAllocs {
			v.ReadAllocs[allocID] = nil
		}
		for allocID := range v.WriteAllocs {
			v.WriteAllocs[allocID] = nil
		}

		err = txn.Insert("csi_volumes", v)
		if err != nil {
			return fmt.Errorf("volume insert: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"csi_volumes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// CSIVolumes returns the unfiltered list of all volumes. Caller should
// snapshot if it wants to also denormalize the plugins.
func (s *StateStore) CSIVolumes(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()
	defer txn.Abort()

	iter, err := txn.Get("csi_volumes", "id")
	if err != nil {
		return nil, fmt.Errorf("csi_volumes lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// CSIVolumeByID is used to lookup a single volume. Returns a copy of the
// volume because its plugins and allocations are denormalized to provide
// accurate Health.
func (s *StateStore) CSIVolumeByID(ws memdb.WatchSet, namespace, id string) (*structs.CSIVolume, error) {
	txn := s.db.ReadTxn()

	watchCh, obj, err := txn.FirstWatch("csi_volumes", "id", namespace, id)
	if err != nil {
		return nil, fmt.Errorf("volume lookup failed for %s: %v", id, err)
	}
	ws.Add(watchCh)

	if obj == nil {
		return nil, nil
	}
	vol := obj.(*structs.CSIVolume)

	// we return the volume with the plugins denormalized by default,
	// because the scheduler needs them for feasibility checking
	return s.csiVolumeDenormalizePluginsTxn(txn, vol.Copy())
}

// CSIVolumesByPluginID looks up csi_volumes by pluginID. Caller should
// snapshot if it wants to also denormalize the plugins.
func (s *StateStore) CSIVolumesByPluginID(ws memdb.WatchSet, namespace, prefix, pluginID string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get("csi_volumes", "plugin_id", pluginID)
	if err != nil {
		return nil, fmt.Errorf("volume lookup failed: %v", err)
	}

	// Filter the iterator by namespace
	f := func(raw interface{}) bool {
		v, ok := raw.(*structs.CSIVolume)
		if !ok {
			return false
		}
		return v.Namespace != namespace && strings.HasPrefix(v.ID, prefix)
	}

	wrap := memdb.NewFilterIterator(iter, f)
	return wrap, nil
}

// CSIVolumesByIDPrefix supports search. Caller should snapshot if it wants to
// also denormalize the plugins. If using a prefix with the wildcard namespace,
// the results will not use the index prefix.
func (s *StateStore) CSIVolumesByIDPrefix(ws memdb.WatchSet, namespace, volumeID string) (memdb.ResultIterator, error) {
	if namespace == structs.AllNamespacesSentinel {
		return s.csiVolumeByIDPrefixAllNamespaces(ws, volumeID)
	}

	txn := s.db.ReadTxn()

	iter, err := txn.Get("csi_volumes", "id_prefix", namespace, volumeID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

func (s *StateStore) csiVolumeByIDPrefixAllNamespaces(ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire csi_volumes table
	iter, err := txn.Get("csi_volumes", "id")

	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	// Filter the iterator by ID prefix
	f := func(raw interface{}) bool {
		v, ok := raw.(*structs.CSIVolume)
		if !ok {
			return false
		}
		return !strings.HasPrefix(v.ID, prefix)
	}
	wrap := memdb.NewFilterIterator(iter, f)
	return wrap, nil
}

// CSIVolumesByNodeID looks up CSIVolumes in use on a node. Caller should
// snapshot if it wants to also denormalize the plugins.
func (s *StateStore) CSIVolumesByNodeID(ws memdb.WatchSet, prefix, nodeID string) (memdb.ResultIterator, error) {
	allocs, err := s.AllocsByNode(ws, nodeID)
	if err != nil {
		return nil, fmt.Errorf("alloc lookup failed: %v", err)
	}

	// Find volume ids for CSI volumes in running allocs, or allocs that we desire to run
	ids := map[string]string{} // Map volumeID to Namespace
	for _, a := range allocs {
		tg := a.Job.LookupTaskGroup(a.TaskGroup)

		if !(a.DesiredStatus == structs.AllocDesiredStatusRun ||
			a.ClientStatus == structs.AllocClientStatusRunning) ||
			len(tg.Volumes) == 0 {
			continue
		}

		for _, v := range tg.Volumes {
			if v.Type != structs.VolumeTypeCSI {
				continue
			}
			ids[v.Source] = a.Namespace
		}
	}

	// Lookup the raw CSIVolumes to match the other list interfaces
	iter := NewSliceIterator()
	txn := s.db.ReadTxn()
	for id, namespace := range ids {
		if strings.HasPrefix(id, prefix) {
			watchCh, raw, err := txn.FirstWatch("csi_volumes", "id", namespace, id)
			if err != nil {
				return nil, fmt.Errorf("volume lookup failed: %s %v", id, err)
			}
			ws.Add(watchCh)
			iter.Add(raw)
		}
	}

	return iter, nil
}

// CSIVolumesByNamespace looks up the entire csi_volumes table
func (s *StateStore) CSIVolumesByNamespace(ws memdb.WatchSet, namespace, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	return s.csiVolumesByNamespaceImpl(txn, ws, namespace, prefix)
}

func (s *StateStore) csiVolumesByNamespaceImpl(txn *txn, ws memdb.WatchSet, namespace, prefix string) (memdb.ResultIterator, error) {

	iter, err := txn.Get("csi_volumes", "id_prefix", namespace, prefix)
	if err != nil {
		return nil, fmt.Errorf("volume lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// CSIVolumeClaim updates the volume's claim count and allocation list
func (s *StateStore) CSIVolumeClaim(index uint64, namespace, id string, claim *structs.CSIVolumeClaim) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	row, err := txn.First("csi_volumes", "id", namespace, id)
	if err != nil {
		return fmt.Errorf("volume lookup failed: %s: %v", id, err)
	}
	if row == nil {
		return fmt.Errorf("volume not found: %s", id)
	}

	orig, ok := row.(*structs.CSIVolume)
	if !ok {
		return fmt.Errorf("volume row conversion error")
	}

	var alloc *structs.Allocation
	if claim.State == structs.CSIVolumeClaimStateTaken {
		alloc, err = s.allocByIDImpl(txn, nil, claim.AllocationID)
		if err != nil {
			s.logger.Error("AllocByID failed", "error", err)
			return fmt.Errorf(structs.ErrUnknownAllocationPrefix)
		}
		if alloc == nil {
			s.logger.Error("AllocByID failed to find alloc", "alloc_id", claim.AllocationID)
			if err != nil {
				return fmt.Errorf(structs.ErrUnknownAllocationPrefix)
			}
		}
	}

	volume, err := s.csiVolumeDenormalizePluginsTxn(txn, orig.Copy())
	if err != nil {
		return err
	}
	volume, err = s.csiVolumeDenormalizeTxn(txn, nil, volume)
	if err != nil {
		return err
	}

	// in the case of a job deregistration, there will be no allocation ID
	// for the claim but we still want to write an updated index to the volume
	// so that volume reaping is triggered
	if claim.AllocationID != "" {
		err = volume.Claim(claim, alloc)
		if err != nil {
			return err
		}
	}

	volume.ModifyIndex = index

	// Allocations are copy on write, so we want to keep the Allocation ID
	// but we need to clear the pointer so that we don't store it when we
	// write the volume to the state store. We'll get it from the db in
	// denormalize.
	for allocID := range volume.ReadAllocs {
		volume.ReadAllocs[allocID] = nil
	}
	for allocID := range volume.WriteAllocs {
		volume.WriteAllocs[allocID] = nil
	}

	if err = txn.Insert("csi_volumes", volume); err != nil {
		return fmt.Errorf("volume update failed: %s: %v", id, err)
	}

	if err = txn.Insert("index", &IndexEntry{"csi_volumes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// CSIVolumeDeregister removes the volume from the server
func (s *StateStore) CSIVolumeDeregister(index uint64, namespace string, ids []string, force bool) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	for _, id := range ids {
		existing, err := txn.First("csi_volumes", "id", namespace, id)
		if err != nil {
			return fmt.Errorf("volume lookup failed: %s: %v", id, err)
		}

		if existing == nil {
			return fmt.Errorf("volume not found: %s", id)
		}

		vol, ok := existing.(*structs.CSIVolume)
		if !ok {
			return fmt.Errorf("volume row conversion error: %s", id)
		}

		// The common case for a volume deregister is when the volume is
		// unused, but we can also let an operator intervene in the case where
		// allocations have been stopped but claims can't be freed because
		// ex. the plugins have all been removed.
		if vol.InUse() {
			if !force || !s.volSafeToForce(txn, vol) {
				return fmt.Errorf("volume in use: %s", id)
			}
		}

		if err = txn.Delete("csi_volumes", existing); err != nil {
			return fmt.Errorf("volume delete failed: %s: %v", id, err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"csi_volumes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// volSafeToForce checks if the any of the remaining allocations
// are in a non-terminal state.
func (s *StateStore) volSafeToForce(txn Txn, v *structs.CSIVolume) bool {
	v = v.Copy()
	vol, err := s.csiVolumeDenormalizeTxn(txn, nil, v)
	if err != nil {
		return false
	}

	for _, alloc := range vol.ReadAllocs {
		if alloc != nil && !alloc.TerminalStatus() {
			return false
		}
	}
	for _, alloc := range vol.WriteAllocs {
		if alloc != nil && !alloc.TerminalStatus() {
			return false
		}
	}
	return true
}

// CSIVolumeDenormalizePlugins returns a CSIVolume with current health and
// plugins, but without allocations.
// Use this for current volume metadata, handling lists of volumes.
// Use CSIVolumeDenormalize for volumes containing both health and current
// allocations.
func (s *StateStore) CSIVolumeDenormalizePlugins(ws memdb.WatchSet, vol *structs.CSIVolume) (*structs.CSIVolume, error) {
	if vol == nil {
		return nil, nil
	}
	txn := s.db.ReadTxn()
	defer txn.Abort()
	return s.csiVolumeDenormalizePluginsTxn(txn, vol)
}

// csiVolumeDenormalizePluginsTxn implements
// CSIVolumeDenormalizePlugins, inside a transaction.
func (s *StateStore) csiVolumeDenormalizePluginsTxn(txn Txn, vol *structs.CSIVolume) (*structs.CSIVolume, error) {
	if vol == nil {
		return nil, nil
	}
	plug, err := s.CSIPluginByIDTxn(txn, nil, vol.PluginID)
	if err != nil {
		return nil, fmt.Errorf("plugin lookup error: %s %v", vol.PluginID, err)
	}
	if plug == nil {
		vol.ControllersHealthy = 0
		vol.NodesHealthy = 0
		vol.Schedulable = false
		return vol, nil
	}

	vol.Provider = plug.Provider
	vol.ProviderVersion = plug.Version
	vol.ControllerRequired = plug.ControllerRequired
	vol.ControllersHealthy = plug.ControllersHealthy
	vol.NodesHealthy = plug.NodesHealthy

	// This value may be stale, but stale is ok
	vol.ControllersExpected = plug.ControllersExpected
	vol.NodesExpected = plug.NodesExpected

	vol.Schedulable = vol.NodesHealthy > 0
	if vol.ControllerRequired {
		vol.Schedulable = vol.ControllersHealthy > 0 && vol.Schedulable
	}

	return vol, nil
}

// CSIVolumeDenormalize returns a CSIVolume with its current
// Allocations and Claims, including creating new PastClaims for
// terminal or garbage collected allocations. This ensures we have a
// consistent state. Note that it mutates the original volume and so
// should always be called on a Copy after reading from the state
// store.
func (s *StateStore) CSIVolumeDenormalize(ws memdb.WatchSet, vol *structs.CSIVolume) (*structs.CSIVolume, error) {
	txn := s.db.ReadTxn()
	return s.csiVolumeDenormalizeTxn(txn, ws, vol)
}

// csiVolumeDenormalizeTxn implements CSIVolumeDenormalize inside a transaction
func (s *StateStore) csiVolumeDenormalizeTxn(txn Txn, ws memdb.WatchSet, vol *structs.CSIVolume) (*structs.CSIVolume, error) {
	if vol == nil {
		return nil, nil
	}

	// note: denormalize mutates the maps we pass in!
	denormalize := func(
		currentAllocs map[string]*structs.Allocation,
		currentClaims, pastClaims map[string]*structs.CSIVolumeClaim,
		fallbackMode structs.CSIVolumeClaimMode) error {

		for id := range currentAllocs {
			a, err := s.allocByIDImpl(txn, ws, id)
			if err != nil {
				return err
			}
			pastClaim := pastClaims[id]
			currentClaim := currentClaims[id]
			if currentClaim == nil {
				// COMPAT(1.4.0): the CSIVolumeClaim fields were added
				// after 0.11.1, so claims made before that may be
				// missing this value. No clusters should see this
				// anymore, so warn nosily in the logs so that
				// operators ask us about it. Remove this block and
				// the now-unused fallbackMode parameter, and return
				// an error if currentClaim is nil in 1.4.0
				s.logger.Warn("volume was missing claim for allocation",
					"volume_id", vol.ID, "alloc", id)
				currentClaim = &structs.CSIVolumeClaim{
					AllocationID: a.ID,
					NodeID:       a.NodeID,
					Mode:         fallbackMode,
					State:        structs.CSIVolumeClaimStateTaken,
				}
				currentClaims[id] = currentClaim
			}

			currentAllocs[id] = a
			if (a == nil || a.TerminalStatus()) && pastClaim == nil {
				// the alloc is garbage collected but nothing has written a PastClaim,
				// so create one now
				pastClaim = &structs.CSIVolumeClaim{
					AllocationID:   id,
					NodeID:         currentClaim.NodeID,
					Mode:           currentClaim.Mode,
					State:          structs.CSIVolumeClaimStateUnpublishing,
					AccessMode:     currentClaim.AccessMode,
					AttachmentMode: currentClaim.AttachmentMode,
				}
				pastClaims[id] = pastClaim
			}

		}
		return nil
	}

	err := denormalize(vol.ReadAllocs, vol.ReadClaims, vol.PastClaims,
		structs.CSIVolumeClaimRead)
	if err != nil {
		return nil, err
	}
	err = denormalize(vol.WriteAllocs, vol.WriteClaims, vol.PastClaims,
		structs.CSIVolumeClaimWrite)
	if err != nil {
		return nil, err
	}

	// COMPAT: the AccessMode and AttachmentMode fields were added to claims
	// in 1.1.0, so claims made before that may be missing this value. In this
	// case, the volume will already have AccessMode/AttachmentMode until it
	// no longer has any claims, so set from those values
	for _, claim := range vol.ReadClaims {
		if claim.AccessMode == "" || claim.AttachmentMode == "" {
			claim.AccessMode = vol.AccessMode
			claim.AttachmentMode = vol.AttachmentMode
		}
	}
	for _, claim := range vol.WriteClaims {
		if claim.AccessMode == "" || claim.AttachmentMode == "" {
			claim.AccessMode = vol.AccessMode
			claim.AttachmentMode = vol.AttachmentMode
		}
	}

	return vol, nil
}

// CSIPlugins returns the unfiltered list of all plugin health status
func (s *StateStore) CSIPlugins(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()
	defer txn.Abort()

	iter, err := txn.Get("csi_plugins", "id")
	if err != nil {
		return nil, fmt.Errorf("csi_plugins lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// CSIPluginsByIDPrefix supports search
func (s *StateStore) CSIPluginsByIDPrefix(ws memdb.WatchSet, pluginID string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get("csi_plugins", "id_prefix", pluginID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// CSIPluginByID returns a named CSIPlugin. This method creates a new
// transaction so you should not call it from within another transaction.
func (s *StateStore) CSIPluginByID(ws memdb.WatchSet, id string) (*structs.CSIPlugin, error) {
	txn := s.db.ReadTxn()
	plugin, err := s.CSIPluginByIDTxn(txn, ws, id)
	if err != nil {
		return nil, err
	}
	return plugin, nil
}

// CSIPluginByIDTxn returns a named CSIPlugin
func (s *StateStore) CSIPluginByIDTxn(txn Txn, ws memdb.WatchSet, id string) (*structs.CSIPlugin, error) {

	watchCh, obj, err := txn.FirstWatch("csi_plugins", "id", id)
	if err != nil {
		return nil, fmt.Errorf("csi_plugin lookup failed: %s %v", id, err)
	}

	ws.Add(watchCh)

	if obj != nil {
		return obj.(*structs.CSIPlugin), nil
	}
	return nil, nil
}

// CSIPluginDenormalize returns a CSIPlugin with allocation details. Always called on a copy of the plugin.
func (s *StateStore) CSIPluginDenormalize(ws memdb.WatchSet, plug *structs.CSIPlugin) (*structs.CSIPlugin, error) {
	txn := s.db.ReadTxn()
	return s.CSIPluginDenormalizeTxn(txn, ws, plug)
}

func (s *StateStore) CSIPluginDenormalizeTxn(txn Txn, ws memdb.WatchSet, plug *structs.CSIPlugin) (*structs.CSIPlugin, error) {
	if plug == nil {
		return nil, nil
	}

	// Get the unique list of allocation ids
	ids := map[string]struct{}{}
	for _, info := range plug.Controllers {
		ids[info.AllocID] = struct{}{}
	}
	for _, info := range plug.Nodes {
		ids[info.AllocID] = struct{}{}
	}

	for id := range ids {
		alloc, err := s.allocByIDImpl(txn, ws, id)
		if err != nil {
			return nil, err
		}
		if alloc == nil {
			continue
		}
		plug.Allocations = append(plug.Allocations, alloc.Stub(nil))
	}
	sort.Slice(plug.Allocations, func(i, j int) bool {
		return plug.Allocations[i].ModifyIndex > plug.Allocations[j].ModifyIndex
	})

	return plug, nil
}

// UpsertCSIPlugin writes the plugin to the state store. Note: there
// is currently no raft message for this, as it's intended to support
// testing use cases.
func (s *StateStore) UpsertCSIPlugin(index uint64, plug *structs.CSIPlugin) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	existing, err := txn.First("csi_plugins", "id", plug.ID)
	if err != nil {
		return fmt.Errorf("csi_plugin lookup error: %s %v", plug.ID, err)
	}

	plug.ModifyIndex = index
	if existing != nil {
		plug.CreateIndex = existing.(*structs.CSIPlugin).CreateIndex
	}

	err = txn.Insert("csi_plugins", plug)
	if err != nil {
		return fmt.Errorf("csi_plugins insert error: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"csi_plugins", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	return txn.Commit()
}

// DeleteCSIPlugin deletes the plugin if it's not in use.
func (s *StateStore) DeleteCSIPlugin(index uint64, id string) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	plug, err := s.CSIPluginByIDTxn(txn, nil, id)
	if err != nil {
		return err
	}

	if plug == nil {
		return nil
	}

	plug, err = s.CSIPluginDenormalizeTxn(txn, nil, plug.Copy())
	if err != nil {
		return err
	}

	jobIDs := set.New[structs.NamespacedID](1)
	for _, alloc := range plug.Allocations {
		jobIDs.Insert(structs.NamespacedID{Namespace: alloc.Namespace, ID: alloc.JobID})
	}

	// after denormalization of allocs, remove any ControllerJobs or NodeJobs
	// that no longer have allocations and have been either purged or updated to
	// no longer include the plugin
	removeInvalidJobs := func(jobDescs structs.JobDescriptions) {
		for ns, namespacedJobDescs := range jobDescs {
			for jobID := range namespacedJobDescs {
				if !jobIDs.Contains(structs.NamespacedID{Namespace: ns, ID: jobID}) {
					job, err := s.JobByID(nil, ns, jobID)
					if err != nil { // programmer error in JobByID only
						s.logger.Error("could not query JobByID", "error", err)
						continue
					}
					if job == nil { // job was purged
						jobDescs.Delete(&structs.Job{ID: jobID, Namespace: ns})
					} else if !job.HasPlugin(plug.ID) {
						// job was updated to a different plugin ID
						jobDescs.Delete(job)
					}
				}
			}
		}
	}

	removeInvalidJobs(plug.ControllerJobs)
	removeInvalidJobs(plug.NodeJobs)

	if !plug.IsEmpty() {
		return structs.ErrCSIPluginInUse
	}

	err = txn.Delete("csi_plugins", plug)
	if err != nil {
		return fmt.Errorf("csi_plugins delete error: %v", err)
	}
	return txn.Commit()
}

// UpsertPeriodicLaunch is used to register a launch or update it.
func (s *StateStore) UpsertPeriodicLaunch(index uint64, launch *structs.PeriodicLaunch) error {
	txn := s.db.WriteTxn(index)
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

	return txn.Commit()
}

// DeletePeriodicLaunch is used to delete the periodic launch
func (s *StateStore) DeletePeriodicLaunch(index uint64, namespace, jobID string) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	err := s.DeletePeriodicLaunchTxn(index, namespace, jobID, txn)
	if err == nil {
		return txn.Commit()
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
	txn := s.db.ReadTxn()

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
	txn := s.db.ReadTxn()

	// Walk the entire table
	iter, err := txn.Get("periodic_launch", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// UpsertEvals is used to upsert a set of evaluations
func (s *StateStore) UpsertEvals(msgType structs.MessageType, index uint64, evals []*structs.Evaluation) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	err := s.UpsertEvalsTxn(index, evals, txn)
	if err == nil {
		return txn.Commit()
	}
	return err
}

// UpsertEvalsTxn is used to upsert a set of evaluations, like UpsertEvals but
// in a transaction.  Useful for when making multiple modifications atomically.
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
func (s *StateStore) nestedUpsertEval(txn *txn, index uint64, eval *structs.Evaluation) error {
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
		for _, blockedEval := range blocked {
			newEval := blockedEval.Copy()
			newEval.Status = structs.EvalStatusCancelled
			newEval.StatusDescription = fmt.Sprintf("evaluation %q successful", eval.ID)
			newEval.ModifyIndex = index
			newEval.ModifyTime = eval.ModifyTime

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
func (s *StateStore) updateEvalModifyIndex(txn *txn, index uint64, evalID string) error {
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

// DeleteEvalsByFilter is used to delete all evals that are both safe to delete
// and match a filter.
func (s *StateStore) DeleteEvalsByFilter(index uint64, filterExpr string, pageToken string, perPage int32) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// These are always user-initiated, so ensure the eval broker is paused.
	_, schedConfig, err := s.schedulerConfigTxn(txn)
	if err != nil {
		return err
	}
	if schedConfig == nil || !schedConfig.PauseEvalBroker {
		return errors.New("eval broker is enabled; eval broker must be paused to delete evals")
	}

	filter, err := bexpr.CreateEvaluator(filterExpr)
	if err != nil {
		return err
	}

	iter, err := s.Evals(nil, SortDefault)
	if err != nil {
		return fmt.Errorf("failed to lookup evals: %v", err)
	}

	// Note: Paginator imports this package for testing so we can't just use
	// Paginator
	pageCount := int32(0)

	for {
		if pageCount >= perPage {
			break
		}
		raw := iter.Next()
		if raw == nil {
			break
		}
		eval := raw.(*structs.Evaluation)
		if eval.ID < pageToken {
			continue
		}

		deleteOk, err := s.EvalIsUserDeleteSafe(nil, eval)
		if !deleteOk || err != nil {
			continue
		}
		match, err := filter.Evaluate(eval)
		if !match || err != nil {
			continue
		}
		if err := txn.Delete("evals", eval); err != nil {
			return fmt.Errorf("eval delete failed: %v", err)
		}
		pageCount++
	}

	err = txn.Commit()
	return err
}

// EvalIsUserDeleteSafe ensures an evaluation is safe to delete based on its
// related allocation and job information. This follows similar, but different
// rules to the eval reap checking, to ensure evaluations for running allocs or
// allocs which need the evaluation detail are not deleted.
//
// Returns both a bool and an error so that error in querying the related
// objects can be differentiated from reporting that the eval isn't safe to
// delete.
func (s *StateStore) EvalIsUserDeleteSafe(ws memdb.WatchSet, eval *structs.Evaluation) (bool, error) {

	job, err := s.JobByID(ws, eval.Namespace, eval.JobID)
	if err != nil {
		return false, fmt.Errorf("failed to lookup job for eval: %v", err)
	}

	allocs, err := s.AllocsByEval(ws, eval.ID)
	if err != nil {
		return false, fmt.Errorf("failed to lookup eval allocs: %v", err)
	}

	return isEvalDeleteSafe(allocs, job), nil
}

func isEvalDeleteSafe(allocs []*structs.Allocation, job *structs.Job) bool {

	// If the job is deleted, stopped, or dead, all allocs are terminal and
	// the eval can be deleted.
	if job == nil || job.Stop || job.Status == structs.JobStatusDead {
		return true
	}

	// Iterate the allocations associated to the eval, if any, and check
	// whether we can delete the eval.
	for _, alloc := range allocs {

		// If the allocation is still classed as running on the client, or
		// might be, we can't delete.
		switch alloc.ClientStatus {
		case structs.AllocClientStatusRunning, structs.AllocClientStatusUnknown:
			return false
		}

		// If the alloc hasn't failed then we don't need to consider it for
		// rescheduling. Rescheduling needs to copy over information from the
		// previous alloc so that it can enforce the reschedule policy.
		if alloc.ClientStatus != structs.AllocClientStatusFailed {
			continue
		}

		var reschedulePolicy *structs.ReschedulePolicy
		tg := job.LookupTaskGroup(alloc.TaskGroup)

		if tg != nil {
			reschedulePolicy = tg.ReschedulePolicy
		}

		// No reschedule policy or rescheduling is disabled
		if reschedulePolicy == nil || (!reschedulePolicy.Unlimited && reschedulePolicy.Attempts == 0) {
			continue
		}

		// The restart tracking information has not been carried forward.
		if alloc.NextAllocation == "" {
			return false
		}

		// This task has unlimited rescheduling and the alloc has not been
		// replaced, so we can't delete the eval yet.
		if reschedulePolicy.Unlimited {
			return false
		}

		// No restarts have been attempted yet.
		if alloc.RescheduleTracker == nil || len(alloc.RescheduleTracker.Events) == 0 {
			return false
		}
	}

	return true
}

// DeleteEval is used to delete an evaluation
func (s *StateStore) DeleteEval(index uint64, evals, allocs []string, userInitiated bool) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// If this deletion has been initiated by an operator, ensure the eval
	// broker is paused.
	if userInitiated {
		_, schedConfig, err := s.schedulerConfigTxn(txn)
		if err != nil {
			return err
		}
		if schedConfig == nil || !schedConfig.PauseEvalBroker {
			return errors.New("eval broker is enabled; eval broker must be paused to delete evals")
		}
	}

	jobs := make(map[structs.NamespacedID]string, len(evals))

	// evalsTableUpdated and allocsTableUpdated allow us to track whether each
	// table has been modified. This allows us to skip updating the index table
	// entries if we do not need to.
	var evalsTableUpdated, allocsTableUpdated bool

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

		// Mark that we have made a successful modification to the evals
		// table.
		evalsTableUpdated = true

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

		// Mark that we have made a successful modification to the allocs
		// table.
		allocsTableUpdated = true

		if err := s.deleteServiceRegistrationByAllocIDTxn(txn, index, alloc); err != nil {
			return fmt.Errorf("service registration delete for alloc failed: %v", err)
		}
	}

	// Update the indexes
	if evalsTableUpdated {
		if err := txn.Insert("index", &IndexEntry{"evals", index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
	}
	if allocsTableUpdated {
		if err := txn.Insert("index", &IndexEntry{"allocs", index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
	}

	// Set the job's status
	if err := s.setJobStatuses(index, txn, jobs, true); err != nil {
		return fmt.Errorf("setting job status failed: %v", err)
	}

	return txn.Commit()
}

// EvalByID is used to lookup an eval by its ID
func (s *StateStore) EvalByID(ws memdb.WatchSet, id string) (*structs.Evaluation, error) {
	txn := s.db.ReadTxn()

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

// EvalsRelatedToID is used to retrieve the evals that are related (next,
// previous, or blocked) to the provided eval ID.
func (s *StateStore) EvalsRelatedToID(ws memdb.WatchSet, id string) ([]*structs.EvaluationStub, error) {
	txn := s.db.ReadTxn()

	raw, err := txn.First("evals", "id", id)
	if err != nil {
		return nil, fmt.Errorf("eval lookup failed: %v", err)
	}
	if raw == nil {
		return nil, nil
	}
	eval := raw.(*structs.Evaluation)

	relatedEvals := []*structs.EvaluationStub{}
	todo := eval.RelatedIDs()
	done := map[string]bool{
		eval.ID: true, // don't place the requested eval in the related list.
	}

	for len(todo) > 0 {
		// Pop the first value from the todo list.
		current := todo[0]
		todo = todo[1:]
		if current == "" {
			continue
		}

		// Skip value if we already have it in the results.
		if done[current] {
			continue
		}

		eval, err := s.EvalByID(ws, current)
		if err != nil {
			return nil, err
		}
		if eval == nil {
			continue
		}

		todo = append(todo, eval.RelatedIDs()...)
		relatedEvals = append(relatedEvals, eval.Stub())
		done[eval.ID] = true
	}

	return relatedEvals, nil
}

// EvalsByIDPrefix is used to lookup evaluations by prefix in a particular
// namespace
func (s *StateStore) EvalsByIDPrefix(ws memdb.WatchSet, namespace, id string, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var iter memdb.ResultIterator
	var err error

	// Get an iterator over all evals by the id prefix
	switch sort {
	case SortReverse:
		iter, err = txn.GetReverse("evals", "id_prefix", id)
	default:
		iter, err = txn.Get("evals", "id_prefix", id)
	}
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

		return namespace != structs.AllNamespacesSentinel &&
			eval.Namespace != namespace
	}
}

// EvalsByJob returns all the evaluations by job id
func (s *StateStore) EvalsByJob(ws memdb.WatchSet, namespace, jobID string) ([]*structs.Evaluation, error) {
	txn := s.db.ReadTxn()

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

// Evals returns an iterator over all the evaluations in ascending or descending
// order of CreationIndex as determined by the reverse parameter.
func (s *StateStore) Evals(ws memdb.WatchSet, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var it memdb.ResultIterator
	var err error

	switch sort {
	case SortReverse:
		it, err = txn.GetReverse("evals", "create")
	default:
		it, err = txn.Get("evals", "create")
	}

	if err != nil {
		return nil, err
	}

	ws.Add(it.WatchCh())

	return it, nil
}

// EvalsByNamespace returns an iterator over all evaluations in no particular
// order.
//
// todo(shoenig): can this be removed?
func (s *StateStore) EvalsByNamespace(ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	it, err := txn.Get("evals", "namespace", namespace)
	if err != nil {
		return nil, err
	}

	ws.Add(it.WatchCh())

	return it, nil
}

func (s *StateStore) EvalsByNamespaceOrdered(ws memdb.WatchSet, namespace string, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var (
		it    memdb.ResultIterator
		err   error
		exact = terminate(namespace)
	)

	switch sort {
	case SortReverse:
		it, err = txn.GetReverse("evals", "namespace_create_prefix", exact)
	default:
		it, err = txn.Get("evals", "namespace_create_prefix", exact)
	}

	if err != nil {
		return nil, err
	}

	ws.Add(it.WatchCh())

	return it, nil
}

// UpdateAllocsFromClient is used to update an allocation based on input
// from a client. While the schedulers are the authority on the allocation for
// most things, some updates are authoritative from the client. Specifically,
// the desired state comes from the schedulers, while the actual state comes
// from clients.
func (s *StateStore) UpdateAllocsFromClient(msgType structs.MessageType, index uint64, allocs []*structs.Allocation) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	// Capture all nodes being affected. Alloc updates from clients are batched
	// so this request may include allocs from several nodes.
	nodeIDs := set.New[string](1)

	// Handle each of the updated allocations
	for _, alloc := range allocs {
		nodeIDs.Insert(alloc.NodeID)
		if err := s.nestedUpdateAllocFromClient(txn, index, alloc); err != nil {
			return err
		}
	}

	// Update the indexes
	if err := txn.Insert("index", &IndexEntry{"allocs", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	// Update the index of when nodes last updated their allocs.
	for nodeID := range nodeIDs.Items() {
		if err := s.updateClientAllocUpdateIndex(txn, index, nodeID); err != nil {
			return fmt.Errorf("node update failed: %v", err)
		}
	}

	return txn.Commit()
}

// nestedUpdateAllocFromClient is used to nest an update of an allocation with client status
func (s *StateStore) nestedUpdateAllocFromClient(txn *txn, index uint64, alloc *structs.Allocation) error {
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
	copyAlloc.NetworkStatus = alloc.NetworkStatus

	// The client can only set its deployment health and timestamp, so just take
	// those
	if copyAlloc.DeploymentStatus != nil && alloc.DeploymentStatus != nil {
		oldHasHealthy := copyAlloc.DeploymentStatus.HasHealth()
		newHasHealthy := alloc.DeploymentStatus.HasHealth()

		// We got new health information from the client
		if newHasHealthy && (!oldHasHealthy || *copyAlloc.DeploymentStatus.Healthy != *alloc.DeploymentStatus.Healthy) {
			// Updated deployment health and timestamp
			copyAlloc.DeploymentStatus.Healthy = pointer.Of(*alloc.DeploymentStatus.Healthy)
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

	if err := s.updatePluginForTerminalAlloc(index, copyAlloc, txn); err != nil {
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

	if copyAlloc.ClientTerminalStatus() {
		if err := s.deleteServiceRegistrationByAllocIDTxn(txn, index, copyAlloc.ID); err != nil {
			return err
		}
	}

	return nil
}

func (s *StateStore) updateClientAllocUpdateIndex(txn *txn, index uint64, nodeID string) error {
	existing, err := txn.First("nodes", "id", nodeID)
	if err != nil {
		return fmt.Errorf("node lookup failed: %v", err)
	}
	if existing == nil {
		return nil
	}

	node := existing.(*structs.Node)
	copyNode := node.Copy()
	copyNode.LastAllocUpdateIndex = index

	if err := txn.Insert("nodes", copyNode); err != nil {
		return fmt.Errorf("node update failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", txn.Index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	return nil
}

// UpsertAllocs is used to evict a set of allocations and allocate new ones at
// the same time.
func (s *StateStore) UpsertAllocs(msgType structs.MessageType, index uint64, allocs []*structs.Allocation) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()
	if err := s.upsertAllocsImpl(index, allocs, txn); err != nil {
		return err
	}
	return txn.Commit()
}

// upsertAllocs is the actual implementation of UpsertAllocs so that it may be
// used with an existing transaction.
func (s *StateStore) upsertAllocsImpl(index uint64, allocs []*structs.Allocation, txn *txn) error {
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

			// If the scheduler is marking this allocation as lost or unknown we do not
			// want to reuse the status of the existing allocation.
			if alloc.ClientStatus != structs.AllocClientStatusLost &&
				alloc.ClientStatus != structs.AllocClientStatusUnknown {
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

		if err := s.updatePluginForTerminalAlloc(index, alloc, txn); err != nil {
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
func (s *StateStore) UpdateAllocsDesiredTransitions(msgType structs.MessageType, index uint64, allocs map[string]*structs.DesiredTransition,
	evals []*structs.Evaluation) error {

	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	// Handle each of the updated allocations
	for id, transition := range allocs {
		if err := s.UpdateAllocDesiredTransitionTxn(txn, index, id, transition); err != nil {
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

	return txn.Commit()
}

// UpdateAllocDesiredTransitionTxn is used to nest an update of an
// allocations desired transition
func (s *StateStore) UpdateAllocDesiredTransitionTxn(
	txn *txn, index uint64, allocID string,
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

	// Update the modify indexes
	copyAlloc.ModifyIndex = index
	copyAlloc.AllocModifyIndex = index

	// Update the allocation
	if err := txn.Insert("allocs", copyAlloc); err != nil {
		return fmt.Errorf("alloc insert failed: %v", err)
	}

	return nil
}

// AllocByID is used to lookup an allocation by its ID
func (s *StateStore) AllocByID(ws memdb.WatchSet, id string) (*structs.Allocation, error) {
	txn := s.db.ReadTxn()
	return s.allocByIDImpl(txn, ws, id)
}

// allocByIDImpl retrives an allocation and is called under and existing
// transaction. An optional watch set can be passed to add allocations to the
// watch set
func (s *StateStore) allocByIDImpl(txn Txn, ws memdb.WatchSet, id string) (*structs.Allocation, error) {
	watchCh, raw, err := txn.FirstWatch("allocs", "id", id)
	if err != nil {
		return nil, fmt.Errorf("alloc lookup failed: %v", err)
	}

	ws.Add(watchCh)

	if raw == nil {
		return nil, nil
	}
	alloc := raw.(*structs.Allocation)
	return alloc, nil
}

// AllocsByIDPrefix is used to lookup allocs by prefix
func (s *StateStore) AllocsByIDPrefix(ws memdb.WatchSet, namespace, id string, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var iter memdb.ResultIterator
	var err error

	switch sort {
	case SortReverse:
		iter, err = txn.GetReverse("allocs", "id_prefix", id)
	default:
		iter, err = txn.Get("allocs", "id_prefix", id)
	}
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

		if namespace == structs.AllNamespacesSentinel {
			return false
		}

		return alloc.Namespace != namespace
	}
}

// AllocsByIDPrefixAllNSs is used to lookup allocs by prefix.
func (s *StateStore) AllocsByIDPrefixAllNSs(ws memdb.WatchSet, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get("allocs", "id_prefix", prefix)
	if err != nil {
		return nil, fmt.Errorf("alloc lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// AllocsByNode returns all the allocations by node
func (s *StateStore) AllocsByNode(ws memdb.WatchSet, node string) ([]*structs.Allocation, error) {
	txn := s.db.ReadTxn()

	return allocsByNodeTxn(txn, ws, node)
}

func allocsByNodeTxn(txn ReadTxn, ws memdb.WatchSet, node string) ([]*structs.Allocation, error) {
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

// AllocsByNodeTerminal returns all the allocations by node and terminal
// status.
func (s *StateStore) AllocsByNodeTerminal(ws memdb.WatchSet, node string, terminal bool) ([]*structs.Allocation, error) {
	txn := s.db.ReadTxn()

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

// AllocsByJob returns allocations by job id
func (s *StateStore) AllocsByJob(ws memdb.WatchSet, namespace, jobID string, anyCreateIndex bool) ([]*structs.Allocation, error) {
	txn := s.db.ReadTxn()

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
		if !anyCreateIndex && job != nil && alloc.Job.CreateIndex != job.CreateIndex {
			continue
		}
		out = append(out, raw.(*structs.Allocation))
	}
	return out, nil
}

// AllocsByEval returns all the allocations by eval id
func (s *StateStore) AllocsByEval(ws memdb.WatchSet, evalID string) ([]*structs.Allocation, error) {
	txn := s.db.ReadTxn()

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
	txn := s.db.ReadTxn()

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

// Allocs returns an iterator over all the evaluations.
func (s *StateStore) Allocs(ws memdb.WatchSet, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var it memdb.ResultIterator
	var err error

	switch sort {
	case SortReverse:
		it, err = txn.GetReverse("allocs", "create")
	default:
		it, err = txn.Get("allocs", "create")
	}

	if err != nil {
		return nil, err
	}

	ws.Add(it.WatchCh())

	return it, nil
}

func (s *StateStore) AllocsByNamespaceOrdered(ws memdb.WatchSet, namespace string, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var (
		it    memdb.ResultIterator
		err   error
		exact = terminate(namespace)
	)

	switch sort {
	case SortReverse:
		it, err = txn.GetReverse("allocs", "namespace_create_prefix", exact)
	default:
		it, err = txn.Get("allocs", "namespace_create_prefix", exact)
	}

	if err != nil {
		return nil, err
	}

	ws.Add(it.WatchCh())

	return it, nil
}

// AllocsByNamespace returns an iterator over all the allocations in the
// namespace
func (s *StateStore) AllocsByNamespace(ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()
	return s.allocsByNamespaceImpl(ws, txn, namespace)
}

// allocsByNamespaceImpl returns an iterator over all the allocations in the
// namespace
func (s *StateStore) allocsByNamespaceImpl(ws memdb.WatchSet, txn *txn, namespace string) (memdb.ResultIterator, error) {
	// Walk the entire table
	iter, err := txn.Get("allocs", "namespace", namespace)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// UpsertVaultAccessor is used to register a set of Vault Accessors.
func (s *StateStore) UpsertVaultAccessor(index uint64, accessors []*structs.VaultAccessor) error {
	txn := s.db.WriteTxn(index)
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

	return txn.Commit()
}

// DeleteVaultAccessors is used to delete a set of Vault Accessors
func (s *StateStore) DeleteVaultAccessors(index uint64, accessors []*structs.VaultAccessor) error {
	txn := s.db.WriteTxn(index)
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

	return txn.Commit()
}

// VaultAccessor returns the given Vault accessor
func (s *StateStore) VaultAccessor(ws memdb.WatchSet, accessor string) (*structs.VaultAccessor, error) {
	txn := s.db.ReadTxn()

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
	txn := s.db.ReadTxn()

	iter, err := txn.Get("vault_accessors", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// VaultAccessorsByAlloc returns all the Vault accessors by alloc id
func (s *StateStore) VaultAccessorsByAlloc(ws memdb.WatchSet, allocID string) ([]*structs.VaultAccessor, error) {
	txn := s.db.ReadTxn()

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
	txn := s.db.ReadTxn()

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
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	for _, accessor := range accessors {
		// set the create index
		accessor.CreateIndex = index

		// insert the accessor
		if err := txn.Insert(siTokenAccessorTable, accessor); err != nil {
			return fmt.Errorf("accessor insert failed: %w", err)
		}
	}

	// update the index for this table
	if err := txn.Insert("index", indexEntry(siTokenAccessorTable, index)); err != nil {
		return fmt.Errorf("index update failed: %w", err)
	}

	return txn.Commit()
}

// DeleteSITokenAccessors is used to delete a set of Service Identity token accessors.
func (s *StateStore) DeleteSITokenAccessors(index uint64, accessors []*structs.SITokenAccessor) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// Lookup each accessor
	for _, accessor := range accessors {
		// Delete the accessor
		if err := txn.Delete(siTokenAccessorTable, accessor); err != nil {
			return fmt.Errorf("accessor delete failed: %w", err)
		}
	}

	// update the index for this table
	if err := txn.Insert("index", indexEntry(siTokenAccessorTable, index)); err != nil {
		return fmt.Errorf("index update failed: %w", err)
	}

	return txn.Commit()
}

// SITokenAccessor returns the given Service Identity token accessor.
func (s *StateStore) SITokenAccessor(ws memdb.WatchSet, accessorID string) (*structs.SITokenAccessor, error) {
	txn := s.db.ReadTxn()
	defer txn.Abort()

	watchCh, existing, err := txn.FirstWatch(siTokenAccessorTable, "id", accessorID)
	if err != nil {
		return nil, fmt.Errorf("accessor lookup failed: %w", err)
	}

	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.SITokenAccessor), nil
	}

	return nil, nil
}

// SITokenAccessors returns an iterator of Service Identity token accessors.
func (s *StateStore) SITokenAccessors(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()
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
	txn := s.db.ReadTxn()
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
	txn := s.db.ReadTxn()
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
func (s *StateStore) UpdateDeploymentStatus(msgType structs.MessageType, index uint64, req *structs.DeploymentStatusUpdateRequest) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	if err := s.updateDeploymentStatusImpl(index, req.DeploymentUpdate, txn); err != nil {
		return err
	}

	// Upsert the job if necessary
	if req.Job != nil {
		if err := s.upsertJobImpl(index, nil, req.Job, false, txn); err != nil {
			return err
		}
	}

	// Upsert the optional eval
	if req.Eval != nil {
		if err := s.nestedUpsertEval(txn, index, req.Eval); err != nil {
			return err
		}
	}

	return txn.Commit()
}

// updateDeploymentStatusImpl is used to make deployment status updates
func (s *StateStore) updateDeploymentStatusImpl(index uint64, u *structs.DeploymentStatusUpdate, txn *txn) error {
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
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	if err := s.updateJobStabilityImpl(index, namespace, jobID, jobVersion, stable, txn); err != nil {
		return err
	}

	return txn.Commit()
}

// updateJobStabilityImpl updates the stability of the given job and version
func (s *StateStore) updateJobStabilityImpl(index uint64, namespace, jobID string, jobVersion uint64, stable bool, txn *txn) error {
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
	return s.upsertJobImpl(index, nil, copy, true, txn)
}

func (s *StateStore) UpdateJobVersionTag(index uint64, namespace string, req *structs.JobApplyTagRequest) error {
	jobID := req.JobID
	jobVersion := req.Version
	tag := req.Tag
	name := req.Name

	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	// if no tag is present, this is a tag removal operation.
	if tag == nil {
		if err := s.unsetJobVersionTagImpl(index, namespace, jobID, name, txn); err != nil {
			return err
		}
	} else {
		if err := s.updateJobVersionTagImpl(index, namespace, jobID, jobVersion, tag, txn); err != nil {
			return err
		}
	}

	return txn.Commit()
}

func (s *StateStore) updateJobVersionTagImpl(index uint64, namespace, jobID string, jobVersion uint64, tag *structs.JobVersionTag, txn *txn) error {
	// Note: could use JobByIDAndVersion to get the specific version we want here,
	// but then we'd have to make a second lookup to make sure we're not applying a duplicate tag name
	versions, err := s.JobVersionsByID(nil, namespace, jobID)
	if err != nil {
		return err
	}

	var job *structs.Job

	for _, version := range versions {
		// Allow for a tag to be updated (new description, for example) but otherwise don't allow a same-tagname to a different version.
		if version.VersionTag != nil && version.VersionTag.Name == tag.Name && version.Version != jobVersion {
			return fmt.Errorf("tag %q already exists on a different version of job %q", tag.Name, jobID)
		}
		if version.Version == jobVersion {
			job = version
		}
	}

	if job == nil {
		return fmt.Errorf("job %q version %d not found", jobID, jobVersion)
	}

	versionCopy := job.Copy()
	versionCopy.VersionTag = tag
	versionCopy.ModifyIndex = index

	latestJob, err := s.JobByID(nil, namespace, jobID)
	if err != nil {
		return err
	}
	if versionCopy.Version == latestJob.Version {
		if err := txn.Insert("jobs", versionCopy); err != nil {
			return err
		}
	}

	return s.upsertJobVersion(index, versionCopy, txn)
}

func (s *StateStore) unsetJobVersionTagImpl(index uint64, namespace, jobID string, name string, txn *txn) error {
	job, err := s.JobVersionByTagName(nil, namespace, jobID, name)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("tag %q not found on job %q", name, jobID)
	}

	versionCopy := job.Copy()
	versionCopy.VersionTag = nil
	versionCopy.ModifyIndex = index
	latestJob, err := s.JobByID(nil, namespace, jobID)
	if err != nil {
		return err
	}
	if versionCopy.Version == latestJob.Version {
		if err := txn.Insert("jobs", versionCopy); err != nil {
			return err
		}
	}

	return s.upsertJobVersion(index, versionCopy, txn)
}

// UpdateDeploymentPromotion is used to promote canaries in a deployment and
// potentially make a evaluation
func (s *StateStore) UpdateDeploymentPromotion(msgType structs.MessageType, index uint64, req *structs.ApplyDeploymentPromoteRequest) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
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
	for _, dstate := range deployment.TaskGroups {
		for _, c := range dstate.PlacedCanaries {
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
	for tg, dstate := range deployment.TaskGroups {
		if _, ok := groupIndex[tg]; !req.All && !ok {
			continue
		}

		need := dstate.DesiredCanaries
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

		// reset the progress deadline
		if status.ProgressDeadline > 0 && !status.RequireProgressBy.IsZero() {
			status.RequireProgressBy = time.Now().Add(status.ProgressDeadline)
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

	return txn.Commit()
}

// UpdateDeploymentAllocHealth is used to update the health of allocations as
// part of the deployment and potentially make a evaluation
func (s *StateStore) UpdateDeploymentAllocHealth(msgType structs.MessageType, index uint64, req *structs.ApplyDeploymentAllocHealthRequest) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
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
			copy.DeploymentStatus.Healthy = pointer.Of(healthy)
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
		if err := s.upsertJobImpl(index, nil, req.Job, false, txn); err != nil {
			return err
		}
	}

	// Upsert the optional eval
	if req.Eval != nil {
		if err := s.nestedUpsertEval(txn, index, req.Eval); err != nil {
			return err
		}
	}

	return txn.Commit()
}

// LatestIndex returns the greatest index value for all indexes.
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
	txn := s.db.ReadTxn()

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

// Indexes returns an iterator over all the indexes
func (s *StateStore) Indexes() (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

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
	txn := s.db.WriteTxn(index)
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
			case structs.AllocClientStatusUnknown:
				tg.Unknown += 1
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
	return txn.Commit()
}

// setJobStatuses is a helper for calling setJobStatus on multiple jobs by ID.
// It takes a map of job IDs to an optional forceStatus string. It returns an
// error if the job doesn't exist or setJobStatus fails.
func (s *StateStore) setJobStatuses(index uint64, txn *txn,
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
func (s *StateStore) setJobStatus(index uint64, txn *txn,
	job *structs.Job, evalDelete bool, forceStatus string) error {

	// Capture the current status so we can check if there is a change
	oldStatus := job.Status
	newStatus := forceStatus

	// If forceStatus is not set, compute the jobs status.
	if forceStatus == "" {
		var err error
		newStatus, err = s.getJobStatus(txn, job, evalDelete)
		if err != nil {
			return err
		}
	}

	// Fast-path if the job has not changed.
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
	if err := s.setJobSummary(txn, updated, index, oldStatus, newStatus); err != nil {
		return fmt.Errorf("job summary update failed %w", err)
	}
	return nil
}

func (s *StateStore) setJobSummary(txn *txn, updated *structs.Job, index uint64, oldStatus, newStatus string) error {
	if updated.ParentID == "" {
		return nil
	}

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
	return nil
}

func (s *StateStore) getJobStatus(txn *txn, job *structs.Job, evalDelete bool) (string, error) {
	// System, Periodic and Parameterized jobs are running until explicitly
	// stopped.
	if job.Type == structs.JobTypeSystem ||
		job.IsParameterized() ||
		job.IsPeriodic() {
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
	txn *txn) error {

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
func (s *StateStore) updateJobScalingPolicies(index uint64, job *structs.Job, txn *txn) error {

	ws := memdb.NewWatchSet()

	scalingPolicies := job.GetScalingPolicies()
	newTargets := map[string]bool{}
	for _, p := range scalingPolicies {
		newTargets[p.JobKey()] = true
	}
	// find existing policies that need to be deleted
	deletedPolicies := []string{}
	iter, err := s.ScalingPoliciesByJobTxn(ws, job.Namespace, job.ID, txn)
	if err != nil {
		return fmt.Errorf("ScalingPoliciesByJob lookup failed: %v", err)
	}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		oldPolicy := raw.(*structs.ScalingPolicy)
		if !newTargets[oldPolicy.JobKey()] {
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

// updateJobSubmission stores the original job source and variables associated that the
// job structure originates from. It is up to the job submitter to include the source
// material, and as such sub may be nil, in which case nothing is stored.
func (s *StateStore) updateJobSubmission(index uint64, sub *structs.JobSubmission, namespace, jobID string, version uint64, txn *txn) error {
	// critical that we operate on a copy; the original must not be modified
	// e.g. in the case of job gc and its last second version bump
	sub = sub.Copy()

	switch {
	case sub == nil:
		return nil
	case namespace == "":
		return errors.New("job_submission requires a namespace")
	case jobID == "":
		return errors.New("job_submission requires a jobID")
	default:
		sub.Namespace = namespace
		sub.JobID = jobID
		sub.JobModifyIndex = index
		sub.Version = version
	}

	// check if we already have a submission for this (namespace, jobID, version)
	obj, err := txn.First("job_submission", "id", namespace, jobID, version)
	if err != nil {
		return err
	}
	if obj != nil {
		// if we already have a submission for this (namespace, jobID, version)
		// then there is nothing to do; manually avoid potential for duplicates
		return nil
	}

	// insert the job submission for this (namespace, jobID, version)
	if err := txn.Insert("job_submission", sub); err != nil {
		return err
	}

	// prune old job submissions
	return s.pruneJobSubmissions(namespace, jobID, txn)
}

func (s *StateStore) pruneJobSubmissions(namespace, jobID string, txn *txn) error {
	// although the number of tracked submissions is the same as the number of
	// tracked job versions, do not assume a 1:1 correlation, as there could be
	// holes in the submissions (or none at all)
	limit := s.config.JobTrackedVersions

	// iterate through all stored submissions
	iter, err := txn.Get("job_submission", "id_prefix", namespace, jobID)
	if err != nil {
		return err
	}

	stored := make([]lang.Pair[uint64, uint64], 0, limit+1)
	for next := iter.Next(); next != nil; next = iter.Next() {
		sub := next.(*structs.JobSubmission)
		// scanning by prefix; make sure we collect exact matches only
		if sub.Namespace == namespace && sub.JobID == jobID {
			stored = append(stored, lang.Pair[uint64, uint64]{First: sub.JobModifyIndex, Second: sub.Version})
		}
	}

	// if we are still below the limit, nothing to do
	if len(stored) <= limit {
		return nil
	}

	// sort by job modify index descending so we can just keep the first N
	slices.SortFunc(stored, func(a, b lang.Pair[uint64, uint64]) int {
		var cmp int = 0
		if a.First < b.First {
			cmp = -1
		}
		if a.First > b.First {
			cmp = +1
		}

		// Convert the sort into a descending sort by inverting the sign
		cmp = cmp * -1
		return cmp
	})

	// remove the outdated submission versions
	for _, sub := range stored[limit:] {
		if err = txn.Delete("job_submission", &structs.JobSubmission{
			Namespace: namespace,
			JobID:     jobID,
			Version:   sub.Second,
		}); err != nil {
			return err
		}
	}
	return nil
}

// updateJobCSIPlugins runs on job update, and indexes the job in the plugin
func (s *StateStore) updateJobCSIPlugins(index uint64, job, prev *structs.Job, txn *txn) error {
	plugIns := make(map[string]*structs.CSIPlugin)

	upsertFn := func(job *structs.Job, delete bool) error {
		for _, tg := range job.TaskGroups {
			for _, t := range tg.Tasks {
				if t.CSIPluginConfig == nil {
					continue
				}

				plugIn, ok := plugIns[t.CSIPluginConfig.ID]
				if !ok {
					p, err := s.CSIPluginByIDTxn(txn, nil, t.CSIPluginConfig.ID)
					if err != nil {
						return err
					}
					if p == nil {
						plugIn = structs.NewCSIPlugin(t.CSIPluginConfig.ID, index)
					} else {
						plugIn = p.Copy()
						plugIn.ModifyIndex = index
					}
					plugIns[plugIn.ID] = plugIn
				}

				if delete {
					plugIn.DeleteJob(job, nil)
				} else {
					plugIn.AddJob(job, nil)
				}
			}
		}

		return nil
	}

	if prev != nil {
		err := upsertFn(prev, true)
		if err != nil {
			return err
		}
	}

	err := upsertFn(job, false)
	if err != nil {
		return err
	}

	for _, plugIn := range plugIns {
		err = txn.Insert("csi_plugins", plugIn)
		if err != nil {
			return fmt.Errorf("csi_plugins insert error: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"csi_plugins", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return nil
}

// updateDeploymentWithAlloc is used to update the deployment state associated
// with the given allocation. The passed alloc may be updated if the deployment
// status has changed to capture the modify index at which it has changed.
func (s *StateStore) updateDeploymentWithAlloc(index uint64, alloc, existing *structs.Allocation, txn *txn) error {
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

	dstate := deploymentCopy.TaskGroups[alloc.TaskGroup]
	dstate.PlacedAllocs += placed
	dstate.HealthyAllocs += healthy
	dstate.UnhealthyAllocs += unhealthy

	// Ensure PlacedCanaries accurately reflects the alloc canary status
	if alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Canary {
		found := false
		for _, canary := range dstate.PlacedCanaries {
			if alloc.ID == canary {
				found = true
				break
			}
		}
		if !found {
			dstate.PlacedCanaries = append(dstate.PlacedCanaries, alloc.ID)
		}
	}

	// Update the progress deadline
	if pd := dstate.ProgressDeadline; pd != 0 {
		// If we are the first placed allocation for the deployment start the progress deadline.
		if placed != 0 && dstate.RequireProgressBy.IsZero() {
			// Use modify time instead of create time because we may in-place
			// update the allocation to be part of a new deployment.
			dstate.RequireProgressBy = time.Unix(0, alloc.ModifyTime).Add(pd)
		} else if healthy != 0 {
			if d := alloc.DeploymentStatus.Timestamp.Add(pd); d.After(dstate.RequireProgressBy) {
				dstate.RequireProgressBy = d
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
	existingAlloc *structs.Allocation, txn *txn) error {

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
		case structs.AllocClientStatusUnknown:
			tgSummary.Unknown += 1
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
		case structs.AllocClientStatusUnknown:
			if tgSummary.Unknown > 0 {
				tgSummary.Unknown -= 1
			}
		case structs.AllocClientStatusFailed, structs.AllocClientStatusComplete:
		default:
			s.logger.Error("invalid old client status for allocation",
				"alloc_id", existingAlloc.ID, "client_status", existingAlloc.ClientStatus)
		}
		summaryChanged = true
	}
	jobSummary.Summary[alloc.TaskGroup] = tgSummary

	if summaryChanged {
		jobSummary.ModifyIndex = index

		s.updatePluginWithJobSummary(index, jobSummary, alloc, txn)

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

// updatePluginForTerminalAlloc updates the CSI plugins for an alloc when the
// allocation is updated or inserted with a terminal server status.
func (s *StateStore) updatePluginForTerminalAlloc(index uint64, alloc *structs.Allocation,
	txn *txn) error {

	if !alloc.ServerTerminalStatus() {
		return nil
	}

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	for _, t := range tg.Tasks {
		if t.CSIPluginConfig != nil {
			pluginID := t.CSIPluginConfig.ID
			plug, err := s.CSIPluginByIDTxn(txn, nil, pluginID)
			if err != nil {
				return err
			}
			if plug == nil {
				// plugin may not have been created because it never
				// became healthy, just move on
				return nil
			}
			plug = plug.Copy()
			err = plug.DeleteAlloc(alloc.ID, alloc.NodeID)
			if err != nil {
				return err
			}
			err = updateOrGCPlugin(index, txn, plug)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// updatePluginWithJobSummary updates the CSI plugins for a job when the
// job summary is updated by an alloc
func (s *StateStore) updatePluginWithJobSummary(index uint64, summary *structs.JobSummary, alloc *structs.Allocation,
	txn *txn) error {

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return nil
	}

	for _, t := range tg.Tasks {
		if t.CSIPluginConfig != nil {
			pluginID := t.CSIPluginConfig.ID
			plug, err := s.CSIPluginByIDTxn(txn, nil, pluginID)
			if err != nil {
				return err
			}
			if plug == nil {
				plug = structs.NewCSIPlugin(pluginID, index)
			} else {
				plug = plug.Copy()
			}

			plug.UpdateExpectedWithJob(alloc.Job, summary,
				alloc.Job.Status == structs.JobStatusDead)

			err = updateOrGCPlugin(index, txn, plug)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// UpsertACLPolicies is used to create or update a set of ACL policies
func (s *StateStore) UpsertACLPolicies(msgType structs.MessageType, index uint64, policies []*structs.ACLPolicy) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
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

	return txn.Commit()
}

// DeleteACLPolicies deletes the policies with the given names
func (s *StateStore) DeleteACLPolicies(msgType structs.MessageType, index uint64, names []string) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
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
	return txn.Commit()
}

// ACLPolicyByName is used to lookup a policy by name
func (s *StateStore) ACLPolicyByName(ws memdb.WatchSet, name string) (*structs.ACLPolicy, error) {
	txn := s.db.ReadTxn()

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
	txn := s.db.ReadTxn()

	iter, err := txn.Get("acl_policy", "id_prefix", prefix)
	if err != nil {
		return nil, fmt.Errorf("acl policy lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// ACLPolicyByJob is used to lookup policies that have been attached to a
// specific job
func (s *StateStore) ACLPolicyByJob(ws memdb.WatchSet, ns, jobID string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get("acl_policy", "job_prefix", ns, jobID)
	if err != nil {
		return nil, fmt.Errorf("acl policy lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// ACLPolicies returns an iterator over all the acl policies
func (s *StateStore) ACLPolicies(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire table
	iter, err := txn.Get("acl_policy", "id")
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

// UpsertACLTokens is used to create or update a set of ACL tokens
func (s *StateStore) UpsertACLTokens(msgType structs.MessageType, index uint64, tokens []*structs.ACLToken) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
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
	return txn.Commit()
}

// DeleteACLTokens deletes the tokens with the given accessor ids
func (s *StateStore) DeleteACLTokens(msgType structs.MessageType, index uint64, ids []string) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
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
	return txn.Commit()
}

// ACLTokenByAccessorID is used to lookup a token by accessor ID
func (s *StateStore) ACLTokenByAccessorID(ws memdb.WatchSet, id string) (*structs.ACLToken, error) {
	if id == "" {
		return nil, fmt.Errorf("acl token lookup failed: missing accessor id")
	}

	txn := s.db.ReadTxn()

	watchCh, existing, err := txn.FirstWatch("acl_token", "id", id)
	if err != nil {
		return nil, fmt.Errorf("acl token lookup failed: %v", err)
	}
	ws.Add(watchCh)

	// If the existing token is nil, this indicates it does not exist in state.
	if existing == nil {
		return nil, nil
	}

	// Assert the token type which allows us to perform additional work on the
	// token that is needed before returning the call.
	token := existing.(*structs.ACLToken)

	// Handle potential staleness of ACL role links.
	if token, err = s.fixTokenRoleLinks(txn, token); err != nil {
		return nil, err
	}
	return token, nil
}

// ACLTokenBySecretID is used to lookup a token by secret ID
func (s *StateStore) ACLTokenBySecretID(ws memdb.WatchSet, secretID string) (*structs.ACLToken, error) {
	if secretID == "" {
		return nil, fmt.Errorf("acl token lookup failed: missing secret id")
	}

	txn := s.db.ReadTxn()

	watchCh, existing, err := txn.FirstWatch("acl_token", "secret", secretID)
	if err != nil {
		return nil, fmt.Errorf("acl token lookup failed: %v", err)
	}
	ws.Add(watchCh)

	// If the existing token is nil, this indicates it does not exist in state.
	if existing == nil {
		return nil, nil
	}

	// Assert the token type which allows us to perform additional work on the
	// token that is needed before returning the call.
	token := existing.(*structs.ACLToken)

	// Handle potential staleness of ACL role links.
	if token, err = s.fixTokenRoleLinks(txn, token); err != nil {
		return nil, err
	}
	return token, nil
}

// ACLTokenByAccessorIDPrefix is used to lookup tokens by prefix
func (s *StateStore) ACLTokenByAccessorIDPrefix(ws memdb.WatchSet, prefix string, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var iter memdb.ResultIterator
	var err error

	switch sort {
	case SortReverse:
		iter, err = txn.GetReverse("acl_token", "id_prefix", prefix)
	default:
		iter, err = txn.Get("acl_token", "id_prefix", prefix)
	}
	if err != nil {
		return nil, fmt.Errorf("acl token lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// ACLTokens returns an iterator over all the tokens
func (s *StateStore) ACLTokens(ws memdb.WatchSet, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var iter memdb.ResultIterator
	var err error

	switch sort {
	case SortReverse:
		iter, err = txn.GetReverse("acl_token", "create")
	default:
		iter, err = txn.Get("acl_token", "create")
	}
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// ACLTokensByGlobal returns an iterator over all the tokens filtered by global value
func (s *StateStore) ACLTokensByGlobal(ws memdb.WatchSet, globalVal bool, sort SortOption) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	var iter memdb.ResultIterator
	var err error

	// Walk the entire table
	switch sort {
	case SortReverse:
		iter, err = txn.GetReverse("acl_token", "global", globalVal)
	default:
		iter, err = txn.Get("acl_token", "global", globalVal)
	}
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

// CanBootstrapACLToken checks if bootstrapping is possible and returns the reset index
func (s *StateStore) CanBootstrapACLToken() (bool, uint64, error) {
	txn := s.db.ReadTxn()

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

// BootstrapACLTokens is used to create an initial ACL token.
func (s *StateStore) BootstrapACLTokens(msgType structs.MessageType, index uint64, resetIndex uint64, token *structs.ACLToken) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
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
	return txn.Commit()
}

// UpsertOneTimeToken is used to create or update a set of ACL
// tokens. Validating that we're not upserting an already-expired token is
// made the responsibility of the caller to facilitate testing.
func (s *StateStore) UpsertOneTimeToken(msgType structs.MessageType, index uint64, token *structs.OneTimeToken) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	// we expect the RPC call to set the ExpiresAt
	if token.ExpiresAt.IsZero() {
		return fmt.Errorf("one-time token must have an ExpiresAt time")
	}

	// Update all the indexes
	token.CreateIndex = index
	token.ModifyIndex = index

	// Create the token
	if err := txn.Insert("one_time_token", token); err != nil {
		return fmt.Errorf("upserting one-time token failed: %v", err)
	}

	// Update the indexes table
	if err := txn.Insert("index", &IndexEntry{"one_time_token", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}
	return txn.Commit()
}

// DeleteOneTimeTokens deletes the tokens with the given ACLToken Accessor IDs
func (s *StateStore) DeleteOneTimeTokens(msgType structs.MessageType, index uint64, ids []string) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	var deleted int
	for _, id := range ids {
		d, err := txn.DeleteAll("one_time_token", "id", id)
		if err != nil {
			return fmt.Errorf("deleting one-time token failed: %v", err)
		}
		deleted += d
	}

	if deleted > 0 {
		if err := txn.Insert("index", &IndexEntry{"one_time_token", index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
	}
	return txn.Commit()
}

// ExpireOneTimeTokens deletes tokens that have expired
func (s *StateStore) ExpireOneTimeTokens(msgType structs.MessageType, index uint64, timestamp time.Time) error {
	txn := s.db.WriteTxnMsgT(msgType, index)
	defer txn.Abort()

	iter, err := s.oneTimeTokensExpiredTxn(txn, nil, timestamp)
	if err != nil {
		return err
	}

	var deleted int
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		ott, ok := raw.(*structs.OneTimeToken)
		if !ok || ott == nil {
			return fmt.Errorf("could not decode one-time token")
		}
		d, err := txn.DeleteAll("one_time_token", "secret", ott.OneTimeSecretID)
		if err != nil {
			return fmt.Errorf("deleting one-time token failed: %v", err)
		}
		deleted += d
	}

	if deleted > 0 {
		if err := txn.Insert("index", &IndexEntry{"one_time_token", index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
	}
	return txn.Commit()
}

// oneTimeTokensExpiredTxn returns an iterator over all expired one-time tokens
func (s *StateStore) oneTimeTokensExpiredTxn(txn *txn, ws memdb.WatchSet, timestamp time.Time) (memdb.ResultIterator, error) {
	iter, err := txn.Get("one_time_token", "id")
	if err != nil {
		return nil, fmt.Errorf("one-time token lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())
	iter = memdb.NewFilterIterator(iter, expiredOneTimeTokenFilter(timestamp))
	return iter, nil
}

// OneTimeTokenBySecret is used to lookup a token by secret
func (s *StateStore) OneTimeTokenBySecret(ws memdb.WatchSet, secret string) (*structs.OneTimeToken, error) {
	if secret == "" {
		return nil, fmt.Errorf("one-time token lookup failed: missing secret")
	}

	txn := s.db.ReadTxn()

	watchCh, existing, err := txn.FirstWatch("one_time_token", "secret", secret)
	if err != nil {
		return nil, fmt.Errorf("one-time token lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.OneTimeToken), nil
	}
	return nil, nil
}

// expiredOneTimeTokenFilter returns a filter function that returns only
// expired one-time tokens
func expiredOneTimeTokenFilter(now time.Time) func(interface{}) bool {
	return func(raw interface{}) bool {
		ott, ok := raw.(*structs.OneTimeToken)
		if !ok {
			return true
		}

		return ott.ExpiresAt.After(now)
	}
}

// SchedulerConfig is used to get the current Scheduler configuration.
func (s *StateStore) SchedulerConfig() (uint64, *structs.SchedulerConfiguration, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()
	return s.schedulerConfigTxn(tx)
}

func (s *StateStore) schedulerConfigTxn(txn *txn) (uint64, *structs.SchedulerConfiguration, error) {

	// Get the scheduler config
	c, err := txn.First("scheduler_config", "id")
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
func (s *StateStore) SchedulerSetConfig(index uint64, config *structs.SchedulerConfiguration) error {
	tx := s.db.WriteTxn(index)
	defer tx.Abort()

	s.schedulerSetConfigTxn(index, tx, config)

	return tx.Commit()
}

func (s *StateStore) ClusterMetadata(ws memdb.WatchSet) (*structs.ClusterMetadata, error) {
	txn := s.db.ReadTxn()
	defer txn.Abort()

	// Get the cluster metadata
	watchCh, m, err := txn.FirstWatch("cluster_meta", "id")
	if err != nil {
		return nil, fmt.Errorf("failed cluster metadata lookup: %w", err)
	}
	ws.Add(watchCh)

	if m != nil {
		return m.(*structs.ClusterMetadata), nil
	}

	return nil, nil
}

func (s *StateStore) ClusterSetMetadata(index uint64, meta *structs.ClusterMetadata) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	if err := s.setClusterMetadata(txn, meta); err != nil {
		return fmt.Errorf("set cluster metadata failed: %w", err)
	}

	return txn.Commit()
}

// WithWriteTransaction executes the passed function within a write transaction,
// and returns its result.  If the invocation returns no error, the transaction
// is committed; otherwise, it's aborted.
func (s *StateStore) WithWriteTransaction(msgType structs.MessageType, index uint64, fn func(Txn) error) error {
	tx := s.db.WriteTxnMsgT(msgType, index)
	defer tx.Abort()

	err := fn(tx)
	if err == nil {
		return tx.Commit()
	}
	return err
}

// SchedulerCASConfig is used to update the scheduler configuration with a
// given Raft index. If the CAS index specified is not equal to the last observed index
// for the config, then the call is a noop.
func (s *StateStore) SchedulerCASConfig(index, cidx uint64, config *structs.SchedulerConfiguration) (bool, error) {
	tx := s.db.WriteTxn(index)
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

	s.schedulerSetConfigTxn(index, tx, config)

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *StateStore) schedulerSetConfigTxn(idx uint64, tx *txn, config *structs.SchedulerConfiguration) error {
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

func (s *StateStore) setClusterMetadata(txn *txn, meta *structs.ClusterMetadata) error {
	// Check for an existing config, if it exists, verify that the cluster ID matches
	existing, err := txn.First("cluster_meta", "id")
	if err != nil {
		return fmt.Errorf("failed cluster meta lookup: %v", err)
	}

	if existing != nil {
		existingClusterID := existing.(*structs.ClusterMetadata).ClusterID
		if meta.ClusterID != existingClusterID && existingClusterID != "" {
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

// UpsertScalingPolicies is used to insert a new scaling policy.
func (s *StateStore) UpsertScalingPolicies(index uint64, scalingPolicies []*structs.ScalingPolicy) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	if err := s.UpsertScalingPoliciesTxn(index, scalingPolicies, txn); err != nil {
		return err
	}

	return txn.Commit()
}

// UpsertScalingPoliciesTxn is used to insert a new scaling policy.
func (s *StateStore) UpsertScalingPoliciesTxn(index uint64, scalingPolicies []*structs.ScalingPolicy,
	txn *txn) error {

	hadUpdates := false

	for _, policy := range scalingPolicies {
		// Check if the scaling policy already exists
		// Policy uniqueness is based on target and type
		it, err := txn.Get("scaling_policy", "target",
			policy.Target[structs.ScalingTargetNamespace],
			policy.Target[structs.ScalingTargetJob],
			policy.Target[structs.ScalingTargetGroup],
			policy.Target[structs.ScalingTargetTask],
		)
		if err != nil {
			return fmt.Errorf("scaling policy lookup failed: %v", err)
		}

		// Check if type matches
		var existing *structs.ScalingPolicy
		for raw := it.Next(); raw != nil; raw = it.Next() {
			p := raw.(*structs.ScalingPolicy)
			if p.Type == policy.Type {
				existing = p
				break
			}
		}

		// Setup the indexes correctly
		if existing != nil {
			if !existing.Diff(policy) {
				continue
			}
			policy.ID = existing.ID
			policy.CreateIndex = existing.CreateIndex
		} else {
			// policy.ID must have been set already in Job.Register before log apply
			policy.CreateIndex = index
		}
		policy.ModifyIndex = index

		// Insert the scaling policy
		hadUpdates = true
		if err := txn.Insert("scaling_policy", policy); err != nil {
			return err
		}
	}

	// Update the indexes table for scaling policy if we updated any policies
	if hadUpdates {
		if err := txn.Insert("index", &IndexEntry{"scaling_policy", index}); err != nil {
			return fmt.Errorf("index update failed: %v", err)
		}
	}

	return nil
}

// NamespaceByName is used to lookup a namespace by name
func (s *StateStore) NamespaceByName(ws memdb.WatchSet, name string) (*structs.Namespace, error) {
	txn := s.db.ReadTxn()
	return s.namespaceByNameImpl(ws, txn, name)
}

// namespaceByNameImpl is used to lookup a namespace by name
func (s *StateStore) namespaceByNameImpl(ws memdb.WatchSet, txn *txn, name string) (*structs.Namespace, error) {
	watchCh, existing, err := txn.FirstWatch(TableNamespaces, "id", name)
	if err != nil {
		return nil, fmt.Errorf("namespace lookup failed: %v", err)
	}
	ws.Add(watchCh)

	if existing != nil {
		return existing.(*structs.Namespace), nil
	}
	return nil, nil
}

// namespaceExists returns whether a namespace exists
func (s *StateStore) namespaceExists(txn *txn, namespace string) (bool, error) {
	if namespace == structs.DefaultNamespace {
		return true, nil
	}

	existing, err := txn.First(TableNamespaces, "id", namespace)
	if err != nil {
		return false, fmt.Errorf("namespace lookup failed: %v", err)
	}

	return existing != nil, nil
}

// NamespacesByNamePrefix is used to lookup namespaces by prefix
func (s *StateStore) NamespacesByNamePrefix(ws memdb.WatchSet, namePrefix string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get(TableNamespaces, "id_prefix", namePrefix)
	if err != nil {
		return nil, fmt.Errorf("namespaces lookup failed: %v", err)
	}
	ws.Add(iter.WatchCh())

	return iter, nil
}

// Namespaces returns an iterator over all the namespaces
func (s *StateStore) Namespaces(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire namespace table
	iter, err := txn.Get(TableNamespaces, "id")
	if err != nil {
		return nil, err
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

func (s *StateStore) NamespaceNames() ([]string, error) {
	it, err := s.Namespaces(nil)
	if err != nil {
		return nil, err
	}

	nses := []string{}
	for {
		next := it.Next()
		if next == nil {
			break
		}
		ns := next.(*structs.Namespace)
		nses = append(nses, ns.Name)
	}

	return nses, nil
}

// UpsertNamespaces is used to register or update a set of namespaces.
func (s *StateStore) UpsertNamespaces(index uint64, namespaces []*structs.Namespace) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	for _, ns := range namespaces {
		// Handle upgrade path.
		ns.Canonicalize()
		if err := s.upsertNamespaceImpl(index, txn, ns); err != nil {
			return err
		}
	}

	if err := txn.Insert("index", &IndexEntry{TableNamespaces, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

// upsertNamespaceImpl is used to upsert a namespace
func (s *StateStore) upsertNamespaceImpl(index uint64, txn *txn, namespace *structs.Namespace) error {
	// Ensure the namespace hash is non-nil. This should be done outside the state store
	// for performance reasons, but we check here for defense in depth.
	ns := namespace
	if len(ns.Hash) == 0 {
		ns.SetHash()
	}

	// Check if the namespace already exists
	existing, err := txn.First(TableNamespaces, "id", ns.Name)
	if err != nil {
		return fmt.Errorf("namespace lookup failed: %v", err)
	}

	// Setup the indexes correctly and determine which quotas need to be
	// reconciled
	var oldQuota string
	if existing != nil {
		exist := existing.(*structs.Namespace)
		ns.CreateIndex = exist.CreateIndex
		ns.ModifyIndex = index

		// Grab the old quota on the namespace
		oldQuota = exist.Quota
	} else {
		ns.CreateIndex = index
		ns.ModifyIndex = index
	}

	// Validate that the quota on the new namespace exists
	if ns.Quota != "" {
		exists, err := s.quotaSpecExists(txn, ns.Quota)
		if err != nil {
			return fmt.Errorf("looking up namespace quota %q failed: %v", ns.Quota, err)
		} else if !exists {
			return fmt.Errorf("namespace %q using non-existent quota %q", ns.Name, ns.Quota)
		}
	}

	// Insert the namespace
	if err := txn.Insert(TableNamespaces, ns); err != nil {
		return fmt.Errorf("namespace insert failed: %v", err)
	}

	// Reconcile changed quotas
	return s.quotaReconcile(index, txn, ns.Quota, oldQuota)
}

// DeleteNamespaces is used to remove a set of namespaces
func (s *StateStore) DeleteNamespaces(index uint64, names []string) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	for _, name := range names {
		// Lookup the namespace
		existing, err := txn.First(TableNamespaces, "id", name)
		if err != nil {
			return fmt.Errorf("namespace lookup failed: %v", err)
		}
		if existing == nil {
			return fmt.Errorf("namespace not found")
		}

		ns := existing.(*structs.Namespace)
		if ns.Name == structs.DefaultNamespace {
			return fmt.Errorf("default namespace can not be deleted")
		}

		// Ensure that the namespace doesn't have any non-terminal jobs
		iter, err := s.jobsByNamespaceImpl(nil, name, txn, SortDefault)
		if err != nil {
			return err
		}

		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			job := raw.(*structs.Job)

			if job.Status != structs.JobStatusDead {
				return fmt.Errorf("namespace %q contains at least one non-terminal job %q. "+
					"All jobs must be terminal in namespace before it can be deleted", name, job.ID)
			}
		}

		vIter, err := s.csiVolumesByNamespaceImpl(txn, nil, name, "")
		if err != nil {
			return err
		}
		rawVol := vIter.Next()
		if rawVol != nil {
			vol := rawVol.(*structs.CSIVolume)
			return fmt.Errorf("namespace %q contains at least one CSI volume %q. "+
				"All CSI volumes in namespace must be deleted before it can be deleted", name, vol.ID)
		}

		varIter, err := s.getVariablesByNamespaceImpl(txn, nil, name)
		if err != nil {
			return err
		}
		if varIter.Next() != nil {
			// unlike job/volume, don't show the path here because the user may
			// not have List permissions on the vars in this namespace
			return fmt.Errorf("namespace %q contains at least one variable. "+
				"All variables in namespace must be deleted before it can be deleted", name)
		}

		// Delete the namespace
		if err := txn.Delete(TableNamespaces, existing); err != nil {
			return fmt.Errorf("namespace deletion failed: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{TableNamespaces, index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	return txn.Commit()
}

func (s *StateStore) DeleteScalingPolicies(index uint64, ids []string) error {
	txn := s.db.WriteTxn(index)
	defer txn.Abort()

	err := s.DeleteScalingPoliciesTxn(index, ids, txn)
	if err == nil {
		return txn.Commit()
	}

	return err
}

// DeleteScalingPoliciesTxn is used to delete a set of scaling policies by ID.
func (s *StateStore) DeleteScalingPoliciesTxn(index uint64, ids []string, txn *txn) error {
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

// ScalingPolicies returns an iterator over all the scaling policies
func (s *StateStore) ScalingPolicies(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	// Walk the entire scaling_policy table
	iter, err := txn.Get("scaling_policy", "id")
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// ScalingPoliciesByTypePrefix returns an iterator over scaling policies with a certain type prefix.
func (s *StateStore) ScalingPoliciesByTypePrefix(ws memdb.WatchSet, t string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get("scaling_policy", "type_prefix", t)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
}

func (s *StateStore) ScalingPoliciesByNamespace(ws memdb.WatchSet, namespace, typ string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get("scaling_policy", "target_prefix", namespace)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	// Wrap the iterator in a filter to exact match the namespace
	iter = memdb.NewFilterIterator(iter, scalingPolicyNamespaceFilter(namespace))

	// If policy type is specified as well, wrap again
	if typ != "" {
		iter = memdb.NewFilterIterator(iter, func(raw interface{}) bool {
			p, ok := raw.(*structs.ScalingPolicy)
			if !ok {
				return true
			}
			return !strings.HasPrefix(p.Type, typ)
		})
	}

	return iter, nil
}

func (s *StateStore) ScalingPoliciesByJob(ws memdb.WatchSet, namespace, jobID, policyType string) (memdb.ResultIterator,
	error) {
	txn := s.db.ReadTxn()
	iter, err := s.ScalingPoliciesByJobTxn(ws, namespace, jobID, txn)
	if err != nil {
		return nil, err
	}

	if policyType == "" {
		return iter, nil
	}

	filter := func(raw interface{}) bool {
		p, ok := raw.(*structs.ScalingPolicy)
		if !ok {
			return true
		}
		return policyType != p.Type
	}

	return memdb.NewFilterIterator(iter, filter), nil
}

func (s *StateStore) ScalingPoliciesByJobTxn(ws memdb.WatchSet, namespace, jobID string,
	txn *txn) (memdb.ResultIterator, error) {

	iter, err := txn.Get("scaling_policy", "target_prefix", namespace, jobID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())

	filter := func(raw interface{}) bool {
		d, ok := raw.(*structs.ScalingPolicy)
		if !ok {
			return true
		}

		return d.Target[structs.ScalingTargetJob] != jobID
	}

	// Wrap the iterator in a filter
	wrap := memdb.NewFilterIterator(iter, filter)
	return wrap, nil
}

func (s *StateStore) ScalingPolicyByID(ws memdb.WatchSet, id string) (*structs.ScalingPolicy, error) {
	txn := s.db.ReadTxn()

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

// ScalingPolicyByTargetAndType returns a fully-qualified policy against a target and policy type,
// or nil if it does not exist. This method does not honor the watchset on the policy type, just the target.
func (s *StateStore) ScalingPolicyByTargetAndType(ws memdb.WatchSet, target map[string]string, typ string) (*structs.ScalingPolicy,
	error) {
	txn := s.db.ReadTxn()

	namespace := target[structs.ScalingTargetNamespace]
	job := target[structs.ScalingTargetJob]
	group := target[structs.ScalingTargetGroup]
	task := target[structs.ScalingTargetTask]

	it, err := txn.Get("scaling_policy", "target", namespace, job, group, task)
	if err != nil {
		return nil, fmt.Errorf("scaling_policy lookup failed: %v", err)
	}

	ws.Add(it.WatchCh())

	// Check for type
	var existing *structs.ScalingPolicy
	for raw := it.Next(); raw != nil; raw = it.Next() {
		p := raw.(*structs.ScalingPolicy)
		if p.Type == typ {
			existing = p
			break
		}
	}

	if existing != nil {
		return existing, nil
	}

	return nil, nil
}

func (s *StateStore) ScalingPoliciesByIDPrefix(ws memdb.WatchSet, namespace string, prefix string) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()

	iter, err := txn.Get("scaling_policy", "id_prefix", prefix)
	if err != nil {
		return nil, fmt.Errorf("scaling policy lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	iter = memdb.NewFilterIterator(iter, scalingPolicyNamespaceFilter(namespace))

	return iter, nil
}

// scalingPolicyNamespaceFilter returns a filter function that filters all
// scaling policies not targeting the given namespace.
func scalingPolicyNamespaceFilter(namespace string) func(interface{}) bool {
	return func(raw interface{}) bool {
		p, ok := raw.(*structs.ScalingPolicy)
		if !ok {
			return true
		}

		return p.Target[structs.ScalingTargetNamespace] != namespace
	}
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
//
// This should only be called on terminal allocs, particularly stopped or preempted allocs
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
			if allocDiff.FollowupEvalID != "" {
				allocCopy.FollowupEvalID = allocDiff.FollowupEvalID
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

func getPreemptedAllocDesiredDescription(preemptedByAllocID string) string {
	return fmt.Sprintf("Preempted by alloc ID %v", preemptedByAllocID)
}
