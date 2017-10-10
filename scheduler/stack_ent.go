// +build ent

package scheduler

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

// QuotaIterator is used to enforce resource quotas. When below the quota limit,
// the iterator acts as a pass through and above it will deny all nodes
type QuotaIterator struct {
	ctx           Context
	source        FeasibleIterator
	buildErr      error
	tg            *structs.TaskGroup
	job           *structs.Job
	quota         *structs.QuotaSpec
	quotaLimits   map[string]*structs.QuotaLimit
	actUsage      *structs.QuotaUsage
	proposedUsage *structs.QuotaUsage
}

// NewQuotaIterator returns a new quota iterator reading from the passed source.
func NewQuotaIterator(ctx Context, source FeasibleIterator) FeasibleIterator {
	return &QuotaIterator{
		ctx:    ctx,
		source: source,
	}
}

func (iter *QuotaIterator) SetTaskGroup(tg *structs.TaskGroup) {
	iter.tg = tg
}

func (iter *QuotaIterator) SetJob(job *structs.Job) {
	iter.job = job

	// Get the converted state object
	state := iter.ctx.State().(StateEnterprise)
	namespace, err := state.NamespaceByName(nil, job.Namespace)
	if err != nil {
		iter.buildErr = fmt.Errorf("failed to lookup job %q namespace %q: %v", job.ID, job.Namespace, err)
		iter.ctx.Logger().Printf("[ERR] scheduler.QuotaIterator: %s", iter.buildErr)
		return
	} else if namespace == nil {
		iter.buildErr = fmt.Errorf("unknown namespace %q referenced by job %q", job.Namespace, job.ID)
		iter.ctx.Logger().Printf("[ERR] scheduler.QuotaIterator: %s", iter.buildErr)
		return
	}

	// There is no quota attached to the namespace so there is nothing for the
	// iterator to do
	if namespace.Quota == "" {
		return
	}

	// Lookup the quota spec
	quota, err := state.QuotaSpecByName(nil, namespace.Quota)
	if err != nil {
		iter.buildErr = fmt.Errorf("failed to lookup quota %q: %v", namespace.Quota, err)
		iter.ctx.Logger().Printf("[ERR] scheduler.QuotaIterator: %s", iter.buildErr)
		return
	} else if quota == nil {
		iter.buildErr = fmt.Errorf("unknown quota %q referenced by namespace %q", namespace.Quota, namespace.Name)
		iter.ctx.Logger().Printf("[ERR] scheduler.QuotaIterator: %s", iter.buildErr)
		return
	}

	// Lookup the current quota usage
	usage, err := state.QuotaUsageByName(nil, namespace.Quota)
	if err != nil {
		iter.buildErr = fmt.Errorf("failed to lookup quota usage %q: %v", namespace.Quota, err)
		iter.ctx.Logger().Printf("[ERR] scheduler.QuotaIterator: %s", iter.buildErr)
		return
	} else if usage == nil {
		iter.buildErr = fmt.Errorf("unknown quota usage %q", namespace.Quota)
		iter.ctx.Logger().Printf("[ERR] scheduler.QuotaIterator: %s", iter.buildErr)
		return
	}

	// There is no limit that applies to us
	if len(usage.Used) == 0 {
		return
	}

	// Store the quota and usage since it applies to us
	iter.quota = quota
	iter.quotaLimits = quota.LimitsMap()
	iter.actUsage = usage
}

func (iter *QuotaIterator) Next() *structs.Node {
	// Get the next option from the source
	option := iter.source.Next()

	// If there is no quota or there was an error building the iterator so
	// just act as a pass through.
	if option == nil || iter.quota == nil || iter.buildErr != nil {
		return option
	}

	// At this point there will be only one limit and it will apply.
	var proposedLimit *structs.QuotaLimit
	for _, l := range iter.proposedUsage.Used {
		proposedLimit = l
	}

	// Add the resources of the propsed task group
	proposedLimit.AddResource(iter.tg.CombinedResources())

	// Get the actual limit
	quotaLimit := iter.quotaLimits[string(proposedLimit.Hash)]

	superset, dimensions := quotaLimit.Superset(proposedLimit)
	if superset {
		return option
	}

	iter.ctx.Metrics().ExhaustQuota(dimensions)
	return nil
}

func (iter *QuotaIterator) Reset() {
	iter.source.Reset()

	// There is nothing more to do
	if iter.quota == nil {
		return
	}

	// Populate the quota usage with proposed allocations
	iter.proposedUsage = iter.actUsage.Copy()

	// As of now there can only be one limit that applies to us
	var limit *structs.QuotaLimit
	for _, l := range iter.proposedUsage.Used {
		limit = l
	}

	// Gather the set of proposed stops.
	for _, stops := range iter.ctx.Plan().NodeUpdate {
		for _, stop := range stops {
			r := &structs.Resources{}
			for _, v := range stop.TaskResources {
				r.Add(v)
			}
			limit.SubtractResource(r)
		}
	}

	// Gather the proposed allocations
	for _, placements := range iter.ctx.Plan().NodeAllocation {
		for _, place := range placements {
			if place.TerminalStatus() {
				// Shouldn't happen, just guarding
				continue
			} else if place.CreateIndex != 0 {
				// The allocation already exists and is thus accounted for
				continue
			}

			r := &structs.Resources{}
			for _, v := range place.TaskResources {
				r.Add(v)
			}
			limit.AddResource(r)
		}
	}
}
