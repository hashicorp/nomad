package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-plugin"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/driver/executor"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/fields"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// The key populated in Node Attributes to indicate presence of the Java
	// driver
	javaDriverAttr = "driver.java"
)

// JavaDriver is a simple driver to execute applications packaged in Jars.
// It literally just fork/execs tasks with the java command.
type JavaDriver struct {
	DriverContext
	fingerprint.StaticFingerprinter

	// A tri-state boolean to know if the fingerprinting has happened and
	// whether it has been successful
	fingerprintSuccess *bool
}

type JavaDriverConfig struct {
	Class     string   `mapstructure:"class"`
	ClassPath string   `mapstructure:"class_path"`
	JarPath   string   `mapstructure:"jar_path"`
	JvmOpts   []string `mapstructure:"jvm_options"`
	Args      []string `mapstructure:"args"`
}

// javaHandle is returned from Start/Open as a handle to the PID
type javaHandle struct {
	pluginClient    *plugin.Client
	userPid         int
	executor        executor.Executor
	isolationConfig *dstructs.IsolationConfig
	taskDir         string

	killTimeout    time.Duration
	maxKillTimeout time.Duration
	version        string
	logger         *log.Logger
	waitCh         chan *dstructs.WaitResult
	doneCh         chan struct{}
}

// NewJavaDriver is used to create a new exec driver
func NewJavaDriver(ctx *DriverContext) Driver {
	return &JavaDriver{DriverContext: *ctx}
}

// Validate is used to validate the driver configuration
func (d *JavaDriver) Validate(config map[string]interface{}) error {
	fd := &fields.FieldData{
		Raw: config,
		Schema: map[string]*fields.FieldSchema{
			"class": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"class_path": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"jar_path": &fields.FieldSchema{
				Type: fields.TypeString,
			},
			"jvm_options": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
			"args": &fields.FieldSchema{
				Type: fields.TypeArray,
			},
		},
	}

	if err := fd.Validate(); err != nil {
		return err
	}

	return nil
}

func (d *JavaDriver) Abilities() DriverAbilities {
	return DriverAbilities{
		SendSignals: true,
		Exec:        true,
	}
}

