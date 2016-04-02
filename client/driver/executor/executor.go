package executor

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/consul"
	"github.com/hashicorp/nomad/client/driver/env"
	"github.com/hashicorp/nomad/client/driver/logging"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Executor is the interface which allows a driver to launch and supervise
// a process
type Executor interface {
	LaunchCmd(command *ExecCommand, ctx *ExecutorContext) (*ProcessState, error)
	LaunchSyslogServer(ctx *ExecutorContext) (*SyslogServerState, error)
	Wait() (*ProcessState, error)
	ShutDown() error
	Exit() error
	UpdateLogConfig(logConfig *structs.LogConfig) error
	UpdateTask(task *structs.Task) error
	SyncServices(ctx *ConsulContext) error
	DeregisterServices() error
	Version() (*ExecutorVersion, error)
}

// ConsulContext holds context to configure the consul client and run checks
type ConsulContext struct {
	// ConsulConfig is the configuration used to create a consul client
	ConsulConfig *consul.ConsulConfig

	// ContainerID is the ID of the container
	ContainerID string

	// TLSCert is the cert which docker client uses while interactng with the docker
	// daemon over TLS
	TLSCert string

	// TLSCa is the CA which the docker client uses while interacting with the docker
	// daeemon over TLS
	TLSCa string

	// TLSKey is the TLS key which the docker client uses while interacting with
	// the docker daemon
	TLSKey string

	// DockerEndpoint is the endpoint of the docker daemon
	DockerEndpoint string
}

// ExecutorContext holds context to configure the command user
// wants to run and isolate it
type ExecutorContext struct {
	// TaskEnv holds information about the environment of a Task
	TaskEnv *env.TaskEnvironment

	// AllocDir is the handle to do operations on the alloc dir of
	// the task
	AllocDir *allocdir.AllocDir

	// Task is the task whose executor is being launched
	Task *structs.Task

	// AllocID is the allocation id to which the task belongs
	AllocID string

	// Driver is the name of the driver that invoked the executor
	Driver string

	// PortUpperBound is the upper bound of the ports that we can use to start
	// the syslog server
	PortUpperBound uint

	// PortLowerBound is the lower bound of the ports that we can use to start
	// the syslog server
	PortLowerBound uint
}

// ExecCommand holds the user command, args, and other isolation related
// settings.
type ExecCommand struct {
	// Cmd is the command that the user wants to run.
	Cmd string

	// Args is the args of the command that the user wants to run.
	Args []string

	// FSIsolation determines whether the command would be run in a chroot.
	FSIsolation bool

	// User is the user which the executor uses to run the command.
	User string

	// ResourceLimits determines whether resource limits are enforced by the
	// executor.
	ResourceLimits bool
}

// ProcessState holds information about the state of a user process.
type ProcessState struct {
	Pid             int
	ExitCode        int
	Signal          int
	IsolationConfig *cstructs.IsolationConfig
	Time            time.Time
}

// SyslogServerState holds the address and islation information of a launched
// syslog server
type SyslogServerState struct {
	IsolationConfig *cstructs.IsolationConfig
	Addr            string
}

// ExecutorVersion is the version of the executor
type ExecutorVersion struct {
	Version string
}

func (v *ExecutorVersion) GoString() string {
	return v.Version
}

// UniversalExecutor is an implementation of the Executor which launches and
// supervises processes. In addition to process supervision it provides resource
// and file system isolation
type UniversalExecutor struct {
	cmd     exec.Cmd
	ctx     *ExecutorContext
	command *ExecCommand

	taskDir       string
	exitState     *ProcessState
	processExited chan interface{}

	lre         *logging.FileRotator
	lro         *logging.FileRotator
	rotatorLock sync.Mutex

	syslogServer *logging.SyslogServer
	syslogChan   chan *logging.SyslogMessage

	groups  *cgroupConfig.Cgroup
	cgPaths map[string]string
	cgLock  sync.Mutex

	consulService *consul.ConsulService
	consulCtx     *ConsulContext
	logger        *log.Logger
}

// NewExecutor returns an Executor
func NewExecutor(logger *log.Logger) Executor {
	return &UniversalExecutor{
		logger:        logger,
		processExited: make(chan interface{}),
	}
}

// Version returns the api version of the executor
func (e *UniversalExecutor) Version() (*ExecutorVersion, error) {
	return &ExecutorVersion{Version: "1.0.0"}, nil
}

