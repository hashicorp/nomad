package base

import (
	"time"

	"github.com/hashicorp/nomad/client/structs"
)

type Driver interface {
	Fingerprint() *Fingerprint
	RecoverTask(*TaskHandle) error
	StartTask(*TaskConfig) (*TaskHandle, error)
	WaitTask(taskID string) chan *TaskResult
	StopTask(taskID string, timeout time.Duration, signal string) error
	DestroyTask(taskID string)
	ListTasks(*ListTasksQuery) ([]*TaskSummary, error)
	InspectTask(taskID string) (*TaskStatus, error)
	TaskStats(taskID string) (*TaskStats, error)
}

type Fingerprint struct {
	Capabilities Capabilities
	Attributes   map[string]string
	Detected     bool
}

type Capabilities struct {
	// SendSignals marks the driver as being able to send signals
	SendSignals bool

	// Exec marks the driver as being able to execute arbitrary commands
	// such as health checks. Used by the ScriptExecutor interface.
	Exec bool

	FSIsolation structs.FSIsolation
}

type TaskConfig struct {
	ID           string
	User         string
	DriverConfig map[string]interface{}
	Env          map[string]string
	Resources    Resources
	Devices      []DeviceConfig
	Mounts       []MountConfig
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
	TaskStatePending TaskState = "pending"
	TaskStateRunning TaskState = "running"
	TaskStateDead    TaskState = "dead"
)

type TaskState string

type TaskResult struct {
	ExitCode int
	Signal   int
	Err      error
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
	ID     string
	CPU    TaskCPUUsage
	Memory TaskMemoryUsage
}

type TaskCPUUsage struct {
	Timestamp            time.Time
	UsageCoreNanoseconds uint64
}

type TaskMemoryUsage struct {
	Timestamp       time.Time
	WorkingSetBytes uint64
}
