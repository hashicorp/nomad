package plugins

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

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

func (e *UniversalExecutor) configureIsolation() error {
	if e.ctx.FSIsolation {
		if err := e.configureChroot(); err != nil {
			return err
		}
	}

	if e.ctx.ResourceLimits {
		if err := e.configureCgroups(e.ctx.Task.Resources); err != nil {
			return fmt.Errorf("error creating cgroups: %v", err)
		}
	}
	return nil
}

func (e *UniversalExecutor) applyLimits() error {
	if !e.ctx.ResourceLimits {
		return nil
	}
	manager := e.getCgroupManager(e.groups)
	if err := manager.Apply(e.cmd.Process.Pid); err != nil {
		e.logger.Printf("[ERROR] unable to join cgroup: %v", err)
		if err := e.Exit(); err != nil {
			e.logger.Printf("[ERROR] unable to kill process: %v", err)
		}
		return err
	}
	return nil
}

// configureCgroups converts a Nomad Resources specification into the equivalent
// cgroup configuration. It returns an error if the resources are invalid.
func (e *UniversalExecutor) configureCgroups(resources *structs.Resources) error {
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

// runAs takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func (e *UniversalExecutor) runAs(userid string) error {
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
func (e *UniversalExecutor) pathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func (e *UniversalExecutor) configureChroot() error {
	allocDir := e.ctx.AllocDir
	if err := allocDir.MountSharedDir(e.ctx.Task.Name); err != nil {
		return err
	}

	if err := allocDir.Embed(e.ctx.Task.Name, chrootEnv); err != nil {
		return err
	}

	// Mount dev
	dev := filepath.Join(e.taskDir, "dev")
	if !e.pathExists(dev) {
		if err := os.Mkdir(dev, 0777); err != nil {
			return fmt.Errorf("Mkdir(%v) failed: %v", dev, err)
		}

		if err := syscall.Mount("none", dev, "devtmpfs", syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("Couldn't mount /dev to %v: %v", dev, err)
		}
	}

	// Mount proc
	proc := filepath.Join(e.taskDir, "proc")
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

	if e.cmd.SysProcAttr == nil {
		e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	e.cmd.SysProcAttr.Chroot = e.taskDir
	e.cmd.Dir = "/"

	return nil
}

// cleanTaskDir is an idempotent operation to clean the task directory and
// should be called when tearing down the task.
func (e *UniversalExecutor) removeChrootMounts() error {
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

// destroyCgroup kills all processes in the cgroup and removes the cgroup
// configuration from the host.
func (e *UniversalExecutor) destroyCgroup() error {
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
func (e *UniversalExecutor) getCgroupManager(groups *cgroupConfig.Cgroup) cgroups.Manager {
	var manager cgroups.Manager
	manager = &cgroupFs.Manager{Cgroups: groups}
	if systemd.UseSystemd() {
		manager = &systemd.Manager{Cgroups: groups}
	}
	return manager
}
