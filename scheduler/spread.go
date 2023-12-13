// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scheduler

import (
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// implicitTarget is used to represent any remaining attribute values
	// when target percentages don't add up to 100
	implicitTarget = "*"
)

// SpreadIterator is used to spread allocations across a specified attribute
// according to preset weights
type SpreadIterator struct {
	ctx    Context
	source RankIterator
	job    *structs.Job
	tg     *structs.TaskGroup

	// jobSpreads is a slice of spread stored at the job level which apply
	// to all task groups
	jobSpreads []*structs.Spread

	// tgSpreadInfo is a map per task group with precomputed
	// values for desired counts and weight
	tgSpreadInfo map[string]spreadAttributeMap

	// sumSpreadWeights tracks the total weight across all spread
	// blocks
	sumSpreadWeights int32

	// lowestSpreadBoost tracks the lowest spread boost across all spread blocks
	lowestSpreadBoost float64

	// hasSpread is used to early return when the job/task group
	// does not have spread configured
	hasSpread bool

	// groupProperySets is a memoized map from task group to property sets.
	// existing allocs are computed once, and allocs from the plan are updated
	// when Reset is called
	groupPropertySets map[string][]*propertySet
}

type spreadAttributeMap map[string]*spreadInfo

type spreadInfo struct {
	weight        int8
	desiredCounts map[string]float64
}

func NewSpreadIterator(ctx Context, source RankIterator) *SpreadIterator {
	iter := &SpreadIterator{
		ctx:               ctx,
		source:            source,
		groupPropertySets: make(map[string][]*propertySet),
		tgSpreadInfo:      make(map[string]spreadAttributeMap),
		lowestSpreadBoost: -1.0,
	}
	return iter
}

func (iter *SpreadIterator) Reset() {
	iter.source.Reset()
	for _, sets := range iter.groupPropertySets {
		for _, ps := range sets {
			ps.PopulateProposed()
		}
	}
}

func (iter *SpreadIterator) SetJob(job *structs.Job) {
	iter.job = job
	if job.Spreads != nil {
		iter.jobSpreads = job.Spreads
	}

	// reset group spread/property so that when we temporarily SetJob
	// to an older version to calculate stops we don't leak old
	// versions of spread/properties to the new job version
	iter.tgSpreadInfo = make(map[string]spreadAttributeMap)
	iter.groupPropertySets = make(map[string][]*propertySet)
}

func (iter *SpreadIterator) SetTaskGroup(tg *structs.TaskGroup) {
	iter.tg = tg

	// Build the property set at the taskgroup level
	if _, ok := iter.groupPropertySets[tg.Name]; !ok {
		// First add property sets that are at the job level for this task group
		for _, spread := range iter.jobSpreads {
			pset := NewPropertySet(iter.ctx, iter.job)
			pset.SetTargetAttribute(spread.Attribute, tg.Name)
			pset.SetTargetValues(helper.ConvertSlice(spread.SpreadTarget,
				func(t *structs.SpreadTarget) string { return t.Value }))
			iter.groupPropertySets[tg.Name] = append(iter.groupPropertySets[tg.Name], pset)
		}

		// Include property sets at the task group level
		for _, spread := range tg.Spreads {
			pset := NewPropertySet(iter.ctx, iter.job)
			pset.SetTargetAttribute(spread.Attribute, tg.Name)
			pset.SetTargetValues(helper.ConvertSlice(spread.SpreadTarget,
				func(t *structs.SpreadTarget) string { return t.Value }))
			iter.groupPropertySets[tg.Name] = append(iter.groupPropertySets[tg.Name], pset)
		}
	}

	// Check if there are any spreads configured
	iter.hasSpread = len(iter.groupPropertySets[tg.Name]) != 0

	// Build tgSpreadInfo at the task group level
	if _, ok := iter.tgSpreadInfo[tg.Name]; !ok {
		iter.computeSpreadInfo(tg)
	}

}

func (iter *SpreadIterator) hasSpreads() bool {
	return iter.hasSpread
}

