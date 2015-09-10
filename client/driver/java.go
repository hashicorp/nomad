package driver

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// JavaDriver is a simple driver to execute applications packaged in Jars.
// It literally just fork/execs tasks with the java command.
type JavaDriver struct {
	logger *log.Logger
}

// javaHandle is returned from Start/Open as a handle to the PID
type javaHandle struct {
	proc   *os.Process
	waitCh chan error
	doneCh chan struct{}
}

// NewJavaDriver is used to create a new exec driver
func NewJavaDriver(logger *log.Logger) Driver {
	d := &JavaDriver{
		logger: logger,
	}
	return d
}

func (d *JavaDriver) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
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
		d.logger.Println("[WARN] Error parsing Java version information, aborting")
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

	fPath := filepath.Join(ctx.AllocDir, path.Base(source))
	f, err := os.OpenFile(fPath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("Error opening file to download to: %s", err)
	}

	defer f.Close()
	defer resp.Body.Close()

	// Copy remote file to local AllocDir for execution
	// TODO: a retry of sort if io.Copy fails, for large binaries
	_, ioErr := io.Copy(f, resp.Body)
	if ioErr != nil {
		return nil, fmt.Errorf("Error copying jar from source: %s", ioErr)
	}

	// Look for arguments
	argRaw, ok := task.Config["args"]
	var userArgs []string
	if ok {
		userArgs = strings.Split(argRaw, " ")
	}
	args := []string{"-jar", f.Name()}

	for _, s := range userArgs {
		args = append(args, s)
	}

	// Setup the command
	// Assumes Java is in the $PATH, but could probably be detected
	cmd := exec.Command("java", args...)
	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start source: %v", err)
	}

	// Return a driver handle
	h := &javaHandle{
		proc:   cmd.Process,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}

	go h.run()
	return h, nil
}

func (d *JavaDriver) Open(ctx *ExecContext, handleID string) (DriverHandle, error) {
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
	h := &javaHandle{
		proc:   proc,
		doneCh: make(chan struct{}),
		waitCh: make(chan error, 1),
	}

	go h.run()
	return h, nil
}

func (h *javaHandle) ID() string {
	// Return a handle to the PID
	return fmt.Sprintf("PID:%d", h.proc.Pid)
}

func (h *javaHandle) WaitCh() chan error {
	return h.waitCh
}

func (h *javaHandle) Update(task *structs.Task) error {
	// Update is not possible
	return nil
}

// Kill is used to terminate the task. We send an Interrupt
// and then provide a 5 second grace period before doing a Kill.
func (h *javaHandle) Kill() error {
	h.proc.Signal(unix.SIGTERM)
	select {
	case <-h.doneCh:
		return nil
	case <-time.After(5 * time.Second):
		return h.proc.Kill()
	}
}

func (h *javaHandle) run() {
	ps, err := h.proc.Wait()
	close(h.doneCh)
	if err != nil {
		h.waitCh <- err
	} else if !ps.Success() {
		h.waitCh <- fmt.Errorf("task exited with error")
	}
	close(h.waitCh)
}
