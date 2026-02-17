// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	raftwal "github.com/hashicorp/raft-wal"
	"github.com/hashicorp/raft-wal/migrate"
	"github.com/shirou/gopsutil/v3/disk"
	"go.etcd.io/bbolt"
)

// migrateBatchBytes is the batch size for copying logs during migration. 64 MiB
// provides a good balance between memory usage and throughput.
//
// Larger batches reduce the number of read/write cycles but increase memory
// pressure; smaller batches reduce memory usage but increase overhead.
const migrateBatchBytes = 64 * 1024 * 1024

const (
	migrationMarkerFile = ".migration-in-progress"
	minRequiredSpace    = 512 * 1024 * 1024 // 512 MiB minimum free space
)

// MigrateToWAL migrates an existing BoltDB raft store to the WAL backend, using
// raft-WAL's built-in migration utilities. raftDir should be the raft data
// directory (containing raft.db). The progress channel, if non-nil, receives
// human-readable status updates and will be closed when migration completes
// (successfully or with error).
//
// On success the old BoltDB file is renamed to raft.db.migrated.<timestamp>.
// On failure any partially written WAL directory is removed and the original
// raft.db is left untouched, allowing the operator to safely retry.
//
// A marker file is created during migration to help detect if the server is
// accidentally started mid-migration.
//
// The Nomad server must be stopped before running this.
func MigrateToWAL(ctx context.Context, raftDir string, progress chan<- string) error {
	var wg sync.WaitGroup
	defer func() {
		// Wait for all drainProgress goroutines to complete before closing.
		wg.Wait()
		if progress != nil {
			close(progress)
		}
	}()

	boltPath := filepath.Join(raftDir, "raft.db")
	walDir := filepath.Join(raftDir, "wal")
	markerPath := filepath.Join(raftDir, migrationMarkerFile)

	sendProgress(progress, "starting migration pre-flight checks")

	if err := preflightChecks(boltPath, walDir, raftDir); err != nil {
		return err
	}

	sendProgress(progress, "pre-flight checks passed")

	// Create marker file to detect if server accidentally starts during migration.
	if err := os.WriteFile(markerPath, []byte(time.Now().Format(time.RFC3339)), 0o600); err != nil {
		return fmt.Errorf("failed to create migration marker: %w", err)
	}
	defer os.Remove(markerPath) // Clean up marker on completion or failure.

	// Open the source BoltDB store.
	src, err := raftboltdb.New(raftboltdb.Options{
		Path: boltPath,
		BoltOptions: &bbolt.Options{
			Timeout: 5 * time.Second,
		},
		MsgpackUseNewTimeFormat: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open BoltDB store: %w", err)
	}

	// Create the destination WAL store.
	if err := os.MkdirAll(walDir, 0o700); err != nil {
		src.Close()
		return fmt.Errorf("failed to create WAL directory: %w", err)
	}

	dst, err := raftwal.Open(walDir)
	if err != nil {
		src.Close()
		cleanupWAL(walDir)
		return fmt.Errorf("failed to open WAL store: %w", err)
	}

	// Copy logs.
	logProgress := make(chan string, 64)
	wg.Add(1)
	go drainProgress(logProgress, progress, &wg)
	if err := migrate.CopyLogs(ctx, dst, src, migrateBatchBytes, logProgress); err != nil {
		dst.Close()
		src.Close()
		cleanupWAL(walDir)
		return fmt.Errorf("failed to copy logs: %w", err)
	}

	// Copy stable store keys.
	stableProgress := make(chan string, 64)
	wg.Add(1)
	go drainProgress(stableProgress, progress, &wg)
	if err := migrate.CopyStable(ctx, dst, src, nil, nil, stableProgress); err != nil {
		dst.Close()
		src.Close()
		cleanupWAL(walDir)
		return fmt.Errorf("failed to copy stable store: %w", err)
	}

	// Verify data integrity before finalizing.
	sendProgress(progress, "verifying migrated data")
	if err := verifyMigration(src, dst); err != nil {
		dst.Close()
		src.Close()
		cleanupWAL(walDir)
		return fmt.Errorf("data verification failed: %w", err)
	}

	// Close both stores before renaming files.
	if err := dst.Close(); err != nil {
		src.Close()
		cleanupWAL(walDir)
		return fmt.Errorf("failed to close WAL store: %w", err)
	}

	if err := src.Close(); err != nil {
		cleanupWAL(walDir)
		return fmt.Errorf("failed to close BoltDB store: %w", err)
	}

	// Rename the old BoltDB file to preserve it as a backup with timestamp.
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.migrated.%s", boltPath, timestamp)
	if err := os.Rename(boltPath, backupPath); err != nil {
		return fmt.Errorf("migration succeeded but failed to rename %s to %s: %w",
			boltPath, backupPath, err)
	}

	sendProgress(progress, fmt.Sprintf("migration complete; old BoltDB file preserved at %s", backupPath))
	return nil
}

func sendProgress(progress chan<- string, msg string) {
	if progress != nil {
		select {
		case progress <- msg:
		default:
			// Drop message if consumer is slow to avoid blocking migration.
		}
	}
}

func drainProgress(sub <-chan string, parent chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()
	if parent == nil {
		for range sub {
		}
		return
	}
	for msg := range sub {
		select {
		case parent <- msg:
		default:
			// Drop message if consumer is slow to avoid blocking migration.
		}
	}
}

func preflightChecks(boltPath, walDir, raftDir string) error {
	// Verify the BoltDB file exists.
	boltInfo, err := os.Stat(boltPath)
	if err != nil {
		return fmt.Errorf("BoltDB store not found at %s: %w", boltPath, err)
	}

	// Verify the WAL directory does not already exist.
	if _, err := os.Stat(walDir); err == nil {
		return fmt.Errorf(
			"WAL directory already exists at %s; remove it before retrying migration",
			walDir)
	}

	// Check write permissions on raft directory.
	testFile := filepath.Join(raftDir, ".permission-test")
	if err := os.WriteFile(testFile, []byte("test"), 0o600); err != nil {
		return fmt.Errorf("insufficient write permissions in %s: %w", raftDir, err)
	}
	os.Remove(testFile)

	// Check available disk space.
	usage, err := disk.Usage(raftDir)
	if err != nil {
		// Log warning but don't fail migration - disk space check is advisory.
		return fmt.Errorf("unable to check available disk space: %w", err)
	}
	availableSpace := usage.Free

	boltSize := uint64(boltInfo.Size())

	// Require at least the BoltDB size + minimum free space buffer.
	// WAL format is typically similar in size to BoltDB, but we add a buffer.
	requiredSpace := boltSize + minRequiredSpace
	if availableSpace < requiredSpace {
		return fmt.Errorf(
			"insufficient disk space: have %d bytes available, need at least %d bytes (BoltDB size %d + %d buffer)",
			availableSpace, requiredSpace, boltSize, minRequiredSpace)
	}

	return nil
}

// cleanupWAL removes the WAL directory, retrying on Windows to handle
// delayed file handle releases.
func cleanupWAL(walDir string) {
	// First attempt immediate cleanup.
	if err := os.RemoveAll(walDir); err == nil {
		return
	}

	// On failure (common on Windows), retry with delays.
	for range 3 {
		time.Sleep(100 * time.Millisecond)
		if err := os.RemoveAll(walDir); err == nil {
			return
		}
	}
}

// verifyMigration performs sanity checks on the migrated data.
// src and dst should implement both LogStore and StableStore interfaces.
func verifyMigration(src interface {
	raft.LogStore
	raft.StableStore
}, dst interface {
	raft.LogStore
	raft.StableStore
}) error {
	// Verify log index ranges match.
	srcFirst, err := src.FirstIndex()
	if err != nil {
		return fmt.Errorf("failed to get source first index: %w", err)
	}

	srcLast, err := src.LastIndex()
	if err != nil {
		return fmt.Errorf("failed to get source last index: %w", err)
	}

	dstFirst, err := dst.FirstIndex()
	if err != nil {
		return fmt.Errorf("failed to get destination first index: %w", err)
	}

	dstLast, err := dst.LastIndex()
	if err != nil {
		return fmt.Errorf("failed to get destination last index: %w", err)
	}

	if srcFirst != dstFirst {
		return fmt.Errorf("first index mismatch: source=%d, destination=%d", srcFirst, dstFirst)
	}

	if srcLast != dstLast {
		return fmt.Errorf("last index mismatch: source=%d, destination=%d", srcLast, dstLast)
	}

	// Verify stable store uint64 keys.
	uint64Keys := []string{"CurrentTerm", "LastVoteTerm"}
	for _, key := range uint64Keys {
		srcVal, err := src.GetUint64([]byte(key))
		if err != nil {
			return fmt.Errorf("failed to get source uint64 key %s: %w", key, err)
		}

		dstVal, err := dst.GetUint64([]byte(key))
		if err != nil {
			return fmt.Errorf("failed to get destination uint64 key %s: %w", key, err)
		}

		if srcVal != dstVal {
			return fmt.Errorf("stable key %s mismatch: source=%d, destination=%d", key, srcVal, dstVal)
		}
	}

	// Verify stable store byte keys.
	byteKeys := []string{"LastVoteCand"}
	for _, key := range byteKeys {
		srcVal, err := src.Get([]byte(key))
		if err != nil {
			srcVal = nil
		}

		dstVal, err := dst.Get([]byte(key))
		if err != nil {
			dstVal = nil
		}

		if string(srcVal) != string(dstVal) {
			return fmt.Errorf("stable key %s mismatch: source=%q, destination=%q", key, srcVal, dstVal)
		}
	}

	// If we have logs, spot-check a few entries for data integrity.
	if srcFirst > 0 && srcLast > 0 {
		indicesToCheck := []uint64{srcFirst}
		if srcLast > srcFirst {
			// Check middle entry.
			middle := srcFirst + (srcLast-srcFirst)/2
			indicesToCheck = append(indicesToCheck, middle)
			// Check last entry.
			indicesToCheck = append(indicesToCheck, srcLast)
		}

		for _, idx := range indicesToCheck {
			var srcLog, dstLog raft.Log
			if err := src.GetLog(idx, &srcLog); err != nil {
				return fmt.Errorf("failed to get source log %d: %w", idx, err)
			}
			if err := dst.GetLog(idx, &dstLog); err != nil {
				return fmt.Errorf("failed to get destination log %d: %w", idx, err)
			}

			if srcLog.Index != dstLog.Index || srcLog.Term != dstLog.Term ||
				srcLog.Type != dstLog.Type || string(srcLog.Data) != string(dstLog.Data) {
				return fmt.Errorf("log entry %d mismatch", idx)
			}
		}
	}

	return nil
}
