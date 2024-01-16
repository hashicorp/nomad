package state

import (
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-set/v2"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *StateStore) JobsByIDs(ws memdb.WatchSet, nsIDs []structs.NamespacedID) (memdb.ResultIterator, error) {
	txn := s.db.ReadTxn()
	// this thing reads through all jobs, which seems pretty inefficient...
	iterAll, err := txn.Get("jobs", "id")
	if err != nil {
		return nil, err
	}
	idSet := set.From[structs.NamespacedID](nsIDs)
	iter := &JobsIterator{
		SuperIter: iterAll,
		Filter: func(j *structs.Job) bool {
			return idSet.Contains(j.NamespacedID())
		},
	}
	ws.Add(iter.WatchCh())
	return iter, nil
}

var _ memdb.ResultIterator = &JobsIterator{}

type JobsIterator struct {
	SuperIter memdb.ResultIterator
	Filter    func(*structs.Job) bool
}

func (j *JobsIterator) WatchCh() <-chan struct{} {
	return j.SuperIter.WatchCh()
}

// Next will always return a *structs.Job, or nil when there are none left.
func (j *JobsIterator) Next() interface{} {
	for {
		next := j.SuperIter.Next()
		// The Santa Clause 3: The Escape Clause
		if next == nil {
			return nil
		}
		job := next.(*structs.Job)
		if j.Filter(job) {
			return job
		}
	}
}

func (s *StateStore) JobsByIDs2(ws memdb.WatchSet, nsIDs []structs.NamespacedID) (memdb.ResultIterator, error) {
	return &JobsIterator2{
		Jobs:  nsIDs,
		state: s,
		ws:    ws,
	}, nil
}

var _ memdb.ResultIterator = &JobsIterator2{}

type JobsIterator2 struct {
	// these states are not protected from concurrent access...
	Jobs []structs.NamespacedID
	idx  int

	// this is feelin pretty wild...
	state   *StateStore
	ws      memdb.WatchSet
	watchCh <-chan struct{}
}

func (j *JobsIterator2) WatchCh() <-chan struct{} {
	return j.watchCh
}

func (j *JobsIterator2) Next() interface{} {
	if len(j.Jobs) < j.idx+1 {
		return nil
	}
	nsID := j.Jobs[j.idx]
	j.idx++
	job, err := j.state.JobByIDTxn(j.ws, nsID.Namespace, nsID.ID, j.state.db.ReadTxn()) // state and ws here feel wrong...
	if err != nil {
		return nil // hmm, losing the error...
	}
	return job
}
