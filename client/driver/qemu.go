package driver

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/client/getter"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

var (
	reQemuVersion = regexp.MustCompile(`version (\d[\.\d+]+)`)
)

// QemuDriver is a driver for running images via Qemu
// We attempt to chose sane defaults for now, with more configuration available
// planned in the future
type QemuDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter
}

type QemuDriverConfig struct {
	ArtifactSource string           `mapstructure:"artifact_source"`
	Checksum       string           `mapstructure:"checksum"`
	Accelerator    string           `mapstructure:"accelerator"`
	PortMap        []map[string]int `mapstructure:"port_map"` // A map of host port labels and to guest ports.
}

// qemuHandle is returned from Start/Open as a handle to the PID
type qemuHandle struct {
	pluginClient   *plugin.Client
	userPid        int
	executor       executor.Executor
	allocDir       *allocdir.AllocDir
	killTimeout    time.Duration
	maxKillTimeout time.Duration
	logger         *log.Logger
	version        string
	waitCh         chan *cstructs.WaitResult
	doneCh         chan struct{}
}

// NewQemuDriver is used to create a new exec driver
func NewQemuDriver(ctx *DriverContext) Driver {
	return &QemuDriver{DriverContext: *ctx}
}

func (d *QemuDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	bin := "qemu-system-x86_64"
	if runtime.GOOS == "windows" {
		// On windows, the "qemu-system-x86_64" command does not respond to the
		// version flag.
		bin = "qemu-img"
	}
	outBytes, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return false, nil
	}
	out := strings.TrimSpace(string(outBytes))

	matches := reQemuVersion.FindStringSubmatch(out)
	if len(matches) != 2 {
		return false, fmt.Errorf("Unable to parse Qemu version string: %#v", matches)
	}

	node.Attributes["driver.qemu"] = "1"
	node.Attributes["driver.qemu.version"] = matches[1]

	return true, nil
}

// Run an existing Qemu image. Start() will pull down an existing, valid Qemu
// image and save it to the Drivers Allocation Dir
func (d *QemuDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	var driverConfig QemuDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}

	if len(driverConfig.PortMap) > 1 {
		return nil, fmt.Errorf("Only one port_map block is allowed in the qemu driver config")
	}

	// Get the image source
	source, ok := task.Config["artifact_source"]
	if !ok || source == "" {
		return nil, fmt.Errorf("Missing source image Qemu driver")
	}

	// Qemu defaults to 128M of RAM for a given VM. Instead, we force users to
	// supply a memory size in the tasks resources
	if task.Resources == nil || task.Resources.MemoryMB == 0 {
		return nil, fmt.Errorf("Missing required Task Resource: Memory")
	}

	// Get the tasks local directory.
	taskDir, ok := ctx.AllocDir.TaskDirs[d.DriverContext.taskName]
	if !ok {
		return nil, fmt.Errorf("Could not find task directory for task: %v", d.DriverContext.taskName)
	}

	// Proceed to download an artifact to be executed.
	vmPath, err := getter.GetArtifact(
		taskDir,
		driverConfig.ArtifactSource,
		driverConfig.Checksum,
		d.logger,
	)
	if err != nil {
		return nil, err
	}

	vmID := filepath.Base(vmPath)

	// Parse configuration arguments
	// Create the base arguments
	accelerator := "tcg"
	if driverConfig.Accelerator != "" {
		accelerator = driverConfig.Accelerator
	}
	// TODO: Check a lower bounds, e.g. the default 128 of Qemu
	mem := fmt.Sprintf("%dM", task.Resources.MemoryMB)

	args := []string{
		"qemu-system-x86_64",
		"-machine", "type=pc,accel=" + accelerator,
		"-name", vmID,
		"-m", mem,
		"-drive", "file=" + vmPath,
		"-nodefconfig",
		"-nodefaults",
		"-nographic",
	}

	// Check the Resources required Networks to add port mappings. If no resources
	// are required, we assume the VM is a purely compute job and does not require
	// the outside world to be able to reach it. VMs ran without port mappings can
	// still reach out to the world, but without port mappings it is effectively
	// firewalled
	protocols := []string{"udp", "tcp"}
	if len(task.Resources.Networks) > 0 && len(driverConfig.PortMap) == 1 {
		// Loop through the port map and construct the hostfwd string, to map
		// reserved ports to the ports listenting in the VM
		// Ex: hostfwd=tcp::22000-:22,hostfwd=tcp::80-:8080
		var forwarding []string
		taskPorts := task.Resources.Networks[0].MapLabelToValues(nil)
		for label, guest := range driverConfig.PortMap[0] {
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
		args = append(args,
			"-enable-kvm",
			"-cpu", "host",
			// Do we have cores information available to the Driver?
			// "-smp", fmt.Sprintf("%d", cores),
		)
	}

	d.logger.Printf("[DEBUG] Starting QemuVM command: %q", strings.Join(args, " "))
	bin, err := discover.NomadExecutable()
	if err != nil {
		return nil, fmt.Errorf("unable to find the nomad binary: %v", err)
	}

	pluginLogFile := filepath.Join(taskDir, fmt.Sprintf("%s-executor.out", task.Name))
	pluginConfig := &plugin.ClientConfig{
		Cmd: exec.Command(bin, "executor", pluginLogFile),
	}

	exec, pluginClient, err := createExecutor(pluginConfig, d.config.LogOutput, d.config)
	if err != nil {
		return nil, err
	}
	executorCtx := &executor.ExecutorContext{
		TaskEnv:       d.taskEnv,
		AllocDir:      ctx.AllocDir,
		TaskName:      task.Name,
		TaskResources: task.Resources,
		LogConfig:     task.LogConfig,
	}
	ps, err := exec.LaunchCmd(&executor.ExecCommand{Cmd: args[0], Args: args[1:]}, executorCtx)
	if err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("error starting process via the plugin: %v", err)
	}
	d.logger.Printf("[INFO] Started new QemuVM: %s", vmID)

	// Create and Return Handle
	maxKill := d.DriverContext.config.MaxKillTimeout
	h := &qemuHandle{
		pluginClient:   pluginClient,
		executor:       exec,
		userPid:        ps.Pid,
		allocDir:       ctx.AllocDir,
		killTimeout:    GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout: maxKill,
		version:        d.config.Version,
		logger:         d.logger,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *cstructs.WaitResult, 1),
	}

	go h.run()
	return h, nil
}

