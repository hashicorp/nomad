package driver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

var (
	reRktVersion  = regexp.MustCompile(`rkt [vV]ersion[:]? (\d[.\d]+)`)
	reAppcVersion = regexp.MustCompile(`appc [vV]ersion[:]? (\d[.\d]+)`)
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
	pluginClient   *plugin.Client
	executorPid    int
	executor       executor.Executor
	allocDir       *allocdir.AllocDir
	logger         *log.Logger
	killTimeout    time.Duration
	maxKillTimeout time.Duration
	waitCh         chan *cstructs.WaitResult
	doneCh         chan struct{}
}

// rktPID is a struct to map the pid running the process to the vm image on
// disk
type rktPID struct {
	PluginConfig   *PluginReattachConfig
	AllocDir       *allocdir.AllocDir
	ExecutorPid    int
	KillTimeout    time.Duration
	MaxKillTimeout time.Duration
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

	// Build the command.
	var cmdArgs []string

	// Add the given trust prefix
	trustPrefix, trustCmd := task.Config["trust_prefix"]
	insecure := false
	if trustCmd {
		var outBuf, errBuf bytes.Buffer
		cmd := exec.Command("rkt", "trust", "--skip-fingerprint-review=true", fmt.Sprintf("--prefix=%s", trustPrefix))
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("Error running rkt trust: %s\n\nOutput: %s\n\nError: %s",
				err, outBuf.String(), errBuf.String())
		}
		d.logger.Printf("[DEBUG] driver.rkt: added trust prefix: %q", trustPrefix)
	} else {
		// Disble signature verification if the trust command was not run.
		insecure = true
	}

	cmdArgs = append(cmdArgs, "run", "--interactive")
	cmdArgs = append(cmdArgs, fmt.Sprintf("--volume=%s,kind=host,source=%s", task.Name, ctx.AllocDir.SharedDir))
	cmdArgs = append(cmdArgs, fmt.Sprintf("--mount=volume=%s,target=%s", task.Name, ctx.AllocDir.SharedDir))
	cmdArgs = append(cmdArgs, img)
	if insecure == true {
		cmdArgs = append(cmdArgs, "--insecure-options=all")
	}

	// Inject enviornment variables
	for k, v := range d.taskEnv.EnvMap() {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--set-env=%v=%v", k, v))
	}

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
		parsed := d.taskEnv.ParseAndReplace(driverConfig.Args)

		// Need to start arguments with "--"
		if len(parsed) > 0 {
			cmdArgs = append(cmdArgs, "--")
		}

		for _, arg := range parsed {
			cmdArgs = append(cmdArgs, fmt.Sprintf("%v", arg))
		}
	}

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
		TaskEnv:          d.taskEnv,
		AllocDir:         ctx.AllocDir,
		TaskName:         task.Name,
		TaskResources:    task.Resources,
		UnprivilegedUser: false,
		LogConfig:        task.LogConfig,
	}

	ps, err := exec.LaunchCmd(&executor.ExecCommand{Cmd: "rkt", Args: cmdArgs}, executorCtx)
	if err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("error starting process via the plugin: %v", err)
	}

	d.logger.Printf("[DEBUG] driver.rkt: started ACI %q with: %v", img, cmdArgs)
	maxKill := d.DriverContext.config.MaxKillTimeout
	h := &rktHandle{
		pluginClient:   pluginClient,
		executor:       exec,
		executorPid:    ps.Pid,
		allocDir:       ctx.AllocDir,
		logger:         d.logger,
		killTimeout:    GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout: maxKill,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *cstructs.WaitResult, 1),
	}
	go h.run()
	return h, nil
}

func (d *RktDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	// Parse the handle
	pidBytes := []byte(strings.TrimPrefix(handleID, "Rkt:"))
	id := &rktPID{}
	if err := json.Unmarshal(pidBytes, id); err != nil {
		return nil, fmt.Errorf("failed to parse Rkt handle '%s': %v", handleID, err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: id.PluginConfig.PluginConfig(),
	}
	executor, pluginClient, err := createExecutor(pluginConfig, d.config.LogOutput, d.config)
	if err != nil {
		d.logger.Println("[ERROR] driver.rkt: error connecting to plugin so destroying plugin pid and user pid")
		if e := destroyPlugin(id.PluginConfig.Pid, id.ExecutorPid); e != nil {
			d.logger.Printf("[ERROR] driver.rkt: error destroying plugin and executor pid: %v", e)
		}
		return nil, fmt.Errorf("error connecting to plugin: %v", err)
	}

	// Return a driver handle
	h := &rktHandle{
		pluginClient:   pluginClient,
		executorPid:    id.ExecutorPid,
		allocDir:       id.AllocDir,
		executor:       executor,
		logger:         d.logger,
		killTimeout:    id.KillTimeout,
		maxKillTimeout: id.MaxKillTimeout,
		doneCh:         make(chan struct{}),
		waitCh:         make(chan *cstructs.WaitResult, 1),
	}

	go h.run()
	return h, nil
}

func (h *rktHandle) ID() string {
	// Return a handle to the PID
	pid := &rktPID{
		PluginConfig:   NewPluginReattachConfig(h.pluginClient.ReattachConfig()),
		KillTimeout:    h.killTimeout,
		MaxKillTimeout: h.maxKillTimeout,
		ExecutorPid:    h.executorPid,
		AllocDir:       h.allocDir,
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
	// Store the updated kill timeout.
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.maxKillTimeout)
	h.executor.UpdateLogConfig(task.LogConfig)

	// Update is not possible
	return nil
}

// Kill is used to terminate the task. We send an Interrupt
// and then provide a 5 second grace period before doing a Kill.
func (h *rktHandle) Kill() error {
	h.executor.ShutDown()
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(h.killTimeout):
		return h.executor.Exit()
	}
}

func (h *rktHandle) run() {
	ps, err := h.executor.Wait()
	close(h.doneCh)
	if ps.ExitCode == 0 && err != nil {
		if e := killProcess(h.executorPid); e != nil {
			h.logger.Printf("[ERROR] driver.rkt: error killing user process: %v", e)
		}
		if e := h.allocDir.UnmountAll(); e != nil {
			h.logger.Printf("[ERROR] driver.rkt: unmounting dev,proc and alloc dirs failed: %v", e)
		}
	}
	h.waitCh <- cstructs.NewWaitResult(ps.ExitCode, 0, err)
	close(h.waitCh)
	h.pluginClient.Kill()
}
