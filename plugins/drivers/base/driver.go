package base

import (
	"path/filepath"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
)

const DriverGoPlugin = "driver"

type Driver interface {
	Fingerprint() *Fingerprint
	Capabilities() *Capabilities
	RecoverTask(*TaskHandle) error
	StartTask(*TaskConfig) (*TaskHandle, error)
	WaitTask(taskID string) chan *TaskResult
	StopTask(taskID string, timeout time.Duration, signal string) error
	DestroyTask(taskID string)
	ListTasks(*ListTasksQuery) ([]*TaskSummary, error)
	InspectTask(taskID string) (*TaskStatus, error)
	TaskStats(taskID string) (*TaskStats, error)
}

type HealthState string

var (
	HealthStateUndetected = HealthState("undetected")
	HealthStateUnhealthy  = HealthState("unhealthy")
	HealthStateHealthy    = HealthState("healthy")
)

type Fingerprint struct {
	Attributes        map[string]string
	Health            HealthState
	HealthDescription string
}

type FSIsolation string

var (
	FSIsolationNone   = FSIsolation("none")
	FSIsolationChroot = FSIsolation("chroot")
	FSIsolationImage  = FSIsolation("image")
)

type Capabilities struct {
	// SendSignals marks the driver as being able to send signals
	SendSignals bool

	// Exec marks the driver as being able to execute arbitrary commands
	// such as health checks. Used by the ScriptExecutor interface.
	Exec bool

	//FSIsolation indicates what kind of filesystem isolation the driver supports.
	FSIsolation FSIsolation
}

type TaskConfig struct {
	ID           string
	Name         string
	DriverConfig map[string]interface{}
	Env          map[string]string
	Resources    Resources
	Devices      []DeviceConfig
	Mounts       []MountConfig
	User         string
	AllocDir     string
}

func (tc *TaskConfig) EnvList() []string {
	l := make([]string, len(tc.Env))
	for k, v := range tc.Env {
		l = append(l, k+"="+v)
	}
	return l
}

func (tc *TaskConfig) TaskDir() *allocdir.TaskDir {
	taskDir := filepath.Join(tc.AllocDir, tc.Name)
	return &allocdir.TaskDir{
		Dir:            taskDir,
		SharedAllocDir: filepath.Join(tc.AllocDir, allocdir.SharedAllocName),
		LogDir:         filepath.Join(tc.AllocDir, allocdir.SharedAllocName, allocdir.LogDirName),
		SharedTaskDir:  filepath.Join(taskDir, allocdir.SharedAllocName),
		LocalDir:       filepath.Join(taskDir, allocdir.TaskLocal),
		SecretsDir:     filepath.Join(taskDir, allocdir.TaskSecrets),
	}
}

type Resources struct {
	CPUPeriod        int64
	CPUQuota         int64
	CPUShares        int64
	MemoryLimitBytes int64
	OOMScoreAdj      int64
	CpusetCPUs       string
	CpusetMems       string
}

type DeviceConfig struct {
	TaskPath    string
	HostPath    string
	Permissions string
}

type MountConfig struct {
	TaskPath string
	HostPath string
	Readonly bool
}

const (
	TaskStateUnknown TaskState = "unknown"
	TaskStateRunning TaskState = "running"
	TaskStateExited  TaskState = "exited"
)

type TaskState string

type TaskResult struct {
	ExitCode  int
	Signal    int
	OOMKilled bool
	Err       error
}

type ListTasksQuery struct {
	State string
}

type TaskSummary struct {
	ID        string
	Name      string
	State     string
	CreatedAt time.Time
}

type TaskStatus struct {
	ID         string
	Name       string
	State      string
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	ExitCode   int
}

type TaskStats struct {
	ID                 string
	Timestamp          int64
	AggResourceUsage   ResourceUsage
	ResourceUsageByPid map[string]*ResourceUsage
}

type ResourceUsage struct {
	CPU    CPUUsage
	Memory MemoryUsage
}
type CPUUsage struct {
	SystemMode       float64
	UserMode         float64
	TotalTicks       float64
	ThrottledPeriods uint64
	ThrottledTime    uint64
	Percent          float64

	// A list of fields whose values were actually sampled
	Measured []string
}

type MemoryUsage struct {
	RSS            uint64
	Cache          uint64
	Swap           uint64
	MaxUsage       uint64
	KernelUsage    uint64
	KernelMaxUsage uint64

	// A list of fields whose values were actually sampled
	Measured []string
}
