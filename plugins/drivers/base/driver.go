package base

import (
	"time"
)

const (
	TaskEventCreated TaskEventType = "created"
	TaskEventStarted TaskEventType = "started"
)

type TaskRecorder interface {
	Record(taskID string, event TaskEvent) error
}

type TaskEventType string

func TaskEventTypeFromString(t string) TaskEventType {
	switch TaskEventType(t) {
	case TaskEventCreated:
		return TaskEventCreated
	case TaskEventStarted:
		return TaskEventStarted
	}
	return ""
}

type TaskEvent struct {
	Type        TaskEventType
	Timestamp   time.Time
	Driver      string
	Description string
	Attrs       map[string]string
}

type Driver interface {
	//Fingerprint()
	RecoverTask(taskID string, events []TaskEvent) error
	CreateTask(TaskConfig) (taskID string, err error)
	StartTask(taskID string) error
	StopTask(taskID string, timeout time.Duration, signal string) error
	DestroyTask(taskID string)
	ListTasks(ListTasksQuery) ([]TaskSummary, error)
	TaskStatus(taskID string) (TaskStatus, error)
	TaskStats(taskID string) (TaskStats, error)
}

type TaskConfig struct {
	Name         string
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
