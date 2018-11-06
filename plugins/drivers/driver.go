package drivers

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/msgpack"
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
	StartTask(*TaskConfig) (*TaskHandle, *cstructs.DriverNetwork, error)
	WaitTask(ctx context.Context, taskID string) (<-chan *ExitResult, error)
	StopTask(taskID string, timeout time.Duration, signal string) error
	DestroyTask(taskID string, force bool) error
	InspectTask(taskID string) (*TaskStatus, error)
	TaskStats(taskID string) (*cstructs.TaskResourceUsage, error)
	TaskEvents(context.Context) (<-chan *TaskEvent, error)

	SignalTask(taskID string, signal string) error
	ExecTask(taskID string, cmd []string, timeout time.Duration) (*ExecTaskResult, error)
}

// DriverSignalTaskNotSupported can be embedded by drivers which don't support
// the SignalTask RPC. This satisfies the SignalTask func requirement for the
// DriverPlugin interface.
type DriverSignalTaskNotSupported struct{}

func (DriverSignalTaskNotSupported) SignalTask(taskID, signal string) error {
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

	// Err is set by the plugin if an error occurred during fingerprinting
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
	FSIsolation cstructs.FSIsolation
}

type TaskConfig struct {
	ID              string
	Name            string
	Env             map[string]string
	Resources       *Resources
	Devices         []DeviceConfig
	Mounts          []MountConfig
	User            string
	AllocDir        string
	rawDriverConfig []byte
	StdoutPath      string
	StderrPath      string
}

func (tc *TaskConfig) Copy() *TaskConfig {
	if tc == nil {
		return nil
	}
	c := new(TaskConfig)
	*c = *tc
	c.Env = helper.CopyMapStringString(c.Env)
	c.Resources = tc.Resources.Copy()
	return c
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

func (tc *TaskConfig) EncodeDriverConfig(val cty.Value) error {
	data, err := msgpack.Marshal(val, val.Type())
	if err != nil {
		return err
	}

	tc.rawDriverConfig = data
	return nil
}

type Resources struct {
	NomadResources *structs.Resources
	LinuxResources *LinuxResources
}

func (r *Resources) Copy() *Resources {
	if r == nil {
		return nil
	}
	res := new(Resources)
	if r.NomadResources != nil {
		res.NomadResources = r.NomadResources.Copy()
	}
	if r.LinuxResources != nil {
		res.LinuxResources = r.LinuxResources.Copy()
	}
	return res
}

type LinuxResources struct {
	CPUPeriod        int64
	CPUQuota         int64
	CPUShares        int64
	MemoryLimitBytes int64
	OOMScoreAdj      int64
	CpusetCPUs       string
	CpusetMems       string
}

func (r *LinuxResources) Copy() *LinuxResources {
	res := new(LinuxResources)
	*res = *r
	return res
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

type ExitResult struct {
	ExitCode  int
	Signal    int
	OOMKilled bool
	Err       error
}

func (r *ExitResult) Successful() bool {
	return r.ExitCode == 0 && r.Signal == 0 && r.Err == nil
}

type TaskStatus struct {
	ID               string
	Name             string
	State            TaskState
	StartedAt        time.Time
	CompletedAt      time.Time
	ExitResult       *ExitResult
	DriverAttributes map[string]string
	NetworkOverride  *cstructs.DriverNetwork
}

type TaskEvent struct {
	TaskID      string
	Timestamp   time.Time
	Message     string
	Annotations map[string]string

	// Err is only used if an error occurred while consuming the RPC stream
	Err error
}

type ExecTaskResult struct {
	Stdout     []byte
	Stderr     []byte
	ExitResult *ExitResult
}