func (d *JavaDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Only enable if we are root and cgroups are mounted when running on linux systems.
	if runtime.GOOS == "linux" && (syscall.Geteuid() != 0 || !cgroupsMounted(node)) {
		if d.fingerprintSuccess == nil || *d.fingerprintSuccess {
			d.logger.Printf("[DEBUG] driver.java: root priviledges and mounted cgroups required on linux, disabling")
		}
		delete(node.Attributes, "driver.java")
		d.fingerprintSuccess = helper.BoolToPtr(false)
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
		delete(node.Attributes, javaDriverAttr)
		d.fingerprintSuccess = helper.BoolToPtr(false)
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
		if d.fingerprintSuccess == nil || *d.fingerprintSuccess {
			d.logger.Println("[WARN] driver.java: error parsing Java version information, aborting")
		}
		delete(node.Attributes, javaDriverAttr)
		d.fingerprintSuccess = helper.BoolToPtr(false)
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
	node.Attributes[javaDriverAttr] = "1"
	node.Attributes["driver.java.version"] = versionString
	node.Attributes["driver.java.runtime"] = info[1]
	node.Attributes["driver.java.vm"] = info[2]
	d.fingerprintSuccess = helper.BoolToPtr(true)

	return true, nil
}

func (d *JavaDriver) Prestart(*ExecContext, *structs.Task) (*PrestartResponse, error) {
	return nil, nil
}

func NewJavaDriverConfig(task *structs.Task, env *env.TaskEnv) (*JavaDriverConfig, error) {
	var driverConfig JavaDriverConfig
	if err := mapstructure.WeakDecode(task.Config, &driverConfig); err != nil {
		return nil, err
	}

	// Interpolate everything
	driverConfig.Class = env.ReplaceEnv(driverConfig.Class)
	driverConfig.ClassPath = env.ReplaceEnv(driverConfig.ClassPath)
	driverConfig.JarPath = env.ReplaceEnv(driverConfig.JarPath)
	driverConfig.JvmOpts = env.ParseAndReplace(driverConfig.JvmOpts)
	driverConfig.Args = env.ParseAndReplace(driverConfig.Args)

	// Validate
	jarSpecified := driverConfig.JarPath != ""
	classSpecified := driverConfig.Class != ""
	if !jarSpecified && !classSpecified {
		return nil, fmt.Errorf("jar_path or class must be specified")
	}

	return &driverConfig, nil
}

func (d *JavaDriver) Start(ctx *ExecContext, task *structs.Task) (*StartResponse, error) {
	driverConfig, err := NewJavaDriverConfig(task, ctx.TaskEnv)
	if err != nil {
		return nil, err
	}

	args := []string{}

	// Look for jvm options
	if len(driverConfig.JvmOpts) != 0 {
		d.logger.Printf("[DEBUG] driver.java: found JVM options: %s", driverConfig.JvmOpts)
		args = append(args, driverConfig.JvmOpts...)
	}

	// Add the classpath
	if driverConfig.ClassPath != "" {
		args = append(args, "-cp", driverConfig.ClassPath)
	}

	// Add the jar
	if driverConfig.JarPath != "" {
		args = append(args, "-jar", driverConfig.JarPath)
	}

	// Add the class
	if driverConfig.Class != "" {
		args = append(args, driverConfig.Class)
	}

	// Add any args
	if len(driverConfig.Args) != 0 {
		args = append(args, driverConfig.Args...)
	}

	pluginLogFile := filepath.Join(ctx.TaskDir.Dir, "executor.out")
	executorConfig := &dstructs.ExecutorConfig{
		LogFile:  pluginLogFile,
		LogLevel: d.config.LogLevel,
	}

	execIntf, pluginClient, err := createExecutor(d.config.LogOutput, d.config, executorConfig)
	if err != nil {
		return nil, err
	}

	// Set the context
	executorCtx := &executor.ExecutorContext{
		TaskEnv: ctx.TaskEnv,
		Driver:  "java",
		AllocID: d.DriverContext.allocID,
		Task:    task,
		TaskDir: ctx.TaskDir.Dir,
		LogDir:  ctx.TaskDir.LogDir,
	}
	if err := execIntf.SetContext(executorCtx); err != nil {
		pluginClient.Kill()
		return nil, fmt.Errorf("failed to set executor context: %v", err)
	}

	absPath, err := GetAbsolutePath("java")
	if err != nil {
		return nil, err
	}

	execCmd := &executor.ExecCommand{
		Cmd:            absPath,
		Args:           args,
		FSIsolation:    true,
		ResourceLimits: true,
		User:           getExecutorUser(task),
	}
	ps, err := execIntf.LaunchCmd(execCmd)
	if err != nil {
		pluginClient.Kill()
		return nil, err
	}
	d.logger.Printf("[DEBUG] driver.java: started process with pid: %v", ps.Pid)

	// Return a driver handle
	maxKill := d.DriverContext.config.MaxKillTimeout
	h := &javaHandle{
		pluginClient:    pluginClient,
		executor:        execIntf,
		userPid:         ps.Pid,
		isolationConfig: ps.IsolationConfig,
		taskDir:         ctx.TaskDir.Dir,
		killTimeout:     GetKillTimeout(task.KillTimeout, maxKill),
		maxKillTimeout:  maxKill,
		version:         d.config.Version,
		logger:          d.logger,
		doneCh:          make(chan struct{}),
		waitCh:          make(chan *dstructs.WaitResult, 1),
	}
	go h.run()
	return &StartResponse{Handle: h}, nil
}

func (d *JavaDriver) Cleanup(*ExecContext, *CreatedResources) error { return nil }

type javaId struct {
	Version         string
	KillTimeout     time.Duration
	MaxKillTimeout  time.Duration
	PluginConfig    *PluginReattachConfig
	IsolationConfig *dstructs.IsolationConfig
	TaskDir         string
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
	exec, pluginClient, err := createExecutorWithConfig(pluginConfig, d.config.LogOutput)
	if err != nil {
		merrs := new(multierror.Error)
		merrs.Errors = append(merrs.Errors, err)
		d.logger.Println("[ERR] driver.java: error connecting to plugin so destroying plugin pid and user pid")
		if e := destroyPlugin(id.PluginConfig.Pid, id.UserPid); e != nil {
			merrs.Errors = append(merrs.Errors, fmt.Errorf("error destroying plugin and userpid: %v", e))
		}
		if id.IsolationConfig != nil {
			ePid := pluginConfig.Reattach.Pid
			if e := executor.ClientCleanup(id.IsolationConfig, ePid); e != nil {
				merrs.Errors = append(merrs.Errors, fmt.Errorf("destroying resource container failed: %v", e))
			}
		}

		return nil, fmt.Errorf("error connecting to plugin: %v", merrs.ErrorOrNil())
	}

	ver, _ := exec.Version()
	d.logger.Printf("[DEBUG] driver.java: version of executor: %v", ver.Version)

	// Return a driver handle
	h := &javaHandle{
		pluginClient:    pluginClient,
		executor:        exec,
		userPid:         id.UserPid,
		isolationConfig: id.IsolationConfig,
		logger:          d.logger,
		version:         id.Version,
		killTimeout:     id.KillTimeout,
		maxKillTimeout:  id.MaxKillTimeout,
		doneCh:          make(chan struct{}),
		waitCh:          make(chan *dstructs.WaitResult, 1),
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
		IsolationConfig: h.isolationConfig,
		TaskDir:         h.taskDir,
	}

	data, err := json.Marshal(id)
	if err != nil {
		h.logger.Printf("[ERR] driver.java: failed to marshal ID to JSON: %s", err)
	}
	return string(data)
}

func (h *javaHandle) WaitCh() chan *dstructs.WaitResult {
	return h.waitCh
}

func (h *javaHandle) Update(task *structs.Task) error {
	// Store the updated kill timeout.
	h.killTimeout = GetKillTimeout(task.KillTimeout, h.maxKillTimeout)
	h.executor.UpdateTask(task)

	// Update is not possible
	return nil
}

func (h *javaHandle) Exec(ctx context.Context, cmd string, args []string) ([]byte, int, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		// No deadline set on context; default to 1 minute
		deadline = time.Now().Add(time.Minute)
	}
	return h.executor.Exec(deadline, cmd, args)
}

