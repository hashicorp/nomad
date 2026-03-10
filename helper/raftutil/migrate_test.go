// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	raftwal "github.com/hashicorp/raft-wal"
	"github.com/shoenig/test/must"
)

func TestMigrateToWAL_Success(t *testing.T) {
	raftDir := t.TempDir()

	logs := makeLogs(1, 10)
	stableKVs := map[string]string{"LastVoteCand": "node-1"}
	stableUint64s := map[string]uint64{"CurrentTerm": 5}

	newTestBoltStore(t, raftDir, logs, stableKVs, stableUint64s)

	progress := make(chan string, 128)

	// MigrateToWal runs on a goroutine so we're only getting progress messages
	// this way but for the purposes of the tests that's fine.
	must.NoError(t, MigrateToWAL(context.Background(), raftDir, progress))

	// Collect progress messages (channel is closed by MigrateToWAL).
	var msgs []string
	for msg := range progress {
		msgs = append(msgs, msg)
	}
	must.SliceNotEmpty(t, msgs)

	// The original BoltDB file should be renamed.
	_, err := os.Stat(filepath.Join(raftDir, "raft.db"))
	must.ErrorIs(t, err, os.ErrNotExist)

	// Find the timestamped backup file.
	entries, err := os.ReadDir(raftDir)
	must.NoError(t, err)
	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "raft.db.migrated.") {
			found = true
			break
		}
	}
	must.True(t, found, must.Sprint("backup file with timestamp not found"))

	// Verify marker file is cleaned up on success.
	_, err = os.Stat(filepath.Join(raftDir, ".migration-in-progress"))
	must.ErrorIs(t, err, os.ErrNotExist)

	// Open the WAL store and verify logs were copied.
	walDir := filepath.Join(raftDir, "wal")
	wal, err := raftwal.Open(walDir)
	must.NoError(t, err)
	t.Cleanup(func() { wal.Close() })

	first, err := wal.FirstIndex()
	must.NoError(t, err)
	must.Eq(t, uint64(1), first)

	last, err := wal.LastIndex()
	must.NoError(t, err)
	must.Eq(t, uint64(10), last)

	// Spot-check a log entry.
	var log raft.Log
	must.NoError(t, wal.GetLog(5, &log))
	must.Eq(t, uint64(5), log.Index)
	must.Eq(t, []byte("test-data"), log.Data)

	// Verify stable store values.
	val, err := wal.Get([]byte("LastVoteCand"))
	must.NoError(t, err)
	must.Eq(t, []byte("node-1"), val)

	term, err := wal.GetUint64([]byte("CurrentTerm"))
	must.NoError(t, err)
	must.Eq(t, uint64(5), term)
}

func TestMigrateToWAL_NoBoltDB(t *testing.T) {
	raftDir := t.TempDir()

	err := MigrateToWAL(context.Background(), raftDir, nil)
	must.ErrorContains(t, err, "BoltDB store not found")
}

func TestMigrateToWAL_WALAlreadyExists(t *testing.T) {
	raftDir := t.TempDir()

	logs := makeLogs(1, 1)
	newTestBoltStore(t, raftDir, logs, nil, nil)

	// Pre-create the WAL directory to simulate a partial migration.
	must.NoError(t, os.MkdirAll(filepath.Join(raftDir, "wal"), 0o700))

	err := MigrateToWAL(context.Background(), raftDir, nil)
	must.ErrorContains(t, err, "WAL directory already exists")
}

func TestMigrateToWAL_EmptyLogs(t *testing.T) {
	raftDir := t.TempDir()

	// An empty BoltDB store (no logs) returns first=0, last=0 which causes
	// CopyLogs to attempt copying a non-existent entry. This is expected to
	// fail — you wouldn't migrate an empty raft store in practice.
	newTestBoltStore(t, raftDir, nil, nil, nil)

	err := MigrateToWAL(context.Background(), raftDir, nil)
	must.ErrorContains(t, err, "failed to copy logs")

	// WAL directory should be cleaned up on failure.
	_, statErr := os.Stat(filepath.Join(raftDir, "wal"))
	must.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestMigrateToWAL_PermissionError(t *testing.T) {
	// This test requires the ability to create a read-only directory.
	// skip when running as root (e.g., on Linux CI with sudo).
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root user to enforce permission checks")
	}

	raftDir := t.TempDir()

	logs := makeLogs(1, 3)
	newTestBoltStore(t, raftDir, logs, nil, nil)

	// Make directory read-only to trigger permission error.
	must.NoError(t, os.Chmod(raftDir, 0o500))
	defer os.Chmod(raftDir, 0o700) // Restore for cleanup

	err := MigrateToWAL(context.Background(), raftDir, nil)
	must.ErrorContains(t, err, "write permissions")
}

func TestVerifyMigration_IndexMismatch(t *testing.T) {
	raftDir := t.TempDir()

	// Create source with 10 logs.
	srcPath := filepath.Join(raftDir, "src.db")
	src, err := raftboltdb.NewBoltStore(srcPath)
	must.NoError(t, err)
	must.NoError(t, src.StoreLogs(makeLogs(1, 10)))
	must.NoError(t, src.SetUint64([]byte("CurrentTerm"), 1))
	must.NoError(t, src.SetUint64([]byte("LastVoteTerm"), 0))
	must.NoError(t, src.Set([]byte("LastVoteCand"), []byte("")))
	src.Close()

	// Create destination with 5 logs.
	walDir := filepath.Join(raftDir, "wal")
	must.NoError(t, os.MkdirAll(walDir, 0o700))
	dst, err := raftwal.Open(walDir)
	must.NoError(t, err)
	must.NoError(t, dst.StoreLogs(makeLogs(1, 5)))
	must.NoError(t, dst.SetUint64([]byte("CurrentTerm"), 1))
	must.NoError(t, dst.SetUint64([]byte("LastVoteTerm"), 0))
	must.NoError(t, dst.Set([]byte("LastVoteCand"), []byte("")))
	must.NoError(t, dst.Close())

	// Reopen both.
	src, err = raftboltdb.NewBoltStore(srcPath)
	must.NoError(t, err)
	t.Cleanup(func() { src.Close() })

	dst, err = raftwal.Open(walDir)
	must.NoError(t, err)
	t.Cleanup(func() { dst.Close() })

	// Verification should fail.
	err = verifyMigration(src, dst)
	must.ErrorContains(t, err, "last index mismatch")
}
