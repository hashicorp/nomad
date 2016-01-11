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

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver/executor"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/client/getter"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
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
	cmd         executor.Executor
	killTimeout time.Duration
	logger      *log.Logger
	waitCh      chan *cstructs.WaitResult
	doneCh      chan struct{}
}

// NewJavaDriver is used to create a new exec driver
func NewJavaDriver(ctx *DriverContext) Driver {
	return &JavaDriver{DriverContext: *ctx}
}

func (d *JavaDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Only enable if we are root when running on non-windows systems.
	if runtime.GOOS == "linux" && syscall.Geteuid() != 0 {
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
		filepath.Join(taskDir, allocdir.TaskLocal),
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
	args = append(args, "-jar", filepath.Join(allocdir.TaskLocal, jarName))
	if len(driverConfig.Args) != 0 {
		args = append(args, driverConfig.Args...)
	}

	// Setup the command
	// Assumes Java is in the $PATH, but could probably be detected
	execCtx := executor.NewExecutorContext(d.taskEnv)
	cmd := executor.Command(execCtx, "java", args...)

	// Populate environment variables
	cmd.Command().Env = d.taskEnv.EnvList()

	if err := cmd.Limit(task.Resources); err != nil {
		return nil, fmt.Errorf("failed to constrain resources: %s", err)
	}

	if err := cmd.ConfigureTaskDir(d.taskName, ctx.AllocDir); err != nil {
		return nil, fmt.Errorf("failed to configure task directory: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start source: %v", err)
	}

	// Return a driver handle
	h := &javaHandle{
		cmd:         cmd,
		killTimeout: d.DriverContext.KillTimeout(task),
		logger:      d.logger,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *cstructs.WaitResult, 1),
	}

	go h.run()
	return h, nil
}

type javaId struct {
	ExecutorId  string
	KillTimeout time.Duration
}

func (d *JavaDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	id := &javaId{}
	if err := json.Unmarshal([]byte(handleID), id); err != nil {
		return nil, fmt.Errorf("Failed to parse handle '%s': %v", handleID, err)
	}

	// Find the process
	execCtx := executor.NewExecutorContext(d.taskEnv)
	cmd, err := executor.OpenId(execCtx, id.ExecutorId)
	if err != nil {
		return nil, fmt.Errorf("failed to open ID %v: %v", id.ExecutorId, err)
	}

	// Return a driver handle
	h := &javaHandle{
		cmd:         cmd,
		logger:      d.logger,
		killTimeout: id.KillTimeout,
		doneCh:      make(chan struct{}),
		waitCh:      make(chan *cstructs.WaitResult, 1),
	}

	go h.run()
	return h, nil
}

func (h *javaHandle) ID() string {
	executorId, _ := h.cmd.ID()
	id := javaId{
		ExecutorId:  executorId,
		KillTimeout: h.killTimeout,
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
	// Update is not possible
	return nil
}

func (h *javaHandle) Kill() error {
	h.cmd.Shutdown()
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(h.killTimeout):
		return h.cmd.ForceStop()
	}
}

func (h *javaHandle) run() {
	res := h.cmd.Wait()
	close(h.doneCh)
	h.waitCh <- res
	close(h.waitCh)
}
