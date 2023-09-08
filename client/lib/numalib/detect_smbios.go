// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package numalib

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

const (
	dmidecodeCmd = "dmidecode"
)

var (
	dmiCurSpeedRe = regexp.MustCompile(`Current Speed:\s+(\d+)\s+MHz`)
)

type Smbios struct {
	data string
}

func (s *Smbios) ScanSystem(top *Topology) {
	if !s.available() {
		return
	}

	// sysfs should work on ec2 for detecting numa nodes
	// and so we skip those steps here at least for now, because reading
	// smbios is very platform specific

	// detect guess-level core performance data
	s.discoverCores(top)
}

func (s *Smbios) available() bool {
	path, err := exec.LookPath(dmidecodeCmd)
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "-t", "4")
	b, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	s.data = string(b)
	return true
}

func (s *Smbios) discoverCores(top *Topology) {
	curSpeeds := dmiCurSpeedRe.FindStringSubmatch(s.data)

	if len(curSpeeds) < 2 {
		return
	}

	maxCurSpeed := 0
	for i := 1; i < len(curSpeeds); i++ {
		curSpeed, err := strconv.Atoi(curSpeeds[i])
		if err == nil {
			if curSpeed > maxCurSpeed {
				maxCurSpeed = curSpeed
			}
		}
	}

	// set the guess speed to the highest detected current speed
	for i := 0; i < len(top.Cores); i++ {
		top.Cores[i].GuessSpeed = MHz(maxCurSpeed)
	}
}
