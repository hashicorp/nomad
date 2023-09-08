// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hoststats

import (
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
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

	if idle != 100.0 {
		t.Errorf("idle: Expected: %f, Got %f", 100.0, idle)
	}
	if user != 0.0 {
		t.Errorf("user: Expected: %f, Got %f", 0.0, user)
	}
	if system != 0.0 {
		t.Errorf("system: Expected: %f, Got %f", 0.0, system)
	}
	if total != 0.0 {
		t.Errorf("total: Expected: %f, Got %f", 0.0, total)
	}
}
