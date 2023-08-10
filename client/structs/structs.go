// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

//go:generate codecgen -c github.com/hashicorp/go-msgpack/codec -st codec -d 102 -t codegen_generated -o structs.generated.go structs.go

import (
	"errors"
	"time"

	"github.com/hashicorp/nomad/client/hoststats"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/device"
)

// RpcError is used for serializing errors with a potential error code
type RpcError struct {
	Message string
	Code    *int64
}

func NewRpcError(err error, code *int64) *RpcError {
	return &RpcError{
		Message: err.Error(),
		Code:    code,
	}
}

func (r *RpcError) Error() string {
	return r.Message
}

// ClientStatsResponse is used to return statistics about a node.
type ClientStatsResponse struct {
	HostStats *hoststats.HostStats
	structs.QueryMeta
}

// MonitorRequest is used to request and stream logs from a client node.
type MonitorRequest struct {
	// LogLevel is the log level filter we want to stream logs on
	LogLevel string

	// LogJSON specifies if log format should be unstructured or json
	LogJSON bool

	// NodeID is the node we want to track the logs of
	NodeID string

	// ServerID is the server we want to track the logs of
	ServerID string

	// PlainText disables base64 encoding.
	PlainText bool

	structs.QueryOptions
}

// AllocFileInfo holds information about a file inside the AllocDir
type AllocFileInfo struct {
	Name        string
	IsDir       bool
	Size        int64
	FileMode    string
	ModTime     time.Time
	ContentType string `json:",omitempty"`
}

// FsListRequest is used to list an allocation's directory.
type FsListRequest struct {
	// AllocID is the allocation to list from
	AllocID string

	// Path is the path to list
	Path string

	structs.QueryOptions
}

// FsListResponse is used to return the listings of an allocation's directory.
type FsListResponse struct {
	// Files are the result of listing a directory.
	Files []*AllocFileInfo

	structs.QueryMeta
}

// FsStatRequest is used to stat a file
type FsStatRequest struct {
	// AllocID is the allocation to stat the file in
	AllocID string

	// Path is the path to list
	Path string

	structs.QueryOptions
}

// FsStatResponse is used to return the stat results of a file
type FsStatResponse struct {
	// Info is the result of stating a file
	Info *AllocFileInfo

	structs.QueryMeta
}

// FsStreamRequest is the initial request for streaming the content of a file.
type FsStreamRequest struct {
	// AllocID is the allocation to stream logs from
	AllocID string

	// Path is the path to the file to stream
	Path string

	// Offset is the offset to start streaming data at.
	Offset int64

	// Origin can either be "start" or "end" and determines where the offset is
	// applied.
	Origin string

	// PlainText disables base64 encoding.
	PlainText bool

	// Limit is the number of bytes to read
	Limit int64

	// Follow follows the file.
	Follow bool

	structs.QueryOptions
}

// FsLogsRequest is the initial request for accessing allocation logs.
type FsLogsRequest struct {
	// AllocID is the allocation to stream logs from
	AllocID string

	// Task is the task to stream logs from
	Task string

	// LogType indicates whether "stderr" or "stdout" should be streamed
	LogType string

	// Offset is the offset to start streaming data at.
	Offset int64

	// Origin can either be "start" or "end" and determines where the offset is
	// applied.
	Origin string

	// PlainText disables base64 encoding.
	PlainText bool

	// Follow follows logs.
	Follow bool

	structs.QueryOptions
}

// StreamErrWrapper is used to serialize output of a stream of a file or logs.
type StreamErrWrapper struct {
	// Error stores any error that may have occurred.
	Error *RpcError

	// Payload is the payload
	Payload []byte
}

// AllocExecRequest is the initial request for execing into an Alloc task
type AllocExecRequest struct {
	// AllocID is the allocation to stream logs from
	AllocID string

	// Task is the task to stream logs from
	Task string

	// Tty indicates whether to allocate a pseudo-TTY
	Tty bool

	// Cmd is the command to be executed
	Cmd []string

	structs.QueryOptions
}

// AllocChecksRequest is used to request the latest nomad service discovery
// check status information of a given allocation.
type AllocChecksRequest struct {
	structs.QueryOptions
	AllocID string
}

// AllocChecksResponse is used to return the latest nomad service discovery
// check status information of a given allocation.
type AllocChecksResponse struct {
	structs.QueryMeta
	Results map[structs.CheckID]*structs.CheckQueryResult
}

// AllocStatsRequest is used to request the resource usage of a given
// allocation, potentially filtering by task
type AllocStatsRequest struct {
	// AllocID is the allocation to retrieves stats for
	AllocID string

	// Task is an optional filter to only request stats for the task.
	Task string

	structs.QueryOptions
}

