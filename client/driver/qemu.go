package driver

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	reQemuVersion = regexp.MustCompile("QEMU emulator version ([\\d\\.]+).+")
)

// QemuDriver is a driver for running images via Qemu
// We attempt to chose sane defaults for now, with more configuration available
// planned in the future
type QemuDriver struct {
	DriverContext
}

// qemuHandle is returned from Start/Open as a handle to the PID
type qemuHandle struct {
	proc   *os.Process
	vmID   string
	waitCh chan error
	doneCh chan struct{}
}

// qemuPID is a struct to map the pid running the process to the vm image on
// disk
type qemuPID struct {
	Pid  int
	VmID string
}

// NewQemuDriver is used to create a new exec driver
func NewQemuDriver(ctx *DriverContext) Driver {
	return &QemuDriver{*ctx}
}

func (d *QemuDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Only enable if we are root when running on non-windows systems.
	if runtime.GOOS != "windows" && syscall.Geteuid() != 0 {
		d.logger.Printf("[DEBUG] driver.qemu: must run as root user, disabling")
		return false, nil
	}

	outBytes, err := exec.Command("qemu-system-x86_64", "-version").Output()
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
	// Get the image source
	source, ok := task.Config["image_source"]
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

	// Create a location to download the binary.
	destDir := filepath.Join(taskDir, allocdir.TaskLocal)
	vmID := fmt.Sprintf("qemu-vm-%s-%s", structs.GenerateUUID(), filepath.Base(source))
	vmPath := filepath.Join(destDir, vmID)
	if err := getter.GetFile(vmPath, source); err != nil {
		return nil, fmt.Errorf("Error downloading artifact for Qemu driver: %s", err)
	}

	// compute and check checksum
	if check, ok := task.Config["checksum"]; ok {
		d.logger.Printf("[DEBUG] Running checksum on (%s)", vmID)
		hasher := sha256.New()
		file, err := os.Open(vmPath)
		if err != nil {
			return nil, fmt.Errorf("Failed to open file for checksum")
		}

		defer file.Close()
		io.Copy(hasher, file)

		sum := hex.EncodeToString(hasher.Sum(nil))
		if sum != check {
			return nil, fmt.Errorf(
				"Error in Qemu: checksums did not match.\nExpected (%s), got (%s)",
				check,
				sum)
		}
	}

	// Parse configuration arguments
	// Create the base arguments
	accelerator := "tcg"
	if acc, ok := task.Config["accelerator"]; ok {
		accelerator = acc
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
	if len(task.Resources.Networks) > 0 {
		// TODO: Consolidate these into map of host/guest port when we have HCL
		// Note: Host port must be open and available
		// Get and split guest ports. The guest_ports configuration must match up with
		// the Reserved ports in the Task Resources
		// Users can supply guest_hosts as a list of posts to map on the guest vm.
		// These map 1:1 with the requested Reserved Ports from the hostmachine.
		ports := strings.Split(task.Config["guest_ports"], ",")
		if len(ports) == 0 {
			return nil, fmt.Errorf("[ERR] driver.qemu: Error parsing required Guest Ports")
		}

		// TODO: support more than a single, default Network
		if len(ports) != len(task.Resources.Networks[0].ReservedPorts) {
			return nil, fmt.Errorf("[ERR] driver.qemu: Error matching Guest Ports with Reserved ports")
		}

		// Loop through the reserved ports and construct the hostfwd string, to map
		// reserved ports to the ports listenting in the VM
		// Ex:
		//    hostfwd=tcp::22000-:22,hostfwd=tcp::80-:8080
		reservedPorts := task.Resources.Networks[0].ReservedPorts
		var forwarding string
		for i, p := range ports {
			forwarding = fmt.Sprintf("%s,hostfwd=tcp::%s-:%s", forwarding, strconv.Itoa(reservedPorts[i]), p)
		}

		if "" == forwarding {
			return nil, fmt.Errorf("[ERR] driver.qemu:  Error constructing port forwarding")
		}

		args = append(args,
			"-netdev",
			fmt.Sprintf("user,id=user.0%s", forwarding),
			"-device", "virtio-net,netdev=user.0",
		)
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

	// Start Qemu
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	d.logger.Printf("[DEBUG] Starting QemuVM command: %q", strings.Join(args, " "))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf(
			"Error running QEMU: %s\n\nOutput: %s\n\nError: %s",
			err, outBuf.String(), errBuf.String())
	}

	d.logger.Printf("[INFO] Started new QemuVM: %s", vmID)

	// Create and Return Handle
	h := &qemuHandle{
		proc:   cmd.Process,
		vmID:   vmPath,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}

	go h.run()
	return h, nil
}

func (d *QemuDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	// Parse the handle
	pidBytes := []byte(strings.TrimPrefix(handleID, "QEMU:"))
	qpid := &qemuPID{}
	if err := json.Unmarshal(pidBytes, qpid); err != nil {
		return nil, fmt.Errorf("failed to parse Qemu handle '%s': %v", handleID, err)
	}

	// Find the process
	proc, err := os.FindProcess(qpid.Pid)
	if proc == nil || err != nil {
		return nil, fmt.Errorf("failed to find Qemu PID %d: %v", qpid.Pid, err)
	}

	// Return a driver handle
	h := &qemuHandle{
		proc:   proc,
		vmID:   qpid.VmID,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}

	go h.run()
	return h, nil
}

func (h *qemuHandle) ID() string {
	// Return a handle to the PID
	pid := &qemuPID{
		Pid:  h.proc.Pid,
		VmID: h.vmID,
	}
	data, err := json.Marshal(pid)
	if err != nil {
		log.Printf("[ERR] failed to marshal Qemu PID to JSON: %s", err)
	}
	return fmt.Sprintf("QEMU:%s", string(data))
}

func (h *qemuHandle) WaitCh() chan error {
	return h.waitCh
}

func (h *qemuHandle) Update(task *structs.Task) error {
	// Update is not possible
	return nil
}

// Kill is used to terminate the task. We send an Interrupt
// and then provide a 5 second grace period before doing a Kill.
//
// TODO: allow a 'shutdown_command' that can be executed over a ssh connection
// to the VM
func (h *qemuHandle) Kill() error {
	h.proc.Signal(os.Interrupt)
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(5 * time.Second):
		return h.proc.Kill()
	}
}

func (h *qemuHandle) run() {
	ps, err := h.proc.Wait()
	close(h.doneCh)
	if err != nil {
		h.waitCh <- err
	} else if !ps.Success() {
		h.waitCh <- fmt.Errorf("task exited with error")
	}
	close(h.waitCh)
}
