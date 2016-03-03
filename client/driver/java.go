package driver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/client/getter"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/nomad/structs"
)

// JavaDriver is a simple driver to execute applications packaged in Jars.
// It literally just fork/execs tasks with the java command.
type JavaDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter
}

type JavaDriverConfig struct {
	JvmOpts        []string `mapstructure:"jvm_options"`
	ArtifactSource string   `mapstructure:"artifact_source"`
	Checksum       string   `mapstructure:"checksum"`
	Args           []string `mapstructure:"args"`
}

// javaHandle is returned from Start/Open as a handle to the PID
type javaHandle struct {
	pluginClient    *plugin.Client
	userPid         int
	executor        executor.Executor
	isolationConfig *cstructs.IsolationConfig

	taskDir        string
	allocDir       *allocdir.AllocDir
	killTimeout    time.Duration
	maxKillTimeout time.Duration
	version        string
	logger         *log.Logger
	waitCh         chan *cstructs.WaitResult
	doneCh         chan struct{}
}

// NewJavaDriver is used to create a new exec driver
func NewJavaDriver(ctx *DriverContext) Driver {
	return &JavaDriver{DriverContext: *ctx}
}

func (d *JavaDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Only enable if we are root and cgroups are mounted when running on linux systems.
	if runtime.GOOS == "linux" && (syscall.Geteuid() != 0 || !d.cgroupsMounted(node)) {
		d.logger.Printf("[DEBUG] driver.java: must run as root user on linux, disabling")
		return false, nil
	}

	// Find java version
	var out bytes.Buffer
	var erOut bytes.Buffer
	cmd := exec.Command("java", "-version")
	cmd.Stdout = &out
	cmd.Stderr = &erOut
	err := cmd.Run()
	if err != nil {
		// assume Java wasn't found
		return false, nil
	}

	// 'java -version' returns output on Stderr typically.
	// Check stdout, but it's probably empty
	var infoString string
	if out.String() != "" {
		infoString = out.String()
	}

	if erOut.String() != "" {
		infoString = erOut.String()
	}

	if infoString == "" {
		d.logger.Println("[WARN] driver.java: error parsing Java version information, aborting")
		return false, nil
	}

	// Assume 'java -version' returns 3 lines:
	//    java version "1.6.0_36"
	//    OpenJDK Runtime Environment (IcedTea6 1.13.8) (6b36-1.13.8-0ubuntu1~12.04)
	//    OpenJDK 64-Bit Server VM (build 23.25-b01, mixed mode)
	// Each line is terminated by \n
	info := strings.Split(infoString, "\n")
	versionString := info[0]
	versionString = strings.TrimPrefix(versionString, "java version ")
	versionString = strings.Trim(versionString, "\"")
	node.Attributes["driver.java"] = "1"
	node.Attributes["driver.java.version"] = versionString
	node.Attributes["driver.java.runtime"] = info[1]
	node.Attributes["driver.java.vm"] = info[2]

	return true, nil
}

func (d *JavaDriver) Start(ctx *ExecContext, task *structs.Task) (DriverHandle, error) {
	var driverConfig JavaDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}
	taskDir, ok := ctx.AllocDir.TaskDirs[d.DriverContext.taskName]
	if !ok {
		return nil, fmt.Errorf("Could not find task directory for task: %v", d.DriverContext.taskName)
	}

	// Proceed to download an artifact to be executed.
	path, err := getter.GetArtifact(
		taskDir,
		driverConfig.ArtifactSource,
		driverConfig.Checksum,
		d.logger,
	)
	if err != nil {
		return nil, err
	}

	jarName := filepath.Base(path)

	args := []string{}
	// Look for jvm options
	if len(driverConfig.JvmOpts) != 0 {
		d.logger.Printf("[DEBUG] driver.java: found JVM options: %s", driverConfig.JvmOpts)
		args = append(args, driverConfig.JvmOpts...)
	}

	// Build the argument list.
	args = append(args, "-jar", jarName)
	if len(driverConfig.Args) != 0 {
		args = append(args, driverConfig.Args...)
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
		LogConfig:        task.LogConfig,
		FSIsolation:      true,
		UnprivilegedUser: true,
		ResourceLimits:   true,
	}

	ps, err := exec.LaunchCmd(&executor.ExecCommand{Cmd: "java", Args: args}, executorCtx)
	if err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("error starting process via the plugin: %v", err)
	}
	d.logger.Printf("[DEBUG] driver.java: started process with pid: %v", ps.Pid)

	// Return a driver handle
	maxKill := d.DriverContext.config.MaxKillTimeout
	h := &javaHandle{
		pluginClient:    pluginClient,
		executor:        exec,
		userPid:         ps.Pid,
		isolationConfig: ps.IsolationConfig,
		taskDir:         taskDir,
		allocDir:        ctx.AllocDir,
		killTimeout:     GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout:  maxKill,
		version:         d.config.Version,
		logger:          d.logger,
		doneCh:          make(chan struct{}),
		waitCh:          make(chan *cstructs.WaitResult, 1),
	}

	go h.run()
	return h, nil
}

