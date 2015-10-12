package driver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/args"
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
	image  string
	logger *log.Logger
	waitCh chan error
	doneCh chan struct{}
}

// rktPID is a struct to map the pid running the process to the vm image on
// disk
type rktPID struct {
	Pid   int
	Image string
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

	node.Attributes["driver.rkt"] = "1"
	node.Attributes["driver.rkt.version"] = rktMatches[0]
	node.Attributes["driver.rkt.appc.version"] = appcMatches[1]

	return true, nil
}

// Run an existing Rkt image.
func (d *RktDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	// Validate that the config is valid.
	img, ok := task.Config["image"]
	if !ok || img == "" {
		return nil, fmt.Errorf("Missing ACI image for rkt")
	}

	// Get the tasks local directory.
	taskName := d.DriverContext.taskName
	taskDir, ok := ctx.AllocDir.TaskDirs[taskName]
	if !ok {
		return nil, fmt.Errorf("Could not find task directory for task: %v", d.DriverContext.taskName)
	}
	taskLocal := filepath.Join(taskDir, allocdir.TaskLocal)

	// Add the given trust prefix
	trust_prefix, trust_cmd := task.Config["trust_prefix"]
	if trust_cmd {
		var outBuf, errBuf bytes.Buffer
		cmd := exec.Command("rkt", "trust", fmt.Sprintf("--prefix=%s", trust_prefix))
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("Error running rkt trust: %s\n\nOutput: %s\n\nError: %s",
				err, outBuf.String(), errBuf.String())
		}
		d.logger.Printf("[DEBUG] driver.rkt: added trust prefix: %q", trust_prefix)
	}

	// Build the command.
	var cmd_args []string

	// Inject the environment variables.
	envVars := TaskEnvironmentVariables(ctx, task)
	for k, v := range envVars.Map() {
		cmd_args = append(cmd_args, fmt.Sprintf("--set-env=%v=%v", k, v))
	}

	// Disble signature verification if the trust command was not run.
	if !trust_cmd {
		cmd_args = append(cmd_args, "--insecure-skip-verify")
	}

	// Append the run command.
	cmd_args = append(cmd_args, "run", "--mds-register=false", img)

	// Check if the user has overriden the exec command.
	if exec_cmd, ok := task.Config["command"]; ok {
		cmd_args = append(cmd_args, fmt.Sprintf("--exec=%v", exec_cmd))
	}

	// Add user passed arguments.
	if userArgs, ok := task.Config["args"]; ok {
		parsed, err := args.ParseAndReplace(userArgs, envVars.Map())
		if err != nil {
			return nil, err
		}

		// Need to start arguments with "--"
		if len(parsed) > 0 {
			cmd_args = append(cmd_args, "--")
		}

		for _, arg := range parsed {
			cmd_args = append(cmd_args, fmt.Sprintf("%v", arg))
		}
	}

	// Create files to capture stdin and out.
	stdoutFilename := filepath.Join(taskLocal, fmt.Sprintf("%s.stdout", taskName))
	stderrFilename := filepath.Join(taskLocal, fmt.Sprintf("%s.stderr", taskName))

	stdo, err := os.OpenFile(stdoutFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("Error opening file to redirect stdout: %v", err)
	}

	stde, err := os.OpenFile(stderrFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("Error opening file to redirect stderr: %v", err)
	}

	cmd := exec.Command("rkt", cmd_args...)
	cmd.Stdout = stdo
	cmd.Stderr = stde

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Error running rkt: %v", err)
	}

	d.logger.Printf("[DEBUG] driver.rkt: started ACI %q with: %v", img, cmd.Args)
	h := &rktHandle{
		proc:   cmd.Process,
		image:  img,
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
		image:  qpid.Image,
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
		Pid:   h.proc.Pid,
		Image: h.image,
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
