// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/raft"
)

// registerLogAppliers is a no-op for community edition only FSMs.
func (n *nomadFSM) registerLogAppliers() {}

// registerSnapshotRestorers is a no-op for community edition only FSMs.
func (n *nomadFSM) registerSnapshotRestorers() {}

// persistEnterpriseTables is a no-op for community edition only FSMs.
func (s *nomadSnapshot) persistEnterpriseTables(_ raft.SnapshotSink, _ *codec.Encoder) error {
	return nil
}
