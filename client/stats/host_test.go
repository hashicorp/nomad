// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stats

import (
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shoenig/test/must"
)

func TestHostCpuStatsCalculator_Nan(t *testing.T) {
	times := cpu.TimesStat{
		User:   0.0,
		Idle:   100.0,
		System: 0.0,
	}

	calculator := NewHostCpuStatsCalculator()
	calculator.Calculate(times)
	idle, user, system, total := calculator.Calculate(times)

	must.Eq(t, 100.0, idle, must.Sprint("unexpected idle stats"))
	must.Eq(t, 0.0, user, must.Sprint("unexpected user stats"))
	must.Eq(t, 0.0, system, must.Sprint("unexpected system stats"))
	must.Eq(t, 0.0, total, must.Sprint("unexpected total stats"))
}

func TestHostCpuStatsCalculator_DecreasedIOWait(t *testing.T) {
	times := cpu.TimesStat{
		CPU:    "cpu0",
		User:   20000,
		Nice:   100,
		System: 9000,
		Idle:   370000,
		Iowait: 700,
	}

	calculator := NewHostCpuStatsCalculator()
	calculator.Calculate(times)

	times = cpu.TimesStat{
		CPU:    "cpu0",
		User:   20000,
		Nice:   100,
		System: 9000,
		Idle:   380000,
		Iowait: 600,
	}

	_, _, _, total := calculator.Calculate(times)
	must.GreaterEq(t, 0.0, total, must.Sprint("total must never be negative"))
}