func (iter *SpreadIterator) Next() *RankedNode {

	for {
		option := iter.source.Next()

		// Hot path if there is nothing to check
		if option == nil || !iter.hasSpreads() {
			return option
		}

		tgName := iter.tg.Name
		propertySets := iter.groupPropertySets[tgName]
		// Iterate over each spread attribute's property set and add a weighted score
		totalSpreadScore := 0.0
		for _, pset := range propertySets {
			nValue, errorMsg, usedCount := pset.UsedCount(option.Node, tgName)

			// Add one to include placement on this node in the scoring calculation
			usedCount += 1
			// Set score to -1 if there were errors in building this attribute
			if errorMsg != "" {
				iter.ctx.Logger().Named("spread").Debug("error building spread attributes for task group", "task_group", tgName, "error", errorMsg)
				totalSpreadScore -= 1.0
				continue
			}
			spreadAttributeMap := iter.tgSpreadInfo[tgName]
			spreadDetails := spreadAttributeMap[pset.targetAttribute]

			if spreadDetails == nil {
				iter.ctx.Logger().Named("spread").Error(
					"error reading spread attribute map for task group",
					"task_group", tgName,
					"target", pset.targetAttribute,
				)
				continue
			}

			if len(spreadDetails.desiredCounts) == 0 {
				// When desired counts map is empty the user didn't specify any targets
				// Use even spreading scoring algorithm for this scenario
				scoreBoost := evenSpreadScoreBoost(pset, option.Node)
				totalSpreadScore += scoreBoost
			} else {
				// Get the desired count
				desiredCount, ok := spreadDetails.desiredCounts[nValue]
				if !ok {
					// See if there is an implicit target
					desiredCount, ok = spreadDetails.desiredCounts[implicitTarget]
					if !ok {
						// The desired count for this attribute is zero if it gets here
						// so use the default negative penalty for this node
						totalSpreadScore -= 1.0
						continue
					}
				}

				// Calculate the relative weight of this specific spread attribute
				spreadWeight := float64(spreadDetails.weight) / float64(iter.sumSpreadWeights)

				if desiredCount == 0 {
					totalSpreadScore += iter.lowestSpreadBoost
					continue
				}

				// Score Boost is proportional the difference between current and desired count
				// It is negative when the used count is greater than the desired count
				// It is multiplied with the spread weight to account for cases where the job has
				// more than one spread attribute
				scoreBoost := ((desiredCount - float64(usedCount)) / desiredCount) * spreadWeight
				totalSpreadScore += scoreBoost
				if scoreBoost < iter.lowestSpreadBoost {
					iter.lowestSpreadBoost = scoreBoost
				}
			}
		}

		if totalSpreadScore != 0.0 {
			option.Scores = append(option.Scores, totalSpreadScore)
			iter.ctx.Metrics().ScoreNode(option.Node, "allocation-spread", totalSpreadScore)
		}
		return option
	}
}

// evenSpreadScoreBoost is a scoring helper that calculates the score
// for the option when even spread is desired (all attribute values get equal preference)
func evenSpreadScoreBoost(pset *propertySet, option *structs.Node) float64 {
	combinedUseMap := pset.GetCombinedUseMap()
	if len(combinedUseMap) == 0 {
		// Nothing placed yet, so return 0 as the score
		return 0.0
	}
	// Get the nodes property value
	nValue, ok := getProperty(option, pset.targetAttribute)

	// Maximum possible penalty when the attribute isn't set on the node
	if !ok {
		return -1.0
	}
	currentAttributeCount := combinedUseMap[nValue]
	minCount := uint64(0)
	maxCount := uint64(0)
	for _, value := range combinedUseMap {
		if minCount == 0 || value < minCount {
			minCount = value
		}
		if maxCount == 0 || value > maxCount {
			maxCount = value
		}
	}

	// calculate boost based on delta between the current and the minimum
	var deltaBoost float64
	if minCount == 0 {
		deltaBoost = -1.0
	} else {
		delta := int(minCount - currentAttributeCount)
		deltaBoost = float64(delta) / float64(minCount)
	}
	if currentAttributeCount != minCount {
		// Boost based on delta between current and min
		return deltaBoost
	} else if minCount == maxCount {
		// Maximum possible penalty when the distribution is even
		return -1.0
	} else if minCount == 0 {
		// Current attribute count is equal to min and both are zero. This means no allocations
		// were placed for this attribute value yet. Should get the maximum possible boost.
		return 1.0
	}

	// Penalty based on delta from max value
	delta := int(maxCount - minCount)
	deltaBoost = float64(delta) / float64(minCount)
	return deltaBoost

}

// computeSpreadInfo computes and stores percentages and total values
// from all spreads that apply to a specific task group
func (iter *SpreadIterator) computeSpreadInfo(tg *structs.TaskGroup) {
	spreadInfos := make(spreadAttributeMap, len(tg.Spreads))
	totalCount := tg.Count

	// Always combine any spread blocks defined at the job level here
	combinedSpreads := make([]*structs.Spread, 0, len(tg.Spreads)+len(iter.jobSpreads))
	combinedSpreads = append(combinedSpreads, tg.Spreads...)
	combinedSpreads = append(combinedSpreads, iter.jobSpreads...)
	for _, spread := range combinedSpreads {
		si := &spreadInfo{weight: spread.Weight, desiredCounts: make(map[string]float64)}
		sumDesiredCounts := 0.0
		for _, st := range spread.SpreadTarget {
			desiredCount := (float64(st.Percent) / float64(100)) * float64(totalCount)
			si.desiredCounts[st.Value] = desiredCount
			sumDesiredCounts += desiredCount
		}
		// Account for remaining count only if there is any spread targets
		if sumDesiredCounts > 0 && sumDesiredCounts < float64(totalCount) {
			remainingCount := float64(totalCount) - sumDesiredCounts
			si.desiredCounts[implicitTarget] = remainingCount
		}
		spreadInfos[spread.Attribute] = si
		iter.sumSpreadWeights += int32(spread.Weight)
	}
	iter.tgSpreadInfo[tg.Name] = spreadInfos
}
