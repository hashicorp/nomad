package driver

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/executor"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	reRktVersion  = regexp.MustCompile("rkt version ([\\d\\.]+).+")
	reAppcVersion = regexp.MustCompile("appc version ([\\d\\.]+).+")
)

// RktDriver is a driver for running images via Rkt
// We attempt to chose sane defaults for now, with more configuration available
// planned in the future
type RktDriver struct {
	DriverContext
}

// rktHandle is returned from Start/Open as a handle to the PID
type rktHandle struct {
	proc   *os.Process
	name   string
	logger *log.Logger
	waitCh chan error
	doneCh chan struct{}
}

// rktPID is a struct to map the pid running the process to the vm image on
// disk
type rktPID struct {
	Pid  int
	Name string
}

// NewRktDriver is used to create a new exec driver
func NewRktDriver(ctx *DriverContext) Driver {
	return &RktDriver{*ctx}
}

func (d *RktDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Only enable if we are root when running on non-windows systems.
	if runtime.GOOS != "windows" && syscall.Geteuid() != 0 {
		d.logger.Printf("[DEBUG] driver.rkt: must run as root user, disabling")
		return false, nil
	}

	outBytes, err := exec.Command("rkt", "version").Output()
	if err != nil {
		return false, nil
	}
	out := strings.TrimSpace(string(outBytes))

	rktMatches := reRktVersion.FindStringSubmatch(out)
	appcMatches := reRktVersion.FindStringSubmatch(out)
	if len(rktMatches) != 2 || len(appcMatches) != 2 {
		return false, fmt.Errorf("Unable to parse Rkt version string: %#v", rktMatches)
	}

	node.Attributes["driver.rkt"] = "true"
	node.Attributes["driver.rkt.version"] = rktMatches[0]
	node.Attributes["driver.rkt.appc.version"] = appcMatches[1]

	return true, nil
}

// Run an existing Rkt image.
func (d *RktDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	trust_prefix, ok := task.Config["trust_prefix"]
	if !ok || trust_prefix == "" {
		return nil, fmt.Errorf("Missing trust prefix for rkt")
	}

	// Add the given trust prefix
	cmd := executor.Command("rkt", "trust", fmt.Sprintf("--prefix=%s", trust_prefix))
	d.logger.Printf("[DEBUG] driver.rkt: starting rkt command: %q", cmd.Command().Args)
	if err := cmd.Command().Run(); err != nil {
		return nil, fmt.Errorf(
			"Error running rkt: %v", err)
	}
	d.logger.Printf("[DEBUG] driver.rkt: added trust prefix: %q", trust_prefix)

	name, ok := task.Config["name"]
	if !ok || name == "" {
		return nil, fmt.Errorf("Missing ACI name for rkt")
	}

	exec_cmd, ok := task.Config["exec"]
	if !ok || exec_cmd == "" {
		d.logger.Printf("[WARN] driver.rkt: could not find a command to execute in the ACI, the default command will be executed")
	}

	// Run the ACI
	run_cmd := []string{
		"rkt",
		"run",
		"--mds-register=false",
		name,
	}
	if exec_cmd != "" {
		splitted := strings.Fields(exec_cmd)
		run_cmd = append(run_cmd, "--exec=", splitted[0], "--")
		run_cmd = append(run_cmd, splitted[1:]...)
		run_cmd = append(run_cmd, "---")
	}
	acmd := executor.Command(run_cmd[0], run_cmd[1:]...)
	if err := acmd.ConfigureTaskDir(d.taskName, ctx.AllocDir); err != nil {
		return nil, fmt.Errorf("failed to configure task directory: %v", err)
	}
	d.logger.Printf("[DEBUG] driver:rkt: starting rkt command: %q", acmd.Command().Args)
	if err := acmd.Start(); err != nil {
		return nil, fmt.Errorf(
			"Error running rkt: %v", err)
	}
	d.logger.Printf("[DEBUG] driver.rkt: started ACI: %q", name)
	h := &rktHandle{
		proc:   acmd.Command().Process,
		name:   name,
		logger: d.logger,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}
	go h.run()
	return h, nil
}

func (d *RktDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	// Parse the handle
	pidBytes := []byte(strings.TrimPrefix(handleID, "Rkt:"))
	qpid := &rktPID{}
	if err := json.Unmarshal(pidBytes, qpid); err != nil {
		return nil, fmt.Errorf("failed to parse Rkt handle '%s': %v", handleID, err)
	}

	// Find the process
	proc, err := os.FindProcess(qpid.Pid)
	if proc == nil || err != nil {
		return nil, fmt.Errorf("failed to find Rkt PID %d: %v", qpid.Pid, err)
	}

	// Return a driver handle
	h := &rktHandle{
		proc:   proc,
		name:   qpid.Name,
		logger: d.logger,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}

	go h.run()
	return h, nil
}

func (h *rktHandle) ID() string {
	// Return a handle to the PID
	pid := &rktPID{
		Pid:  h.proc.Pid,
		Name: h.name,
	}
	data, err := json.Marshal(pid)
	if err != nil {
		h.logger.Printf("[ERR] driver.rkt: failed to marshal rkt PID to JSON: %s", err)
	}
	return fmt.Sprintf("Rkt:%s", string(data))
}

func (h *rktHandle) WaitCh() chan error {
	return h.waitCh
}

func (h *rktHandle) Update(task *structs.Task) error {
	// Update is not possible
	return nil
}

// Kill is used to terminate the task. We send an Interrupt
// and then provide a 5 second grace period before doing a Kill.
func (h *rktHandle) Kill() error {
	h.proc.Signal(os.Interrupt)
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(5 * time.Second):
		return h.proc.Kill()
	}
}

func (h *rktHandle) run() {
	ps, err := h.proc.Wait()
	close(h.doneCh)
	if err != nil {
		h.waitCh <- err
	} else if !ps.Success() {
		h.waitCh <- fmt.Errorf("task exited with error")
	}
	close(h.waitCh)
}
