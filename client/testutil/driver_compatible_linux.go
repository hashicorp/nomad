// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package testutil

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

// CgroupsCompatible returns true if either cgroups.v1 or cgroups.v2 is supported.
func CgroupsCompatible(t *testing.T) bool {
	return cgroupsCompatibleV1(t) || cgroupsCompatibleV2(t)
}

// CgroupsCompatibleV1 skips tests unless:
// - cgroup.v1 mount point is detected
func CgroupsCompatibleV1(t *testing.T) {
	if !cgroupsCompatibleV1(t) {
		t.Skipf("Test requires cgroup.v1 support")
	}
}

func cgroupsCompatibleV1(t *testing.T) bool {
	// build tags mean this will never run outside of linux

	if cgroupsCompatibleV2(t) {
		t.Log("No cgroup.v1 mount point: running in cgroup.v2 mode")
		return false
	}
	mount, err := cgroups.GetCgroupMounts(false)
	if err != nil {
		t.Logf("Unable to detect cgroup.v1 mount point: %v", err)
		return false
	}
	if len(mount) == 0 {
		t.Logf("No cgroup.v1 mount point: empty path")
		return false
	}
	return true
}

// CgroupsCompatibleV2 skips tests unless:
// - cgroup.v2 unified mode is detected
func CgroupsCompatibleV2(t *testing.T) {
	if !cgroupsCompatibleV2(t) {
		t.Skip("Test requires cgroup.v2 support")
	}
}

func cgroupsCompatibleV2(t *testing.T) bool {
	return cgroups.IsCgroup2UnifiedMode()
}
