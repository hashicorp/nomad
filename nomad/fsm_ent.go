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
	n.enterpriseAppliers[structs.QuotaSpecUpsertRequestType] = n.applyQuotaSpecUpsert
	n.enterpriseAppliers[structs.QuotaSpecDeleteRequestType] = n.applyQuotaSpecDelete
}

// registerEntSnapshotRestorers registers all the Nomad Enterprise snapshot restorers
func (n *nomadFSM) registerEntSnapshotRestorers() {
	n.enterpriseRestorers[SentinelPolicySnapshot] = restoreSentinelPolicy
	n.enterpriseRestorers[QuotaSpecSnapshot] = restoreQuotaSpec
	n.enterpriseRestorers[QuotaUsageSnapshot] = restoreQuotaUsage
}

// applySentinelPolicyUpsert is used to upsert a set of policies
func (n *nomadFSM) applySentinelPolicyUpsert(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_sentinel_policy_upsert"}, time.Now())
	var req structs.SentinelPolicyUpsertRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertSentinelPolicies(index, req.Policies); err != nil {
		n.logger.Error("UpsertSentinelPolicies failed", "error", err)
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
		n.logger.Error("DeleteSentinelPolicies failed", "error", err)
		return err
	}
	return nil
}

// applyQuotaSpecUpsert is used to upsert a set of quota specifications
func (n *nomadFSM) applyQuotaSpecUpsert(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_quota_spec_upsert"}, time.Now())
	var req structs.QuotaSpecUpsertRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.UpsertQuotaSpecs(index, req.Quotas); err != nil {
		n.logger.Error("UpsertQuotaSpecs failed", "error", err)
		return err
	}

	// Unblock the quotas. This will be a no-op if there are no evals blocked on
	// the quota.
	for _, q := range req.Quotas {
		n.blockedEvals.UnblockQuota(q.Name, index)
	}

	return nil
}

// applyQuotaSpecDelete is used to delete a set of policies
func (n *nomadFSM) applyQuotaSpecDelete(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "apply_quota_spec_delete"}, time.Now())
	var req structs.QuotaSpecDeleteRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	if err := n.state.DeleteQuotaSpecs(index, req.Names); err != nil {
		n.logger.Error("DeleteQuotaSpecs failed", "error", err)
		return err
	}
	return nil
}

// allocQuota returns the quota object associated with the allocation.
func (n *nomadFSM) allocQuota(allocID string) (string, error) {
	alloc, err := n.state.AllocByID(nil, allocID)
	if err != nil {
		return "", err
	}

	// Guard against the client updating a non-existent allocation.
	if alloc == nil {
		return "", nil
	}

	ns, err := n.state.NamespaceByName(nil, alloc.Namespace)
	if err != nil {
		return "", err
	}
	if ns == nil {
		return "", fmt.Errorf("unknown namespace %q attached to alloc %q", alloc.Namespace, alloc.ID)
	}

	return ns.Quota, nil
}

// restoreSentinelPolicy is used to restore a sentinel policy
func restoreSentinelPolicy(restore *state.StateRestore, dec *codec.Decoder) error {
	policy := new(structs.SentinelPolicy)
	if err := dec.Decode(policy); err != nil {
		return err
	}
	return restore.SentinelPolicyRestore(policy)
}

// restoreQuotaSpec is used to restore a quota spec
func restoreQuotaSpec(restore *state.StateRestore, dec *codec.Decoder) error {
	spec := new(structs.QuotaSpec)
	if err := dec.Decode(spec); err != nil {
		return err
	}
	return restore.QuotaSpecRestore(spec)
}

// restoreQuotaUsage is used to restore a quota usage
func restoreQuotaUsage(restore *state.StateRestore, dec *codec.Decoder) error {
	usage := new(structs.QuotaUsage)
	if err := dec.Decode(usage); err != nil {
		return err
	}
	return restore.QuotaUsageRestore(usage)
}

// persistEntTables persists all the Nomad Enterprise state store tables.
func (s *nomadSnapshot) persistEntTables(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	if err := s.persistSentinelPolicies(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistQuotaSpecs(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistQuotaUsages(sink, encoder); err != nil {
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

// persistQuotaSpecs is used to persist quota specifications
func (s *nomadSnapshot) persistQuotaSpecs(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the specs
	ws := memdb.NewWatchSet()
	policies, err := s.snap.QuotaSpecs(ws)
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
		spec := raw.(*structs.QuotaSpec)

		// Write out a spec registration
		sink.Write([]byte{byte(QuotaSpecSnapshot)})
		if err := encoder.Encode(spec); err != nil {
			return err
		}
	}
	return nil
}

// persistQuotaUsages is used to persist quota usages
func (s *nomadSnapshot) persistQuotaUsages(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the usages
	ws := memdb.NewWatchSet()
	policies, err := s.snap.QuotaUsages(ws)
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
		usage := raw.(*structs.QuotaUsage)

		// Write out a spec registration
		sink.Write([]byte{byte(QuotaUsageSnapshot)})
		if err := encoder.Encode(usage); err != nil {
			return err
		}
	}
	return nil
}
