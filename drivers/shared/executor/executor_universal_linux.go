// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/drivers/shared/executor/procstats"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"golang.org/x/sys/unix"
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

func (e *UniversalExecutor) ListProcesses() *set.Set[procstats.ProcessID] {
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
func (e *UniversalExecutor) configureResourceContainer(command *ExecCommand, pid int) (func(), error) {

	// get our cgroup reference (cpuset in v1)
	cgroup := command.Cgroup()

	// cgCleanup will be called after the task has been launched
	// v1: remove the executor process from the task's cgroups
	// v2: let go of the file descriptor of the task's cgroup
	var cgCleanup func()

	// manually configure cgroup for cpu / memory constraints
	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
		e.configureCG1(cgroup, command)
		cgCleanup = e.enterCG1(cgroup)
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
func (e *UniversalExecutor) enterCG1(cgroup string) func() {
	pid := strconv.Itoa(unix.Getpid())

	// write pid to all the groups
	ifaces := []string{"freezer", "cpu", "memory"} // todo: cpuset
	for _, iface := range ifaces {
		ed := cgroupslib.OpenFromCpusetCG1(cgroup, iface)
		err := ed.Write("cgroup.procs", pid)
		if err != nil {
			e.logger.Warn("failed to write cgroup", "interface", iface, "error", err)
		}
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

func (e *UniversalExecutor) configureCG1(cgroup string, command *ExecCommand) {
	memHard, memSoft := e.computeMemory(command)
	ed := cgroupslib.OpenFromCpusetCG1(cgroup, "memory")
	_ = ed.Write("memory.limit_in_bytes", strconv.FormatInt(memHard, 10))
	if memSoft > 0 {
		ed = cgroupslib.OpenFromCpusetCG1(cgroup, "memory")
		_ = ed.Write("memory.soft_limit_in_bytes", strconv.FormatInt(memSoft, 10))
	}

	// set memory swappiness
	swappiness := cgroupslib.MaybeDisableMemorySwappiness()
	if swappiness != nil {
		ed := cgroupslib.OpenFromCpusetCG1(cgroup, "memory")
		value := int64(*swappiness)
		_ = ed.Write("memory.swappiness", strconv.FormatInt(value, 10))
	}

	// write cpu shares file
	cpuShares := strconv.FormatInt(command.Resources.LinuxResources.CPUShares, 10)
	ed = cgroupslib.OpenFromCpusetCG1(cgroup, "cpu")
	_ = ed.Write("cpu.shares", cpuShares)

	// TODO(shoenig) manage cpuset
	e.logger.Info("TODO CORES", "cpuset", command.Resources.LinuxResources.CpusetCpus)
}

func (e *UniversalExecutor) configureCG2(cgroup string, command *ExecCommand) {
	// write memory cgroup files
	memHard, memSoft := e.computeMemory(command)
	ed := cgroupslib.OpenPath(cgroup)
	_ = ed.Write("memory.max", strconv.FormatInt(memHard, 10))
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

	// write cpu cgroup files
	cpuWeight := e.computeCPU(command)
	ed = cgroupslib.OpenPath(cgroup)
	_ = ed.Write("cpu.weight", strconv.FormatUint(cpuWeight, 10))

	// TODO(shoenig) manage cpuset
	e.logger.Info("TODO CORES", "cpuset", command.Resources.LinuxResources.CpusetCpus)
}

func (*UniversalExecutor) computeCPU(command *ExecCommand) uint64 {
	cpuShares := command.Resources.LinuxResources.CPUShares
	cpuWeight := cgroups.ConvertCPUSharesToCgroupV2Value(uint64(cpuShares))
	return cpuWeight
}

// computeMemory returns the hard and soft memory limits for the task
func (*UniversalExecutor) computeMemory(command *ExecCommand) (int64, int64) {
	mem := command.Resources.NomadResources.Memory
	memHard, memSoft := mem.MemoryMaxMB, mem.MemoryMB
	if memHard <= 0 {
		memHard = mem.MemoryMB
		memSoft = 0
	}
	memHardBytes := memHard * 1024 * 1024
	memSoftBytes := memSoft * 1024 * 1024
	return memHardBytes, memSoftBytes
}

// withNetworkIsolation calls the passed function the network namespace `spec`
func withNetworkIsolation(f func() error, spec *drivers.NetworkIsolationSpec) error {
	if spec != nil && spec.Path != "" {
		// Get a handle to the target network namespace
		netNS, err := ns.GetNS(spec.Path)
		if err != nil {
			return err
		}

		// Start the container in the network namespace
		return netNS.Do(func(ns.NetNS) error {
			return f()
		})
	}
	return f()
}
