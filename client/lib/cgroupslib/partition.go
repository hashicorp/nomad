// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cgroupslib

import (
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

// A Partition is used to track reserved vs. shared cpu cores.
type Partition interface {
	Restore(*idset.Set[hw.CoreID])
	Reserve(*idset.Set[hw.CoreID]) error
	Release(*idset.Set[hw.CoreID]) error
}

// SharePartition is the name of the cgroup containing cgroups for tasks
// making use of the 'cpu' resource, which pools and shares cpu cores.
func SharePartition() string {
	switch GetMode() {
	case CG1:
		return "share"
	case CG2:
		return "share.slice"
	default:
		return ""
	}
}

// ReservePartition is the name of the cgroup containing cgroups for tasks
// making use of the 'cores' resource, which reserves specific cpu cores.
func ReservePartition() string {
	switch GetMode() {
	case CG1:
		return "reserve"
	case CG2:
		return "reserve.slice"
	default:
		return ""
	}
}

// GetPartitionFromCores returns the name of the cgroup that should contain
// the cgroup of the task, which is determined by inspecting the cores
// parameter, which is a non-empty string if the task is using reserved
// cores.
func GetPartitionFromCores(cores string) string {
	if cores == "" {
		return SharePartition()
	}
	return ReservePartition()
}

// GetPartitionFromBool returns the name of the cgroup that should contain
// the cgroup of the task, which is determined from the cores parameter,
// which indicates whether the task is making use of reserved cores.
func GetPartitionFromBool(cores bool) string {
	if cores {
		return ReservePartition()
	}
	return SharePartition()
}
