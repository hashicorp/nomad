package base

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"golang.org/x/net/context"
)

const DriverGoPlugin = "driver"

type DriverPlugin interface {
	base.BasePlugin

	TaskConfigSchema() (*hclspec.Spec, error)
	Capabilities() (*Capabilities, error)
	Fingerprint() (chan *Fingerprint, error)

	RecoverTask(*TaskHandle) error
	StartTask(*TaskConfig) (*TaskHandle, error)
	WaitTask(ctx context.Context, taskID string) chan *ExitResult
	StopTask(taskID string, timeout time.Duration, signal string) error
	DestroyTask(taskID string, force bool) error
	InspectTask(taskID string) (*TaskStatus, error)
	TaskStats(taskID string) (*TaskStats, error)
	TaskEvents() (chan *TaskEvent, error)

	SignalTask(taskID string, signal string) error
	ExecTask(taskID string, cmd []string, timeout time.Duration) (*ExecTaskResult, error)
}

type DriverSignalTaskNotSupported struct{}

func (_ DriverSignalTaskNotSupported) SignalTask(taskID, signal string) error {
	return fmt.Errorf("SignalTask is not supported by this driver")
}

type DriverExecTaskNotSupported struct{}

func (_ DriverExecTaskNotSupported) ExecTask(taskID, signal string) error {
	return fmt.Errorf("ExecTask is not supported by this driver")
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

type NetworkOverride struct {
	PortMap       map[string]int32
	Addr          string
	AutoAdvertise bool
}

type ExitResult struct {
	ExitCode  int
	Signal    int
	OOMKilled bool
	Err       error
}

type TaskSummary struct {
	ID        string
	Name      string
	State     string
	CreatedAt time.Time
}

type TaskStatus struct {
	ID               string
	Name             string
	State            string
	SizeOnDiskMB     int64
	StartedAt        time.Time
	CompletedAt      time.Time
	ExitResult       *ExitResult
	DriverAttributes map[string]string
	NetworkOverride  *NetworkOverride
}

type TaskStats struct {
	ID                 string
	Timestamp          int64
	AggResourceUsage   *ResourceUsage
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

type TaskEvent struct {
	TaskID      string
	Timestamp   time.Time
	Message     string
	Annotations map[string]string
}

type ExecTaskResult struct {
	Stdout     []byte
	Stderr     []byte
	ExitResult *ExitResult
}
