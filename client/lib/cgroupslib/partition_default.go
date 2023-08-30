// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package cgroupslib

// GetPartition creates a no-op Partition that does not do anything.
func GetPartition(*idset.Set[idset.CoreID]) Partition {
	return NoopPartition()
}
