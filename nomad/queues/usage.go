// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package queues

type ResourceUsage struct {
	CPU    float64
	Memory float64
}

func (r *ResourceUsage) Add(addedUsage *ResourceUsage) *ResourceUsage {
	r.CPU += addedUsage.CPU
	r.Memory += addedUsage.Memory
	return r
}

func (r *ResourceUsage) AddCpu(amount float64) {
	r.CPU += amount
}

func (r *ResourceUsage) AddMemory(amount float64) {
	r.Memory += amount
}

func (r *ResourceUsage) Total() float64 {
	total := 0.0

	total += r.CPU
	total += r.Memory

	return total
}

func (r *ResourceUsage) UsageByResource() map[string]float64 {
	return map[string]float64{
		"cpu":    r.CPU,
		"memory": r.Memory,
	}
}
