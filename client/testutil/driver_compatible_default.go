// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package testutil

import (
	"testing"
)

// CgroupsCompatible returns false on non-Linux operating systems.
func CgroupsCompatible(t *testing.T) {
	t.Skipf("Test requires cgroups support on Linux")
}

// CgroupsCompatibleV1 skips tests on non-Linux operating systems.
func CgroupsCompatibleV1(t *testing.T) {
	t.Skipf("Test requires cgroup.v1 support on Linux")
}

// CgroupsCompatibleV2 skips tests on non-Linux operating systems.
func CgroupsCompatibleV2(t *testing.T) {
	t.Skipf("Test requires cgroup.v2 support on Linux")
}
