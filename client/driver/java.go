package driver

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/executor"
	"github.com/hashicorp/nomad/nomad/structs"
)

// JavaDriver is a simple driver to execute applications packaged in Jars.
// It literally just fork/execs tasks with the java command.
type JavaDriver struct {
	DriverContext
}

// javaHandle is returned from Start/Open as a handle to the PID
type javaHandle struct {
	cmd    executor.Executor
	waitCh chan error
	doneCh chan struct{}
}

// NewJavaDriver is used to create a new exec driver
func NewJavaDriver(ctx *DriverContext) Driver {
	return &JavaDriver{*ctx}
}

func (d *JavaDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Only enable if we are root when running on non-windows systems.
	if runtime.GOOS != "windows" && syscall.Geteuid() != 0 {
		d.logger.Printf("[DEBUG] driver.java: must run as root user, disabling")
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
	// Get the jar source
	source, ok := task.Config["jar_source"]
	if !ok || source == "" {
		return nil, fmt.Errorf("missing jar source for Java Jar driver")
	}

	// Attempt to download the thing
	// Should be extracted to some kind of Http Fetcher
	// Right now, assume publicly accessible HTTP url
	resp, err := http.Get(source)
	if err != nil {
		return nil, fmt.Errorf("Error downloading source for Java driver: %s", err)
	}

	// Get the tasks local directory.
	taskDir, ok := ctx.AllocDir.TaskDirs[d.DriverContext.taskName]
	if !ok {
		return nil, fmt.Errorf("Could not find task directory for task: %v", d.DriverContext.taskName)
	}
	taskLocal := filepath.Join(taskDir, allocdir.TaskLocal)

	// Create a location to download the binary.
	fName := path.Base(source)
	fPath := filepath.Join(taskLocal, fName)
	f, err := os.OpenFile(fPath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("Error opening file to download to: %s", err)
	}

	defer f.Close()
	defer resp.Body.Close()

	// Copy remote file to local directory for execution
	// TODO: a retry of sort if io.Copy fails, for large binaries
	_, ioErr := io.Copy(f, resp.Body)
	if ioErr != nil {
		return nil, fmt.Errorf("Error copying jar from source: %s", ioErr)
	}

	// Get the environment variables.
	envVars := TaskEnvironmentVariables(ctx, task)

	args := []string{}
	// Look for jvm options
	jvm_options, ok := task.Config["jvm_options"]
	if ok && jvm_options != "" {
		d.logger.Printf("[DEBUG] driver.java: found JVM options: %s", jvm_options)
	}

	// Build the argument list
	args = append(args, "-jar", filepath.Join(allocdir.TaskLocal, fName))

	// Build the argument list.
	if argRaw, ok := task.Config["args"]; ok {
		args = append(args, argRaw)
	}

	// Setup the command
	// Assumes Java is in the $PATH, but could probably be detected
	cmd := executor.Command("java", args...)

	// Populate environment variables
	cmd.Command().Env = envVars.List()

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
		cmd:    cmd,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}

	go h.run()
	return h, nil
}

func (d *JavaDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
	// Find the process
	cmd, err := executor.OpenId(handleID)
	if err != nil {
		return nil, fmt.Errorf("failed to open ID %v: %v", handleID, err)
	}

	// Return a driver handle
	h := &javaHandle{
		cmd:    cmd,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}

	go h.run()
	return h, nil
}

func (h *javaHandle) ID() string {
	id, _ := h.cmd.ID()
	return id
}

func (h *javaHandle) WaitCh() chan error {
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
	case <-time.After(5 * time.Second):
		return h.cmd.ForceStop()
	}
}

func (h *javaHandle) run() {
	err := h.cmd.Wait()
	close(h.doneCh)
	if err != nil {
		h.waitCh <- err
	}
	close(h.waitCh)
}