// cgroupsMounted returns true if the cgroups are mounted on a system otherwise
// returns false
func (d *JavaDriver) cgroupsMounted(node *structs.Node) bool {
	_, ok := node.Attributes["unique.cgroup.mountpoint"]
	return ok
}

type javaId struct {
	Version         string
	KillTimeout     time.Duration
	MaxKillTimeout  time.Duration
	PluginConfig    *PluginReattachConfig
	IsolationConfig *cstructs.IsolationConfig
	TaskDir         string
	AllocDir        *allocdir.AllocDir
	UserPid         int
}

func (d *JavaDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &javaId{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	pluginConfig := &plugin.ClientConfig{
		Reattach: id.PluginConfig.PluginConfig(),
	}
	exec, pluginClient, err := createExecutor(pluginConfig, d.config.LogOutput, d.config)
	if err != nil {
		merrs := new(multierror.Error)
		merrs.Errors = append(merrs.Errors, err)
		d.logger.Println("[ERR] driver.java: error connecting to plugin so destroying plugin pid and user pid")
		if e := destroyPlugin(id.PluginConfig.Pid, id.UserPid); e != nil {
			merrs.Errors = append(merrs.Errors, fmt.Errorf("error destroying plugin and userpid: %v", e))
		}
		if id.IsolationConfig != nil {
			if e := executor.DestroyCgroup(id.IsolationConfig.Cgroup); e != nil {
				merrs.Errors = append(merrs.Errors, fmt.Errorf("destroying cgroup failed: %v", e))
			}
		}
		if e := ctx.AllocDir.UnmountAll(); e != nil {
			merrs.Errors = append(merrs.Errors, e)
		}

		return nil, fmt.Errorf("error connecting to plugin: %v", merrs.ErrorOrNil())
	}

	// Return a driver handle
	h := &javaHandle{
		pluginClient:    pluginClient,
		executor:        exec,
		userPid:         id.UserPid,
		isolationConfig: id.IsolationConfig,
		taskDir:         id.TaskDir,
		allocDir:        id.AllocDir,
		logger:          d.logger,
		version:         id.Version,
		killTimeout:     id.KillTimeout,
		maxKillTimeout:  id.MaxKillTimeout,
		doneCh:          make(chan struct{}),
		waitCh:          make(chan *cstructs.WaitResult, 1),
	}

	go h.run()
	return h, nil
}

func (h *javaHandle) ID() string {
	id := javaId{
		Version:         h.version,
		KillTimeout:     h.killTimeout,
		MaxKillTimeout:  h.maxKillTimeout,
		PluginConfig:    NewPluginReattachConfig(h.pluginClient.ReattachConfig()),
		UserPid:         h.userPid,
		TaskDir:         h.taskDir,
		AllocDir:        h.allocDir,
		IsolationConfig: h.isolationConfig,
	}

	data, err := json.Marshal(id)
	if err != nil {
		h.logger.Printf("[ERR] driver.java: failed to marshal ID to JSON: %s", err)
	}
	return string(data)
}

func (h *javaHandle) WaitCh() chan *cstructs.WaitResult {
	return h.waitCh
}

func (h *javaHandle) Update(task *structs.Task) error {
	// Store the updated kill timeout.
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.maxKillTimeout)
	h.executor.UpdateLogConfig(task.LogConfig)

	// Update is not possible
	return nil
}

func (h *javaHandle) Kill() error {
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

func (h *javaHandle) run() {
	ps, err := h.executor.Wait()
	close(h.doneCh)
	if ps.ExitCode == 0 && err != nil {
		if h.isolationConfig != nil {
			if e := executor.DestroyCgroup(h.isolationConfig.Cgroup); e != nil {
				h.logger.Printf("[ERR] driver.java: destroying cgroup failed while killing cgroup: %v", e)
			}
		} else {
			if e := killProcess(h.userPid); e != nil {
				h.logger.Printf("[ERR] driver.java: error killing user process: %v", e)
			}
		}
		if e := h.allocDir.UnmountAll(); e != nil {
			h.logger.Printf("[ERR] driver.java: unmounting dev,proc and alloc dirs failed: %v", e)
		}
	}
	h.waitCh <- &cstructs.WaitResult{ExitCode: ps.ExitCode, Signal: 0, Err: err}
	close(h.waitCh)
	h.pluginClient.Kill()
}
