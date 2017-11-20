// +build ent

package structs

// UpdateUsageFromPlan updates the given quota usage with the planned allocation
// changes and returns the effected QuotaLimits
func UpdateUsageFromPlan(usage *QuotaUsage, plan *Plan) []*QuotaLimit {
	// There is no limit that applies to us.
	if plan == nil || usage == nil || len(usage.Used) == 0 {
		return nil
	}

	// As of now there can only be one limit that applies to us. We know it
	// will exists since the UpsertQuotaSpec method will create the usage object
	// for this region
	var limit *QuotaLimit
	for _, l := range usage.Used {
		limit = l
	}

	// Gather the set of proposed stops.
	for _, stops := range plan.NodeUpdate {
		for _, stop := range stops {
			r := &Resources{}
			for _, v := range stop.TaskResources {
				r.Add(v)
			}
			limit.SubtractResource(r)
		}
	}

	// Gather the proposed allocations
	for _, placements := range plan.NodeAllocation {
		for _, place := range placements {
			if place.TerminalStatus() {
				// Shouldn't happen, just guarding
				continue
			} else if place.CreateIndex != 0 {
				// The allocation already exists and is thus accounted for
				continue
			}

			r := &Resources{}
			for _, v := range place.TaskResources {
				r.Add(v)
			}
			limit.AddResource(r)
		}
	}

	return []*QuotaLimit{limit}
}

// FindRegionLimit takes a set of QuotaLimits and returns the one matching the
// given region.
func FindRegionLimit(limits map[string]*QuotaLimit, region string) *QuotaLimit {
	for _, l := range limits {
		if l.Region == region {
			return l
		}
	}
	return nil
}
