package stats

import (
	"math"
	"os"
	"testing"
	"time"

	shelpers "github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/stretchr/testify/assert"
)

func TestCpuStatsPercent(t *testing.T) {
	cs := NewCpuStats()
	cs.Percent(79.7)
	time.Sleep(1 * time.Second)
	percent := cs.Percent(80.69)
	expectedPercent := 98.00
	if percent < expectedPercent && percent > (expectedPercent+1.00) {
		t.Fatalf("expected: %v, actual: %v", expectedPercent, percent)
	}
}

func TestHostStats_CPU(t *testing.T) {
	assert.Nil(t, shelpers.Init())

	logger := testlog.HCLogger(t)
	cwd, err := os.Getwd()
	assert.Nil(t, err)
	hs := NewHostStatsCollector(logger, cwd, nil)

	// Collect twice so we can calculate percents we need to generate some work
	// so that the cpu values change
	assert.Nil(t, hs.Collect())
	total := 0
	for i := 1; i < 1000000000; i++ {
		total *= i
		total = total % i
	}
	assert.Nil(t, hs.Collect())
	stats := hs.Stats()

	assert.NotZero(t, stats.CPUTicksConsumed)
	assert.NotZero(t, len(stats.CPU))

	for _, cpu := range stats.CPU {
		assert.False(t, math.IsNaN(cpu.Idle))
		assert.False(t, math.IsNaN(cpu.Total))
		assert.False(t, math.IsNaN(cpu.System))
		assert.False(t, math.IsNaN(cpu.User))

		assert.False(t, math.IsInf(cpu.Idle, 0))
		assert.False(t, math.IsInf(cpu.Total, 0))
		assert.False(t, math.IsInf(cpu.System, 0))
		assert.False(t, math.IsInf(cpu.User, 0))
	}
}
