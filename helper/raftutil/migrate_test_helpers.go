// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/shoenig/test/must"
)

// newTestBoltStore creates a BoltDB raft store at raftDir/raft.db, populates it
// with the given log entries and stable-store key/values, and returns the
// closed store path. The store is closed before returning so MigrateToWAL can
// open it exclusively.
func newTestBoltStore(t *testing.T, raftDir string, logs []*raft.Log,
	stableKVs map[string]string, stableUint64s map[string]uint64) {

	t.Helper()

	boltPath := filepath.Join(raftDir, "raft.db")
	store, err := raftboltdb.NewBoltStore(boltPath)
	must.NoError(t, err)

	if len(logs) > 0 {
		must.NoError(t, store.StoreLogs(logs))
	}

	// CopyStable always reads CurrentTerm, LastVoteTerm, and LastVoteCand.
	// Seed defaults so migration doesn't fail on missing keys.
	if _, ok := stableUint64s["CurrentTerm"]; !ok {
		must.NoError(t, store.SetUint64([]byte("CurrentTerm"), 1))
	}
	if _, ok := stableUint64s["LastVoteTerm"]; !ok {
		must.NoError(t, store.SetUint64([]byte("LastVoteTerm"), 0))
	}
	if _, ok := stableKVs["LastVoteCand"]; !ok {
		must.NoError(t, store.Set([]byte("LastVoteCand"), []byte("")))
	}

	for k, v := range stableKVs {
		must.NoError(t, store.Set([]byte(k), []byte(v)))
	}
	for k, v := range stableUint64s {
		must.NoError(t, store.SetUint64([]byte(k), v))
	}
	must.NoError(t, store.Close())
}

func makeLogs(start, count uint64) []*raft.Log {
	logs := make([]*raft.Log, count)
	for i := range count {
		logs[i] = &raft.Log{
			Index: start + i,
			Term:  1,
			Type:  raft.LogCommand,
			Data:  []byte("test-data"),
		}
	}
	return logs
}
