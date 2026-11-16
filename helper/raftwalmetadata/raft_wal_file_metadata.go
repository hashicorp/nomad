// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: BUSL-1.1

package raftwalmetadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/raft-wal/metadb"
	"github.com/hashicorp/raft-wal/types"
)

const (
	// FileMetaDBFileName is the name of the JSON metadata file.
	FileMetaDBFileName = "wal-meta.json"
)

// persistedData is the JSON-serialisable envelope written to FileMetaDBFileName.
// Both halves of the MetaStore interface are flushed together in one atomic
// write so the file is always self-consistent.
//
// Stable-store keys are stored as plain Go strings (string([]byte)). All keys
// used by hashicorp/raft are valid UTF-8, which is a requirement of the JSON
// object-key encoding. The []byte values are base64-encoded automatically by
// encoding/json.
type persistedData struct {
	State  types.PersistentState `json:"state"`
	Stable map[string][]byte     `json:"stable"`
}

// FileMetaDB implements types.MetaStore with a single pretty-printed JSON file
// written via an atomic rename sequence:
//
//  1. Serialise all state to <file>.tmp
//  2. fsync the temp file (data durable)
//  3. rename(tmp → final)  (POSIX-atomic)
//  4. fsync the parent directory (rename durable)
//
// The entire dataset is kept in memory after Load returns, so read operations
// (GetStable) never touch the disk. Write operations (CommitState, SetStable)
// always flush the complete, consistent state in one shot.
//
// Because the file is plain JSON it can be inspected with ordinary text tools
// (cat, jq, …).
type FileMetaDB struct {
	mu     sync.RWMutex
	dir    string
	state  types.PersistentState
	stable map[string][]byte // nil → not yet loaded / already closed
}

// Load implements types.MetaStore.
//
// It is safe to call Load more than once with the same directory; subsequent
// calls are no-ops that return the current in-memory state. Calling Load with
// a different directory after a successful Load returns an error.
//
// If a BoltDB metadata file (wal-meta.db) is found in dir but no JSON file
// exists yet, Load returns an error rather than silently discarding the
// existing segment list (which would cause the WAL to delete all segment
// files on startup). Migrate the metadata first, or open the WAL with
// WithMetaStore(&metadb.BoltMetaDB{}) to continue using BoltDB.
func (db *FileMetaDB) Load(dir string) (types.PersistentState, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Already open from the same dir: return the cached state.
	if db.stable != nil {
		if db.dir != dir {
			return types.PersistentState{}, fmt.Errorf(
				"can't load dir %s, already open in dir %s", dir, db.dir)
		}
		return db.state, nil
	}

	// Confirm the directory exists before touching any files inside it.
	if _, err := os.Stat(dir); err != nil {
		return types.PersistentState{}, err
	}

	mainPath := filepath.Join(dir, FileMetaDBFileName)

	// Detect an existing BoltDB deployment. Silently returning an empty
	// PersistentState here would cause the WAL to delete all segment files,
	// so we fail loudly and direct the operator to migrate.
	if _, err := os.Stat(filepath.Join(dir, metadb.FileName)); err == nil {
		if _, err := os.Stat(mainPath); errors.Is(err, os.ErrNotExist) {
			return types.PersistentState{}, fmt.Errorf(
				"found existing BoltDB metadata file %q but no JSON metadata "+
					"file %q: migrate the metadata store before switching to "+
					"FileMetaDB, or open the WAL with "+
					"WithMetaStore(&metadb.BoltMetaDB{}) to keep using BoltDB",
				metadb.FileName, FileMetaDBFileName)
		}
	}

	db.dir = dir
	db.stable = make(map[string][]byte)

	// Remove any temp file left behind by a previously-crashed write.
	os.Remove(filepath.Join(dir, FileMetaDBFileName+".tmp"))

	data, err := os.ReadFile(mainPath)
	if errors.Is(err, os.ErrNotExist) {
		// Fresh directory — caller receives a zero-value PersistentState and
		// the WAL will initialise itself from scratch.
		return db.state, nil
	}
	if err != nil {
		db.dir, db.stable = "", nil
		return types.PersistentState{}, fmt.Errorf(
			"failed to read %s: %w", FileMetaDBFileName, err)
	}

	var pd persistedData
	if err := json.Unmarshal(data, &pd); err != nil {
		db.dir, db.stable = "", nil
		return types.PersistentState{}, fmt.Errorf(
			"%w: failed to parse %s: %s", types.ErrCorrupt, FileMetaDBFileName, err)
	}

	db.state = pd.State
	if pd.Stable != nil {
		db.stable = pd.Stable
	}
	return db.state, nil
}

