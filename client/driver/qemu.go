package driver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/fields"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

var (
	reQemuVersion = regexp.MustCompile(`version (\d[\.\d+]+)`)

	// Prior to qemu 2.10.1, monitor socket paths are truncated to 108 bytes.
	// We should consider this if driver.qemu.version is < 2.10.1 and the
	// generated monitor path is too long.

	//
	// Relevant fix is here:
	// https://github.com/qemu/qemu/commit/ad9579aaa16d5b385922d49edac2c96c79bcfb6
	qemuVersionLongSocketPathFix = semver.New("2.10.1")
)

const (
	// The key populated in Node Attributes to indicate presence of the Qemu driver
	qemuDriverAttr        = "driver.qemu"
	qemuDriverVersionAttr = "driver.qemu.version"
	// Represents an ACPI shutdown request to the VM (emulates pressing a physical power button)
	// Reference: https://en.wikibooks.org/wiki/QEMU/Monitor
	qemuGracefulShutdownMsg = "system_powerdown\n"
	qemuMonitorSocketName   = "qemu-monitor.sock"
	// Maximum socket path length prior to qemu 2.10.1
	qemuLegacyMaxMonitorPathLen = 108
)

// QemuDriver is a driver for running images via Qemu
// We attempt to chose sane defaults for now, with more configuration available
// planned in the future
type QemuDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter

	driverConfig *QemuDriverConfig
}

type QemuDriverConfig struct {
	ImagePath        string           `mapstructure:"image_path"`
	Accelerator      string           `mapstructure:"accelerator"`
	GracefulShutdown bool             `mapstructure:"graceful_shutdown"`
	PortMap          []map[string]int `mapstructure:"port_map"` // A map of host port labels and to guest ports.
	Args             []string         `mapstructure:"args"`     // extra arguments to qemu executable
}

// qemuHandle is returned from Start/Open as a handle to the PID
type qemuHandle struct {
	pluginClient   *plugin.Client
	userPid        int
	executor       executor.Executor
	monitorPath    string
	killTimeout    time.Duration
	maxKillTimeout time.Duration
	logger         *log.Logger
	version        string
	waitCh         chan *dstructs.WaitResult
	doneCh         chan struct{}
}

// getMonitorPath is used to determine whether a qemu monitor socket can be
// safely created and accessed in the task directory by the version of qemu
// present on the host. If it is safe to use, the socket's full path is
// returned along with a nil error. Otherwise, an empty string is returned
// along with a descriptive error.
func (d *QemuDriver) getMonitorPath(dir string) (string, error) {
	var longPathSupport bool
	currentQemuVer := d.DriverContext.node.Attributes[qemuDriverVersionAttr]
	currentQemuSemver := semver.New(currentQemuVer)
	if currentQemuSemver.LessThan(*qemuVersionLongSocketPathFix) {
		longPathSupport = false
		d.logger.Printf("[DEBUG] driver.qemu: long socket paths are not available in this version of QEMU (%s)", currentQemuVer)
	} else {
		longPathSupport = true
		d.logger.Printf("[DEBUG] driver.qemu: long socket paths available in this version of QEMU (%s)", currentQemuVer)
	}
	fullSocketPath := fmt.Sprintf("%s/%s", dir, qemuMonitorSocketName)
	if len(fullSocketPath) > qemuLegacyMaxMonitorPathLen && longPathSupport == false {
		return "", fmt.Errorf("monitor path is too long for this version of qemu")
	}
	return fullSocketPath, nil
}

// NewQemuDriver is used to create a new exec driver
func NewQemuDriver(ctx *DriverContext) Driver {
	return &QemuDriver{DriverContext: *ctx}
}

// Validate is used to validate the driver configuration
func (d *QemuDriver) Validate(config map[string]interface{}) error {
	fd := &fields.FieldData{
		Raw: config,
		Schema: map[string]*fields.FieldSchema{
			"image_path": {
				Type:     fields.TypeString,
				Required: true,
			},
			"accelerator": {
				Type: fields.TypeString,
			},
			"graceful_shutdown": {
				Type:     fields.TypeBool,
				Required: false,
			},
			"port_map": {
				Type: fields.TypeArray,
			},
			"args": {
				Type: fields.TypeArray,
			},
		},
	}

	if err := fd.Validate(); err != nil {
		return err
	}

	return nil
}

func (d *QemuDriver) Abilities() DriverAbilities {
	return DriverAbilities{
		SendSignals: false,
		Exec:        false,
	}
}

