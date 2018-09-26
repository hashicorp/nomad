package drivers

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"golang.org/x/net/context"
)

// DriverPlugin is the interface with drivers will implement. It is also
// implemented by a plugin client which proxies the calls to go-plugin. See
// the proto/driver.proto file for detailed information about each RPC and
// message structure.
type DriverPlugin interface {
	base.BasePlugin

	TaskConfigSchema() (*hclspec.Spec, error)
	Capabilities() (*Capabilities, error)
	Fingerprint(context.Context) (<-chan *Fingerprint, error)

	RecoverTask(*TaskHandle) error
	StartTask(*TaskConfig) (*TaskHandle, error)
	WaitTask(ctx context.Context, taskID string) (<-chan *ExitResult, error)
	StopTask(taskID string, timeout time.Duration, signal string) error
	DestroyTask(taskID string, force bool) error
	InspectTask(taskID string) (*TaskStatus, error)
	TaskStats(taskID string) (*TaskStats, error)
	TaskEvents(context.Context) (<-chan *TaskEvent, error)

	SignalTask(taskID string, signal string) error
	ExecTask(taskID string, cmd []string, timeout time.Duration) (*ExecTaskResult, error)
}

// DriverSignalTaskNotSupported can be embedded by drivers which don't support
// the SignalTask RPC. This satisfies the SignalTask func requirement for the
// DriverPlugin interface.
type DriverSignalTaskNotSupported struct{}

func (_ DriverSignalTaskNotSupported) SignalTask(taskID, signal string) error {
	return fmt.Errorf("SignalTask is not supported by this driver")
}

// DriverExecTaskNotSupported can be embedded by drivers which don't support
// the ExecTask RPC. This satisfies the ExecTask func requirement of the
// DriverPlugin interface.
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

	// Err is set by the plugin if an error occured during fingerprinting
	Err error
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
	ID              string
	Name            string
	Env             map[string]string
	Resources       Resources
	Devices         []DeviceConfig
	Mounts          []MountConfig
	User            string
	AllocDir        string
	rawDriverConfig []byte
}

func (tc *TaskConfig) EnvList() []string {
	l := make([]string, 0, len(tc.Env))
	for k, v := range tc.Env {
		l = append(l, k+"="+v)
	}

	sort.Strings(l)
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

func (tc *TaskConfig) DecodeDriverConfig(t interface{}) error {
	return base.MsgPackDecode(tc.rawDriverConfig, t)
}

func (tc *TaskConfig) EncodeDriverConfig(t interface{}) error {
	return base.MsgPackEncode(&tc.rawDriverConfig, t)
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

type TaskStatus struct {
	ID               string
	Name             string
	State            TaskState
	StartedAt        time.Time
	CompletedAt      time.Time
	ExitResult       *ExitResult
	DriverAttributes map[string]string
	NetworkOverride  *NetworkOverride
}

type TaskStats struct {
	ID                 string
	Timestamp          int64
	AggResourceUsage   *cstructs.ResourceUsage
	ResourceUsageByPid map[string]*cstructs.ResourceUsage
}

type TaskEvent struct {
	TaskID      string
	Timestamp   time.Time
	Message     string
	Annotations map[string]string

	// Err is only used if an error occured while consuming the RPC stream
	Err error
}

type ExecTaskResult struct {
	Stdout     []byte
	Stderr     []byte
	ExitResult *ExitResult
}
