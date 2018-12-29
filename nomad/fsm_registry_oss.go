// +build !pro,!ent

package nomad

import (
	"github.com/hashicorp/raft"
	"github.com/ugorji/go/codec"
)

// registerLogAppliers is a no-op for open-source only FSMs.
func (n *nomadFSM) registerLogAppliers() {}

// registerSnapshotRestorers is a no-op for open-source only FSMs.
func (n *nomadFSM) registerSnapshotRestorers() {}

// persistEnterpriseTables is a no-op for open-source only FSMs.
func (s *nomadSnapshot) persistEnterpriseTables(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	return nil
}
