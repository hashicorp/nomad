package executor

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

// configureIsolation configures chroot and creates cgroups
func (e *UniversalExecutor) configureIsolation() error {
	if e.ctx.FSIsolation {
		if err := e.configureChroot(); err != nil {
			return err
		}
	}

	if e.ctx.ResourceLimits {
		if err := e.configureCgroups(e.ctx.TaskResources); err != nil {
			return fmt.Errorf("error creating cgroups: %v", err)
		}
		if err := e.applyLimits(os.Getpid()); err != nil {
			if er := DestroyCgroup(e.groups); er != nil {
				e.logger.Printf("[ERR] executor: error destroying cgroup: %v", er)
			}
			if er := e.removeChrootMounts(); er != nil {
				e.logger.Printf("[ERR] executor: error removing chroot: %v", er)
			}
			return fmt.Errorf("error entering the plugin process in the cgroup: %v:", err)
		}
	}
	return nil
}

// applyLimits puts a process in a pre-configured cgroup
func (e *UniversalExecutor) applyLimits(pid int) error {
	if !e.ctx.ResourceLimits {
		return nil
	}

	// Entering the process in the cgroup
	manager := getCgroupManager(e.groups)
	if err := manager.Apply(pid); err != nil {
		e.logger.Printf("[ERR] executor: unable to join cgroup: %v", err)
		if err := e.Exit(); err != nil {
			e.logger.Printf("[ERR] executor: unable to kill process: %v", err)
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
	cgroupName := structs.GenerateUUID()
	cgPath, err := cgroups.GetThisCgroupDir("devices")
	if err != nil {
		return fmt.Errorf("unable to get mount point for devices sub-system: %v", err)
	}
	e.groups.Path = filepath.Join(cgPath, cgroupName)

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

// configureChroot configures a chroot
func (e *UniversalExecutor) configureChroot() error {
	allocDir := e.ctx.AllocDir
	if err := allocDir.MountSharedDir(e.ctx.TaskName); err != nil {
		return err
	}

	if err := allocDir.Embed(e.ctx.TaskName, chrootEnv); err != nil {
		return err
	}

	// Set the tasks AllocDir environment variable.
	e.ctx.TaskEnv.SetAllocDir(filepath.Join("/", allocdir.SharedAllocName)).SetTaskLocalDir(filepath.Join("/", allocdir.TaskLocal)).Build()

	if e.cmd.SysProcAttr == nil {
		e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	e.cmd.SysProcAttr.Chroot = e.taskDir
	e.cmd.Dir = "/"

	if err := allocDir.MountSpecialDirs(e.taskDir); err != nil {
		return err
	}

	return nil
}

// cleanTaskDir is an idempotent operation to clean the task directory and
// should be called when tearing down the task.
func (e *UniversalExecutor) removeChrootMounts() error {
	// Prevent a race between Wait/ForceStop
	e.lock.Lock()
	defer e.lock.Unlock()
	return e.ctx.AllocDir.UnmountAll()
}

// destroyCgroup kills all processes in the cgroup and removes the cgroup
// configuration from the host.
func DestroyCgroup(groups *cgroupConfig.Cgroup) error {
	merrs := new(multierror.Error)
	if groups == nil {
		return fmt.Errorf("Can't destroy: cgroup configuration empty")
	}

	manager := getCgroupManager(groups)
	if pids, perr := manager.GetPids(); perr == nil {
		for _, pid := range pids {
			// If the pid is the pid of the executor then we don't kill it, the
			// executor is going to be killed by the driver once the Wait
			// returns
			if pid == os.Getpid() {
				continue
			}
			proc, err := os.FindProcess(pid)
			if err != nil {
				merrs.Errors = append(merrs.Errors, fmt.Errorf("error finding process %v: %v", pid, err))
			} else {
				if e := proc.Kill(); e != nil {
					merrs.Errors = append(merrs.Errors, fmt.Errorf("error killing process %v: %v", pid, e))
				}
			}
		}
	} else {
		merrs.Errors = append(merrs.Errors, fmt.Errorf("error getting pids: %v", perr))
	}

	// Remove the cgroup.
	if err := manager.Destroy(); err != nil {
		multierror.Append(merrs, fmt.Errorf("Failed to delete the cgroup directories: %v", err))
	}

	if len(merrs.Errors) != 0 {
		return fmt.Errorf("errors while destroying cgroup: %v", merrs)
	}
	return nil
}

// getCgroupManager returns the correct libcontainer cgroup manager.
func getCgroupManager(groups *cgroupConfig.Cgroup) cgroups.Manager {
	var manager cgroups.Manager
	manager = &cgroupFs.Manager{Cgroups: groups}
	if systemd.UseSystemd() {
		manager = &systemd.Manager{Cgroups: groups}
	}
	return manager
}
