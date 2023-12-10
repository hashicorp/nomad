// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

// BlockedStats returns all the stats about the blocked eval tracker.
type BlockedStats struct {
	// TotalEscaped is the total number of blocked evaluations that have escaped
	// computed node classes.
	TotalEscaped int

	// TotalBlocked is the total number of blocked evaluations.
	TotalBlocked int

	// TotalQuotaLimit is the total number of blocked evaluations that are due
	// to the quota limit being reached.
	TotalQuotaLimit int

	// BlockedResources stores the amount of resources requested by blocked
	// evaluations.
	BlockedResources *BlockedResourcesStats
}

// classInDC is a coordinate of a specific class in a specific datacenter
type classInDC struct {
	dc    string
	class string
}

// NewBlockedStats returns a new BlockedStats.
func NewBlockedStats() *BlockedStats {
	return &BlockedStats{
		BlockedResources: NewBlockedResourcesStats(),
	}
}

// Block updates the stats for the blocked eval tracker with the details of the
// evaluation being blocked.
func (b *BlockedStats) Block(eval *structs.Evaluation) {
	b.TotalBlocked++
	resourceStats := generateResourceStats(eval)
	b.BlockedResources = b.BlockedResources.Add(resourceStats)
}

// Unblock updates the stats for the blocked eval tracker with the details of the
// evaluation being unblocked.
func (b *BlockedStats) Unblock(eval *structs.Evaluation) {
	b.TotalBlocked--
	resourceStats := generateResourceStats(eval)
	b.BlockedResources = b.BlockedResources.Subtract(resourceStats)
}

// prune deletes any key zero metric values older than the cutoff.
func (b *BlockedStats) prune(cutoff time.Time) {
	shouldPrune := func(s BlockedResourcesSummary) bool {
		return s.Timestamp.Before(cutoff) && s.IsZero()
	}

	for k, v := range b.BlockedResources.ByJob {
		if shouldPrune(v) {
			delete(b.BlockedResources.ByJob, k)
		}
	}

	for k, v := range b.BlockedResources.ByClassInDC {
		if shouldPrune(v) {
			delete(b.BlockedResources.ByClassInDC, k)
		}
	}
}

// generateResourceStats returns a summary of the resources requested by the
// input evaluation.
func generateResourceStats(eval *structs.Evaluation) *BlockedResourcesStats {
	dcs := make(map[string]struct{})
	classes := make(map[string]struct{})

	resources := BlockedResourcesSummary{
		Timestamp: time.Now().UTC(),
	}

	for _, allocMetrics := range eval.FailedTGAllocs {
		for dc := range allocMetrics.NodesAvailable {
			dcs[dc] = struct{}{}
		}
		for class := range allocMetrics.ClassExhausted {
			classes[class] = struct{}{}
		}
		if len(allocMetrics.ClassExhausted) == 0 {
			// some evaluations have no class
			classes[""] = struct{}{}
		}
		for _, r := range allocMetrics.ResourcesExhausted {
			resources.CPU += r.CPU
			resources.MemoryMB += r.MemoryMB
		}
	}

	byJob := make(map[structs.NamespacedID]BlockedResourcesSummary)
	nsID := structs.NewNamespacedID(eval.JobID, eval.Namespace)
	byJob[nsID] = resources

	byClassInDC := make(map[classInDC]BlockedResourcesSummary)
	for dc := range dcs {
		for class := range classes {
			k := classInDC{dc: dc, class: class}
			byClassInDC[k] = resources
		}
	}

	return &BlockedResourcesStats{
		ByJob:       byJob,
		ByClassInDC: byClassInDC,
	}
}

// BlockedResourcesStats stores resources requested by blocked evaluations,
// tracked both by job and by node.
type BlockedResourcesStats struct {
	ByJob       map[structs.NamespacedID]BlockedResourcesSummary
	ByClassInDC map[classInDC]BlockedResourcesSummary
}

// NewBlockedResourcesStats returns a new BlockedResourcesStats.
func NewBlockedResourcesStats() *BlockedResourcesStats {
	return &BlockedResourcesStats{
		ByJob:       make(map[structs.NamespacedID]BlockedResourcesSummary),
		ByClassInDC: make(map[classInDC]BlockedResourcesSummary),
	}
}

// Copy returns a deep copy of the blocked resource stats.
func (b *BlockedResourcesStats) Copy() *BlockedResourcesStats {
	result := NewBlockedResourcesStats()

	for k, v := range b.ByJob {
		result.ByJob[k] = v // value copy
	}

	for k, v := range b.ByClassInDC {
		result.ByClassInDC[k] = v // value copy
	}

	return result
}

// Add returns a new BlockedResourcesStats with the values set to the current
// resource values plus the input.
func (b *BlockedResourcesStats) Add(a *BlockedResourcesStats) *BlockedResourcesStats {
	result := b.Copy()

	for k, v := range a.ByJob {
		result.ByJob[k] = b.ByJob[k].Add(v)
	}

	for k, v := range a.ByClassInDC {
		result.ByClassInDC[k] = b.ByClassInDC[k].Add(v)
	}

	return result
}

// Subtract returns a new BlockedResourcesStats with the values set to the
// current resource values minus the input.
func (b *BlockedResourcesStats) Subtract(a *BlockedResourcesStats) *BlockedResourcesStats {
	result := b.Copy()

	for k, v := range a.ByJob {
		result.ByJob[k] = b.ByJob[k].Subtract(v)
	}

	for k, v := range a.ByClassInDC {
		result.ByClassInDC[k] = b.ByClassInDC[k].Subtract(v)
	}

	return result
}

// BlockedResourcesSummary stores resource values for blocked evals.
type BlockedResourcesSummary struct {
	Timestamp time.Time
	CPU       int
	MemoryMB  int
}

// Add returns a new BlockedResourcesSummary with each resource set to the
// current value plus the input.
func (b BlockedResourcesSummary) Add(a BlockedResourcesSummary) BlockedResourcesSummary {
	return BlockedResourcesSummary{
		Timestamp: a.Timestamp,
		CPU:       b.CPU + a.CPU,
		MemoryMB:  b.MemoryMB + a.MemoryMB,
	}
}

// Subtract returns a new BlockedResourcesSummary with each resource set to the
// current value minus the input.
func (b BlockedResourcesSummary) Subtract(a BlockedResourcesSummary) BlockedResourcesSummary {
	return BlockedResourcesSummary{
		Timestamp: a.Timestamp,
		CPU:       b.CPU - a.CPU,
		MemoryMB:  b.MemoryMB - a.MemoryMB,
	}
}

// IsZero returns true if all resource values are zero.
func (b BlockedResourcesSummary) IsZero() bool {
	return b.CPU == 0 && b.MemoryMB == 0
}