func (d *QemuDriver) FSIsolation() cstructs.FSIsolation {
	return cstructs.FSIsolationImage
}

func (d *QemuDriver) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	bin := "qemu-system-x86_64"
	if runtime.GOOS == "windows" {
		// On windows, the "qemu-system-x86_64" command does not respond to the
		// version flag.
		bin = "qemu-img"
	}
	outBytes, err := exec.Command(bin, "--version").Output()
	if err != nil {
		// return no error, as it isn't an error to not find qemu, it just means we
		// can't use it.
		return nil
	}
	out := strings.TrimSpace(string(outBytes))

	matches := reQemuVersion.FindStringSubmatch(out)
	if len(matches) != 2 {
		resp.RemoveAttribute(qemuDriverAttr)
		return fmt.Errorf("Unable to parse Qemu version string: %#v", matches)
	}
	currentQemuVersion := matches[1]

	resp.AddAttribute(qemuDriverAttr, "1")
	resp.AddAttribute(qemuDriverVersionAttr, currentQemuVersion)
	resp.Detected = true

	return nil
}

func (d *QemuDriver) Prestart(_ *ExecContext, task *structs.Task) (*PrestartResponse, error) {
	var driverConfig QemuDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}

	if len(driverConfig.PortMap) > 1 {
		return nil, fmt.Errorf("Only one port_map block is allowed in the qemu driver config")
	}

	d.driverConfig = &driverConfig

	r := NewPrestartResponse()
	if len(driverConfig.PortMap) == 1 {
		r.Network = &cstructs.DriverNetwork{
			PortMap: driverConfig.PortMap[0],
		}
	}
	return r, nil
}

