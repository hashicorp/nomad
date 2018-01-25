package structs

//go:generate codecgen -d 102 -o structs.generated.go structs.go

import (
	"crypto/md5"
	"io"
	"strconv"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/stats"
	"github.com/hashicorp/nomad/nomad/structs"
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
	HostStats *stats.HostStats
	structs.QueryMeta
}

// AllocFileInfo holds information about a file inside the AllocDir
type AllocFileInfo struct {
	Name     string
	IsDir    bool
	Size     int64
	FileMode string
	ModTime  time.Time
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
	MaxUsage       uint64
	KernelUsage    uint64
	KernelMaxUsage uint64

	// A list of fields whose values were actually sampled
	Measured []string
}

func (ms *MemoryStats) Add(other *MemoryStats) {
	ms.RSS += other.RSS
	ms.Cache += other.Cache
	ms.Swap += other.Swap
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
}

func (ru *ResourceUsage) Add(other *ResourceUsage) {
	ru.MemoryStats.Add(other.MemoryStats)
	ru.CpuStats.Add(other.CpuStats)
}

// TaskResourceUsage holds aggregated resource usage of all processes in a Task
// and the resource usage of the individual pids
type TaskResourceUsage struct {
	ResourceUsage *ResourceUsage
	Timestamp     int64
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

// FSIsolation is an enumeration to describe what kind of filesystem isolation
// a driver supports.
type FSIsolation int

const (
	// FSIsolationNone means no isolation. The host filesystem is used.
	FSIsolationNone FSIsolation = 0

	// FSIsolationChroot means the driver will use a chroot on the host
	// filesystem.
	FSIsolationChroot FSIsolation = 1

	// FSIsolationImage means the driver uses an image.
	FSIsolationImage FSIsolation = 2
)

func (f FSIsolation) String() string {
	switch f {
	case 0:
		return "none"
	case 1:
		return "chroot"
	case 2:
		return "image"
	default:
		return "INVALID"
	}
}

// DriverNetwork is the network created by driver's (eg Docker's bridge
// network) during Prestart.
type DriverNetwork struct {
	// PortMap can be set by drivers to replace ports in environment
	// variables with driver-specific mappings.
	PortMap map[string]int

	// IP is the IP address for the task created by the driver.
	IP string

	// AutoAdvertise indicates whether the driver thinks services that
	// choose to auto-advertise-addresses should use this IP instead of the
	// host's. eg If a Docker network plugin is used
	AutoAdvertise bool
}

// Advertise returns true if the driver suggests using the IP set. May be
// called on a nil Network in which case it returns false.
func (d *DriverNetwork) Advertise() bool {
	return d != nil && d.AutoAdvertise
}

// Copy a DriverNetwork struct. If it is nil, nil is returned.
func (d *DriverNetwork) Copy() *DriverNetwork {
	if d == nil {
		return nil
	}
	pm := make(map[string]int, len(d.PortMap))
	for k, v := range d.PortMap {
		pm[k] = v
	}
	return &DriverNetwork{
		PortMap:       pm,
		IP:            d.IP,
		AutoAdvertise: d.AutoAdvertise,
	}
}

// Hash the contents of a DriverNetwork struct to detect changes. If it is nil,
// an empty slice is returned.
func (d *DriverNetwork) Hash() []byte {
	if d == nil {
		return []byte{}
	}
	h := md5.New()
	io.WriteString(h, d.IP)
	io.WriteString(h, strconv.FormatBool(d.AutoAdvertise))
	for k, v := range d.PortMap {
		io.WriteString(h, k)
		io.WriteString(h, strconv.Itoa(v))
	}
	return h.Sum(nil)
}

// FingerprintRequest is a request which a fingerprinter accepts to fingerprint
// the node
type FingerprintRequest struct {
	Config *config.Config
	Node   *structs.Node
}

// FingerprintResponse is the response which a fingerprinter annotates with the
// results of the fingerprint method
type FingerprintResponse struct {
	Attributes map[string]string
	Links      map[string]string
	Resources  *structs.Resources

	// Detected is a boolean indicating whether the fingerprinter detected
	// if the resource was available
	Detected bool

	// Drivers is a map of driver names to driver info. This allows the
	// fingerprint method of each driver to set whether the driver is enabled or
	// not, as well as its attributes
	Drivers map[string]*structs.DriverInfo
}

func (f *FingerprintResponse) AddDriver(name string, value *structs.DriverInfo) {
	if f.Drivers == nil {
		f.Drivers = make(map[string]*structs.DriverInfo, 0)
	}

	f.Drivers[name] = value
}

// AddAttribute adds the name and value for a node attribute to the fingerprint
// response
func (f *FingerprintResponse) AddAttribute(name, value string) {
	// initialize Attributes if it has not been already
	if f.Attributes == nil {
		f.Attributes = make(map[string]string, 0)
	}

	f.Attributes[name] = value
}

// RemoveAttribute sets the given attribute to empty, which will later remove
// it entirely from the node
func (f *FingerprintResponse) RemoveAttribute(name string) {
	// initialize Attributes if it has not been already
	if f.Attributes == nil {
		f.Attributes = make(map[string]string, 0)
	}

	f.Attributes[name] = ""
}

// AddLink adds a link entry to the fingerprint response
func (f *FingerprintResponse) AddLink(name, value string) {
	// initialize Links if it has not been already
	if f.Links == nil {
		f.Links = make(map[string]string, 0)
	}

	f.Links[name] = value
}

// RemoveLink removes a link entry from the fingerprint response. This will
// later remove it entirely from the node
func (f *FingerprintResponse) RemoveLink(name string) {
	// initialize Links if it has not been already
	if f.Links == nil {
		f.Links = make(map[string]string, 0)
	}

	f.Links[name] = ""
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
		h.Drivers = make(map[string]*structs.DriverInfo, 0)
	}

	h.Drivers[name] = driverInfo
}
