// +build pro ent

package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
	"github.com/ugorji/go/codec"
)

// Offset the Nomad Pro specific values so that we don't overlap
// the OSS/Enterprise values.
const (
	NamespaceSnapshot SnapshotType = (64 + iota)
)

// registerProLogAppliers registers all the Nomad Pro Raft log appliers
func (n *nomadFSM) registerProLogAppliers() {
	n.enterpriseAppliers[structs.NamespaceUpsertRequestType] = n.applyNamespaceUpsert
	n.enterpriseAppliers[structs.NamespaceDeleteRequestType] = n.applyNamespaceDelete
}

// registerProSnapshotRestorers registers all the Nomad Pro snapshot restorers
func (n *nomadFSM) registerProSnapshotRestorers() {
	n.enterpriseRestorers[NamespaceSnapshot] = restoreNamespace
}

// applyNamespaceUpsert is used to upsert a set of namespaces
func (n *nomadFSM) applyNamespaceUpsert(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_namespace_upsert"}, time.Now())
	var req structs.NamespaceUpsertRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertNamespaces(index, req.Namespaces); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpsertNamespaces failed: %v", err)
		return err
	}

	return nil
}

// applyNamespaceDelete is used to delete a set of namespaces
func (n *nomadFSM) applyNamespaceDelete(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_namespace_delete"}, time.Now())
	var req structs.NamespaceDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteNamespaces(index, req.Namespaces); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: DeleteNamespaces failed: %v", err)
		return err
	}

	return nil
}

// restoreNamespace is used to restore a namespace snapshot
func restoreNamespace(restore *state.StateRestore, dec *codec.Decoder) error {
	namespace := new(structs.Namespace)
	if err := dec.Decode(namespace); err != nil {
		return err
	}
	return restore.NamespaceRestore(namespace)
}

// persistProTables persists all the Nomad Pro state store tables.
func (s *nomadSnapshot) persistProTables(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	if err := s.persistNamespaces(sink, encoder); err != nil {
		return err
	}

	return nil
}

// persistNamespaces persists all the namespaces.
func (s *nomadSnapshot) persistNamespaces(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	// Get all the jobs
	ws := memdb.NewWatchSet()
	namespaces, err := s.snap.Namespaces(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := namespaces.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		namespace := raw.(*structs.Namespace)

		// Write out a namespace registration
		sink.Write([]byte{byte(NamespaceSnapshot)})
		if err := encoder.Encode(namespace); err != nil {
			return err
		}
	}
	return nil
}
