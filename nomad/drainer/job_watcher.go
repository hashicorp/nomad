package drainer

import (
	"context"
	"log"
	"sync"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// jobWatcher watches allocation changes for jobs with at least one allocation
// on a draining node.
type jobWatcher struct {
	// allocsIndex to start watching from
	allocsIndex uint64

	// job -> node.ID
	jobs   map[jobKey]string
	jobsMu sync.Mutex

	jobsCh chan map[jobKey]struct{}

	state *state.StateStore

	logger *log.Logger
}

func newJobWatcher(logger *log.Logger, jobs map[jobKey]string, allocsIndex uint64, state *state.StateStore) *jobWatcher {
	return &jobWatcher{
		allocsIndex: allocsIndex,
		logger:      logger,
		jobs:        jobs,
		jobsCh:      make(chan map[jobKey]struct{}),
		state:       state,
	}
}

func (j *jobWatcher) watch(k jobKey, nodeID string) {
	j.logger.Printf("[TRACE] nomad.drain: watching job %s on draining node %s", k.jobid, nodeID[:6])
	j.jobsMu.Lock()
	j.jobs[k] = nodeID
	j.jobsMu.Unlock()
}

func (j *jobWatcher) nodeDone(nodeID string) {
	j.jobsMu.Lock()
	defer j.jobsMu.Unlock()
	for k, v := range j.jobs {
		if v == nodeID {
			j.logger.Printf("[TRACE] nomad.drain: UNwatching job %s on done draining node %s", k.jobid, nodeID[:6])
			delete(j.jobs, k)
		}
	}
}

func (j *jobWatcher) WaitCh() <-chan map[jobKey]struct{} {
	return j.jobsCh
}

func (j *jobWatcher) run(ctx context.Context) {
	var resp interface{}
	var err error

	for {
		//FIXME have watchAllocs create a closure and give it a copy of j.jobs to remove locking?
		//FIXME it seems possible for this to return a nil error and a 0 index, what to do in that case?
		var newIndex uint64
		resp, newIndex, err = j.state.BlockingQuery(j.watchAllocs, j.allocsIndex, ctx)
		if err != nil {
			if err == context.Canceled {
				j.logger.Printf("[TRACE] nomad.drain: job watcher shutting down")
				return
			}
			j.logger.Printf("[ERR] nomad.drain: error blocking on alloc updates: %v", err)
			return
		}

		j.logger.Printf("[TRACE] nomad.drain: job watcher old index: %d new index: %d", j.allocsIndex, newIndex)
		j.allocsIndex = newIndex

		changedJobs := resp.(map[jobKey]struct{})
		if len(changedJobs) > 0 {
			select {
			case j.jobsCh <- changedJobs:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (j *jobWatcher) watchAllocs(ws memdb.WatchSet, state *state.StateStore) (interface{}, uint64, error) {
	iter, err := state.Allocs(ws)
	if err != nil {
		return nil, 0, err
	}

	index, err := state.Index("allocs")
	if err != nil {
		return nil, 0, err
	}

	skipped := 0

	// job ids
	resp := map[jobKey]struct{}{}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		alloc := raw.(*structs.Allocation)

		j.jobsMu.Lock()
		_, ok := j.jobs[jobKey{alloc.Namespace, alloc.JobID}]
		j.jobsMu.Unlock()

		if !ok {
			// alloc is not part of a draining job
			skipped++
			continue
		}

		// don't wake drain loop if alloc hasn't updated its health
		if alloc.DeploymentStatus.IsHealthy() || alloc.DeploymentStatus.IsUnhealthy() {
			j.logger.Printf("[TRACE] nomad.drain: job watcher found alloc %s - deployment status: %t", alloc.ID[:6], *alloc.DeploymentStatus.Healthy)
			resp[jobKey{alloc.Namespace, alloc.JobID}] = struct{}{}
		} else {
			j.logger.Printf("[TRACE] nomad.drain: job watcher ignoring alloc %s - no deployment status", alloc.ID[:6])
		}
	}

	j.logger.Printf("[TRACE] nomad.drain: job watcher ignoring %d allocs - not part of draining job at index %d", skipped, index)

	return resp, index, nil
}
