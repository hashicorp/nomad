package drivers

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/drivers/proto"
	"github.com/hashicorp/nomad/plugins/shared/hclspec"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/msgpack"
)

const (
	// DriverHealthy is the default health description that should be used
	// if the driver is nominal
	DriverHealthy = "Healthy"

	// Pre09TaskHandleVersion is the version used to identify that the task
	// handle is from a driver that existed before driver plugins (v0.9). The
	// driver should take appropriate action to handle the old driver state.
	Pre09TaskHandleVersion = 0
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
	StartTask(*TaskConfig) (*TaskHandle, *DriverNetwork, error)
	WaitTask(ctx context.Context, taskID string) (<-chan *ExitResult, error)
	StopTask(taskID string, timeout time.Duration, signal string) error
	DestroyTask(taskID string, force bool) error
	InspectTask(taskID string) (*TaskStatus, error)
	TaskStats(ctx context.Context, taskID string, interval time.Duration) (<-chan *cstructs.TaskResourceUsage, error)
	TaskEvents(context.Context) (<-chan *TaskEvent, error)

	SignalTask(taskID string, signal string) error
	ExecTask(taskID string, cmd []string, timeout time.Duration) (*ExecTaskResult, error)
}

// ExecTaskStreamingDriver marks that a driver supports streaming exec task.  This represents a user friendly
// interface to implement, as an alternative to the ExecTaskStreamingRawDriver, the low level interface.
type ExecTaskStreamingDriver interface {
	ExecTaskStreaming(ctx context.Context, taskID string, execOptions *ExecOptions) (*ExitResult, error)
}

type ExecOptions struct {
	// Command is command to run
	Command []string

	// Tty indicates whether pseudo-terminal is to be allocated
	Tty bool

	// streams
	Stdin  io.ReadCloser
	Stdout io.WriteCloser
	Stderr io.WriteCloser

	// terminal size channel
	ResizeCh <-chan TerminalSize
}

