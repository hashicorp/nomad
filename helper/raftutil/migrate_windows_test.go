// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package raftutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	raftwal "github.com/hashicorp/raft-wal"
	"github.com/shoenig/test/must"
)

// TestMigrateToWAL_Success_Windows is a Windows-specific version that handles
// file locking issues more carefully.
func TestMigrateToWAL_Success_Windows(t *testing.T) {
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

	// Wait for file handles to be released on Windows
	time.Sleep(200 * time.Millisecond)

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
	// Give extra time for Windows to release locks
	var wal *raftwal.WAL
	walDir := filepath.Join(raftDir, "wal")
	for range 5 {
		wal, err = raftwal.Open(walDir)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	must.NoError(t, err)
	t.Cleanup(func() {
		if wal != nil {
			wal.Close()
		}
	})

	first, err := wal.FirstIndex()
	must.NoError(t, err)
	must.Eq(t, uint64(1), first)

	last, err := wal.LastIndex()
	must.NoError(t, err)
	must.Eq(t, uint64(10), last)
}

// TestMigrateToWAL_NilProgress_Windows tests migration with nil progress channel on Windows.
func TestMigrateToWAL_NilProgress_Windows(t *testing.T) {
	raftDir := t.TempDir()

	logs := makeLogs(1, 3)
	newTestBoltStore(t, raftDir, logs, nil, nil)

	err := MigrateToWAL(context.Background(), raftDir, nil)
	must.NoError(t, err)

	// Wait for file handles to be released on Windows
	time.Sleep(200 * time.Millisecond)

	// Verify WAL was created and BoltDB was renamed with timestamp.
	_, err = os.Stat(filepath.Join(raftDir, "raft.db"))
	must.ErrorIs(t, err, os.ErrNotExist)

	// Try opening WAL with retry
	var wal *raftwal.WAL
	walDir := filepath.Join(raftDir, "wal")
	for i := 0; i < 5; i++ {
		wal, err = raftwal.Open(walDir)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	must.NoError(t, err)
	t.Cleanup(func() {
		if wal != nil {
			wal.Close()
		}
	})

	last, err := wal.LastIndex()
	must.NoError(t, err)
	must.Eq(t, uint64(3), last)
}

// TestCleanupWAL_Windows tests WAL cleanup with Windows-specific delays
func TestCleanupWAL_Windows(t *testing.T) {
	walDir := filepath.Join(t.TempDir(), "wal")
	must.NoError(t, os.MkdirAll(walDir, 0o700))
	must.NoError(t, os.WriteFile(filepath.Join(walDir, "test.txt"), []byte("data"), 0o600))

	// Close any open handles
	time.Sleep(100 * time.Millisecond)

	cleanupWAL(walDir)

	// Give Windows extra time to complete removal
	time.Sleep(200 * time.Millisecond)

	// Verify directory is removed.
	_, err := os.Stat(walDir)
	must.ErrorIs(t, err, os.ErrNotExist)
}
