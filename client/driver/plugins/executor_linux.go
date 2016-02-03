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
	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/structs"
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

	groups  *cgroupConfig.Cgroup
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

	stdoPath := filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stdout", ctx.Task.Name))
	stdo, err := os.OpenFile(stdoPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	e.cmd.Stdout = stdo

	stdePath := filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stderr", ctx.Task.Name))
	stde, err := os.OpenFile(stdePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	e.cmd.Stderr = stde

	e.configureChroot()

	if err := e.configureCgroups(e.ctx.Task.Resources); err != nil {
		return nil, fmt.Errorf("error creating cgroups: %v", err)
	}

	if err := e.cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting command: %v", err)
	}

	manager := e.getCgroupManager(e.groups)
	if err := manager.Apply(e.cmd.Process.Pid); err != nil {
		e.logger.Printf("[ERROR] unable to join cgroup: %v", err)
		if err := e.Exit(); err != nil {
			e.logger.Printf("[ERROR] unable to kill process: %v", err)
		}
		return nil, err
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
	e.destroyCgroup()
	return &ProcessState{Pid: 0, ExitCode: exitCode, Time: time.Now()}, nil
}

func (e *LinuxExecutor) Exit() error {
	e.logger.Printf("[INFO] Exiting plugin for task %q", e.ctx.Task.Name)
	proc, err := os.FindProcess(e.cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failied to find user process %v: %v", e.cmd.Process.Pid, err)
	}
	e.cleanTaskDir()
	e.destroyCgroup()
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

// configureCgroups converts a Nomad Resources specification into the equivalent
// cgroup configuration. It returns an error if the resources are invalid.
func (e *LinuxExecutor) configureCgroups(resources *structs.Resources) error {
	e.groups = &cgroupConfig.Cgroup{}
	e.groups.Resources = &cgroupConfig.Resources{}
	e.groups.Name = structs.GenerateUUID()

	// TODO: verify this is needed for things like network access
	e.groups.Resources.AllowAllDevices = true

	if resources.MemoryMB > 0 {
		// Total amount of memory allowed to consume
		e.groups.Resources.Memory = int64(resources.MemoryMB * 1024 * 1024)
		// Disable swap to avoid issues on the machine
		e.groups.Resources.MemorySwap = int64(-1)
	}

	if resources.CPU < 2 {
		return fmt.Errorf("resources.CPU must be equal to or greater than 2: %v", resources.CPU)
	}

	// Set the relative CPU shares for this cgroup.
	e.groups.Resources.CpuShares = int64(resources.CPU)

	if resources.IOPS != 0 {
		// Validate it is in an acceptable range.
		if resources.IOPS < 10 || resources.IOPS > 1000 {
			return fmt.Errorf("resources.IOPS must be between 10 and 1000: %d", resources.IOPS)
		}

		e.groups.Resources.BlkioWeight = uint16(resources.IOPS)
	}

	return nil
}

// destroyCgroup kills all processes in the cgroup and removes the cgroup
// configuration from the host.
func (e *LinuxExecutor) destroyCgroup() error {
	if e.groups == nil {
		return fmt.Errorf("Can't destroy: cgroup configuration empty")
	}

	// Prevent a race between Wait/ForceStop
	e.lock.Lock()
	defer e.lock.Unlock()

	manager := e.getCgroupManager(e.groups)
	pids, err := manager.GetPids()
	if err != nil {
		return fmt.Errorf("Failed to get pids in the cgroup %v: %v", e.groups.Name, err)
	}

	errs := new(multierror.Error)
	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			multierror.Append(errs, fmt.Errorf("Failed to find Pid %v: %v", pid, err))
			continue
		}

		if err := process.Kill(); err != nil && err.Error() != "os: process already finished" {
			multierror.Append(errs, fmt.Errorf("Failed to kill Pid %v: %v", pid, err))
			continue
		}
	}

	// Remove the cgroup.
	if err := manager.Destroy(); err != nil {
		multierror.Append(errs, fmt.Errorf("Failed to delete the cgroup directories: %v", err))
	}

	if len(errs.Errors) != 0 {
		return fmt.Errorf("Failed to destroy cgroup: %v", errs)
	}

	return nil
}

// getCgroupManager returns the correct libcontainer cgroup manager.
func (e *LinuxExecutor) getCgroupManager(groups *cgroupConfig.Cgroup) cgroups.Manager {
	var manager cgroups.Manager
	manager = &cgroupFs.Manager{Cgroups: groups}
	if systemd.UseSystemd() {
		manager = &systemd.Manager{Cgroups: groups}
	}
	return manager
}
