//go:build !ent
// +build !ent

package fsm

import (
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
)

// registerLogAppliers is a no-op for open-source only FSMs.
func (n *FSM) registerLogAppliers() {}

// registerSnapshotRestorers is a no-op for open-source only FSMs.
func (n *FSM) registerSnapshotRestorers() {}

// persistEnterpriseTables is a no-op for open-source only FSMs.
func (s *nomadSnapshot) persistEnterpriseTables(_ raft.SnapshotSink, _ *codec.Encoder) error {
	return nil
}