// LaunchCmd launches a process and returns it's state. It also configures an
// applies isolation on certain platforms.
func (e *UniversalExecutor) LaunchCmd(command *ExecCommand, ctx *ExecutorContext) (*ProcessState, error) {
	e.logger.Printf("[DEBUG] executor: launching command %v %v", command.Cmd, strings.Join(command.Args, " "))

	e.ctx = ctx
	e.command = command

	// configuring the task dir
	if err := e.configureTaskDir(); err != nil {
		return nil, err
	}

	// configuring the chroot, cgroup and enters the plugin process in the
	// chroot
	if err := e.configureIsolation(); err != nil {
		return nil, err
	}

	// setting the user of the process
	if command.User != "" {
		e.logger.Printf("[DEBUG] executor: running command as %s", command.User)
		if err := e.runAs(command.User); err != nil {
			return nil, err
		}
	}

	// Setup the loggers
	if err := e.configureLoggers(); err != nil {
		return nil, err
	}
	e.cmd.Stdout = e.lro
	e.cmd.Stderr = e.lre

	e.ctx.TaskEnv.Build()

	// Look up the binary path and make it executable
	absPath, err := e.lookupBin(ctx.TaskEnv.ReplaceEnv(command.Cmd))
	if err != nil {
		return nil, err
	}

	if err := e.makeExecutable(absPath); err != nil {
		return nil, err
	}

	// Determine the path to run as it may have to be relative to the chroot.
	path := absPath
	if e.command.FSIsolation {
		rel, err := filepath.Rel(e.taskDir, absPath)
		if err != nil {
			return nil, err
		}
		path = rel
	}

	// Set the commands arguments
	e.cmd.Path = path
	e.cmd.Args = append([]string{path}, ctx.TaskEnv.ParseAndReplace(command.Args)...)
	e.cmd.Env = ctx.TaskEnv.EnvList()

	// Start the process
	if err := e.cmd.Start(); err != nil {
		return nil, err
	}
	if err := e.applyLimits(e.cmd.Process.Pid); err != nil {
		return nil, err
	}
	go e.wait()
	ic := &cstructs.IsolationConfig{Cgroup: e.groups, CgroupPaths: e.cgPaths}
	return &ProcessState{Pid: e.cmd.Process.Pid, ExitCode: -1, IsolationConfig: ic, Time: time.Now()}, nil
}

// configureLoggers sets up the standard out/error file rotators
func (e *UniversalExecutor) configureLoggers() error {
	e.rotatorLock.Lock()
	defer e.rotatorLock.Unlock()

	logFileSize := int64(e.ctx.Task.LogConfig.MaxFileSizeMB * 1024 * 1024)
	if e.lro == nil {
		lro, err := logging.NewFileRotator(e.ctx.AllocDir.LogDir(), fmt.Sprintf("%v.stdout", e.ctx.Task.Name),
			e.ctx.Task.LogConfig.MaxFiles, logFileSize, e.logger)
		if err != nil {
			return err
		}
		e.lro = lro
	}

	if e.lre == nil {
		lre, err := logging.NewFileRotator(e.ctx.AllocDir.LogDir(), fmt.Sprintf("%v.stderr", e.ctx.Task.Name),
			e.ctx.Task.LogConfig.MaxFiles, logFileSize, e.logger)
		if err != nil {
			return err
		}
		e.lre = lre
	}
	return nil
}

// Wait waits until a process has exited and returns it's exitcode and errors
func (e *UniversalExecutor) Wait() (*ProcessState, error) {
	<-e.processExited
	return e.exitState, nil
}

// COMPAT: prior to Nomad 0.3.2, UpdateTask didn't exist.
// UpdateLogConfig updates the log configuration
func (e *UniversalExecutor) UpdateLogConfig(logConfig *structs.LogConfig) error {
	e.ctx.Task.LogConfig = logConfig
	if e.lro == nil {
		return fmt.Errorf("log rotator for stdout doesn't exist")
	}
	e.lro.MaxFiles = logConfig.MaxFiles
	e.lro.FileSize = int64(logConfig.MaxFileSizeMB * 1024 * 1024)

	if e.lre == nil {
		return fmt.Errorf("log rotator for stderr doesn't exist")
	}
	e.lre.MaxFiles = logConfig.MaxFiles
	e.lre.FileSize = int64(logConfig.MaxFileSizeMB * 1024 * 1024)
	return nil
}