// Run an existing Qemu image. Start() will pull down an existing, valid Qemu
// image and save it to the Drivers Allocation Dir
func (d *QemuDriver) Start(ctx *ExecContext, task *structs.Task) (*StartResponse, error) {
	// Get the image source
	vmPath := d.driverConfig.ImagePath
	if vmPath == "" {
		return nil, fmt.Errorf("image_path must be set")
	}
	vmID := filepath.Base(vmPath)

	// Parse configuration arguments
	// Create the base arguments
	accelerator := "tcg"
	if d.driverConfig.Accelerator != "" {
		accelerator = d.driverConfig.Accelerator
	}

	if task.Resources.MemoryMB < 128 || task.Resources.MemoryMB > 4000000 {
		return nil, fmt.Errorf("Qemu memory assignment out of bounds")
	}
	mem := fmt.Sprintf("%dM", task.Resources.MemoryMB)

	absPath, err := GetAbsolutePath("qemu-system-x86_64")
	if err != nil {
		return nil, err
	}

	args := []string{
		absPath,
		"-machine", "type=pc,accel=" + accelerator,
		"-name", vmID,
		"-m", mem,
		"-drive", "file=" + vmPath,
		"-nographic",
	}

	var monitorPath string
	if d.driverConfig.GracefulShutdown {
		if runtime.GOOS == "windows" {
			return nil, errors.New("QEMU graceful shutdown is unsupported on the Windows platform")
		}
		// This socket will be used to manage the virtual machine (for example,
		// to perform graceful shutdowns)
		monitorPath, err = d.getMonitorPath(ctx.TaskDir.Dir)
		if err != nil {
			d.logger.Printf("[ERR] driver.qemu: could not get qemu monitor path: %s", err)
			return nil, err
		}
		d.logger.Printf("[DEBUG] driver.qemu: got monitor path OK: %s", monitorPath)
		args = append(args, "-monitor", fmt.Sprintf("unix:%s,server,nowait", monitorPath))
	}

	// Add pass through arguments to qemu executable. A user can specify
	// these arguments in driver task configuration. These arguments are
	// passed directly to the qemu driver as command line options.
	// For example, args = [ "-nodefconfig", "-nodefaults" ]
	// This will allow a VM with embedded configuration to boot successfully.
	args = append(args, d.driverConfig.Args...)

	// Check the Resources required Networks to add port mappings. If no resources
	// are required, we assume the VM is a purely compute job and does not require
	// the outside world to be able to reach it. VMs ran without port mappings can
	// still reach out to the world, but without port mappings it is effectively
	// firewalled
	protocols := []string{"udp", "tcp"}
	if len(task.Resources.Networks) > 0 && len(d.driverConfig.PortMap) == 1 {
		// Loop through the port map and construct the hostfwd string, to map
		// reserved ports to the ports listenting in the VM
		// Ex: hostfwd=tcp::22000-:22,hostfwd=tcp::80-:8080
		var forwarding []string
		taskPorts := task.Resources.Networks[0].PortLabels()
		for label, guest := range d.driverConfig.PortMap[0] {
			host, ok := taskPorts[label]
			if !ok {
				return nil, fmt.Errorf("Unknown port label %q", label)
			}

			for _, p := range protocols {
				forwarding = append(forwarding, fmt.Sprintf("hostfwd=%s::%d-:%d", p, host, guest))
			}
		}

		if len(forwarding) != 0 {
			args = append(args,
				"-netdev",
				fmt.Sprintf("user,id=user.0,%s", strings.Join(forwarding, ",")),
				"-device", "virtio-net,netdev=user.0",
			)
		}
	}

	// If using KVM, add optimization args
	if accelerator == "kvm" {
		if runtime.GOOS == "windows" {
			return nil, errors.New("KVM accelerator is unsupported on the Windows platform")
		}
		args = append(args,
			"-enable-kvm",
			"-cpu", "host",
			// Do we have cores information available to the Driver?
			// "-smp", fmt.Sprintf("%d", cores),
		)
	}

	d.logger.Printf("[DEBUG] driver.qemu: starting QemuVM command: %q", strings.Join(args, " "))
	pluginLogFile := filepath.Join(ctx.TaskDir.Dir, "executor.out")
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: d.config.LogLevel,
	}

	exec, pluginClient, err := createExecutor(d.config.LogOutput, d.config, executorConfig)
	if err != nil {
		return nil, err
	}
	executorCtx := &executor.ExecutorContext{
		TaskEnv: ctx.TaskEnv,
		Driver:  "qemu",
		Task:    task,
		TaskDir: ctx.TaskDir.Dir,
		LogDir:  ctx.TaskDir.LogDir,
	}
	if err := exec.SetContext(executorCtx); err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("failed to set executor context: %v", err)
	}

	execCmd := &executor.ExecCommand{
		Cmd:  args[0],
		Args: args[1:],
		User: task.User,
	}
	ps, err := exec.LaunchCmd(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, err
	}
	d.logger.Printf("[INFO] driver.qemu: started new QemuVM: %s", vmID)

	// Create and Return Handle
	maxKill := d.DriverContext.config.MaxKillTimeout
	h := &qemuHandle{
		pluginClient:   pluginClient,
		executor:       exec,
		userPid:        ps.Pid,
		killTimeout:    GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout: maxKill,
		monitorPath:    monitorPath,
		version:        d.config.Version.VersionNumber(),
		logger:         d.logger,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *dstructs.WaitResult, 1),
	}
	go h.run()
	resp := &StartResponse{Handle: h}
	if len(d.driverConfig.PortMap) == 1 {
		resp.Network = &cstructs.DriverNetwork{
			PortMap: d.driverConfig.PortMap[0],
		}
	}
	return resp, nil
}

type qemuId struct {
	Version        string
	KillTimeout    time.Duration
	MaxKillTimeout time.Duration
	UserPid        int
	PluginConfig   *PluginReattachConfig
}

func (d *QemuDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &qemuId{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle %q: %v", handleID, err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: id.PluginConfig.PluginConfig(),
	}

	exec, pluginClient, err := createExecutorWithConfig(pluginConfig, d.config.LogOutput)
	if err != nil {
		d.logger.Printf("[ERR] driver.qemu: error connecting to plugin so destroying plugin pid %d and user pid %d", id.PluginConfig.Pid, id.UserPid)
		if e := destroyPlugin(id.PluginConfig.Pid, id.UserPid); e != nil {
			d.logger.Printf("[ERR] driver.qemu: error destroying plugin pid %d and userpid %d: %v", id.PluginConfig.Pid, id.UserPid, e)
		}
		return nil, fmt.Errorf("error connecting to plugin: %v", err)
	}

	ver, _ := exec.Version()
	d.logger.Printf("[DEBUG] driver.qemu: version of executor: %v", ver.Version)
	// Return a driver handle
	h := &qemuHandle{
		pluginClient:   pluginClient,
		executor:       exec,
		userPid:        id.UserPid,
		logger:         d.logger,
		killTimeout:    id.KillTimeout,
		maxKillTimeout: id.MaxKillTimeout,
		version:        id.Version,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *dstructs.WaitResult, 1),
	}
	go h.run()
	return h, nil
}