// DriverNetworkManager is the interface with exposes function for creating a
// network namespace for which tasks can join. This only needs to be implemented
// if the driver MUST create the network namespace
type DriverNetworkManager interface {
	CreateNetwork(allocID string) (*NetworkIsolationSpec, bool, error)
	DestroyNetwork(allocID string, spec *NetworkIsolationSpec) error
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

func (_ DriverExecTaskNotSupported) ExecTask(taskID string, cmd []string, timeout time.Duration) (*ExecTaskResult, error) {
	return nil, fmt.Errorf("ExecTask is not supported by this driver")
}

type HealthState string

var (
	HealthStateUndetected = HealthState("undetected")
	HealthStateUnhealthy  = HealthState("unhealthy")
	HealthStateHealthy    = HealthState("healthy")
)

type Fingerprint struct {
	Attributes        map[string]*pstructs.Attribute
	Health            HealthState
	HealthDescription string

	// Err is set by the plugin if an error occurred during fingerprinting
	Err error
}

// FSIsolation is an enumeration to describe what kind of filesystem isolation
// a driver supports.
type FSIsolation string

var (
	// FSIsolationNone means no isolation. The host filesystem is used.
	FSIsolationNone = FSIsolation("none")

	// FSIsolationChroot means the driver will use a chroot on the host
	// filesystem.
	FSIsolationChroot = FSIsolation("chroot")

	// FSIsolationImage means the driver uses an image.
	FSIsolationImage = FSIsolation("image")
)

type Capabilities struct {
	// SendSignals marks the driver as being able to send signals
	SendSignals bool

	// Exec marks the driver as being able to execute arbitrary commands
	// such as health checks. Used by the ScriptExecutor interface.
	Exec bool

	//FSIsolation indicates what kind of filesystem isolation the driver supports.
	FSIsolation FSIsolation

	//NetIsolationModes lists the set of isolation modes supported by the driver
	NetIsolationModes []NetIsolationMode

	// MustInitiateNetwork tells Nomad that the driver must create the network
	// namespace and that the CreateNetwork and DestroyNetwork RPCs are implemented.
	MustInitiateNetwork bool

	// MountConfigs tells Nomad which mounting config options the driver supports.
	MountConfigs MountConfigSupport
}

func (c *Capabilities) HasNetIsolationMode(m NetIsolationMode) bool {
	for _, mode := range c.NetIsolationModes {
		if mode == m {
			return true
		}
	}
	return false
}

type NetIsolationMode string

var (
	// NetIsolationModeHost disables network isolation and uses the host network
	NetIsolationModeHost = NetIsolationMode("host")

	// NetIsolationModeGroup uses the group network namespace for isolation
	NetIsolationModeGroup = NetIsolationMode("group")

	// NetIsolationModeTask isolates the network to just the task
	NetIsolationModeTask = NetIsolationMode("task")

	// NetIsolationModeNone indicates that there is no network to isolate and is
	// intended to be used for tasks that the client manages remotely
	NetIsolationModeNone = NetIsolationMode("none")
)

type NetworkIsolationSpec struct {
	Mode   NetIsolationMode
	Path   string
	Labels map[string]string
}

// MountConfigSupport is an enum that defaults to "all" for backwards
// compatibility with community drivers.
type MountConfigSupport int32

const (
	MountConfigSupportAll MountConfigSupport = iota
	MountConfigSupportNone
)

type TerminalSize struct {
	Height int
	Width  int
}

type DNSConfig struct {
	Servers  []string
	Searches []string
	Options  []string
}

func (c *DNSConfig) Copy() *DNSConfig {
	if c == nil {
		return nil
	}

	cfg := new(DNSConfig)
	if len(c.Servers) > 0 {
		cfg.Servers = make([]string, len(c.Servers))
		copy(cfg.Servers, c.Servers)
	}
	if len(c.Searches) > 0 {
		cfg.Searches = make([]string, len(c.Searches))
		copy(cfg.Searches, c.Searches)
	}
	if len(c.Options) > 0 {
		cfg.Options = make([]string, len(c.Options))
		copy(cfg.Options, c.Options)
	}

	return cfg
}

type TaskConfig struct {
	ID               string
	JobName          string
	TaskGroupName    string
	Name             string
	Env              map[string]string
	DeviceEnv        map[string]string
	Resources        *Resources
	Devices          []*DeviceConfig
	Mounts           []*MountConfig
	User             string
	AllocDir         string
	rawDriverConfig  []byte
	StdoutPath       string
	StderrPath       string
	AllocID          string
	NetworkIsolation *NetworkIsolationSpec
	DNS              *DNSConfig
}

func (tc *TaskConfig) Copy() *TaskConfig {
	if tc == nil {
		return nil
	}
	c := new(TaskConfig)
	*c = *tc
	c.Env = helper.CopyMapStringString(c.Env)
	c.DeviceEnv = helper.CopyMapStringString(c.DeviceEnv)
	c.Resources = tc.Resources.Copy()
	c.DNS = tc.DNS.Copy()

	if c.Devices != nil {
		dc := make([]*DeviceConfig, len(c.Devices))
		for i, c := range c.Devices {
			dc[i] = c.Copy()
		}
		c.Devices = dc
	}

	if c.Mounts != nil {
		mc := make([]*MountConfig, len(c.Mounts))
		for i, m := range c.Mounts {
			mc[i] = m.Copy()
		}
		c.Mounts = mc
	}

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

func (tc *TaskConfig) EncodeConcreteDriverConfig(t interface{}) error {
	data := []byte{}
	err := base.MsgPackEncode(&data, t)
	if err != nil {
		return err
	}

	tc.rawDriverConfig = data
	return nil
}

type Resources struct {
	NomadResources *structs.AllocatedTaskResources
	LinuxResources *LinuxResources
	Ports          *structs.AllocatedPorts
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

	if r.Ports != nil {
		ports := structs.AllocatedPorts(append(make([]structs.AllocatedPortMapping, 0, len(*r.Ports)), *r.Ports...))
		res.Ports = &ports
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

	// PrecentTicks is used to calculate the CPUQuota, currently the docker
	// driver exposes cpu period and quota through the driver configuration
	// and thus the calculation for CPUQuota cannot be done on the client.
	// This is a capatability and should only be used by docker until the docker
	// specific options are deprecated in favor of exposes CPUPeriod and
	// CPUQuota at the task resource stanza.
	PercentTicks float64
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

func (d *DeviceConfig) Copy() *DeviceConfig {
	if d == nil {
		return nil
	}

	dc := new(DeviceConfig)
	*dc = *d
	return dc
}

type MountConfig struct {
	TaskPath        string
	HostPath        string
	Readonly        bool
	PropagationMode string
}

func (m *MountConfig) IsEqual(o *MountConfig) bool {
	return m.TaskPath == o.TaskPath &&
		m.HostPath == o.HostPath &&
		m.Readonly == o.Readonly &&
		m.PropagationMode == o.PropagationMode
}

func (m *MountConfig) Copy() *MountConfig {
	if m == nil {
		return nil
	}

	mc := new(MountConfig)
	*mc = *m
	return mc
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

func (r *ExitResult) Copy() *ExitResult {
	if r == nil {
		return nil
	}
	res := new(ExitResult)
	*res = *r
	return res
}

type TaskStatus struct {
	ID               string
	Name             string
	State            TaskState
	StartedAt        time.Time
	CompletedAt      time.Time
	ExitResult       *ExitResult
	DriverAttributes map[string]string
	NetworkOverride  *DriverNetwork
}

type TaskEvent struct {
	TaskID      string
	TaskName    string
	AllocID     string
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

//// helper types for operating on raw exec operation
// we alias proto instances as much as possible to avoid conversion overhead

// ExecTaskStreamingRawDriver represents a low-level interface for executing a streaming exec
// call, and is intended to be used when driver instance is to delegate exec handling to another
// backend, e.g. to a executor or a driver behind a grpc/rpc protocol
//
// Nomad client would prefer this interface method over `ExecTaskStreaming` if driver implements it.
type ExecTaskStreamingRawDriver interface {
	ExecTaskStreamingRaw(
		ctx context.Context,
		taskID string,
		command []string,
		tty bool,
		stream ExecTaskStream) error
}

// ExecTaskStream represents a stream of exec streaming messages,
// and is a handle to get stdin and tty size and send back
// stdout/stderr and exit operations.
//
// The methods are not concurrent safe; callers must ensure that methods are called
// from at most one goroutine.
type ExecTaskStream interface {
	// Send relays response message back to API.
	//
	// The call is synchronous and no references to message is held: once
	// method call completes, the message reference can be reused or freed.
	Send(*ExecTaskStreamingResponseMsg) error

	// Receive exec streaming messages from API.  Returns `io.EOF` on completion of stream.
	Recv() (*ExecTaskStreamingRequestMsg, error)
}

type ExecTaskStreamingRequestMsg = proto.ExecTaskStreamingRequest
type ExecTaskStreamingResponseMsg = proto.ExecTaskStreamingResponse

// InternalCapabilitiesDriver is an experimental interface enabling a driver
// to disable some nomad functionality (e.g. logs or metrics).
//
// Intended for internal drivers only while the interface is stabalized.
type InternalCapabilitiesDriver interface {
	InternalCapabilities() InternalCapabilities
}

// InternalCapabilities flags disabled functionality.
// Zero value means all is supported.
type InternalCapabilities struct {
	DisableLogCollection     bool
	DisableMetricsCollection bool
}
