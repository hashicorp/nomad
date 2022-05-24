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

// node stores information related to nodes.
type node struct {
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

	for k, v := range b.BlockedResources.ByNode {
		if shouldPrune(v) {
			delete(b.BlockedResources.ByNode, k)
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
		for _, r := range allocMetrics.ResourcesExhausted {
			resources.CPU += r.CPU
			resources.MemoryMB += r.MemoryMB
		}
	}

	byJob := make(map[structs.NamespacedID]BlockedResourcesSummary)
	nsID := structs.NewNamespacedID(eval.JobID, eval.Namespace)
	byJob[nsID] = resources

	byNodeInfo := make(map[node]BlockedResourcesSummary)
	for dc := range dcs {
		for class := range classes {
			k := node{dc: dc, class: class}
			byNodeInfo[k] = resources
		}
	}

	return &BlockedResourcesStats{
		ByJob:  byJob,
		ByNode: byNodeInfo,
	}
}

// BlockedResourcesStats stores resources requested by blocked evaluations,
// tracked both by job and by node.
type BlockedResourcesStats struct {
	ByJob  map[structs.NamespacedID]BlockedResourcesSummary
	ByNode map[node]BlockedResourcesSummary
}

// NewBlockedResourcesStats returns a new BlockedResourcesStats.
func NewBlockedResourcesStats() *BlockedResourcesStats {
	return &BlockedResourcesStats{
		ByJob:  make(map[structs.NamespacedID]BlockedResourcesSummary),
		ByNode: make(map[node]BlockedResourcesSummary),
	}
}

// Copy returns a deep copy of the blocked resource stats.
func (b *BlockedResourcesStats) Copy() *BlockedResourcesStats {
	result := NewBlockedResourcesStats()

	for k, v := range b.ByJob {
		result.ByJob[k] = v // value copy
	}

	for k, v := range b.ByNode {
		result.ByNode[k] = v // value copy
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

	for k, v := range a.ByNode {
		result.ByNode[k] = b.ByNode[k].Add(v)
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

	for k, v := range a.ByNode {
		result.ByNode[k] = b.ByNode[k].Subtract(v)
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
