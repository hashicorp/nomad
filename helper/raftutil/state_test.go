// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/ci"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRaftStateInfo_InUse asserts that commands that inspect raft
// state such as "nomad operator raft info" and "nomad operator raft
// logs" fail with a helpful error message when called on an inuse
// database.
func TestRaftStateInfo_InUse(t *testing.T) {
	ci.Parallel(t) // since there's a 1s timeout.

	// First create an empty raft db
	dir := filepath.Join(t.TempDir(), "raft.db")

	fakedb, err := raftboltdb.NewBoltStore(dir)
	require.NoError(t, err)

	// Next try to read the db without closing it
	s, _, _, err := RaftStateInfo(dir)
	assert.Nil(t, s)
	require.EqualError(t, err, errAlreadyOpen.Error())

	// LogEntries should produce the same error
	_, _, err = LogEntries(dir)
	require.EqualError(t, err, "failed to open raft logs: "+errAlreadyOpen.Error())

	// Commands should work once the db is closed
	require.NoError(t, fakedb.Close())

	s, _, _, err = RaftStateInfo(dir)
	assert.NotNil(t, s)
	require.NoError(t, err)
	require.NoError(t, s.Close())

	logCh, errCh, err := LogEntries(dir)
	require.NoError(t, err)

	// Consume entries to cleanly close db
	for closed := false; closed; {
		select {
		case _, closed = <-logCh:
		case <-errCh:
		}
	}
}
