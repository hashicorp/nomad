// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package numalib

import (
	"os"
	"os/exec"
	"testing"

	"github.com/shoenig/test/must"
)

func requiresSMBIOS(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root")
	}

	p, err := exec.LookPath("dmidecode")
	if err != nil {
		t.Skip("requires dmidecode package")
	}

	if p == "" {
		t.Skip("requires dmidecode on path")
	}
}

func TestSmbios_detectSpeed(t *testing.T) {
	requiresSMBIOS(t)

	top := new(Topology)
	sysfs := new(Sysfs)
	smbios := new(Smbios)

	sysfs.ScanSystem(top)
	smbios.ScanSystem(top)

	for _, core := range top.Cores {
		must.Positive(t, core.GuessSpeed)
	}
}
