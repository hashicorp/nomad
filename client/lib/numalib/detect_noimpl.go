// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package numalib

// NoImpl will check that the topology has been set, otherwise set  a default
// value of 1 core @ 1 ghz. This should only be activated in tests that
// disable the cpu fingerprinter.
func NoImpl(top *Topology) *Topology {
	if top == nil || len(top.Cores) == 0 {
		return &Topology{
			Cores: []Core{
				{GuessSpeed: 1000},
			},
		}
	}
	return top
}
