// +build ent

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

// registerEntLogAppliers registers all the Nomad Enterprise Raft log appliers
func (n *nomadFSM) registerEntLogAppliers() {
	n.enterpriseAppliers[structs.SentinelPolicyUpsertRequestType] = n.applySentinelPolicyUpsert
	n.enterpriseAppliers[structs.SentinelPolicyDeleteRequestType] = n.applySentinelPolicyDelete
}

// registerEntSnapshotRestorers registers all the Nomad Enterprise snapshot restorers
func (n *nomadFSM) registerEntSnapshotRestorers() {
	n.enterpriseRestorers[SentinelPolicySnapshot] = restoreSentinelPolicy
}

// applySentinelPolicyUpsert is used to upsert a set of policies
func (n *nomadFSM) applySentinelPolicyUpsert(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_sentinel_policy_upsert"}, time.Now())
	var req structs.SentinelPolicyUpsertRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertSentinelPolicies(index, req.Policies); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: UpsertSentinelPolicies failed: %v", err)
		return err
	}
	return nil
}

// applySentinelPolicyDelete is used to delete a set of policies
func (n *nomadFSM) applySentinelPolicyDelete(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_sentinel_policy_delete"}, time.Now())
	var req structs.SentinelPolicyDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteSentinelPolicies(index, req.Names); err != nil {
		n.logger.Printf("[ERR] nomad.fsm: DeleteSentinelPolicies failed: %v", err)
		return err
	}
	return nil
}

// restoreSentinelPolicy is used to restore a sentinel policy
func restoreSentinelPolicy(restore *state.StateRestore, dec *codec.Decoder) error {
	policy := new(structs.SentinelPolicy)
	if err := dec.Decode(policy); err != nil {
		return err
	}
	return restore.SentinelPolicyRestore(policy)
}

// persistEntTables persists all the Nomad Enterprise state store tables.
func (s *nomadSnapshot) persistEntTables(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	if err := s.persistSentinelPolicies(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	return nil
}

// persistSentinelPolicies is used to persist sentinel policies
func (s *nomadSnapshot) persistSentinelPolicies(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the policies
	ws := memdb.NewWatchSet()
	policies, err := s.snap.SentinelPolicies(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := policies.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		policy := raw.(*structs.SentinelPolicy)

		// Write out a policy registration
		sink.Write([]byte{byte(SentinelPolicySnapshot)})
		if err := encoder.Encode(policy); err != nil {
			return err
		}
	}
	return nil
}
