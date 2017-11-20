// +build pro

package nomad

import (
	"github.com/hashicorp/raft"
	"github.com/ugorji/go/codec"
)

// registerLogAppliers registers all the Nomad Pro Raft log appliers.
func (n *nomadFSM) registerLogAppliers() {
	n.registerProLogAppliers()
}

// registerSnapshotRestorers registers all the Nomad Enterprise snapshot
// restorers.
func (n *nomadFSM) registerSnapshotRestorers() {
	n.registerProSnapshotRestorers()
}

// persistEnterpriseTables persists all the Nomad Pro state store tables.
func (s *nomadSnapshot) persistEnterpriseTables(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	return s.persistProTables(sink, encoder)
}
