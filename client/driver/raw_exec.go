package driver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/args"
        "github.com/hashicorp/nomad/client/getter"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// The option that enables this driver in the Config.Options map.
	rawExecConfigOption = "driver.raw_exec.enable"

	// Null files to use as stdin.
	unixNull    = "/dev/null"
	windowsNull = "nul"
)

// The RawExecDriver is a privileged version of the exec driver. It provides no
// resource isolation and just fork/execs. The Exec driver should be preferred
// and this should only be used when explicitly needed.
type RawExecDriver struct {
	DriverContext
}

// rawExecHandle is returned from Start/Open as a handle to the PID
type rawExecHandle struct {
	proc   *os.Process
	waitCh chan error
	doneCh chan struct{}
}

// NewRawExecDriver is used to create a new raw exec driver
func NewRawExecDriver(ctx *DriverContext) Driver {
	return &RawExecDriver{*ctx}
}

func (d *RawExecDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Check that the user has explicitly enabled this executor.
	enabled, err := strconv.ParseBool(cfg.ReadDefault(rawExecConfigOption, "false"))
	if err != nil {
		return false, fmt.Errorf("Failed to parse %v option: %v", rawExecConfigOption, err)
	}

	if enabled {
		d.logger.Printf("[WARN] driver.raw_exec: raw exec is enabled. Only enable if needed")
		node.Attributes["driver.raw_exec"] = "1"
		return true, nil
	}

	return false, nil
}

func (d *RawExecDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	// Get the tasks local directory.
	taskName := d.DriverContext.taskName
	taskDir, ok := ctx.AllocDir.TaskDirs[taskName]
	if !ok {
		return nil, fmt.Errorf("Could not find task directory for task: %v", d.DriverContext.taskName)
	}
	taskLocal := filepath.Join(taskDir, allocdir.TaskLocal)

	// Get the command to be ran
	command, ok := task.Config["command"]
	if !ok || command == "" {
		return nil, fmt.Errorf("missing command for Raw Exec driver")
	}

	// Check if an artificat is specified and attempt to download it
	source, ok := task.Config["artifact_source"]
	if ok && source != "" {
		// Proceed to download an artifact to be executed.
                _, err := getter.GetArtifact(
                        filepath.Join(taskDir, allocdir.TaskLocal),
                        task.Config["artifact_source"],
                        task.Config["checksum"],
                        d.logger,
                )
                if err != nil {
                        return nil, err
		}
	}

	// Get the environment variables.
	envVars := TaskEnvironmentVariables(ctx, task)

	// expand NOMAD_TASK_DIR
	parsedPath, err := args.ParseAndReplace(command, envVars.Map())
	if err != nil {
		return nil, fmt.Errorf("failure to parse arguments in command path: %v", command)
	} else if len(parsedPath) != 1 {
		return nil, fmt.Errorf("couldn't properly parse command path: %v", command)
	}

	cm := parsedPath[0]

	// Look for arguments
	var cmdArgs []string
	if argRaw, ok := task.Config["args"]; ok {
		parsed, err := args.ParseAndReplace(argRaw, envVars.Map())
		if err != nil {
			return nil, err
		}
		cmdArgs = append(cmdArgs, parsed...)
	}

	// Setup the command
	cmd := exec.Command(cm, cmdArgs...)
	cmd.Dir = taskDir
	cmd.Env = envVars.List()

	// Capture the stdout/stderr and redirect stdin to /dev/null
	stdoutFilename := filepath.Join(taskLocal, fmt.Sprintf("%s.stdout", taskName))
	stderrFilename := filepath.Join(taskLocal, fmt.Sprintf("%s.stderr", taskName))
	stdinFilename := unixNull
	if runtime.GOOS == "windows" {
		stdinFilename = windowsNull
	}

	stdo, err := os.OpenFile(stdoutFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("Error opening file to redirect stdout: %v", err)
	}

	stde, err := os.OpenFile(stderrFilename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("Error opening file to redirect stderr: %v", err)
	}

	stdi, err := os.OpenFile(stdinFilename, os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("Error opening file to redirect stdin: %v", err)
	}

	cmd.Stdout = stdo
	cmd.Stderr = stde
	cmd.Stdin = stdi

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %v", err)
	}

	// Return a driver handle
	h := &rawExecHandle{
		proc:   cmd.Process,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}
	go h.run()
	return h, nil
}

func (d *RawExecDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	// Split the handle
	pidStr := strings.TrimPrefix(handleID, "PID:")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse handle '%s': %v", handleID, err)
	}

	// Find the process
	proc, err := os.FindProcess(pid)
	if proc == nil || err != nil {
		return nil, fmt.Errorf("failed to find PID %d: %v", pid, err)
	}

	// Return a driver handle
	h := &rawExecHandle{
		proc:   proc,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}
	go h.run()
	return h, nil
}

func (h *rawExecHandle) ID() string {
	// Return a handle to the PID
	return fmt.Sprintf("PID:%d", h.proc.Pid)
}

func (h *rawExecHandle) WaitCh() chan error {
	return h.waitCh
}

func (h *rawExecHandle) Update(task *structs.Task) error {
	// Update is not possible
	return nil
}

// Kill is used to terminate the task. We send an Interrupt
// and then provide a 5 second grace period before doing a Kill on supported
// OS's, otherwise we kill immediately.
func (h *rawExecHandle) Kill() error {
	if runtime.GOOS == "windows" {
		return h.proc.Kill()
	}

	h.proc.Signal(os.Interrupt)
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(5 * time.Second):
		return h.proc.Kill()
	}
}

func (h *rawExecHandle) run() {
	ps, err := h.proc.Wait()
	close(h.doneCh)
	if err != nil {
		h.waitCh <- err
	} else if !ps.Success() {
		h.waitCh <- fmt.Errorf("task exited with error")
	}
	close(h.waitCh)
}
