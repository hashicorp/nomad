// +build ent

package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// refreshIndex returns the index the scheduler should refresh to as the maximum
// of the allocation, node, and quota tables.
func refreshIndex(snap *state.StateSnapshot) (uint64, error) {
	allocIndex, err := snap.Index("allocs")
	if err != nil {
		return 0, err
	}
	nodeIndex, err := snap.Index("nodes")
	if err != nil {
		return 0, err
	}
	quotaIndex, err := snap.Index(state.TableQuotaSpec)
	if err != nil {
		return 0, err
	}
	return maxUint64(nodeIndex, allocIndex, quotaIndex), nil
}

// evaluatePlanQuota returns whether the plan would be over quota
func evaluatePlanQuota(snap *state.StateSnapshot, plan *structs.Plan) (bool, error) {
	// If the job is nil, we are deregistering the job and as such can not
	// transition into exceeding quota.
	job := plan.Job
	if job == nil {
		return false, nil
	}

	// Get the namespace
	namespace, err := snap.NamespaceByName(nil, job.Namespace)
	if err != nil {
		return false, fmt.Errorf("failed to lookup job %q namespace %q: %v", job.ID, job.Namespace, err)
	} else if namespace == nil {
		return false, fmt.Errorf("unknown namespace %q referenced by job %q", job.Namespace, job.ID)
	}

	// There is no quota attached to the namespace so there is nothing to verify
	if namespace.Quota == "" {
		return false, nil
	}

	// Lookup the quota spec
	quota, err := snap.QuotaSpecByName(nil, namespace.Quota)
	if err != nil {
		return false, fmt.Errorf("failed to lookup quota %q: %v", namespace.Quota, err)
	} else if quota == nil {
		return false, fmt.Errorf("unknown quota %q referenced by namespace %q", namespace.Quota, namespace.Name)
	}

	// Lookup the current quota usage
	usage, err := snap.QuotaUsageByName(nil, namespace.Quota)
	if err != nil {
		return false, fmt.Errorf("failed to lookup quota usage %q: %v", namespace.Quota, err)
	} else if usage == nil {
		return false, fmt.Errorf("unknown quota usage %q", namespace.Quota)
	}

	// Copy the limit so we don't modify the underlying object
	proposedUsage := usage.Copy()
	effectedLimits := structs.UpdateUsageFromPlan(proposedUsage, plan)

	// No changes to the quota
	if len(effectedLimits) == 0 {
		return false, nil
	}

	// Get the actual limit and check if we exceed it
	proposedLimit := effectedLimits[0]
	quotaLimit := quota.LimitsMap()[string(proposedLimit.Hash)]
	superset, _ := quotaLimit.Superset(proposedLimit)
	return !superset, nil
}
