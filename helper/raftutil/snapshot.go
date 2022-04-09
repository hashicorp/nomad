package raftutil

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/snapshot"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/raft"
)

func RestoreFromArchive(archive io.Reader) (*state.StateStore, *raft.SnapshotMeta, error) {
	logger := hclog.L()

	fsm, err := dummyFSM(logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create FSM: %w", err)
	}

	snap, err := ioutil.TempFile("", "snap-")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create a temp file: %w", err)
	}
	defer os.Remove(snap.Name())
	defer snap.Close()

	meta, err := snapshot.CopySnapshot(archive, snap)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read snapshot: %w", err)
	}

	_, err = snap.Seek(0, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to seek: %w", err)
	}

	err = fsm.Restore(snap)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	return fsm.State(), meta, nil
}