// AllocStatsResponse is used to return the resource usage of a given
// allocation.
type AllocStatsResponse struct {
	Stats *AllocResourceUsage
	structs.QueryMeta
}

// MemoryStats holds memory usage related stats
type MemoryStats struct {
	RSS            uint64
	Cache          uint64
	Swap           uint64
	MappedFile     uint64
	Usage          uint64
	MaxUsage       uint64
	KernelUsage    uint64
	KernelMaxUsage uint64

	// A list of fields whose values were actually sampled
	Measured []string
}

func (ms *MemoryStats) Add(other *MemoryStats) {
	if other == nil {
		return
	}

	ms.RSS += other.RSS
	ms.Cache += other.Cache
	ms.Swap += other.Swap
	ms.MappedFile += other.MappedFile
	ms.Usage += other.Usage
	ms.MaxUsage += other.MaxUsage
	ms.KernelUsage += other.KernelUsage
	ms.KernelMaxUsage += other.KernelMaxUsage
	ms.Measured = joinStringSet(ms.Measured, other.Measured)
}

// CpuStats holds cpu usage related stats
type CpuStats struct {
	SystemMode       float64
	UserMode         float64
	TotalTicks       float64
	ThrottledPeriods uint64
	ThrottledTime    uint64
	Percent          float64

	// A list of fields whose values were actually sampled
	Measured []string
}

func (cs *CpuStats) Add(other *CpuStats) {
	if other == nil {
		return
	}

	cs.SystemMode += other.SystemMode
	cs.UserMode += other.UserMode
	cs.TotalTicks += other.TotalTicks
	cs.ThrottledPeriods += other.ThrottledPeriods
	cs.ThrottledTime += other.ThrottledTime
	cs.Percent += other.Percent
	cs.Measured = joinStringSet(cs.Measured, other.Measured)
}

// ResourceUsage holds information related to cpu and memory stats
type ResourceUsage struct {
	MemoryStats *MemoryStats
	CpuStats    *CpuStats
	DeviceStats []*device.DeviceGroupStats
}

func (ru *ResourceUsage) Add(other *ResourceUsage) {
	ru.MemoryStats.Add(other.MemoryStats)
	ru.CpuStats.Add(other.CpuStats)
	ru.DeviceStats = append(ru.DeviceStats, other.DeviceStats...)
}

// TaskResourceUsage holds aggregated resource usage of all processes in a Task
// and the resource usage of the individual pids
type TaskResourceUsage struct {
	ResourceUsage *ResourceUsage
	Timestamp     int64 // UnixNano
	Pids          map[string]*ResourceUsage
}

// AllocResourceUsage holds the aggregated task resource usage of the
// allocation.
type AllocResourceUsage struct {
	// ResourceUsage is the summation of the task resources
	ResourceUsage *ResourceUsage

	// Tasks contains the resource usage of each task
	Tasks map[string]*TaskResourceUsage

	// The max timestamp of all the Tasks
	Timestamp int64
}

// joinStringSet takes two slices of strings and joins them
func joinStringSet(s1, s2 []string) []string {
	lookup := make(map[string]struct{}, len(s1))
	j := make([]string, 0, len(s1))
	for _, s := range s1 {
		j = append(j, s)
		lookup[s] = struct{}{}
	}

	for _, s := range s2 {
		if _, ok := lookup[s]; !ok {
			j = append(j, s)
		}
	}

	return j
}

// HealthCheckRequest is the request type for a type that fulfils the Health
// Check interface
type HealthCheckRequest struct{}

// HealthCheckResponse is the response type for a type that fulfills the Health
// Check interface
type HealthCheckResponse struct {
	// Drivers is a map of driver names to current driver information
	Drivers map[string]*structs.DriverInfo
}

type HealthCheckIntervalRequest struct{}
type HealthCheckIntervalResponse struct {
	Eligible bool
	Period   time.Duration
}

// AddDriverInfo adds information about a driver to the fingerprint response.
// If the Drivers field has not yet been initialized, it does so here.
func (h *HealthCheckResponse) AddDriverInfo(name string, driverInfo *structs.DriverInfo) {
	// initialize Drivers if it has not been already
	if h.Drivers == nil {
		h.Drivers = make(map[string]*structs.DriverInfo)
	}

	h.Drivers[name] = driverInfo
}

// CheckBufSize is the size of the buffer that is used for job output
const CheckBufSize = 4 * 1024

// DriverStatsNotImplemented is the error to be returned if a driver doesn't
// implement stats.
var DriverStatsNotImplemented = errors.New("stats not implemented for driver")

// NodeRegistration stores data about the client's registration with the server
type NodeRegistration struct {
	HasRegistered bool
}
