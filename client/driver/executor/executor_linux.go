package executor

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/go-ps"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"

	"github.com/hashicorp/nomad/client/stats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// The statistics the executor exposes when using cgroups
	ExecutorCgroupMeasuredMemStats = []string{"RSS", "Cache", "Swap", "Max Usage", "Kernel Usage", "Kernel Max Usage"}
	ExecutorCgroupMeasuredCpuStats = []string{"System Mode", "User Mode", "Throttled Periods", "Throttled Time", "Percent"}
)

// configureIsolation configures chroot and creates cgroups
func (e *UniversalExecutor) configureIsolation() error {
	if e.command.FSIsolation {
		if err := e.configureChroot(); err != nil {
			return err
		}
	}

	if e.command.ResourceLimits || e.command.BasicProcessCgroup {
		if err := e.configureCgroups(e.ctx.Task.Resources); err != nil {
			return fmt.Errorf("error creating cgroups: %v", err)
		}
	}
	return nil
}

// applyLimits puts a process in a pre-configured cgroup
func (e *UniversalExecutor) applyLimits(pid int) error {
	if !(e.command.ResourceLimits || e.command.BasicProcessCgroup) {
		return nil
	}

	// Entering the process in the cgroup
	manager := getCgroupManager(e.resConCtx.groups, nil)
	if err := manager.Apply(pid); err != nil {
		e.logger.Printf("[ERR] executor: error applying pid to cgroup: %v", err)
		return err
	}

	e.resConCtx.cgPaths = manager.GetPaths()

	// Don't enter all the cgroups since we will inherit resources limits. Only
	// use devices (required by libcontainer) and freezer. Freezer allows us to
	// capture all pids and stop any fork/execs from happening while we are
	// cleaning up.
	if !e.command.ResourceLimits {
		// Move the executor into the global cgroup so that the task specific
		// cgroup can be destroyed.
		nilGroup := &cgroupConfig.Cgroup{}
		nilGroup.Path = "/"
		nilGroup.Resources = e.resConCtx.groups.Resources
		nilManager := getCgroupManager(nilGroup, nil)
		err := nilManager.Apply(pid)
		if err != nil {
			return fmt.Errorf("failed to remove executor pid %d: %v", pid, err)
		}

		// Grab the freezer and devices cgroup paths. We do this from the old
		// manager after the executor pid has been applied since there is no
		// other way to determine what the proper cgroup paths would be.
		freezer := &cgroupFs.FreezerGroup{}
		devices := &cgroupFs.DevicesGroup{}
		freezerName, devicesName := freezer.Name(), devices.Name()
		newPath := map[string]string{
			freezerName: e.resConCtx.cgPaths[freezerName],
			devicesName: e.resConCtx.cgPaths[devicesName],
		}

		// Clear the cgroups paths so that everything is properly cleaned except
		// the groups we want our process to stay in. This will delete the
		// directories from disk.
		manager.Cgroups.Paths = nil
		delete(manager.Paths, freezerName)
		delete(manager.Paths, devicesName)
		if err := manager.Destroy(); err != nil {
			e.logger.Printf("[ERR] executor: failed to destroy original: %v", err)
			return err
		}

		// Update our context such that the new cgroup manager only is tracking
		// the paths we care about now.
		e.resConCtx.cgPaths = newPath
		e.resConCtx.groups.Paths = newPath

		// Apply just the freezer and devices now
		manager = getCgroupManager(e.resConCtx.groups, e.resConCtx.cgPaths)
		if err := manager.Apply(pid); err != nil {
			e.logger.Printf("[ERR] executor: error applying pid to cgroup subset %v: %v", e.resConCtx.cgPaths, err)
			return err
		}
	}

	cgConfig := cgroupConfig.Config{Cgroups: e.resConCtx.groups}
	if err := manager.Set(&cgConfig); err != nil {
		e.logger.Printf("[ERR] executor: error setting cgroup config: %v", err)
		if er := DestroyCgroup(e.resConCtx.groups, e.resConCtx.cgPaths, os.Getpid()); er != nil {
			e.logger.Printf("[ERR] executor: error destroying cgroup: %v", er)
		}
		return err
	}
	return nil
}

// configureCgroups converts a Nomad Resources specification into the equivalent
// cgroup configuration. It returns an error if the resources are invalid.
func (e *UniversalExecutor) configureCgroups(resources *structs.Resources) error {
	e.resConCtx.groups = &cgroupConfig.Cgroup{}
	e.resConCtx.groups.Resources = &cgroupConfig.Resources{}
	cgroupName := uuid.Generate()
	e.resConCtx.groups.Path = filepath.Join("/nomad", cgroupName)

	// Allow access to /dev/
	e.resConCtx.groups.Resources.AllowAllDevices = true

	// Use a cgroup but don't apply limits
	if !e.command.ResourceLimits {
		return nil
	}

	if resources.MemoryMB > 0 {
		// Total amount of memory allowed to consume
		e.resConCtx.groups.Resources.Memory = int64(resources.MemoryMB * 1024 * 1024)
		// Disable swap to avoid issues on the machine
		var memSwappiness int64 = 0
		e.resConCtx.groups.Resources.MemorySwappiness = &memSwappiness
	}

	if resources.CPU < 2 {
		return fmt.Errorf("resources.CPU must be equal to or greater than 2: %v", resources.CPU)
	}

	// Set the relative CPU shares for this cgroup.
	e.resConCtx.groups.Resources.CpuShares = int64(resources.CPU)

	if resources.IOPS != 0 {
		// Validate it is in an acceptable range.
		if resources.IOPS < 10 || resources.IOPS > 1000 {
			return fmt.Errorf("resources.IOPS must be between 10 and 1000: %d", resources.IOPS)
		}

		e.resConCtx.groups.Resources.BlkioWeight = uint16(resources.IOPS)
	}

	return nil
}

