// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/snapshot"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
)

func RestoreFromArchive(archive io.Reader, filter *nomad.FSMFilter) (raft.FSM, *state.StateStore, *raft.SnapshotMeta, error) {
	logger := hclog.L()

	fsm, err := dummyFSM(logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create FSM: %w", err)
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

	err = fsm.RestoreWithFilter(r, filter)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	select {
	case err := <-errCh:
		return nil, nil, nil, err
	case meta := <-metaCh:
		return fsm, fsm.State(), meta, nil
	}
}

func RedactSnapshot(srcFile *os.File) error {
	srcFile.Seek(0, 0)
	fsm, store, meta, err := RestoreFromArchive(srcFile, nil)
	if err != nil {
		return fmt.Errorf("Failed to load snapshot from archive: %w", err)
	}

	iter, err := store.RootKeys(nil)
	if err != nil {
		return fmt.Errorf("Failed to query for root keys: %v", err)
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		rootKey := raw.(*structs.RootKey)
		if rootKey == nil {
			break
		}
		if len(rootKey.WrappedKeys) > 0 {
			rootKey.KeyID = rootKey.KeyID + " [REDACTED]"
			rootKey.WrappedKeys = nil
		}
		msg, err := structs.Encode(structs.WrappedRootKeysUpsertRequestType,
			&structs.KeyringUpsertWrappedRootKeyRequest{
				WrappedRootKeys: rootKey,
			})
		if err != nil {
			return fmt.Errorf("Could not re-encode redacted key: %v", err)
		}

		fsm.Apply(&raft.Log{
			Type: raft.LogCommand,
			Data: msg,
		})
	}

	snap, err := snapshot.NewFromFSM(hclog.Default(), fsm, meta)
	if err != nil {
		return fmt.Errorf("Failed to create redacted snapshot: %v", err)
	}

	srcFile.Truncate(0)
	srcFile.Seek(0, 0)

	_, err = io.Copy(srcFile, snap)
	if err != nil {
		return fmt.Errorf("Failed to copy snapshot to temporary file: %v", err)
	}

	return srcFile.Sync()
}
