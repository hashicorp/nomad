package drivers

import (
	cstructs "github.com/hashicorp/nomad/client/structs"
)

// This files present an indirection layer to client structs used by drivers,
// and represent the public interface for drivers, as client interfaces are
// internal and subject to change.

// MemoryStats holds memory usage related stats
type MemoryStats = cstructs.MemoryStats

// CpuStats holds cpu usage related stats
type CpuStats = cstructs.CpuStats

// ResourceUsage holds information related to cpu and memory stats
type ResourceUsage = cstructs.ResourceUsage

// TaskResourceUsage holds aggregated resource usage of all processes in a Task
// and the resource usage of the individual pids
type TaskResourceUsage = cstructs.TaskResourceUsage

// CheckBufSize is the size of the buffer that is used for job output
const CheckBufSize = cstructs.CheckBufSize

// DriverStatsNotImplemented is the error to be returned if a driver doesn't
// implement stats.
var DriverStatsNotImplemented = cstructs.DriverStatsNotImplemented
