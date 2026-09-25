// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package dynamic

type FairshareResources struct {
	CPU    float64
	Memory float64
}

func (r *FairshareResources) Total() float64 {
	return r.CPU + r.Memory
}

func (r *FairshareResources) ByResource() map[string]float64 {
	return map[string]float64{
		"cpu":    r.CPU,
		"memory": r.Memory,
	}
}
