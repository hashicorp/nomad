// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: BUSL-1.1

package raftwalmetadata

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/raft-wal/metadb"
	"github.com/hashicorp/raft-wal/types"
	"github.com/stretchr/testify/require"
)

func makeState(nSegs int) *types.PersistentState {
	startIdx := 1000
	perSegment := 100
	startID := 1234

	startTime := time.Now().UTC().Round(time.Second).Add(time.Duration(-1*nSegs) * time.Minute)

	state := &types.PersistentState{
		NextSegmentID: uint64(startID + nSegs),
	}

	for i := 0; i < (nSegs - 1); i++ {
		si := types.SegmentInfo{
			ID:         uint64(startID + i),
			BaseIndex:  uint64(startIdx + (i * perSegment)),
			MinIndex:   uint64(startIdx + (i * perSegment)),
			MaxIndex:   uint64(startIdx + ((i + 1) * perSegment) - 1),
			Codec:      1,
			IndexStart: 123456,
			CreateTime: startTime.Add(time.Duration(i) * time.Minute),
			SealTime:   startTime.Add(time.Duration(i+1) * time.Minute),
			SizeLimit:  64 * 1024 * 1024,
		}
		state.Segments = append(state.Segments, si)
	}
	if nSegs > 0 {
		// Append an unsealed tail
		i := nSegs - 1
		si := types.SegmentInfo{
			ID:         uint64(startID + i),
			BaseIndex:  uint64(startIdx + (i * perSegment)),
			MinIndex:   uint64(startIdx + (i * perSegment)),
			Codec:      1,
			CreateTime: startTime.Add(time.Duration(i) * time.Minute),
			SizeLimit:  64 * 1024 * 1024,
		}
		state.Segments = append(state.Segments, si)
	}
	return state
}

func TestFileMetaDB(t *testing.T) {
	cases := []struct {
		name        string
		writeState  *types.PersistentState
		writeStable map[string][]byte
	}{
		{
			name:       "basic storage",
			writeState: makeState(4),
			writeStable: map[string][]byte{
				"CurrentTerm":  {0, 0, 0, 0, 0, 0, 0, 5},
				"LastVoteTerm": {0, 0, 0, 0, 0, 0, 0, 5},
				"LastVoteCand": []byte("server1"),
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "raft-wal-file-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			{
				// Open a fresh DB, confirm it's empty, write state and stable values.
				var db FileMetaDB
				gotState, err := db.Load(tmpDir)
				require.NoError(t, err)
				defer db.Close()

				require.Equal(t, 0, int(gotState.NextSegmentID))
				require.Empty(t, gotState.Segments)

				if tc.writeState != nil {
					require.NoError(t, db.CommitState(*tc.writeState))
				}
				for k, v := range tc.writeStable {
					require.NoError(t, db.SetStable([]byte(k), v))
				}

				// Close and re-open to verify persistence.
				db.Close()
			}

			var db FileMetaDB
			gotState, err := db.Load(tmpDir)
			require.NoError(t, err)
			defer db.Close()

			require.Equal(t, *tc.writeState, gotState)

			for k, v := range tc.writeStable {
				got, err := db.GetStable([]byte(k))
				require.NoError(t, err)
				require.Equal(t, v, got)
			}
		})
	}
}

func TestFileMetaDBErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-wal-file-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	var db FileMetaDB

	// Calling anything before Load is an error.
	require.ErrorIs(t, db.CommitState(types.PersistentState{NextSegmentID: 1234}), metadb.ErrUnintialized)

	_, err = db.GetStable([]byte("foo"))
	require.ErrorIs(t, err, metadb.ErrUnintialized)

	err = db.SetStable([]byte("foo"), []byte("bar"))
	require.ErrorIs(t, err, metadb.ErrUnintialized)

	// Loading twice from the same dir is OK.
	_, err = db.Load(tmpDir)
	require.NoError(t, err)
	_, err = db.Load(tmpDir)
	require.NoError(t, err)

	// Loading from a different dir is not.
	tmpDir2, err := os.MkdirTemp("", "raft-wal-file-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir2)

	_, err = db.Load(tmpDir2)
	require.ErrorContains(t, err, "already open in dir")

	// Loading from a non-existent dir is an error.
	var db2 FileMetaDB
	_, err = db2.Load("fake-dir-that-does-not-exist")
	require.ErrorContains(t, err, "no such file or directory")
}