// CommitState implements types.MetaStore.
func (db *FileMetaDB) CommitState(state types.PersistentState) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.stable == nil {
		return metadb.ErrUnintialized
	}
	db.state = state
	return db.persist()
}

// GetStable implements types.MetaStore. Safe for concurrent use with all other
// methods.
func (db *FileMetaDB) GetStable(key []byte) ([]byte, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.stable == nil {
		return nil, metadb.ErrUnintialized
	}
	val := db.stable[string(key)]
	if val == nil {
		return nil, nil
	}
	// Return an independent copy: the caller must not be able to mutate the
	// in-memory stable store through the returned slice.
	cp := make([]byte, len(val))
	copy(cp, val)
	return cp, nil
}

// SetStable implements types.MetaStore. Safe for concurrent use with all other
// methods.
func (db *FileMetaDB) SetStable(key []byte, value []byte) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.stable == nil {
		return metadb.ErrUnintialized
	}
	if value == nil {
		delete(db.stable, string(key))
	} else {
		// Store a defensive copy so that later mutations of the caller's slice
		// don't silently corrupt our in-memory state.
		cp := make([]byte, len(value))
		copy(cp, value)
		db.stable[string(key)] = cp
	}
	return db.persist()
}

// Close implements io.Closer.
func (db *FileMetaDB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.dir = ""
	db.stable = nil
	db.state = types.PersistentState{}
	return nil
}

// persist serialises the complete in-memory state and writes it to disk via
// an atomic rename. It must be called with db.mu held for writing.
//
// The four-step sequence provides the following crash guarantees:
//   - Crash before step 1: old file (if any) is still valid.
//   - Crash during step 1 or 2: temp file is incomplete; old file is intact.
//     The stale temp file is removed on the next Load.
//   - Crash during step 3: POSIX rename(2) is atomic — either the old name or
//     the new name is visible; both refer to a fully-written, fsynced file.
//   - Crash after step 3: new file is durable; step 4 may need to be
//     replayed by the OS journal but the data itself is safe.
func (db *FileMetaDB) persist() error {
	data, err := json.Marshal(persistedData{
		State:  db.state,
		Stable: db.stable,
	})
	if err != nil {
		return fmt.Errorf("failed to encode metadata: %w", err)
	}

	tmpPath := filepath.Join(db.dir, FileMetaDBFileName+".tmp")
	mainPath := filepath.Join(db.dir, FileMetaDBFileName)

	// Step 1: write new state to a temp file.
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create temp metadata file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Step 2: fsync the temp file so the bytes are durable before we rename.
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to sync metadata file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp metadata file: %w", err)
	}

	// Step 3: atomically replace the canonical file.
	if err := os.Rename(tmpPath, mainPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to commit metadata file: %w", err)
	}

	// Step 4: fsync the parent directory so the rename directory entry is
	// durable. Without this a crash could leave the directory pointing to the
	// old file even though the new one was fully written.
	dirF, err := os.Open(db.dir)
	if err != nil {
		return fmt.Errorf("failed to open dir for sync: %w", err)
	}
	syncErr := dirF.Sync()
	closeErr := dirF.Close()
	if syncErr != nil {
		return fmt.Errorf("failed to sync directory: %w", syncErr)
	}
	return closeErr
}
