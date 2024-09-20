// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package benchmarks

// Test helper functions for running scheduling tests and benchmarks
// against real world state snapshots or data directories. These live
// here and not in the parent scheduler package because it would
// create circular imports between the scheduler and raftutils package
// (via the nomad package)

import (
	"errors"
	"os"
	"testing"

	"github.com/hashicorp/nomad/helper/raftutil"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/stretchr/testify/require"
)

// NewBenchmarkingHarness creates a starting test harness with state
// store. The starting contents of the state store depends on which
// env var is set:
// - NOMAD_BENCHMARK_DATADIR: path to data directory
// - NOMAD_BENCHMARK_SNAPSHOT: path to raft snapshot
// - neither: empty starting state
func NewBenchmarkingHarness(t testing.TB) *scheduler.Harness {
	// create the Harness and starting state.
	datadir := os.Getenv("NOMAD_BENCHMARK_DATADIR")
	if datadir != "" {
		h, err := NewHarnessFromDataDir(t, datadir)
		require.NoError(t, err)
		return h
	} else {
		snapshotPath := os.Getenv("NOMAD_BENCHMARK_SNAPSHOT")
		if snapshotPath != "" {
			h, err := NewHarnessFromSnapshot(t, snapshotPath)
			require.NoError(t, err)
			return h
		}
	}
	return scheduler.NewHarness(t)
}

// NewHarnessFromDataDir creates a new scheduler test harness with
// state loaded from an existing datadir.
func NewHarnessFromDataDir(t testing.TB, datadirPath string) (*scheduler.Harness, error) {
	if datadirPath == "" {
		return nil, errors.New("datadir path was not set")
	}
	fsm, err := raftutil.NewFSM(datadirPath)
	if err != nil {
		return nil, err
	}
	_, _, err = fsm.ApplyAll()
	if err != nil {
		return nil, err
	}

	return scheduler.NewHarnessWithState(t, fsm.State()), nil
}

// NewHarnessFromDataDir creates a new harness with state loaded
// from an existing raft snapshot.
func NewHarnessFromSnapshot(t testing.TB, snapshotPath string) (*scheduler.Harness, error) {
	if snapshotPath == "" {
		return nil, errors.New("snapshot path was not set")
	}
	f, err := os.Open(snapshotPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	_, state, _, err := raftutil.RestoreFromArchive(f, nil)
	if err != nil {
		return nil, err
	}

	return scheduler.NewHarnessWithState(t, state), nil
}