type qemuId struct {
	Version        string
	KillTimeout    time.Duration
	MaxKillTimeout time.Duration
	UserPid        int
	PluginConfig   *PluginReattachConfig
	AllocDir       *allocdir.AllocDir
}

func (d *QemuDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &qemuId{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: id.PluginConfig.PluginConfig(),
	}

	executor, pluginClient, err := createExecutor(pluginConfig, d.config.LogOutput, d.config)
	if err != nil {
		d.logger.Println("[ERR] driver.qemu: error connecting to plugin so destroying plugin pid and user pid")
		if e := destroyPlugin(id.PluginConfig.Pid, id.UserPid); e != nil {
			d.logger.Printf("[ERR] driver.qemu: error destroying plugin and userpid: %v", e)
		}
		return nil, fmt.Errorf("error connecting to plugin: %v", err)
	}

	// Return a driver handle
	h := &qemuHandle{
		pluginClient:   pluginClient,
		executor:       executor,
		userPid:        id.UserPid,
		allocDir:       id.AllocDir,
		logger:         d.logger,
		killTimeout:    id.KillTimeout,
		maxKillTimeout: id.MaxKillTimeout,
		version:        id.Version,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *cstructs.WaitResult, 1),
	}
	go h.run()
	return h, nil
}

func (h *qemuHandle) ID() string {
	id := qemuId{
		Version:        h.version,
		KillTimeout:    h.killTimeout,
		MaxKillTimeout: h.maxKillTimeout,
		PluginConfig:   NewPluginReattachConfig(h.pluginClient.ReattachConfig()),
		UserPid:        h.userPid,
		AllocDir:       h.allocDir,
	}

	data, err := json.Marshal(id)
	if err != nil {
		h.logger.Printf("[ERR] driver.qemu: failed to marshal ID to JSON: %s", err)
	}
	return string(data)
}

func (h *qemuHandle) WaitCh() chan *cstructs.WaitResult {
	return h.waitCh
}

func (h *qemuHandle) Update(task *structs.Task) error {
	// Store the updated kill timeout.
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.maxKillTimeout)
	h.executor.UpdateLogConfig(task.LogConfig)

	// Update is not possible
	return nil
}

// TODO: allow a 'shutdown_command' that can be executed over a ssh connection
// to the VM
func (h *qemuHandle) Kill() error {
	if err := h.executor.ShutDown(); err != nil {
		if h.pluginClient.Exited() {
			return nil
		}
		return fmt.Errorf("executor Shutdown failed: %v", err)
	}

	select {
	case <-h.doneCh:
		return nil
	case <-time.After(h.killTimeout):
		if h.pluginClient.Exited() {
			return nil
		}
		if err := h.executor.Exit(); err != nil {
			return fmt.Errorf("executor Exit failed: %v", err)
		}

		return nil
	}
}

func (h *qemuHandle) run() {
	ps, err := h.executor.Wait()
	if ps.ExitCode == 0 && err != nil {
		if e := killProcess(h.userPid); e != nil {
			h.logger.Printf("[ERR] driver.qemu: error killing user process: %v", e)
		}
		if e := h.allocDir.UnmountAll(); e != nil {
			h.logger.Printf("[ERR] driver.qemu: unmounting dev,proc and alloc dirs failed: %v", e)
		}
	}
	close(h.doneCh)
	h.waitCh <- &cstructs.WaitResult{ExitCode: ps.ExitCode, Signal: 0, Err: err}
	close(h.waitCh)
	h.pluginClient.Kill()
}