// TestFileMetaDBRoundTrip verifies that Close followed by Load correctly
// reloads all state from disk.
func TestFileMetaDBRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-wal-file-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	state := makeState(3)

	var db FileMetaDB
	_, err = db.Load(tmpDir)
	require.NoError(t, err)
	require.NoError(t, db.CommitState(*state))
	require.NoError(t, db.SetStable([]byte("CurrentTerm"), []byte{0, 0, 0, 0, 0, 0, 0, 7}))
	db.Close()

	// Re-open and verify both halves round-tripped correctly.
	var db2 FileMetaDB
	got, err := db2.Load(tmpDir)
	require.NoError(t, err)
	defer db2.Close()
	require.Equal(t, *state, got)

	term, err := db2.GetStable([]byte("CurrentTerm"))
	require.NoError(t, err)
	require.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 7}, term)
}

// TestFileMetaDBSetStableDelete checks that passing nil to SetStable removes
// the key, and that GetStable on a missing key returns nil without error.
func TestFileMetaDBSetStableDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-wal-file-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	var db FileMetaDB
	_, err = db.Load(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	key := []byte("mykey")

	// Key absent → nil, no error.
	got, err := db.GetStable(key)
	require.NoError(t, err)
	require.Nil(t, got)

	// Write then read back.
	require.NoError(t, db.SetStable(key, []byte("myvalue")))
	got, err = db.GetStable(key)
	require.NoError(t, err)
	require.Equal(t, []byte("myvalue"), got)

	// Delete by passing nil.
	require.NoError(t, db.SetStable(key, nil))
	got, err = db.GetStable(key)
	require.NoError(t, err)
	require.Nil(t, got)

	// Deletion must be persisted across a close/reopen.
	db.Close()
	var db2 FileMetaDB
	_, err = db2.Load(tmpDir)
	require.NoError(t, err)
	defer db2.Close()
	got, err = db2.GetStable(key)
	require.NoError(t, err)
	require.Nil(t, got)
}

// TestFileMetaDBBoltDetection verifies that Load returns an informative error
// when a wal-meta.db file exists but wal-meta.json does not, preventing silent
// data loss during a BoltDB → FileMetaDB migration.
func TestFileMetaDBBoltDetection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-wal-file-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Simulate an existing BoltDB deployment.
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, metadb.FileName), []byte("fake bolt data"), 0o644,
	))

	var db FileMetaDB
	_, err = db.Load(tmpDir)
	require.Error(t, err)
	require.ErrorContains(t, err, "BoltDB metadata file")
	require.ErrorContains(t, err, "migrate")

	// Once wal-meta.json also exists the error must not fire (both files can
	// legitimately coexist right after a migration before the old file is
	// cleaned up).
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, FileMetaDBFileName), []byte(`{"state":{},"stable":{}}`), 0o644,
	))
	var db2 FileMetaDB
	_, err = db2.Load(tmpDir)
	require.NoError(t, err)
	db2.Close()
}

// TestFileMetaDBGetStableIsolation verifies that the slice returned by
// GetStable is an independent copy: mutating it must not affect stored state.
func TestFileMetaDBGetStableIsolation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-wal-file-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	var db FileMetaDB
	_, err = db.Load(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	original := []byte{1, 2, 3}
	require.NoError(t, db.SetStable([]byte("k"), original))

	got, err := db.GetStable([]byte("k"))
	require.NoError(t, err)
	require.Equal(t, original, got)

	// Mutating the returned slice must not affect the stored value.
	got[0] = 99
	got2, err := db.GetStable([]byte("k"))
	require.NoError(t, err)
	require.Equal(t, original, got2)
}

// TestFileMetaDBStaleTempCleanup verifies that a leftover .tmp file from a
// previously-crashed write does not prevent Load from succeeding.
func TestFileMetaDBStaleTempCleanup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-wal-file-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Simulate a stale temp file (e.g. crash between write and rename).
	stale := filepath.Join(tmpDir, FileMetaDBFileName+".tmp")
	require.NoError(t, os.WriteFile(stale, []byte("incomplete garbage"), 0o644))

	var db FileMetaDB
	_, err = db.Load(tmpDir)
	require.NoError(t, err) // must not fail
	db.Close()

	// The stale file must have been removed.
	_, err = os.Stat(stale)
	require.ErrorIs(t, err, os.ErrNotExist)
}
