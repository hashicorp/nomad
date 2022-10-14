package raftutil

import (
	"fmt"
	"io"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"

	"github.com/hashicorp/nomad/helper/snapshot"
	"github.com/hashicorp/nomad/nomad/fsm"
	"github.com/hashicorp/nomad/nomad/state"
)

func RestoreFromArchive(archive io.Reader, filter *fsm.Filter) (*state.StateStore, *raft.SnapshotMeta, error) {
	logger := hclog.L()

	dummyFSM, err := dummyFSM(logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create FSM: %w", err)
	}

	// r is closed by RestoreFiltered, w is closed by CopySnapshot
	r, w := io.Pipe()

	errCh := make(chan error)
	metaCh := make(chan *raft.SnapshotMeta)

	go func() {
		meta, err := snapshot.CopySnapshot(archive, w)
		if err != nil {
			errCh <- fmt.Errorf("failed to read snapshot: %w", err)
		} else {
			metaCh <- meta
		}
	}()

	err = dummyFSM.RestoreWithFilter(r, filter)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	select {
	case err := <-errCh:
		return nil, nil, err
	case meta := <-metaCh:
		return dummyFSM.State(), meta, nil
	}
}
