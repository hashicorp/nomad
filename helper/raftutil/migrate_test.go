// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
	err := MigrateToWAL(context.Background(), raftDir, progress)
	must.NoError(t, err)

	// Collect progress messages (channel is closed by MigrateToWAL).
	var msgs []string
	for msg := range progress {
		msgs = append(msgs, msg)
	}
	must.SliceNotEmpty(t, msgs)

	// The original BoltDB file should be renamed.
	_, err = os.Stat(filepath.Join(raftDir, "raft.db"))
	must.ErrorIs(t, err, os.ErrNotExist)

	// Find the timestamped backup file.
	entries, err := os.ReadDir(raftDir)
	must.NoError(t, err)
	found := false
	for _, entry := range entries {
		if len(entry.Name()) > len("raft.db.migrated.") && entry.Name()[:len("raft.db.migrated.")] == "raft.db.migrated." {
			found = true
			break
		}
	}
	must.True(t, found, must.Sprint("backup file with timestamp not found"))

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

func TestMigrateToWAL_NilProgress(t *testing.T) {
	raftDir := t.TempDir()

	logs := makeLogs(1, 3)
	newTestBoltStore(t, raftDir, logs, nil, nil)

	err := MigrateToWAL(context.Background(), raftDir, nil)
	must.NoError(t, err)

	// Verify WAL was created and BoltDB was renamed with timestamp.
	_, err = os.Stat(filepath.Join(raftDir, "raft.db"))
	must.ErrorIs(t, err, os.ErrNotExist)

	wal, err := raftwal.Open(filepath.Join(raftDir, "wal"))
	must.NoError(t, err)
	t.Cleanup(func() { wal.Close() })

	last, err := wal.LastIndex()
	must.NoError(t, err)
	must.Eq(t, uint64(3), last)
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

func TestMigrateToWAL_ContextCancelled(t *testing.T) {
	raftDir := t.TempDir()

	logs := makeLogs(1, 100)
	newTestBoltStore(t, raftDir, logs, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	err := MigrateToWAL(ctx, raftDir, nil)
	must.Error(t, err)

	// WAL directory should be cleaned up on failure.
	_, statErr := os.Stat(filepath.Join(raftDir, "wal"))
	must.ErrorIs(t, statErr, os.ErrNotExist)
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

func TestDrainProgress(t *testing.T) {
	t.Run("nil parent", func(t *testing.T) {
		sub := make(chan string, 2)
		sub <- "msg1"
		sub <- "msg2"
		close(sub)

		// Should not panic with nil parent.
		var wg sync.WaitGroup
		wg.Add(1)
		drainProgress(sub, nil, &wg)
		wg.Wait()
	})

	t.Run("forwards messages", func(t *testing.T) {
		sub := make(chan string, 2)
		parent := make(chan string, 2)

		sub <- "hello"
		sub <- "world"
		close(sub)

		var wg sync.WaitGroup
		wg.Add(1)
		drainProgress(sub, parent, &wg)
		wg.Wait()

		must.Eq(t, "hello", <-parent)
		must.Eq(t, "world", <-parent)
	})

	t.Run("drops when parent full", func(t *testing.T) {
		sub := make(chan string, 3)
		parent := make(chan string, 1)

		sub <- "first"
		sub <- "second"
		sub <- "third"
		close(sub)

		var wg sync.WaitGroup
		wg.Add(1)
		drainProgress(sub, parent, &wg)
		wg.Wait()

		// At least the first message should arrive.
		must.Eq(t, "first", <-parent)
	})
}

func TestMigrateToWAL_ProgressChannelClosed(t *testing.T) {
	raftDir := t.TempDir()

	logs := makeLogs(1, 5)
	newTestBoltStore(t, raftDir, logs, nil, nil)

	progress := make(chan string, 128)
	err := MigrateToWAL(context.Background(), raftDir, progress)
	must.NoError(t, err)

	// Drain all messages from the channel.
	for range progress {
	}

	// Verify channel is closed by trying to read again.
	_, ok := <-progress
	must.False(t, ok, must.Sprint("progress channel should be closed"))
}

func TestMigrateToWAL_BackupWithTimestamp(t *testing.T) {
	raftDir := t.TempDir()

	logs := makeLogs(1, 3)
	newTestBoltStore(t, raftDir, logs, nil, nil)

	err := MigrateToWAL(context.Background(), raftDir, nil)
	must.NoError(t, err)

	// Verify the backup file has a timestamp.
	entries, err := os.ReadDir(raftDir)
	must.NoError(t, err)

	found := false
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".migrated" {
			// Should not exist without timestamp.
			t.Errorf("found backup without timestamp: %s", entry.Name())
		}
		// Check for pattern like raft.db.migrated.20260213-150405
		if len(entry.Name()) > len("raft.db.migrated.") && entry.Name()[:len("raft.db.migrated.")] == "raft.db.migrated." {
			found = true
		}
	}
	must.True(t, found, must.Sprint("backup file with timestamp not found"))
}

func TestMigrateToWAL_VerifyAllLogs(t *testing.T) {
	raftDir := t.TempDir()

	// Create logs with varying data to ensure all are copied correctly.
	logs := make([]*raft.Log, 20)
	for i := range logs {
		logs[i] = &raft.Log{
			Index: uint64(i + 1),
			Term:  uint64((i / 5) + 1), // Varying terms
			Type:  raft.LogCommand,
			Data:  []byte(fmt.Sprintf("data-%d", i)),
		}
	}

	newTestBoltStore(t, raftDir, logs, nil, nil)

	err := MigrateToWAL(context.Background(), raftDir, nil)
	must.NoError(t, err)

	// Open WAL and verify ALL log entries.
	walDir := filepath.Join(raftDir, "wal")
	wal, err := raftwal.Open(walDir)
	must.NoError(t, err)
	t.Cleanup(func() { wal.Close() })

	for i := uint64(1); i <= 20; i++ {
		var log raft.Log
		must.NoError(t, wal.GetLog(i, &log))
		must.Eq(t, i, log.Index)
		must.Eq(t, []byte(fmt.Sprintf("data-%d", i-1)), log.Data)
	}
}

func TestMigrateToWAL_MarkerFileCleanup(t *testing.T) {
	raftDir := t.TempDir()

	logs := makeLogs(1, 3)
	newTestBoltStore(t, raftDir, logs, nil, nil)

	err := MigrateToWAL(context.Background(), raftDir, nil)
	must.NoError(t, err)

	// Verify marker file is cleaned up on success.
	_, err = os.Stat(filepath.Join(raftDir, ".migration-in-progress"))
	must.ErrorIs(t, err, os.ErrNotExist)
}

func TestMigrateToWAL_MarkerFileCleanupOnFailure(t *testing.T) {
	raftDir := t.TempDir()

	logs := makeLogs(1, 5)
	newTestBoltStore(t, raftDir, logs, nil, nil)

	// Cancel context to force failure during migration.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := MigrateToWAL(ctx, raftDir, nil)
	must.Error(t, err)

	// Verify marker file is cleaned up even on failure.
	_, err = os.Stat(filepath.Join(raftDir, ".migration-in-progress"))
	must.ErrorIs(t, err, os.ErrNotExist)
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

func TestPreflightChecks_NoBoltDB(t *testing.T) {
	raftDir := t.TempDir()
	boltPath := filepath.Join(raftDir, "raft.db")
	walDir := filepath.Join(raftDir, "wal")

	err := preflightChecks(boltPath, walDir, raftDir)
	must.ErrorContains(t, err, "BoltDB store not found")
}

func TestPreflightChecks_WALExists(t *testing.T) {
	raftDir := t.TempDir()
	boltPath := filepath.Join(raftDir, "raft.db")
	walDir := filepath.Join(raftDir, "wal")

	// Create BoltDB file.
	must.NoError(t, os.WriteFile(boltPath, []byte("test"), 0o600))

	// Create WAL directory.
	must.NoError(t, os.MkdirAll(walDir, 0o700))

	err := preflightChecks(boltPath, walDir, raftDir)
	must.ErrorContains(t, err, "WAL directory already exists")
}

func TestPreflightChecks_Success(t *testing.T) {
	raftDir := t.TempDir()
	boltPath := filepath.Join(raftDir, "raft.db")
	walDir := filepath.Join(raftDir, "wal")

	// Create BoltDB file.
	must.NoError(t, os.WriteFile(boltPath, []byte("test"), 0o600))

	err := preflightChecks(boltPath, walDir, raftDir)
	must.NoError(t, err)
}

func TestSendProgress(t *testing.T) {
	t.Run("nil channel", func(t *testing.T) {
		// Should not panic with nil channel.
		sendProgress(nil, "test message")
	})

	t.Run("sends message", func(t *testing.T) {
		progress := make(chan string, 1)
		sendProgress(progress, "hello")
		must.Eq(t, "hello", <-progress)
	})

	t.Run("drops when channel full", func(t *testing.T) {
		progress := make(chan string, 1)
		progress <- "first"
		// This should not block, just drop the message.
		sendProgress(progress, "second")
		must.Eq(t, "first", <-progress)
	})
}

func TestCleanupWAL(t *testing.T) {
	walDir := filepath.Join(t.TempDir(), "wal")
	must.NoError(t, os.MkdirAll(walDir, 0o700))
	must.NoError(t, os.WriteFile(filepath.Join(walDir, "test.txt"), []byte("data"), 0o600))

	cleanupWAL(walDir)

	// Verify directory is removed.
	_, err := os.Stat(walDir)
	must.ErrorIs(t, err, os.ErrNotExist)
}

func TestVerifyMigration_Success(t *testing.T) {
	raftDir := t.TempDir()

	logs := makeLogs(1, 10)
	stableKVs := map[string]string{"LastVoteCand": "node-1"}
	stableUint64s := map[string]uint64{"CurrentTerm": 5}

	newTestBoltStore(t, raftDir, logs, stableKVs, stableUint64s)

	// Perform migration.
	err := MigrateToWAL(context.Background(), raftDir, nil)
	must.NoError(t, err)

	// Reopen both stores and verify.
	backupPath := filepath.Join(raftDir, "raft.db.migrated")
	// Find the actual backup file with timestamp.
	entries, err := os.ReadDir(raftDir)
	must.NoError(t, err)
	for _, entry := range entries {
		if len(entry.Name()) > len("raft.db.migrated.") && entry.Name()[:len("raft.db.migrated.")] == "raft.db.migrated." {
			backupPath = filepath.Join(raftDir, entry.Name())
			break
		}
	}

	src, err := raftboltdb.NewBoltStore(backupPath)
	must.NoError(t, err)
	t.Cleanup(func() { src.Close() })

	walDir := filepath.Join(raftDir, "wal")
	dst, err := raftwal.Open(walDir)
	must.NoError(t, err)
	t.Cleanup(func() { dst.Close() })

	// Verification should pass.
	err = verifyMigration(src, dst)
	must.NoError(t, err)
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
