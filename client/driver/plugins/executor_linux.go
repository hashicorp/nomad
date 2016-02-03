package plugins

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	//"github.com/opencontainers/runc/libcontainer/cgroups"
	//cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	//"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	//cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"

	"github.com/hashicorp/nomad/client/allocdir"
)

var (
	// A mapping of directories on the host OS to attempt to embed inside each
	// task's chroot.
	chrootEnv = map[string]string{
		"/bin":       "/bin",
		"/etc":       "/etc",
		"/lib":       "/lib",
		"/lib32":     "/lib32",
		"/lib64":     "/lib64",
		"/usr/bin":   "/usr/bin",
		"/usr/lib":   "/usr/lib",
		"/usr/share": "/usr/share",
	}
)

type LinuxExecutor struct {
	cmd exec.Cmd
	ctx *ExecutorContext

	//groups  *cgroupConfig.Cgroup
	taskDir string

	logger *log.Logger
	lock   sync.Mutex
}

func NewExecutor(logger *log.Logger) Executor {
	return &LinuxExecutor{logger: logger}
}

func (e *LinuxExecutor) LaunchCmd(command *ExecCommand, ctx *ExecutorContext) (*ProcessState, error) {
	e.ctx = ctx
	e.cmd.Path = command.Cmd
	e.cmd.Args = append([]string{command.Cmd}, command.Args...)
	if filepath.Base(command.Cmd) == command.Cmd {
		if lp, err := exec.LookPath(command.Cmd); err != nil {
		} else {
			e.cmd.Path = lp
		}
	}
	if err := e.configureTaskDir(); err != nil {
		return nil, err
	}
	if err := e.runAs("nobody"); err != nil {
		return nil, err
	}
	e.cmd.Env = ctx.TaskEnv.EnvList()

	stdoPath := filepath.Join(e.cmd.Dir, allocdir.TaskLocal, fmt.Sprintf("%v.stdout", ctx.Task.Name))
	stdo, err := os.OpenFile(stdoPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	e.cmd.Stdout = stdo

	stdePath := filepath.Join(e.cmd.Dir, allocdir.TaskLocal, fmt.Sprintf("%v.stderr", ctx.Task.Name))
	stde, err := os.OpenFile(stdePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	e.cmd.Stderr = stde

	e.configureChroot()

	if err := e.cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting command: %v", err)
	}

	return &ProcessState{Pid: e.cmd.Process.Pid, ExitCode: -1, Time: time.Now()}, nil
}

// ConfigureTaskDir creates the necessary directory structure for a proper
// chroot. cleanTaskDir should be called after.
func (e *LinuxExecutor) configureTaskDir() error {
	taskName := e.ctx.Task.Name
	allocDir := e.ctx.AllocDir
	taskDir, ok := allocDir.TaskDirs[taskName]
	if !ok {
		fmt.Errorf("Couldn't find task directory for task %v", taskName)
	}
	e.taskDir = taskDir

	if err := allocDir.MountSharedDir(taskName); err != nil {
		return err
	}

	if err := allocDir.Embed(taskName, chrootEnv); err != nil {
		return err
	}

	// Mount dev
	dev := filepath.Join(taskDir, "dev")
	if !e.pathExists(dev) {
		if err := os.Mkdir(dev, 0777); err != nil {
			return fmt.Errorf("Mkdir(%v) failed: %v", dev, err)
		}

		if err := syscall.Mount("none", dev, "devtmpfs", syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("Couldn't mount /dev to %v: %v", dev, err)
		}
	}

	// Mount proc
	proc := filepath.Join(taskDir, "proc")
	if !e.pathExists(proc) {
		if err := os.Mkdir(proc, 0777); err != nil {
			return fmt.Errorf("Mkdir(%v) failed: %v", proc, err)
		}

		if err := syscall.Mount("none", proc, "proc", syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("Couldn't mount /proc to %v: %v", proc, err)
		}
	}

	// Set the tasks AllocDir environment variable.
	e.ctx.TaskEnv.SetAllocDir(filepath.Join("/", allocdir.SharedAllocName)).SetTaskLocalDir(filepath.Join("/", allocdir.TaskLocal)).Build()
	return nil
}

// runAs takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func (e *LinuxExecutor) runAs(userid string) error {
	u, err := user.Lookup(userid)
	if err != nil {
		return fmt.Errorf("Failed to identify user %v: %v", userid, err)
	}

	// Convert the uid and gid
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert userid to uint32: %s", err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert groupid to uint32: %s", err)
	}

	// Set the command to run as that user and group.
	if e.cmd.SysProcAttr == nil {
		e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if e.cmd.SysProcAttr.Credential == nil {
		e.cmd.SysProcAttr.Credential = &syscall.Credential{}
	}
	e.cmd.SysProcAttr.Credential.Uid = uint32(uid)
	e.cmd.SysProcAttr.Credential.Gid = uint32(gid)

	return nil
}

// pathExists is a helper function to check if the path exists.
func (e *LinuxExecutor) pathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// configureChroot enters the user command into a chroot if specified in the
// config and on an OS that supports Chroots.
func (e *LinuxExecutor) configureChroot() {
	if !e.ctx.Chroot {
		return
	}
	if e.cmd.SysProcAttr == nil {
		e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	e.cmd.SysProcAttr.Chroot = e.taskDir
	e.cmd.Dir = "/"
}

// cleanTaskDir is an idempotent operation to clean the task directory and
// should be called when tearing down the task.
func (e *LinuxExecutor) cleanTaskDir() error {
	// Prevent a race between Wait/ForceStop
	e.lock.Lock()
	defer e.lock.Unlock()

	// Unmount dev.
	errs := new(multierror.Error)
	dev := filepath.Join(e.taskDir, "dev")
	if e.pathExists(dev) {
		if err := syscall.Unmount(dev, 0); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to unmount dev (%v): %v", dev, err))
		}

		if err := os.RemoveAll(dev); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to delete dev directory (%v): %v", dev, err))
		}
	}

	// Unmount
	// proc.
	proc := filepath.Join(e.taskDir, "proc")
	if e.pathExists(proc) {
		if err := syscall.Unmount(proc, 0); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to unmount proc (%v): %v", proc, err))
		}

		if err := os.RemoveAll(proc); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to delete proc directory (%v): %v", dev, err))
		}
	}

	return errs.ErrorOrNil()
}

func (e *LinuxExecutor) Wait() (*ProcessState, error) {
	err := e.cmd.Wait()
	if err == nil {
		return &ProcessState{Pid: 0, ExitCode: 0, Time: time.Now()}, nil
	}
	exitCode := 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}
	}
	e.cleanTaskDir()
	return &ProcessState{Pid: 0, ExitCode: exitCode, Time: time.Now()}, nil
}

func (e *LinuxExecutor) Exit() error {
	proc, err := os.FindProcess(e.cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failied to find user process %v: %v", e.cmd.Process.Pid, err)
	}
	e.cleanTaskDir()
	return proc.Kill()
}

func (e *LinuxExecutor) ShutDown() error {
	proc, err := os.FindProcess(e.cmd.Process.Pid)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return proc.Kill()
	}
	return proc.Signal(os.Interrupt)
}
