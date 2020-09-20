package raftutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/raft"
)

// FSMState returns a dump of the FSM state as found in data-dir, as of lastIndx value
func FSMState(p string, plastIdx int64) (interface{}, error) {
	store, firstIdx, lastIdx, err := RaftStateInfo(filepath.Join(p, "raft.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to open raft logs: %v", err)
	}
	defer store.Close()

	snaps, err := raft.NewFileSnapshotStore(p, 1000, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot dir: %v", err)
	}

	logger := hclog.L()

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

	fsm, err := nomad.NewFSM(fsmConfig)
	if err != nil {
		return nil, err
	}

	// restore from snapshot first
	sFirstIdx, err := restoreFromSnapshot(fsm, snaps, logger)
	if err != nil {
		return nil, err
	}

	if sFirstIdx+1 < firstIdx {
		return nil, fmt.Errorf("missing logs after snapshot [%v,%v]", sFirstIdx+1, firstIdx-1)
	} else if sFirstIdx > 0 {
		firstIdx = sFirstIdx + 1
	}

	lastIdx = lastIndex(lastIdx, plastIdx)

	for i := firstIdx; i <= lastIdx; i++ {
		var e raft.Log
		err := store.GetLog(i, &e)
		if err != nil {
			return nil, fmt.Errorf("failed to read log entry at index %d: %v", i, err)
		}

		if e.Type == raft.LogCommand {
			fsm.Apply(&e)
		}
	}

	state := fsm.State()
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

	return result, nil
}

func restoreFromSnapshot(fsm raft.FSM, snaps raft.SnapshotStore, logger hclog.Logger) (uint64, error) {
	snapshots, err := snaps.List()
	if err != nil {
		return 0, err
	}
	logger.Debug("found snapshots", "count", len(snapshots))

	for _, snapshot := range snapshots {
		_, source, err := snaps.Open(snapshot.ID)
		if err != nil {
			logger.Warn("failed to open a snapshot", "snapshot_id", snapshot.ID, "error", err)
			continue
		}

		err = fsm.Restore(source)
		source.Close()
		if err != nil {
			logger.Warn("failed to restore a snapshot", "snapshot_id", snapshot.ID, "error", err)
			continue
		}

		return snapshot.Index, nil
	}

	return 0, nil
}

func lastIndex(raftLastIdx uint64, cliLastIdx int64) uint64 {
	switch {
	case cliLastIdx < 0:
		if raftLastIdx > uint64(-cliLastIdx) {
			return raftLastIdx - uint64(-cliLastIdx)
		} else {
			return 0
		}
	case cliLastIdx == 0:
		return raftLastIdx
	case uint64(cliLastIdx) < raftLastIdx:
		return uint64(cliLastIdx)
	default:
		return raftLastIdx
	}
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
