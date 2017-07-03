package structs

import (
	"crypto/md5"
	"io"
	"strconv"
)

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
