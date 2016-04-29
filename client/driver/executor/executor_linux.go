package executor

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/system"

	"github.com/hashicorp/nomad/client/allocdir"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (

	// A mapping of directories on the host OS to attempt to embed inside each
	// task's chroot.
	chrootEnv = map[string]string{
		"/bin":            "/bin",
		"/etc":            "/etc",
		"/lib":            "/lib",
		"/lib32":          "/lib32",
		"/lib64":          "/lib64",
		"/run/resolvconf": "/run/resolvconf",
		"/sbin":           "/sbin",
		"/usr":            "/usr",
	}

	clockTicks = uint64(system.GetClockTicks())

	nanosecondsInSecond = uint64(1000000000)
)

// configureIsolation configures chroot and creates cgroups
func (e *UniversalExecutor) configureIsolation() error {
	if e.command.FSIsolation {
		if err := e.configureChroot(); err != nil {
			return err
		}
	}

	if e.command.ResourceLimits {
		if err := e.configureCgroups(e.ctx.Task.Resources); err != nil {
			return fmt.Errorf("error creating cgroups: %v", err)
		}
	}
	return nil
}

// applyLimits puts a process in a pre-configured cgroup
func (e *UniversalExecutor) applyLimits(pid int) error {
	if !e.command.ResourceLimits {
		return nil
	}

	// Entering the process in the cgroup
	manager := getCgroupManager(e.groups, nil)
	if err := manager.Apply(pid); err != nil {
		e.logger.Printf("[ERR] executor: error applying pid to cgroup: %v", err)
		if er := e.removeChrootMounts(); er != nil {
			e.logger.Printf("[ERR] executor: error removing chroot: %v", er)
		}
		return err
	}
	e.cgPaths = manager.GetPaths()
	cgConfig := cgroupConfig.Config{Cgroups: e.groups}
	if err := manager.Set(&cgConfig); err != nil {
		e.logger.Printf("[ERR] executor: error setting cgroup config: %v", err)
		if er := DestroyCgroup(e.groups, e.cgPaths, os.Getpid()); er != nil {
			e.logger.Printf("[ERR] executor: error destroying cgroup: %v", er)
		}
		if er := e.removeChrootMounts(); er != nil {
			e.logger.Printf("[ERR] executor: error removing chroot: %v", er)
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
	e.groups.Path = filepath.Join("/nomad", cgroupName)

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

func (e *UniversalExecutor) Stats() (*cstructs.TaskResourceUsage, error) {
	manager := getCgroupManager(e.groups, e.cgPaths)
	stats, err := manager.GetStats()
	if err != nil {
		return nil, err
	}

	// Memory Related Stats
	rss := stats.MemoryStats.Stats["rss"]
	cache := stats.MemoryStats.Stats["cache"]
	ms := &cstructs.MemoryStats{
		RSS:   rss,
		Cache: cache,
	}

	// CPU Related Stats
	userModeTime := stats.CpuStats.CpuUsage.UsageInUsermode
	kernelModeTime := stats.CpuStats.CpuUsage.UsageInKernelmode

	umTicks := (userModeTime * clockTicks) / nanosecondsInSecond
	kmTicks := (kernelModeTime * clockTicks) / nanosecondsInSecond

	cs := &cstructs.CpuUsage{
		SystemMode: kmTicks,
		UserMode:   umTicks,
	}
	return &cstructs.TaskResourceUsage{MemoryStats: ms, CpuStats: cs}, nil
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
	if err := allocDir.MountSharedDir(e.ctx.Task.Name); err != nil {
		return err
	}

	if err := allocDir.Embed(e.ctx.Task.Name, chrootEnv); err != nil {
		return err
	}

	// Set the tasks AllocDir environment variable.
	e.ctx.TaskEnv.
		SetAllocDir(filepath.Join("/", allocdir.SharedAllocName)).
		SetTaskLocalDir(filepath.Join("/", allocdir.TaskLocal)).
		Build()

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
	e.cgLock.Lock()
	defer e.cgLock.Unlock()
	return e.ctx.AllocDir.UnmountAll()
}

// destroyCgroup kills all processes in the cgroup and removes the cgroup
// configuration from the host. This function is idempotent.
func DestroyCgroup(groups *cgroupConfig.Cgroup, cgPaths map[string]string, executorPid int) error {
	mErrs := new(multierror.Error)
	if groups == nil {
		return fmt.Errorf("Can't destroy: cgroup configuration empty")
	}

	// Move the executor into the global cgroup so that the task specific
	// cgroup can be destroyed.
	nilGroup := &cgroupConfig.Cgroup{}
	nilGroup.Path = "/"
	nilGroup.Resources = groups.Resources
	nilManager := getCgroupManager(nilGroup, nil)
	err := nilManager.Apply(executorPid)
	if err != nil && !strings.Contains(err.Error(), "no such process") {
		return fmt.Errorf("failed to remove executor pid %d: %v", executorPid, err)
	}

	// Freeze the Cgroup so that it can not continue to fork/exec.
	manager := getCgroupManager(groups, cgPaths)
	err = manager.Freeze(cgroupConfig.Frozen)
	if err != nil && !strings.Contains(err.Error(), "no such file or directory") {
		return fmt.Errorf("failed to freeze cgroup: %v", err)
	}

	var procs []*os.Process
	pids, err := manager.GetAllPids()
	if err != nil {
		multierror.Append(mErrs, fmt.Errorf("error getting pids: %v", err))

		// Unfreeze the cgroup.
		err = manager.Freeze(cgroupConfig.Thawed)
		if err != nil && !strings.Contains(err.Error(), "no such file or directory") {
			multierror.Append(mErrs, fmt.Errorf("failed to unfreeze cgroup: %v", err))
		}
		return mErrs.ErrorOrNil()
	}

	// Kill the processes in the cgroup
	for _, pid := range pids {
		proc, err := os.FindProcess(pid)
		if err != nil {
			multierror.Append(mErrs, fmt.Errorf("error finding process %v: %v", pid, err))
			continue
		}

		procs = append(procs, proc)
		if e := proc.Kill(); e != nil {
			multierror.Append(mErrs, fmt.Errorf("error killing process %v: %v", pid, e))
		}
	}

	// Unfreeze the cgroug so we can wait.
	err = manager.Freeze(cgroupConfig.Thawed)
	if err != nil && !strings.Contains(err.Error(), "no such file or directory") {
		multierror.Append(mErrs, fmt.Errorf("failed to unfreeze cgroup: %v", err))
	}

	// Wait on the killed processes to ensure they are cleaned up.
	for _, proc := range procs {
		// Don't capture the error because we expect this to fail for
		// processes we didn't fork.
		proc.Wait()
	}

	// Remove the cgroup.
	if err := manager.Destroy(); err != nil {
		multierror.Append(mErrs, fmt.Errorf("failed to delete the cgroup directories: %v", err))
	}
	return mErrs.ErrorOrNil()
}

// getCgroupManager returns the correct libcontainer cgroup manager.
func getCgroupManager(groups *cgroupConfig.Cgroup, paths map[string]string) cgroups.Manager {
	return &cgroupFs.Manager{Cgroups: groups, Paths: paths}
}