func (d *QemuDriver) Cleanup(*ExecContext, *CreatedResources) error { return nil }

func (h *qemuHandle) ID() string {
	id := qemuId{
		Version:        h.version,
		KillTimeout:    h.killTimeout,
		MaxKillTimeout: h.maxKillTimeout,
		PluginConfig:   NewPluginReattachConfig(h.pluginClient.ReattachConfig()),
		UserPid:        h.userPid,
	}

	data, err := json.Marshal(id)
	if err != nil {
		h.logger.Printf("[ERR] driver.qemu: failed to marshal ID to JSON: %s", err)
	}
	return string(data)
}

func (h *qemuHandle) WaitCh() chan *dstructs.WaitResult {
	return h.waitCh
}

func (h *qemuHandle) Update(task *structs.Task) error {
	// Store the updated kill timeout.
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.maxKillTimeout)
	h.executor.UpdateTask(task)

	// Update is not possible
	return nil
}

func (h *qemuHandle) Exec(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
	return nil, 0, fmt.Errorf("Qemu driver can't execute commands")
}

func (h *qemuHandle) Signal(s os.Signal) error {
	return fmt.Errorf("Qemu driver can't send signals")
}

func (h *qemuHandle) Kill() error {
	gracefulShutdownSent := false
	// Attempt a graceful shutdown only if it was configured in the job
	if h.monitorPath != "" {
		if err := sendQemuShutdown(h.logger, h.monitorPath, h.userPid); err == nil {
			gracefulShutdownSent = true
		} else {
			h.logger.Printf("[DEBUG] driver.qemu: error sending graceful shutdown for user process pid %d: %s", h.userPid, err)
		}
	}

	// If Nomad did not send a graceful shutdown signal, issue an interrupt to
	// the qemu process as a last resort
	if gracefulShutdownSent == false {
		h.logger.Printf("[DEBUG] driver.qemu: graceful shutdown is not enabled, sending an interrupt signal to pid: %d", h.userPid)
		if err := h.executor.ShutDown(); err != nil {
			if h.pluginClient.Exited() {
				return nil
			}
			return fmt.Errorf("executor Shutdown failed: %v", err)
		}
	}

	// If the qemu process exits before the kill timeout is reached, doneChan
	// will close and we'll exit without an error. If it takes too long, the
	// timer will fire and we'll attempt to kill the process.
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(h.killTimeout):
		h.logger.Printf("[DEBUG] driver.qemu: kill timeout of %s exceeded for user process pid %d", h.killTimeout.String(), h.userPid)

		if h.pluginClient.Exited() {
			return nil
		}
		if err := h.executor.Exit(); err != nil {
			return fmt.Errorf("executor Exit failed: %v", err)
		}
		return nil
	}
}

func (h *qemuHandle) Stats() (*cstructs.TaskResourceUsage, error) {
	return h.executor.Stats()
}

func (h *qemuHandle) run() {
	ps, werr := h.executor.Wait()
	if ps.ExitCode == 0 && werr != nil {
		if e := killProcess(h.userPid); e != nil {
			h.logger.Printf("[ERR] driver.qemu: error killing user process pid %d: %v", h.userPid, e)
		}
	}
	close(h.doneCh)

	// Exit the executor
	h.executor.Exit()
	h.pluginClient.Kill()

	// Send the results
	h.waitCh <- &dstructs.WaitResult{ExitCode: ps.ExitCode, Signal: ps.Signal, Err: werr}
	close(h.waitCh)
}

// sendQemuShutdown attempts to issue an ACPI power-off command via the qemu
// monitor
func sendQemuShutdown(logger *log.Logger, monitorPath string, userPid int) error {
	if monitorPath == "" {
		return errors.New("monitorPath not set")
	}
	monitorSocket, err := net.Dial("unix", monitorPath)
	if err != nil {
		logger.Printf("[WARN] driver.qemu: could not connect to qemu monitor %q for user process pid %d: %s", monitorPath, userPid, err)
		return err
	}
	defer monitorSocket.Close()
	logger.Printf("[DEBUG] driver.qemu: sending graceful shutdown command to qemu monitor socket %q for user process pid %d", monitorPath, userPid)
	_, err = monitorSocket.Write([]byte(qemuGracefulShutdownMsg))
	if err != nil {
		logger.Printf("[WARN] driver.qemu: failed to send shutdown message %q to monitor socket %q for user process pid %d: %s", qemuGracefulShutdownMsg, monitorPath, userPid, err)
	}
	return err
}
