// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package executor

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/lib/nsutil"
	"github.com/hashicorp/nomad/drivers/shared/executor/procstats"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"golang.org/x/sys/unix"
)

const (
	// memoryNoLimit is a sentinel value for memory_max that indicates the
	// raw_exec driver should not enforce a maximum memory limit
	memoryNoLimit = -1
)

// setCmdUser takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func setCmdUser(cmd *exec.Cmd, userid string) error {
	u, err := users.Lookup(userid)
	if err != nil {
		return fmt.Errorf("failed to identify user %v: %v", userid, err)
	}

	// Get the groups the user is a part of
	gidStrings, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("unable to lookup user's group membership: %v", err)
	}

	gids := make([]uint32, len(gidStrings))
	for _, gidString := range gidStrings {
		u, err := strconv.ParseUint(gidString, 10, 32)
		if err != nil {
			return fmt.Errorf("unable to convert user's group to uint32 %s: %v", gidString, err)
		}

		gids = append(gids, uint32(u))
	}

	// Convert the uid and gid
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("unable to convert userid to uint32: %s", err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("unable to convert groupid to uint32: %s", err)
	}

	// Set the command to run as that user and group.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if cmd.SysProcAttr.Credential == nil {
		cmd.SysProcAttr.Credential = &syscall.Credential{}
	}
	cmd.SysProcAttr.Credential.Uid = uint32(uid)
	cmd.SysProcAttr.Credential.Gid = uint32(gid)
	cmd.SysProcAttr.Credential.Groups = gids

	return nil
}

// setSubCmdCgroup sets the cgroup for non-Task child processes of the
// executor.Executor (since in cg2 it lives outside the task's cgroup)
func (e *UniversalExecutor) setSubCmdCgroup(cmd *exec.Cmd, cgroup string) (func(), error) {
	if cgroup == "" {
		panic("cgroup must be set")
	}

	// make sure attrs struct has been set
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}

	switch cgroupslib.GetMode() {
	case cgroupslib.CG2:
		fd, cleanup, err := e.statCG(cgroup)
		if err != nil {
			return nil, err
		}
		cmd.SysProcAttr.UseCgroupFD = true
		cmd.SysProcAttr.CgroupFD = fd
		return cleanup, nil
	default:
		return func() {}, nil
	}
}

func (e *UniversalExecutor) ListProcesses() set.Collection[procstats.ProcessID] {
	return procstats.List(e.command)
}

func (e *UniversalExecutor) statCG(cgroup string) (int, func(), error) {
	fd, err := unix.Open(cgroup, unix.O_PATH, 0)
	cleanup := func() {
		_ = unix.Close(fd)
	}
	return fd, cleanup, err
}

// configureResourceContainer on Linux configures the cgroups to be used to track
// pids created by the executor
//
// pid: pid of the executor (i.e. ourself)
func (e *UniversalExecutor) configureResourceContainer(command *ExecCommand, pid int) (func(), error) {
	cgroup := command.StatsCgroup()

	// ensure tasks get the desired oom_score_adj value set
	if err := e.setOomAdj(command.OOMScoreAdj); err != nil {
		return nil, err
	}

	// cgCleanup will be called after the task has been launched
	// v1: remove the executor process from the task's cgroups
	// v2: let go of the file descriptor of the task's cgroup
	var cgCleanup func()

	// manually configure cgroup for cpu / memory constraints
	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
		if err := e.configureCG1(cgroup, command); err != nil {
			return nil, err
		}
		cgCleanup = e.enterCG1(cgroup, command.CpusetCgroup())
	default:
		e.configureCG2(cgroup, command)
		// configure child process to spawn in the cgroup
		// get file descriptor of the cgroup made for this task
		fd, cleanup, err := e.statCG(cgroup)
		if err != nil {
			return nil, err
		}
		e.childCmd.SysProcAttr.UseCgroupFD = true
		e.childCmd.SysProcAttr.CgroupFD = fd
		cgCleanup = cleanup
	}

	e.logger.Info("configured cgroup for executor", "pid", pid)

	return cgCleanup, nil
}

// enterCG1 will write the executor PID (i.e. itself) into the cgroups we
// created for the task - so that the task and its children will spawn in
// those cgroups. The cleanup function moves the executor out of the task's
// cgroups and into the nomad/ parent cgroups.
func (e *UniversalExecutor) enterCG1(statsCgroup, cpusetCgroup string) func() {
	pid := strconv.Itoa(unix.Getpid())

	// write pid to all the normal interfaces
	ifaces := []string{"freezer", "cpu", "memory"}
	for _, iface := range ifaces {
		ed := cgroupslib.OpenFromFreezerCG1(statsCgroup, iface)
		err := ed.Write("cgroup.procs", pid)
		if err != nil {
			e.logger.Warn("failed to write cgroup", "interface", iface, "error", err)
		}
	}

	// write pid to the cpuset interface, which varies between reserve/share
	ed := cgroupslib.OpenPath(cpusetCgroup)
	err := ed.Write("cgroup.procs", pid)
	if err != nil {
		e.logger.Warn("failed to write cpuset cgroup", "error", err)
	}

	// cleanup func that moves executor back up to nomad cgroup
	return func() {
		for _, iface := range ifaces {
			err := cgroupslib.WriteNomadCG1(iface, "cgroup.procs", pid)
			if err != nil {
				e.logger.Warn("failed to move executor cgroup", "interface", iface, "error", err)
			}
		}
	}
}

