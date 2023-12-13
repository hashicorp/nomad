// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin && arm64 && cgo

package stats

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestCPU_Init(t *testing.T) {
	must.NoError(t, Init())
}

func TestCPU_CPUNumCores(t *testing.T) {
	big, little := CPUNumCores()
	must.Between(t, 4, big, 32)
	must.Between(t, 2, little, 8)
}

func TestCPU_CPUMHzPerCore(t *testing.T) {
	big, little := CPUMHzPerCore()
	must.Between(t, 3_000, big, 6_000)
	must.Between(t, 2_000, little, 4_000)
}

func TestCPU_CPUModelName(t *testing.T) {
	name := CPUModelName()
	must.NotEq(t, "", name)
}

func TestCPU_CPUCpuTotalTicks(t *testing.T) {
	ticks := CpuTotalTicks()
	must.Positive(t, ticks)
}
