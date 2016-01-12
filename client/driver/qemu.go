package driver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/client/getter"
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
	cmd         executor.Executor
	killTimeout time.Duration
	logger      *log.Logger
	waitCh      chan *cstructs.WaitResult
	doneCh      chan struct{}
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
		filepath.Join(taskDir, allocdir.TaskLocal),
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

	// Setup the command
	cmd := executor.NewBasicExecutor()
	executor.SetCommand(cmd, args[0], args[1:])
	if err := cmd.Limit(task.Resources); err != nil {
		return nil, fmt.Errorf("failed to constrain resources: %s", err)
	}

	if err := cmd.ConfigureTaskDir(d.taskName, ctx.AllocDir); err != nil {
		return nil, fmt.Errorf("failed to configure task directory: %v", err)
	}

	d.logger.Printf("[DEBUG] Starting QemuVM command: %q", strings.Join(args, " "))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %v", err)
	}
	d.logger.Printf("[INFO] Started new QemuVM: %s", vmID)

	// Create and Return Handle
	h := &qemuHandle{
		cmd:         cmd,
		killTimeout: d.DriverContext.KillTimeout(task),
		logger:      d.logger,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *cstructs.WaitResult, 1),
	}

	go h.Wait()
	return h, nil
}

type qemuId struct {
	ExecutorId  string
	KillTimeout time.Duration
}

func (d *QemuDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &qemuId{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	// Find the process
	cmd := executor.NewBasicExecutor()
	if err := cmd.Open(id.ExecutorId); err != nil {
		return nil, fmt.Errorf("failed to open ID %v: %v", id.ExecutorId, err)
	}

	// Return a driver handle
	h := &execHandle{
		cmd:         cmd,
		logger:      d.logger,
		killTimeout: id.KillTimeout,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *cstructs.WaitResult, 1),
	}
	return h, nil
}

func (h *qemuHandle) ID() string {
	executorId, _ := h.cmd.ID()
	id := qemuId{
		ExecutorId:  executorId,
		KillTimeout: h.killTimeout,
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
	// Update is not possible
	return nil
}

// TODO: allow a 'shutdown_command' that can be executed over a ssh connection
// to the VM
func (h *qemuHandle) Kill() error {
	h.cmd.Shutdown()
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(h.killTimeout):
		return h.cmd.ForceStop()
	}
}

func (h *qemuHandle) Wait() {
	res := h.cmd.Wait()
	close(h.doneCh)
	h.waitCh <- res
	close(h.waitCh)
}

func (h *qemuHandle) Logs(w io.Writer, follow bool, stdout bool, stderr bool, lines int64) error {
	return nil
}
