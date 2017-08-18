package state

import (
	"fmt"
	"io"
	"log"

	"github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// IndexEntry is used with the "index" table
// for managing the latest Raft index affecting a table.
type IndexEntry struct {
	Key   string
	Value uint64
}

// The StateStore is responsible for maintaining all the Nomad
// state. It is manipulated by the FSM which maintains consistency
// through the use of Raft. The goals of the StateStore are to provide
// high concurrency for read operations without blocking writes, and
// to provide write availability in the face of reads. EVERY object
// returned as a result of a read against the state store should be
// considered a constant and NEVER modified in place.
type StateStore struct {
	logger *log.Logger
	db     *memdb.MemDB

	// abandonCh is used to signal watchers that this state store has been
	// abandoned (usually during a restore). This is only ever closed.
	abandonCh chan struct{}
}

// NewStateStore is used to create a new state store
func NewStateStore(logOutput io.Writer) (*StateStore, error) {
	// Create the MemDB
	db, err := memdb.NewMemDB(stateStoreSchema())
	if err != nil {
		return nil, fmt.Errorf("state store setup failed: %v", err)
	}

	// Create the state store
	s := &StateStore{
		logger:    log.New(logOutput, "", log.LstdFlags),
		db:        db,
		abandonCh: make(chan struct{}),
	}
	return s, nil
}

// Snapshot is used to create a point in time snapshot. Because
// we use MemDB, we just need to snapshot the state of the underlying
// database.
func (s *StateStore) Snapshot() (*StateSnapshot, error) {
	snap := &StateSnapshot{
		StateStore: StateStore{
			logger: s.logger,
			db:     s.db.Snapshot(),
		},
	}
	return snap, nil
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

// UpsertPlanResults is used to upsert the results of a plan.
func (s *StateStore) UpsertPlanResults(index uint64, results *structs.ApplyPlanResultsRequest) error {
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

	// Attach the job to all the allocations. It is pulled out in the payload to
	// avoid the redundancy of encoding, but should be denormalized prior to
	// being inserted into MemDB.
	structs.DenormalizeAllocationJobs(results.Job, results.Alloc)

	// Calculate the total resources of allocations. It is pulled out in the
	// payload to avoid encoding something that can be computed, but should be
	// denormalized prior to being inserted into MemDB.
	for _, alloc := range results.Alloc {
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

	// Upsert the allocations
	if err := s.upsertAllocsImpl(index, results.Alloc, txn); err != nil {
		return err
	}

	txn.Commit()
	return nil
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
	existing, err := txn.First("job_summary", "id", jobSummary.JobID)
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
func (s *StateStore) DeleteJobSummary(index uint64, id string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Delete the job summary
	if _, err := txn.DeleteAll("job_summary", "id", id); err != nil {
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
		if err := s.updateJobStabilityImpl(index, deployment.JobID, deployment.JobVersion, true, txn); err != nil {
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

func (s *StateStore) DeploymentsByIDPrefix(ws memdb.WatchSet, deploymentID string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	// Walk the entire deployments table
	iter, err := txn.Get("deployment", "id_prefix", deploymentID)
	if err != nil {
		return nil, err
	}

	ws.Add(iter.WatchCh())
	return iter, nil
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

func (s *StateStore) DeploymentsByJobID(ws memdb.WatchSet, jobID string) ([]*structs.Deployment, error) {
	txn := s.db.Txn(false)

	// Get an iterator over the deployments
	iter, err := txn.Get("deployment", "job", jobID)
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
		out = append(out, d)
	}

	return out, nil
}

// LatestDeploymentByJobID returns the latest deployment for the given job. The
// latest is determined strictly by CreateIndex.
func (s *StateStore) LatestDeploymentByJobID(ws memdb.WatchSet, jobID string) (*structs.Deployment, error) {
	txn := s.db.Txn(false)

	// Get an iterator over the deployments
	iter, err := txn.Get("deployment", "job", jobID)
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
// of drain which is set by the scheduler.
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
		node.Drain = exist.Drain // Retain the drain mode
	} else {
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

// DeleteNode is used to deregister a node
func (s *StateStore) DeleteNode(index uint64, nodeID string) error {
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

	// Delete the node
	if err := txn.Delete("nodes", existing); err != nil {
		return fmt.Errorf("node delete failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"nodes", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// UpdateNodeStatus is used to update the status of a node
func (s *StateStore) UpdateNodeStatus(index uint64, nodeID, status string) error {
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
	copyNode := new(structs.Node)
	*copyNode = *existingNode

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

// UpdateNodeDrain is used to update the drain of a node
func (s *StateStore) UpdateNodeDrain(index uint64, nodeID string, drain bool) error {
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
	copyNode := new(structs.Node)
	*copyNode = *existingNode

	// Update the drain in the copy
	copyNode.Drain = drain
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

// upsertJobImpl is the implementation for registering a job or updating a job definition
func (s *StateStore) upsertJobImpl(index uint64, job *structs.Job, keepVersion bool, txn *memdb.Txn) error {
	// Check if the job already exists
	existing, err := txn.First("jobs", "id", job.ID)
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
		updated, err := txn.First("jobs", "id", job.ID)
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

	// Create the EphemeralDisk if it's nil by adding up DiskMB from task resources.
	// COMPAT 0.4.1 -> 0.5
	s.addEphemeralDiskToTaskGroups(job)

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
func (s *StateStore) DeleteJob(index uint64, jobID string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Lookup the node
	existing, err := txn.First("jobs", "id", jobID)
	if err != nil {
		return fmt.Errorf("job lookup failed: %v", err)
	}
	if existing == nil {
		return fmt.Errorf("job not found")
	}

	// Check if we should update a parent job summary
	job := existing.(*structs.Job)
	if job.ParentID != "" {
		summaryRaw, err := txn.First("job_summary", "id", job.ParentID)
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
	if _, err = txn.DeleteAll("job_summary", "id", jobID); err != nil {
		return fmt.Errorf("deleing job summary failed: %v", err)
	}
	if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
		return fmt.Errorf("index update failed: %v", err)
	}

	txn.Commit()
	return nil
}

// deleteJobVersions deletes all versions of the given job.
func (s *StateStore) deleteJobVersions(index uint64, job *structs.Job, txn *memdb.Txn) error {
	iter, err := txn.Get("job_version", "id_prefix", job.ID)
	if err != nil {
		return err
	}

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

		if _, err = txn.DeleteAll("job_version", "id", j.ID, j.Version); err != nil {
			return fmt.Errorf("deleting job versions failed: %v", err)
		}
	}

	if err := txn.Insert("index", &IndexEntry{"job_summary", index}); err != nil {
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
	all, err := s.jobVersionByID(txn, nil, job.ID)
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
func (s *StateStore) JobByID(ws memdb.WatchSet, id string) (*structs.Job, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("jobs", "id", id)
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
func (s *StateStore) JobsByIDPrefix(ws memdb.WatchSet, id string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("jobs", "id_prefix", id)
	if err != nil {
		return nil, fmt.Errorf("job lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// JobVersionsByID returns all the tracked versions of a job.
func (s *StateStore) JobVersionsByID(ws memdb.WatchSet, id string) ([]*structs.Job, error) {
	txn := s.db.Txn(false)
	return s.jobVersionByID(txn, &ws, id)
}

// jobVersionByID is the underlying implementation for retrieving all tracked
// versions of a job and is called under an existing transaction. A watch set
// can optionally be passed in to add the job histories to the watch set.
func (s *StateStore) jobVersionByID(txn *memdb.Txn, ws *memdb.WatchSet, id string) ([]*structs.Job, error) {
	// Get all the historic jobs for this ID
	iter, err := txn.Get("job_version", "id_prefix", id)
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

	// Reverse so that highest versions first
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	return all, nil
}

// JobByIDAndVersion returns the job identified by its ID and Version. The
// passed watchset may be nil.
func (s *StateStore) JobByIDAndVersion(ws memdb.WatchSet, id string, version uint64) (*structs.Job, error) {
	txn := s.db.Txn(false)
	return s.jobByIDAndVersionImpl(ws, id, version, txn)
}

// jobByIDAndVersionImpl returns the job identified by its ID and Version. The
// passed watchset may be nil.
func (s *StateStore) jobByIDAndVersionImpl(ws memdb.WatchSet, id string, version uint64, txn *memdb.Txn) (*structs.Job, error) {
	watchCh, existing, err := txn.FirstWatch("job_version", "id", id, version)
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
func (s *StateStore) JobSummaryByID(ws memdb.WatchSet, jobID string) (*structs.JobSummary, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("job_summary", "id", jobID)
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
func (s *StateStore) JobSummaryByPrefix(ws memdb.WatchSet, id string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("job_summary", "id_prefix", id)
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
	existing, err := txn.First("periodic_launch", "id", launch.ID)
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
func (s *StateStore) DeletePeriodicLaunch(index uint64, jobID string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	// Lookup the launch
	existing, err := txn.First("periodic_launch", "id", jobID)
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

	txn.Commit()
	return nil
}

// PeriodicLaunchByID is used to lookup a periodic launch by the periodic job
// ID.
func (s *StateStore) PeriodicLaunchByID(ws memdb.WatchSet, id string) (*structs.PeriodicLaunch, error) {
	txn := s.db.Txn(false)

	watchCh, existing, err := txn.FirstWatch("periodic_launch", "id", id)
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

	// Do a nested upsert
	jobs := make(map[string]string, len(evals))
	for _, eval := range evals {
		if err := s.nestedUpsertEval(txn, index, eval); err != nil {
			return err
		}

		jobs[eval.JobID] = ""
	}

	// Set the job's status
	if err := s.setJobStatuses(index, txn, jobs, false); err != nil {
		return fmt.Errorf("setting job status failed: %v", err)
	}

	txn.Commit()
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
	summaryRaw, err := txn.First("job_summary", "id", eval.JobID)
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
				s.logger.Printf("[ERR] state_store: unable to update queued for job %q and task group %q", eval.JobID, tg)
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
		iter, err := txn.Get("evals", "job", eval.JobID, structs.EvalStatusBlocked)
		if err != nil {
			return fmt.Errorf("failed to get blocked evals for job %q: %v", eval.JobID, err)
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

// DeleteEval is used to delete an evaluation
func (s *StateStore) DeleteEval(index uint64, evals []string, allocs []string) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	jobs := make(map[string]string, len(evals))
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
		jobID := existing.(*structs.Evaluation).JobID
		jobs[jobID] = ""
	}

	for _, alloc := range allocs {
		existing, err := txn.First("allocs", "id", alloc)
		if err != nil {
			return fmt.Errorf("alloc lookup failed: %v", err)
		}
		if existing == nil {
			continue
		}
		if err := txn.Delete("allocs", existing); err != nil {
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

// EvalsByIDPrefix is used to lookup evaluations by prefix
func (s *StateStore) EvalsByIDPrefix(ws memdb.WatchSet, id string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("evals", "id_prefix", id)
	if err != nil {
		return nil, fmt.Errorf("eval lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	return iter, nil
}

// EvalsByJob returns all the evaluations by job id
func (s *StateStore) EvalsByJob(ws memdb.WatchSet, jobID string) ([]*structs.Evaluation, error) {
	txn := s.db.Txn(false)

	// Get an iterator over the node allocations
	iter, err := txn.Get("evals", "job_prefix", jobID)
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
	copyAlloc.DeploymentStatus = alloc.DeploymentStatus

	// Update the modify index
	copyAlloc.ModifyIndex = index

	// TODO TEST
	if err := s.updateDeploymentWithAlloc(index, copyAlloc, exist, txn); err != nil {
		return fmt.Errorf("error updating deployment: %v", err)
	}

	if err := s.updateSummaryWithAlloc(index, copyAlloc, exist, txn); err != nil {
		return fmt.Errorf("error updating job summary: %v", err)
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
	jobs := map[string]string{exist.JobID: forceStatus}
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
	jobs := make(map[string]string, 1)
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

		if err := s.updateDeploymentWithAlloc(index, alloc, exist, txn); err != nil {
			return fmt.Errorf("error updating deployment: %v", err)
		}

		if err := s.updateSummaryWithAlloc(index, alloc, exist, txn); err != nil {
			return fmt.Errorf("error updating job summary: %v", err)
		}

		// Create the EphemeralDisk if it's nil by adding up DiskMB from task resources.
		// COMPAT 0.4.1 -> 0.5
		if alloc.Job != nil {
			s.addEphemeralDiskToTaskGroups(alloc.Job)
		}

		if err := txn.Insert("allocs", alloc); err != nil {
			return fmt.Errorf("alloc insert failed: %v", err)
		}

		// If the allocation is running, force the job to running status.
		forceStatus := ""
		if !alloc.TerminalStatus() {
			forceStatus = structs.JobStatusRunning
		}
		jobs[alloc.JobID] = forceStatus
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
func (s *StateStore) AllocsByIDPrefix(ws memdb.WatchSet, id string) (memdb.ResultIterator, error) {
	txn := s.db.Txn(false)

	iter, err := txn.Get("allocs", "id_prefix", id)
	if err != nil {
		return nil, fmt.Errorf("alloc lookup failed: %v", err)
	}

	ws.Add(iter.WatchCh())

	return iter, nil
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
func (s *StateStore) AllocsByJob(ws memdb.WatchSet, jobID string, all bool) ([]*structs.Allocation, error) {
	txn := s.db.Txn(false)

	// Get the job
	var job *structs.Job
	rawJob, err := txn.First("jobs", "id", jobID)
	if err != nil {
		return nil, err
	}
	if rawJob != nil {
		job = rawJob.(*structs.Job)
	}

	// Get an iterator over the node allocations
	iter, err := txn.Get("allocs", "job", jobID)
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
		if err := s.updateJobStabilityImpl(index, copy.JobID, copy.JobVersion, true, txn); err != nil {
			return fmt.Errorf("failed to update job stability: %v", err)
		}
	}

	return nil
}

// UpdateJobStability updates the stability of the given job and version to the
// desired status.
func (s *StateStore) UpdateJobStability(index uint64, jobID string, jobVersion uint64, stable bool) error {
	txn := s.db.Txn(true)
	defer txn.Abort()

	if err := s.updateJobStabilityImpl(index, jobID, jobVersion, stable, txn); err != nil {
		return err
	}

	txn.Commit()
	return nil
}

// updateJobStabilityImpl updates the stability of the given job and version
func (s *StateStore) updateJobStabilityImpl(index uint64, jobID string, jobVersion uint64, stable bool, txn *memdb.Txn) error {
	// Get the job that is referenced
	job, err := s.jobByIDAndVersionImpl(nil, jobID, jobVersion, txn)
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

	groupIndex := make(map[string]struct{}, len(req.Groups))
	for _, g := range req.Groups {
		groupIndex[g] = struct{}{}
	}

	canaryIndex := make(map[string]struct{}, len(deployment.TaskGroups))
	for _, state := range deployment.TaskGroups {
		for _, c := range state.PlacedCanaries {
			canaryIndex[c] = struct{}{}
		}
	}

	haveCanaries := false
	var unhealthyErr multierror.Error
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
		if !alloc.DeploymentStatus.IsHealthy() {
			multierror.Append(&unhealthyErr, fmt.Errorf("Canary allocation %q for group %q is not healthy", alloc.ID, alloc.TaskGroup))
			continue
		}

		haveCanaries = true
	}

	if err := unhealthyErr.ErrorOrNil(); err != nil {
		return err
	}

	if !haveCanaries {
		return fmt.Errorf("no canaries to promote")
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
		setAllocHealth := func(id string, healthy bool) error {
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
			copy.DeploymentStatus.ModifyIndex = index

			if err := s.updateDeploymentWithAlloc(index, copy, old, txn); err != nil {
				return fmt.Errorf("error updating deployment: %v", err)
			}

			if err := txn.Insert("allocs", copy); err != nil {
				return fmt.Errorf("alloc insert failed: %v", err)
			}

			return nil
		}

		for _, id := range req.HealthyAllocationIDs {
			if err := setAllocHealth(id, true); err != nil {
				return err
			}
		}
		for _, id := range req.UnhealthyAllocationIDs {
			if err := setAllocHealth(id, false); err != nil {
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
	for {
		rawJob := iter.Next()
		if rawJob == nil {
			break
		}
		job := rawJob.(*structs.Job)

		// Create a job summary for the job
		summary := &structs.JobSummary{
			JobID:   job.ID,
			Summary: make(map[string]structs.TaskGroupSummary),
		}
		for _, tg := range job.TaskGroups {
			summary.Summary[tg.Name] = structs.TaskGroupSummary{}
		}

		// Find all the allocations for the jobs
		iterAllocs, err := txn.Get("allocs", "job", job.ID)
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
				s.logger.Printf("[ERR] state_store: invalid client status: %v in allocation %q", alloc.ClientStatus, alloc.ID)
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
	jobs map[string]string, evalDelete bool) error {
	for job, forceStatus := range jobs {
		existing, err := txn.First("jobs", "id", job)
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
		summaryRaw, err := txn.First("job_summary", "id", updated.ParentID)
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
	allocs, err := txn.Get("allocs", "job", job.ID)
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

	evals, err := txn.Get("evals", "job_prefix", job.ID)
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

	// system jobs are running until explicitly stopped (which is handled elsewhere)
	if job.Type == structs.JobTypeSystem {
		if job.Stop {
			return structs.JobStatusDead, nil
		}

		// Pending until at least one eval has completed
		return structs.JobStatusRunning, nil
	}

	// The job is dead if all the allocations and evals are terminal or if there
	// are no evals because of garbage collection.
	if evalDelete || hasEval || hasAlloc {
		return structs.JobStatusDead, nil
	}

	// If there are no allocations or evaluations it is a new job. If the
	// job is periodic or is a parameterized job, we mark it as running as
	// it will never have an allocation/evaluation against it.
	if job.IsPeriodic() || job.IsParameterized() {
		// If the job is stopped mark it as dead
		if job.Stop {
			return structs.JobStatusDead, nil
		}

		return structs.JobStatusRunning, nil
	}
	return structs.JobStatusPending, nil
}

// updateSummaryWithJob creates or updates job summaries when new jobs are
// upserted or existing ones are updated
func (s *StateStore) updateSummaryWithJob(index uint64, job *structs.Job,
	txn *memdb.Txn) error {

	// Update the job summary
	summaryRaw, err := txn.First("job_summary", "id", job.ID)
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
	existingHealthSet := existing != nil && existing.DeploymentStatus != nil && existing.DeploymentStatus.Healthy != nil
	allocHealthSet := alloc.DeploymentStatus != nil && alloc.DeploymentStatus.Healthy != nil
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

	summaryRaw, err := txn.First("job_summary", "id", alloc.JobID)
	if err != nil {
		return fmt.Errorf("unable to lookup job summary for job id %q: %v", alloc.JobID, err)
	}

	if summaryRaw == nil {
		// Check if the job is de-registered
		rawJob, err := txn.First("jobs", "id", alloc.JobID)
		if err != nil {
			return fmt.Errorf("unable to query job: %v", err)
		}

		// If the job is de-registered then we skip updating it's summary
		if rawJob == nil {
			return nil
		}

		return fmt.Errorf("job summary for job %q is not present", alloc.JobID)
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
			s.logger.Printf("[ERR] state_store: new allocation inserted into state store with id: %v and state: %v",
				alloc.ID, alloc.DesiredStatus)
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
			s.logger.Printf("[ERR] state_store: new allocation inserted into state store with id: %v and state: %v",
				alloc.ID, alloc.ClientStatus)
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
			tgSummary.Running -= 1
		case structs.AllocClientStatusPending:
			tgSummary.Starting -= 1
		case structs.AllocClientStatusLost:
			tgSummary.Lost -= 1
		case structs.AllocClientStatusFailed, structs.AllocClientStatusComplete:
		default:
			s.logger.Printf("[ERR] state_store: invalid old state of allocation with id: %v, and state: %v",
				existingAlloc.ID, existingAlloc.ClientStatus)
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

// addEphemeralDiskToTaskGroups adds missing EphemeralDisk objects to TaskGroups
func (s *StateStore) addEphemeralDiskToTaskGroups(job *structs.Job) {
	for _, tg := range job.TaskGroups {
		var diskMB int
		for _, task := range tg.Tasks {
			if task.Resources != nil {
				diskMB += task.Resources.DiskMB
				task.Resources.DiskMB = 0
			}
		}
		if tg.EphemeralDisk != nil {
			continue
		}
		tg.EphemeralDisk = &structs.EphemeralDisk{
			SizeMB: diskMB,
		}
	}
}

// StateSnapshot is used to provide a point-in-time snapshot
type StateSnapshot struct {
	StateStore
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
	// Create the EphemeralDisk if it's nil by adding up DiskMB from task resources.
	// COMPAT 0.4.1 -> 0.5
	r.addEphemeralDiskToTaskGroups(job)

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
	// Set the shared resources if it's not present
	// COMPAT 0.4.1 -> 0.5
	if alloc.SharedResources == nil {
		alloc.SharedResources = &structs.Resources{
			DiskMB: alloc.Resources.DiskMB,
		}
	}

	// Create the EphemeralDisk if it's nil by adding up DiskMB from task resources.
	if alloc.Job != nil {
		r.addEphemeralDiskToTaskGroups(alloc.Job)
	}

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

// addEphemeralDiskToTaskGroups adds missing EphemeralDisk objects to TaskGroups
func (r *StateRestore) addEphemeralDiskToTaskGroups(job *structs.Job) {
	for _, tg := range job.TaskGroups {
		if tg.EphemeralDisk != nil {
			continue
		}
		var sizeMB int
		for _, task := range tg.Tasks {
			if task.Resources != nil {
				sizeMB += task.Resources.DiskMB
				task.Resources.DiskMB = 0
			}
		}
		tg.EphemeralDisk = &structs.EphemeralDisk{
			SizeMB: sizeMB,
		}
	}
}