func (e *UniversalExecutor) configureCG1(cgroup string, command *ExecCommand) error {
	// some drivers like qemu entirely own resource management
	if command.Resources == nil || command.Resources.LinuxResources == nil {
		return nil
	}

	// if custom cgroups are set join those instead of configuring the /nomad
	// cgroups we are not going to use
	if len(e.command.OverrideCgroupV1) > 0 {
		pid := unix.Getpid()
		for controller, path := range e.command.OverrideCgroupV1 {
			absPath := cgroupslib.CustomPathCG1(controller, path)
			ed := cgroupslib.OpenPath(absPath)
			err := ed.Write("cgroup.procs", strconv.Itoa(pid))
			if err != nil {
				e.logger.Error("unable to write to custom cgroup", "error", err)
				return fmt.Errorf("unable to write to custom cgroup: %v", err)
			}
		}
		return nil
	}

	// write memory limits
	memHard, memSoft := e.computeMemory(command)
	ed := cgroupslib.OpenFromFreezerCG1(cgroup, "memory")
	_ = ed.Write("memory.limit_in_bytes", strconv.FormatInt(memHard, 10))
	if memSoft > 0 {
		_ = ed.Write("memory.soft_limit_in_bytes", strconv.FormatInt(memSoft, 10))
	}

	// write memory swappiness
	swappiness := cgroupslib.MaybeDisableMemorySwappiness()
	if swappiness != nil {
		value := int64(*swappiness)
		_ = ed.Write("memory.swappiness", strconv.FormatInt(value, 10))
	}

	// write cpu shares
	cpuShares := strconv.FormatInt(command.Resources.LinuxResources.CPUShares, 10)
	ed = cgroupslib.OpenFromFreezerCG1(cgroup, "cpu")
	_ = ed.Write("cpu.shares", cpuShares)

	// write cpuset, if set
	if cpuSet := command.Resources.LinuxResources.CpusetCpus; cpuSet != "" {
		cpusetPath := command.Resources.LinuxResources.CpusetCgroupPath
		ed = cgroupslib.OpenPath(cpusetPath)
		_ = ed.Write("cpuset.cpus", cpuSet)
	}

	return nil
}

func (e *UniversalExecutor) configureCG2(cgroup string, command *ExecCommand) {
	// some drivers like qemu entirely own resource management
	if command.Resources == nil || command.Resources.LinuxResources == nil {
		return
	}

	// write memory cgroup files
	memHard, memSoft := e.computeMemory(command)
	ed := cgroupslib.OpenPath(cgroup)
	if memHard == memoryNoLimit {
		_ = ed.Write("memory.max", "max")
	} else {
		_ = ed.Write("memory.max", strconv.FormatInt(memHard, 10))
	}
	if memSoft > 0 {
		ed = cgroupslib.OpenPath(cgroup)
		_ = ed.Write("memory.low", strconv.FormatInt(memSoft, 10))
	}

	// set memory swappiness
	swappiness := cgroupslib.MaybeDisableMemorySwappiness()
	if swappiness != nil {
		ed := cgroupslib.OpenPath(cgroup)
		value := int64(*swappiness)
		_ = ed.Write("memory.swappiness", strconv.FormatInt(value, 10))
	}

	// write cpu weight cgroup file
	cpuWeight := e.computeCPU(command)
	ed = cgroupslib.OpenPath(cgroup)
	_ = ed.Write("cpu.weight", strconv.FormatUint(cpuWeight, 10))

	// write cpuset cgroup file, if set
	cpusetCpus := command.Resources.LinuxResources.CpusetCpus
	_ = ed.Write("cpuset.cpus", cpusetCpus)
}

func (e *UniversalExecutor) setOomAdj(oomScore int32) error {
	// /proc/self/oom_score_adj should work on both cgroups v1 and v2 systems
	// range is -1000 to 1000; 0 is the default
	return os.WriteFile("/proc/self/oom_score_adj", []byte(strconv.Itoa(int(oomScore))), 0644)
}

func (*UniversalExecutor) computeCPU(command *ExecCommand) uint64 {
	cpuShares := command.Resources.LinuxResources.CPUShares
	cpuWeight := cgroups.ConvertCPUSharesToCgroupV2Value(uint64(cpuShares))
	return cpuWeight
}

func mbToBytes(n int64) int64 {
	return n * 1024 * 1024
}

// computeMemory returns the hard and soft memory limits for the task
func (*UniversalExecutor) computeMemory(command *ExecCommand) (int64, int64) {
	mem := command.Resources.NomadResources.Memory
	memHard, memSoft := mem.MemoryMaxMB, mem.MemoryMB

	switch memHard {
	case 0:
		// typical case where 'memory' is the hard limit
		memHard = mem.MemoryMB
		return mbToBytes(memHard), 0
	case memoryNoLimit:
		// special oversub case where 'memory' is soft limit and there is no
		// hard limit - helping re-create old raw_exec behavior
		return memoryNoLimit, mbToBytes(memSoft)
	default:
		// typical oversub case where 'memory' is soft limit and 'memory_max'
		// is hard limit
		return mbToBytes(memHard), mbToBytes(memSoft)
	}
}

// withNetworkIsolation calls the passed function the network namespace `spec`
func withNetworkIsolation(f func() error, spec *drivers.NetworkIsolationSpec) error {
	if spec != nil && spec.Path != "" {
		// Get a handle to the target network namespace
		netNS, err := nsutil.GetNS(spec.Path)
		if err != nil {
			return err
		}

		// Start the container in the network namespace
		return netNS.Do(func(nsutil.NetNS) error {
			return f()
		})
	}
	return f()
}