func (h *javaHandle) Signal(s os.Signal) error {
	return h.executor.Signal(s)
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
	case <-time.After(h.killTimeout):
		if h.pluginClient.Exited() {
			break
		}
		if err := h.executor.Exit(); err != nil {
			return fmt.Errorf("executor Exit failed: %v", err)
		}

	}
	return nil
}

func (h *javaHandle) Stats() (*cstructs.TaskResourceUsage, error) {
	return h.executor.Stats()
}

func (h *javaHandle) run() {
	ps, werr := h.executor.Wait()
	close(h.doneCh)
	if ps.ExitCode == 0 && werr != nil {
		if h.isolationConfig != nil {
			ePid := h.pluginClient.ReattachConfig().Pid
			if e := executor.ClientCleanup(h.isolationConfig, ePid); e != nil {
				h.logger.Printf("[ERR] driver.java: destroying resource container failed: %v", e)
			}
		} else {
			if e := killProcess(h.userPid); e != nil {
				h.logger.Printf("[ERR] driver.java: error killing user process: %v", e)
			}
		}
	}

	// Exit the executor
	h.executor.Exit()
	h.pluginClient.Kill()

	// Send the results
	h.waitCh <- &dstructs.WaitResult{ExitCode: ps.ExitCode, Signal: ps.Signal, Err: werr}
	close(h.waitCh)
}
