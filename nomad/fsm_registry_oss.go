// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
)

// registerLogAppliers is a no-op for open-source only FSMs.
func (n *nomadFSM) registerLogAppliers() {}

// registerSnapshotRestorers is a no-op for open-source only FSMs.
func (n *nomadFSM) registerSnapshotRestorers() {}

// persistEnterpriseTables is a no-op for open-source only FSMs.
func (s *nomadSnapshot) persistEnterpriseTables(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	return nil
}
