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
	"github.com/hashicorp/nomad/client/lib/proclib/cgroupslib"
	"github.com/hashicorp/nomad/drivers/shared/executor/procstats"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"golang.org/x/sys/unix"
	// "github.com/opencontainers/runc/libcontainer/configs"
	// "github.com/opencontainers/runc/libcontainer/specconv"
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

func (e *UniversalExecutor) ListProcesses() *set.Set[procstats.ProcessID] {
	return procstats.List(e.commandCfg)
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
	// SETH TODO
	// - do pid "containment" (group so we can track utilization and kill later)
	cgroup := command.Cgroup()

	// get file descriptor of the cgroup made for this task
	fd, fdCleanup, err := e.statCG(cgroup)
	if err != nil {
		return nil, err
	}
	// configure child process to spawn in the cgroup
	e.childCmd.SysProcAttr.UseCgroupFD = true
	e.childCmd.SysProcAttr.CgroupFD = fd

	// manually configure cgroup for cpu / memory constraints
	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
	default:
		e.configureCG2(cgroup, command)
	}

	e.logger.Info("configured cgroup for executor", "pid", pid)

	return fdCleanup, nil
}

func (e *UniversalExecutor) configureCG1() {
	panic("todo")
}

func (e *UniversalExecutor) configureCG2(cgroup string, command *ExecCommand) {
	// write memory cgroup files
	memHard, memSoft := e.computeMemory(command)
	ed := cgroupslib.OpenScopeFile(cgroup, "memory.max")
	_ = ed.Write(fmt.Sprintf("%d", memHard))
	if memSoft > 0 {
		ed = cgroupslib.OpenScopeFile(cgroup, "memory.low")
		_ = ed.Write(fmt.Sprintf("%d", memSoft))
	}

	// write cpu cgroup files
	// YOU ARE HERE (finish writing all the files with correct values)
	cpuWeight := e.computeCPU(command)
	ed = cgroupslib.OpenScopeFile(cgroup, "cpu.weight")
	_ = ed.Write(fmt.Sprintf("%d", cpuWeight))

	// cores?
	e.logger.Info("CORES", "cpuset", command.Resources.LinuxResources.CpusetCpus)
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