// Stats reports the resource utilization of the cgroup. If there is no resource
// isolation we aggregate the resource utilization of all the pids launched by
// the executor.
func (e *UniversalExecutor) Stats() (*cstructs.TaskResourceUsage, error) {
	// If we don't use full resource limits fallback to normal collection. It is
	// not enough to be in the Cgroup since you must be in the memory, cpu, and
	// cpuacct cgroup to gather the correct statistics.
	if !e.command.ResourceLimits {
		pidStats, err := e.pidStats()
		if err != nil {
			return nil, err
		}
		return e.aggregatedResourceUsage(pidStats), nil
	}
	ts := time.Now()
	manager := getCgroupManager(e.resConCtx.groups, e.resConCtx.cgPaths)
	stats, err := manager.GetStats()
	if err != nil {
		return nil, err
	}

	// Memory Related Stats
	swap := stats.MemoryStats.SwapUsage
	maxUsage := stats.MemoryStats.Usage.MaxUsage
	rss := stats.MemoryStats.Stats["rss"]
	cache := stats.MemoryStats.Stats["cache"]
	ms := &cstructs.MemoryStats{
		RSS:            rss,
		Cache:          cache,
		Swap:           swap.Usage,
		MaxUsage:       maxUsage,
		KernelUsage:    stats.MemoryStats.KernelUsage.Usage,
		KernelMaxUsage: stats.MemoryStats.KernelUsage.MaxUsage,
		Measured:       ExecutorCgroupMeasuredMemStats,
	}

	// CPU Related Stats
	totalProcessCPUUsage := float64(stats.CpuStats.CpuUsage.TotalUsage)
	userModeTime := float64(stats.CpuStats.CpuUsage.UsageInUsermode)
	kernelModeTime := float64(stats.CpuStats.CpuUsage.UsageInKernelmode)

	totalPercent := e.totalCpuStats.Percent(totalProcessCPUUsage)
	cs := &cstructs.CpuStats{
		SystemMode:       e.systemCpuStats.Percent(kernelModeTime),
		UserMode:         e.userCpuStats.Percent(userModeTime),
		Percent:          totalPercent,
		ThrottledPeriods: stats.CpuStats.ThrottlingData.ThrottledPeriods,
		ThrottledTime:    stats.CpuStats.ThrottlingData.ThrottledTime,
		TotalTicks:       e.systemCpuStats.TicksConsumed(totalPercent),
		Measured:         ExecutorCgroupMeasuredCpuStats,
	}
	taskResUsage := cstructs.TaskResourceUsage{
		ResourceUsage: &cstructs.ResourceUsage{
			MemoryStats: ms,
			CpuStats:    cs,
		},
		Timestamp: ts.UTC().UnixNano(),
	}
	if pidStats, err := e.pidStats(); err == nil {
		taskResUsage.Pids = pidStats
	}
	return &taskResUsage, nil
}

// runAs takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func (e *UniversalExecutor) runAs(userid string) error {
	u, err := user.Lookup(userid)
	if err != nil {
		return fmt.Errorf("Failed to identify user %v: %v", userid, err)
	}

	// Get the groups the user is a part of
	gidStrings, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("Unable to lookup user's group membership: %v", err)
	}

	gids := make([]uint32, len(gidStrings))
	for _, gidString := range gidStrings {
		u, err := strconv.Atoi(gidString)
		if err != nil {
			return fmt.Errorf("Unable to convert user's group to int %s: %v", gidString, err)
		}

		gids = append(gids, uint32(u))
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
	e.cmd.SysProcAttr.Credential.Groups = gids

	e.logger.Printf("[DEBUG] executor: running as user:group %d:%d with group membership in %v", uid, gid, gids)

	return nil
}

// configureChroot configures a chroot
func (e *UniversalExecutor) configureChroot() error {
	if e.cmd.SysProcAttr == nil {
		e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	e.cmd.SysProcAttr.Chroot = e.ctx.TaskDir
	e.cmd.Dir = "/"

	e.fsIsolationEnforced = true
	return nil
}

// getAllPids returns the pids of all the processes spun up by the executor. We
// use the libcontainer apis to get the pids when the user is using cgroup
// isolation and we scan the entire process table if the user is not using any
// isolation
func (e *UniversalExecutor) getAllPids() (map[int]*nomadPid, error) {
	if e.command.ResourceLimits || e.command.BasicProcessCgroup {
		manager := getCgroupManager(e.resConCtx.groups, e.resConCtx.cgPaths)
		pids, err := manager.GetAllPids()
		if err != nil {
			return nil, err
		}
		np := make(map[int]*nomadPid, len(pids))
		for _, pid := range pids {
			np[pid] = &nomadPid{
				pid:           pid,
				cpuStatsTotal: stats.NewCpuStats(),
				cpuStatsSys:   stats.NewCpuStats(),
				cpuStatsUser:  stats.NewCpuStats(),
			}
		}
		return np, nil
	}
	allProcesses, err := ps.Processes()
	if err != nil {
		return nil, err
	}
	return e.scanPids(os.Getpid(), allProcesses)
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

	// Clear the cgroups paths so that everything is properly cleaned
	manager.Cgroups.Paths = nil

	// Remove the cgroup.
	if err := manager.Destroy(); err != nil {
		multierror.Append(mErrs, fmt.Errorf("failed to delete the cgroup directories: %v", err))
	}
	return mErrs.ErrorOrNil()
}

// getCgroupManager returns the correct libcontainer cgroup manager.
func getCgroupManager(groups *cgroupConfig.Cgroup, paths map[string]string) *cgroupFs.Manager {
	return &cgroupFs.Manager{Cgroups: groups, Paths: paths}
}