func (e *UniversalExecutor) UpdateTask(task *structs.Task) error {
	e.ctx.Task = task

	// Updating Log Config
	fileSize := int64(task.LogConfig.MaxFileSizeMB * 1024 * 1024)
	e.lro.MaxFiles = task.LogConfig.MaxFiles
	e.lro.FileSize = fileSize
	e.lre.MaxFiles = task.LogConfig.MaxFiles
	e.lre.FileSize = fileSize

	// Re-syncing task with consul service
	if e.consulService != nil {
		if err := e.consulService.SyncTask(task); err != nil {
			return err
		}
	}
	return nil
}

func (e *UniversalExecutor) wait() {
	defer close(e.processExited)
	err := e.cmd.Wait()
	ic := &cstructs.IsolationConfig{Cgroup: e.groups, CgroupPaths: e.cgPaths}
	if err == nil {
		e.exitState = &ProcessState{Pid: 0, ExitCode: 0, IsolationConfig: ic, Time: time.Now()}
		return
	}
	exitCode := 1
	var signal int
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
			if status.Signaled() {
				signal = int(status.Signal())
				exitCode = 128 + signal
			}
		}
	}
	e.exitState = &ProcessState{Pid: 0, ExitCode: exitCode, Signal: signal, IsolationConfig: ic, Time: time.Now()}
}

var (
	// finishedErr is the error message received when trying to kill and already
	// exited process.
	finishedErr = "os: process already finished"
)

// Exit cleans up the alloc directory, destroys cgroups and kills the user
// process
func (e *UniversalExecutor) Exit() error {
	var merr multierror.Error
	if e.syslogServer != nil {
		e.syslogServer.Shutdown()
	}
	e.lre.Close()
	e.lro.Close()

	if e.command != nil && e.cmd.Process != nil {
		proc, err := os.FindProcess(e.cmd.Process.Pid)
		if err != nil {
			e.logger.Printf("[ERR] executor: can't find process with pid: %v, err: %v",
				e.cmd.Process.Pid, err)
		} else if err := proc.Kill(); err != nil && err.Error() != finishedErr {
			merr.Errors = append(merr.Errors,
				fmt.Errorf("can't kill process with pid: %v, err: %v", e.cmd.Process.Pid, err))
		}
	}

	if e.command != nil && e.command.FSIsolation {
		if err := e.removeChrootMounts(); err != nil {
			merr.Errors = append(merr.Errors, err)
		}
	}
	if e.command != nil && e.command.ResourceLimits {
		e.cgLock.Lock()
		if err := DestroyCgroup(e.groups, e.cgPaths); err != nil {
			merr.Errors = append(merr.Errors, err)
		}
		e.cgLock.Unlock()
	}
	return merr.ErrorOrNil()
}

// Shutdown sends an interrupt signal to the user process
func (e *UniversalExecutor) ShutDown() error {
	if e.cmd.Process == nil {
		return fmt.Errorf("executor.shutdown error: no process found")
	}
	proc, err := os.FindProcess(e.cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("executor.shutdown error: %v", err)
	}
	if runtime.GOOS == "windows" {
		return proc.Kill()
	}
	if err = proc.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("executor.shutdown error: %v", err)
	}
	return nil
}

func (e *UniversalExecutor) SyncServices(ctx *ConsulContext) error {
	e.logger.Printf("[INFO] executor: registering services")
	e.consulCtx = ctx
	if e.consulService == nil {
		cs, err := consul.NewConsulService(ctx.ConsulConfig, e.logger, e.ctx.AllocID)
		if err != nil {
			return err
		}
		cs.SetDelegatedChecks(e.createCheckMap(), e.createCheck)
		e.consulService = cs
	}
	if e.ctx != nil {
		e.interpolateServices(e.ctx.Task)
	}
	err := e.consulService.SyncTask(e.ctx.Task)
	go e.consulService.PeriodicSync()
	return err
}

func (e *UniversalExecutor) DeregisterServices() error {
	e.logger.Printf("[INFO] executor: de-registering services and shutting down consul service")
	if e.consulService != nil {
		return e.consulService.Shutdown()
	}
	return nil
}

// configureTaskDir sets the task dir in the executor
func (e *UniversalExecutor) configureTaskDir() error {
	taskDir, ok := e.ctx.AllocDir.TaskDirs[e.ctx.Task.Name]
	e.taskDir = taskDir
	if !ok {
		return fmt.Errorf("couldn't find task directory for task %v", e.ctx.Task.Name)
	}
	e.cmd.Dir = taskDir
	return nil
}

