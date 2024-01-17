// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package resources

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type MicroSecond uint64

type Percent float64

type Utilization struct {
	Memory uint64
	Swap   uint64
	Cache  uint64

	System          Percent
	User            Percent
	Percent         Percent
	ThrottlePeriods uint64
	ThrottleTime    uint64
	Ticks           Percent
}

type TrackCPU struct {
	prevTime   time.Time
	prevUser   MicroSecond
	prevSystem MicroSecond
	prevTotal  MicroSecond
}

// Percent returns the percentage of time spent in user, system, total CPU usage.
func (t *TrackCPU) Percent(user, system, total MicroSecond) (Percent, Percent, Percent) {
	now := time.Now()

	if t.prevUser == 0 && t.prevSystem == 0 {
		t.prevUser = user
		t.prevSystem = system
		t.prevTotal = total
		return 0.0, 0.0, 0.0
	}

	elapsed := now.Sub(t.prevTime).Microseconds()
	userPct := t.percent(t.prevUser, user, elapsed)
	systemPct := t.percent(t.prevSystem, system, elapsed)
	totalPct := t.percent(t.prevTotal, total, elapsed)
	t.prevUser = user
	t.prevSystem = system
	t.prevTotal = total
	t.prevTime = now
	return userPct, systemPct, totalPct
}

func (t *TrackCPU) percent(t1, t2 MicroSecond, elapsed int64) Percent {
	delta := t2 - t1
	if elapsed <= 0 || delta <= 0.0 {
		return 0.0
	}
	return Percent(float64(delta)/float64(elapsed)) * 100.0
}

type Specs struct {
	MHz   int
	Cores int
}

func (s *Specs) Ticks() int {
	return s.Cores * s.MHz
}

var (
	mhzRe       = regexp.MustCompile(`cpu MHz\s+:\s+(\d+)\.\d+`)
	processorRe = regexp.MustCompile(`processor\s+:\s+(\d+)`)
)

func Get() (*Specs, error) {
	// todo: read base_freq instead, for more accurate per-core
	// information similar to how m1 stuff works. probably in a library.
	// cannot really do this until nomad and all task drivers agree

	// todo: cache this value, it should never change

	var speed int
	b, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq")
	if err == nil {
		i, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err == nil {
			speed = i / 1000
		}
	}

	// need this for core count (for now) and fallback speeds
	b, err = os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return nil, err
	}
	content := string(b)

	// if the devices info doesn't work (i.e. running on ec2), fallback to
	// reading live frequencies from cpuinfo and pick the largest one
	if speed == 0 {
		results := mhzRe.FindAllStringSubmatch(content, -1)
		for _, result := range results {
			if mhz, _ := strconv.Atoi(result[1]); mhz > speed {
				speed = mhz
			}
		}
	}

	// number of cores really means number of hyperthreads
	cores := len(processorRe.FindAllStringSubmatch(content, -1))

	return &Specs{
		MHz:   speed,
		Cores: cores,
	}, nil
}

// Bandwidth computes the CPU bandwidth given a mhz value from task config.
// We assume the bandwidth per-core base is 100_000 which is the default.
func Bandwidth(mhz uint64) (uint64, error) {
	specs, err := Get()
	if err != nil {
		return 0, err
	}

	v := (mhz * 100000) / uint64(specs.MHz)
	return v, nil
}
