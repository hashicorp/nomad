// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package executor

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/test/must"
)

func Test_computeMemory(t *testing.T) {
	cases := []struct {
		memory    int64
		memoryMax int64
		expSoft   int64
		expHard   int64
	}{
		{
			// typical case; only 'memory' is set and that is used as the hard
			// memory limit
			memory:    100,
			memoryMax: 0,
			expSoft:   0,
			expHard:   mbToBytes(100),
		},
		{
			// oversub case; both 'memory' and 'memory_max' are set and used as
			// the soft and hard memory limits
			memory:    100,
			memoryMax: 200,
			expSoft:   mbToBytes(100),
			expHard:   mbToBytes(200),
		},
		{
			// special oversub case; 'memory' is set and 'memory_max' is set to
			// -1; which indicates there should be no hard limit (i.e. -1 / max)
			memory:    100,
			memoryMax: memoryNoLimit,
			expSoft:   mbToBytes(100),
			expHard:   memoryNoLimit,
		},
	}

	for _, tc := range cases {
		name := fmt.Sprintf("(%d,%d)", tc.memory, tc.memoryMax)
		t.Run(name, func(t *testing.T) {
			command := &ExecCommand{
				Resources: &drivers.Resources{
					NomadResources: &structs.AllocatedTaskResources{
						Memory: structs.AllocatedMemoryResources{
							MemoryMB:    tc.memory,
							MemoryMaxMB: tc.memoryMax,
						},
					},
				},
			}
			hard, soft := (*UniversalExecutor)(nil).computeMemory(command)
			must.Eq(t, tc.expSoft, soft)
			must.Eq(t, tc.expHard, hard)
		})
	}
}
