package raftutil

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

var ErrNoMoreLogs = fmt.Errorf("no more logs")

type nomadFSM interface {
	raft.FSM
	State() *state.StateStore
	Restore(io.ReadCloser) error
}

type FSMHelper struct {
	path string

	logger hclog.Logger

	// nomad state
	store *raftboltdb.BoltStore
	fsm   nomadFSM
	snaps *raft.FileSnapshotStore

	// raft
	logFirstIdx uint64
	logLastIdx  uint64
	nextIdx     uint64
}

func NewFSM(p string) (*FSMHelper, error) {
	store, firstIdx, lastIdx, err := RaftStateInfo(filepath.Join(p, "raft.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to open raft database %v: %v", p, err)
	}

	logger := hclog.L()

	snaps, err := raft.NewFileSnapshotStoreWithLogger(p, 1000, logger)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("failed to open snapshot dir: %v", err)
	}

	fsm, err := dummyFSM(logger)
	if err != nil {
		store.Close()
		return nil, err
	}

	return &FSMHelper{
		path:   p,
		logger: logger,
		store:  store,
		fsm:    fsm,
		snaps:  snaps,

		logFirstIdx: firstIdx,
		logLastIdx:  lastIdx,
		nextIdx:     uint64(1),
	}, nil
}

func dummyFSM(logger hclog.Logger) (nomadFSM, error) {
	// use dummy non-enabled FSM dependencies
	periodicDispatch := nomad.NewPeriodicDispatch(logger, nil)
	blockedEvals := nomad.NewBlockedEvals(nil, logger)
	evalBroker, err := nomad.NewEvalBroker(1, 1, 1, 1)
	if err != nil {
		return nil, err
	}

	fsmConfig := &nomad.FSMConfig{
		EvalBroker: evalBroker,
		Periodic:   periodicDispatch,
		Blocked:    blockedEvals,
		Logger:     logger,
		Region:     "default",
	}

	return nomad.NewFSM(fsmConfig)
}

func (f *FSMHelper) Close() {
	f.store.Close()

}

func (f *FSMHelper) ApplyNext() (index uint64, term uint64, err error) {
	if f.nextIdx == 1 {
		// check snapshots first
		index, term, err := f.restoreFromSnapshot()
		if err != nil {
			return 0, 0, err
		}

		if index != 0 {
			f.nextIdx = index + 1
			return index, term, nil
		}
	}

	if f.nextIdx < f.logFirstIdx {
		return 0, 0, fmt.Errorf("missing logs [%v, %v]", f.nextIdx, f.logFirstIdx-1)
	}

	if f.nextIdx > f.logLastIdx {
		return 0, 0, ErrNoMoreLogs
	}

	var e raft.Log
	err = f.store.GetLog(f.nextIdx, &e)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read log entry at index %d: %v", f.nextIdx, err)
	}

	defer func() {
		r := recover()
		if r != nil && strings.HasPrefix(fmt.Sprint(r), "failed to apply request") {
			// Enterprise specific log entries will fail to load in OSS repository with "failed to apply request."
			// If not relevant to investigation, we can ignore them and simply worn.
			f.logger.Warn("failed to apply log; loading Enterprise data-dir in OSS binary?", "index", e.Index)

			f.nextIdx++
		} else if r != nil {
			panic(r)
		}
	}()

	if e.Type == raft.LogCommand {
		f.fsm.Apply(&e)
	}

	f.nextIdx++
	return e.Index, e.Term, nil
}

// ApplyUntil applies all raft entries until (inclusive) the passed index.
func (f *FSMHelper) ApplyUntil(stopIdx uint64) (idx uint64, term uint64, err error) {
	var lastIdx, lastTerm uint64
	for {
		idx, term, err := f.ApplyNext()
		if err == ErrNoMoreLogs {
			return lastIdx, lastTerm, nil
		} else if err != nil {
			return lastIdx, lastTerm, err
		} else if idx >= stopIdx {
			return lastIdx, lastTerm, nil
		}

		lastIdx, lastTerm = idx, term
	}
}

func (f *FSMHelper) ApplyAll() (index uint64, term uint64, err error) {
	var lastIdx, lastTerm uint64
	for {
		idx, term, err := f.ApplyNext()
		if err == ErrNoMoreLogs {
			return lastIdx, lastTerm, nil
		} else if err != nil {
			return lastIdx, lastTerm, err
		}

		lastIdx, lastTerm = idx, term
	}
}

func (f *FSMHelper) State() *state.StateStore {
	return f.fsm.State()
}

func (f *FSMHelper) StateAsMap() map[string][]interface{} {
	return StateAsMap(f.fsm.State())
}

// StateAsMap returns a json-able representation of the state
func StateAsMap(state *state.StateStore) map[string][]interface{} {
	result := map[string][]interface{}{
		"ACLPolicies":      toArray(state.ACLPolicies(nil)),
		"ACLTokens":        toArray(state.ACLTokens(nil)),
		"Allocs":           toArray(state.Allocs(nil)),
		"CSIPlugins":       toArray(state.CSIPlugins(nil)),
		"CSIVolumes":       toArray(state.CSIVolumes(nil)),
		"Deployments":      toArray(state.Deployments(nil)),
		"Evals":            toArray(state.Evals(nil)),
		"Indexes":          toArray(state.Indexes()),
		"JobSummaries":     toArray(state.JobSummaries(nil)),
		"JobVersions":      toArray(state.JobVersions(nil)),
		"Jobs":             toArray(state.Jobs(nil)),
		"Nodes":            toArray(state.Nodes(nil)),
		"PeriodicLaunches": toArray(state.PeriodicLaunches(nil)),
		"SITokenAccessors": toArray(state.SITokenAccessors(nil)),
		"ScalingEvents":    toArray(state.ScalingEvents(nil)),
		"ScalingPolicies":  toArray(state.ScalingPolicies(nil)),
		"VaultAccessors":   toArray(state.VaultAccessors(nil)),
	}

	insertEnterpriseState(result, state)

	return result

}

func (f *FSMHelper) restoreFromSnapshot() (index uint64, term uint64, err error) {
	snapshots, err := f.snaps.List()
	if err != nil {
		return 0, 0, err
	}
	f.logger.Debug("found snapshots", "count", len(snapshots))

	for _, snapshot := range snapshots {
		_, source, err := f.snaps.Open(snapshot.ID)
		if err != nil {
			f.logger.Warn("failed to open a snapshot", "snapshot_id", snapshot.ID, "error", err)
			continue
		}

		err = f.fsm.Restore(source)
		source.Close()
		if err != nil {
			f.logger.Warn("failed to restore a snapshot", "snapshot_id", snapshot.ID, "error", err)
			continue
		}

		return snapshot.Index, snapshot.Term, nil
	}

	return 0, 0, nil
}

func toArray(iter memdb.ResultIterator, err error) []interface{} {
	if err != nil {
		return []interface{}{err}
	}

	r := []interface{}{}

	if iter != nil {
		item := iter.Next()
		for item != nil {
			r = append(r, item)
			item = iter.Next()
		}
	}

	return r
}
