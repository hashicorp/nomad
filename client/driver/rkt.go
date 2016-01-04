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

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

var (
	reRktVersion  = regexp.MustCompile(`rkt version (\d[.\d]+)`)
	reAppcVersion = regexp.MustCompile(`appc version (\d[.\d]+)`)
)

const (
	// minRktVersion is the earliest supported version of rkt. rkt added support
	// for CPU and memory isolators in 0.14.0. We cannot support an earlier
	// version to maintain an uniform interface across all drivers
	minRktVersion = "0.14.0"

	// bytesToMB is the conversion from bytes to megabytes.
	bytesToMB = 1024 * 1024
)

// RktDriver is a driver for running images via Rkt
// We attempt to chose sane defaults for now, with more configuration available
// planned in the future
type RktDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter
}

type RktDriverConfig struct {
	ImageName string   `mapstructure:"image"`
	Args      []string `mapstructure:"args"`
}

// rktHandle is returned from Start/Open as a handle to the PID
type rktHandle struct {
	proc        *os.Process
	image       string
	logger      *log.Logger
	killTimeout time.Duration
	waitCh      chan *cstructs.WaitResult
	doneCh      chan struct{}
}

// rktPID is a struct to map the pid running the process to the vm image on
// disk
type rktPID struct {
	Pid         int
	Image       string
	KillTimeout time.Duration
}

// NewRktDriver is used to create a new exec driver
func NewRktDriver(ctx *DriverContext) Driver {
	return &RktDriver{DriverContext: *ctx}
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
	appcMatches := reAppcVersion.FindStringSubmatch(out)
	if len(rktMatches) != 2 || len(appcMatches) != 2 {
		return false, fmt.Errorf("Unable to parse Rkt version string: %#v", rktMatches)
	}

	node.Attributes["driver.rkt"] = "1"
	node.Attributes["driver.rkt.version"] = rktMatches[1]
	node.Attributes["driver.rkt.appc.version"] = appcMatches[1]

	minVersion, _ := version.NewVersion(minRktVersion)
	currentVersion, _ := version.NewVersion(node.Attributes["driver.rkt.version"])
	if currentVersion.LessThan(minVersion) {
		// Do not allow rkt < 0.14.0
		d.logger.Printf("[WARN] driver.rkt: please upgrade rkt to a version >= %s", minVersion)
		node.Attributes["driver.rkt"] = "0"
	}
	return true, nil
}

// Run an existing Rkt image.
func (d *RktDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	var driverConfig RktDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}
	// Validate that the config is valid.
	img := driverConfig.ImageName
	if img == "" {
		return nil, fmt.Errorf("Missing ACI image for rkt")
	}

	// Get the tasks local directory.
	taskName := d.DriverContext.taskName
	taskDir, ok := ctx.AllocDir.TaskDirs[taskName]
	if !ok {
		return nil, fmt.Errorf("Could not find task directory for task: %v", d.DriverContext.taskName)
	}
	taskLocal := filepath.Join(taskDir, allocdir.TaskLocal)

	// Build the command.
	var cmdArgs []string

	// Add the given trust prefix
	trustPrefix, trustCmd := task.Config["trust_prefix"]
	if trustCmd {
		var outBuf, errBuf bytes.Buffer
		cmd := exec.Command("rkt", "trust", fmt.Sprintf("--prefix=%s", trustPrefix))
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("Error running rkt trust: %s\n\nOutput: %s\n\nError: %s",
				err, outBuf.String(), errBuf.String())
		}
		d.logger.Printf("[DEBUG] driver.rkt: added trust prefix: %q", trustPrefix)
	} else {
		// Disble signature verification if the trust command was not run.
		cmdArgs = append(cmdArgs, "--insecure-options=all")
	}

	// Inject the environment variables.
	envVars := TaskEnvironmentVariables(ctx, task)

	// Clear the task directories as they are not currently supported.
	envVars.ClearTaskLocalDir()
	envVars.ClearAllocDir()

	for k, v := range envVars.Map() {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--set-env=%v=%v", k, v))
	}

	// Append the run command.
	cmdArgs = append(cmdArgs, "run", "--mds-register=false", img)

	// Check if the user has overriden the exec command.
	if execCmd, ok := task.Config["command"]; ok {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--exec=%v", execCmd))
	}

	if task.Resources.MemoryMB == 0 {
		return nil, fmt.Errorf("Memory limit cannot be zero")
	}
	if task.Resources.CPU == 0 {
		return nil, fmt.Errorf("CPU limit cannot be zero")
	}

	// Add memory isolator
	cmdArgs = append(cmdArgs, fmt.Sprintf("--memory=%vM", int64(task.Resources.MemoryMB)*bytesToMB))

	// Add CPU isolator
	cmdArgs = append(cmdArgs, fmt.Sprintf("--cpu=%vm", int64(task.Resources.CPU)))

	// Add user passed arguments.
	if len(driverConfig.Args) != 0 {
		parsed := args.ParseAndReplace(driverConfig.Args, envVars.Map())

		// Need to start arguments with "--"
		if len(parsed) > 0 {
			cmdArgs = append(cmdArgs, "--")
		}

		for _, arg := range parsed {
			cmdArgs = append(cmdArgs, fmt.Sprintf("%v", arg))
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

	cmd := exec.Command("rkt", cmdArgs...)
	cmd.Stdout = stdo
	cmd.Stderr = stde

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Error running rkt: %v", err)
	}

	d.logger.Printf("[DEBUG] driver.rkt: started ACI %q with: %v", img, cmd.Args)
	h := &rktHandle{
		proc:        cmd.Process,
		image:       img,
		logger:      d.logger,
		killTimeout: d.DriverContext.KillTimeout(task),
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *cstructs.WaitResult, 1),
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
		proc:        proc,
		image:       qpid.Image,
		logger:      d.logger,
		killTimeout: qpid.KillTimeout,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *cstructs.WaitResult, 1),
	}

	go h.run()
	return h, nil
}

func (h *rktHandle) ID() string {
	// Return a handle to the PID
	pid := &rktPID{
		Pid:         h.proc.Pid,
		Image:       h.image,
		KillTimeout: h.killTimeout,
	}
	data, err := json.Marshal(pid)
	if err != nil {
		h.logger.Printf("[ERR] driver.rkt: failed to marshal rkt PID to JSON: %s", err)
	}
	return fmt.Sprintf("Rkt:%s", string(data))
}

func (h *rktHandle) WaitCh() chan *cstructs.WaitResult {
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
	case <-time.After(h.killTimeout):
		return h.proc.Kill()
	}
}

func (h *rktHandle) run() {
	ps, err := h.proc.Wait()
	close(h.doneCh)
	code := 0
	if !ps.Success() {
		// TODO: Better exit code parsing.
		code = 1
	}
	h.waitCh <- cstructs.NewWaitResult(code, 0, err)
	close(h.waitCh)
}
