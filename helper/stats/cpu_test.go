// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stats

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
)

func TestCpuStatsPercent(t *testing.T) {
	ci.Parallel(t)

	cs := NewCpuStats()
	cs.Percent(79.7)
	time.Sleep(1 * time.Second)
	percent := cs.Percent(80.69)
	expectedPercent := 98.00
	if percent < expectedPercent && percent > (expectedPercent+1.00) {
		t.Fatalf("expected: %v, actual: %v", expectedPercent, percent)
	}
}