// lookupBin looks for path to the binary to run by looking for the binary in
// the following locations, in-order: task/local/, task/, based on host $PATH.
// The return path is absolute.
func (e *UniversalExecutor) lookupBin(bin string) (string, error) {
	// Check in the local directory
	local := filepath.Join(e.taskDir, allocdir.TaskLocal, bin)
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	// Check at the root of the task's directory
	root := filepath.Join(e.taskDir, bin)
	if _, err := os.Stat(root); err == nil {
		return root, nil
	}

	// Check the $PATH
	if host, err := exec.LookPath(bin); err == nil {
		return host, nil
	}

	return "", fmt.Errorf("binary %q could not be found", bin)
}

// makeExecutable makes the given file executable for root,group,others.
func (e *UniversalExecutor) makeExecutable(binPath string) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	fi, err := os.Stat(binPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("binary %q does not exist", binPath)
		}
		return fmt.Errorf("specified binary is invalid: %v", err)
	}

	// If it is not executable, make it so.
	perm := fi.Mode().Perm()
	req := os.FileMode(0555)
	if perm&req != req {
		if err := os.Chmod(binPath, perm|req); err != nil {
			return fmt.Errorf("error making %q executable: %s", binPath, err)
		}
	}
	return nil
}

// getFreePort returns a free port ready to be listened on between upper and
// lower bounds
func (e *UniversalExecutor) getListener(lowerBound uint, upperBound uint) (net.Listener, error) {
	if runtime.GOOS == "windows" {
		return e.listenerTCP(lowerBound, upperBound)
	}

	return e.listenerUnix()
}

// listenerTCP creates a TCP listener using an unused port between an upper and
// lower bound
func (e *UniversalExecutor) listenerTCP(lowerBound uint, upperBound uint) (net.Listener, error) {
	for i := lowerBound; i <= upperBound; i++ {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("localhost:%v", i))
		if err != nil {
			return nil, err
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			continue
		}
		return l, nil
	}
	return nil, fmt.Errorf("No free port found")
}

// listenerUnix creates a Unix domain socket
func (e *UniversalExecutor) listenerUnix() (net.Listener, error) {
	f, err := ioutil.TempFile("", "plugin")
	if err != nil {
		return nil, err
	}
	path := f.Name()

	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.Remove(path); err != nil {
		return nil, err
	}

	return net.Listen("unix", path)
}

// createCheckMap creates a map of checks that the executor will handle on it's
// own
func (e *UniversalExecutor) createCheckMap() map[string]struct{} {
	checks := map[string]struct{}{
		"script": struct{}{},
	}
	return checks
}

// createCheck creates NomadCheck from a ServiceCheck
func (e *UniversalExecutor) createCheck(check *structs.ServiceCheck, checkID string) (consul.Check, error) {
	if check.Type == structs.ServiceCheckScript && e.ctx.Driver == "docker" {
		return &DockerScriptCheck{
			id:          checkID,
			interval:    check.Interval,
			containerID: e.consulCtx.ContainerID,
			logger:      e.logger,
			cmd:         check.Command,
			args:        check.Args,
		}, nil
	}

	if check.Type == structs.ServiceCheckScript && e.ctx.Driver == "exec" {
		return &ExecScriptCheck{
			id:          checkID,
			interval:    check.Interval,
			cmd:         check.Command,
			args:        check.Args,
			taskDir:     e.taskDir,
			FSIsolation: e.command.FSIsolation,
		}, nil

	}
	return nil, fmt.Errorf("couldn't create check for %v", check.Name)
}

// interpolateServices interpolates tags in a service and checks with values from the
// task's environment.
func (e *UniversalExecutor) interpolateServices(task *structs.Task) {
	e.ctx.TaskEnv.Build()
	for _, service := range task.Services {
		for _, check := range service.Checks {
			if check.Type == structs.ServiceCheckScript {
				check.Name = e.ctx.TaskEnv.ReplaceEnv(check.Name)
				check.Command = e.ctx.TaskEnv.ReplaceEnv(check.Command)
				check.Args = e.ctx.TaskEnv.ParseAndReplace(check.Args)
				check.Path = e.ctx.TaskEnv.ReplaceEnv(check.Path)
				check.Protocol = e.ctx.TaskEnv.ReplaceEnv(check.Protocol)
			}
		}
		service.Name = e.ctx.TaskEnv.ReplaceEnv(service.Name)
		service.Tags = e.ctx.TaskEnv.ParseAndReplace(service.Tags)
	}
}
