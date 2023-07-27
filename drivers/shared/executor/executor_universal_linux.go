// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package executor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/client/lib/proclib/cgroupslib"
	"github.com/hashicorp/nomad/drivers/shared/executor/procstats"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/plugins/drivers"
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
	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
		return e.listProcessesCG1()
	case cgroupslib.CG2:
		return e.listProcessesCG2()
	default:
		return nil
	}
}

func (e *UniversalExecutor) listProcessesCG1() *set.Set[procstats.ProcessID] {

	return nil
}

func (e *UniversalExecutor) listProcessesCG2() *set.Set[procstats.ProcessID] {
	cgroup := filepath.Join(e.commandCfg.Cgroup(), "cgroup.procs")
	ed := cgroupslib.OpenPath(cgroup)
	v, err := ed.Read()
	if err != nil {
		e.logger.Error("ListProcs", "error", err, "cgroup", cgroup)
		return set.New[procstats.ProcessID](0)
	}
	e.logger.Info("ListProcs", "v", v, "path", cgroup)
	fields := strings.Fields(v)
	return set.FromFunc(fields, func(s string) procstats.ProcessID {
		i, err := strconv.Atoi(s)
		e.logger.Info("Atoi", "error", err)
		return procstats.ProcessID(i)
	})
}

func (e *UniversalExecutor) openCG(command *ExecCommand) (int, func(), error) {
	cgroup := command.Cgroup()
	e.logger.Info("OPEN CG", "cgroup", cgroup)
	fd, err := unix.Open(cgroup, unix.O_PATH, 0)
	cleanup := func() {
		_ = unix.Close(fd)
	}
	return fd, cleanup, err
}

// configureResourceContainer configurs the cgroups to be used to track pids
// created by the executor
func (e *UniversalExecutor) configureResourceContainer(command *ExecCommand, pid int) (func(), error) {
	// SETH
	// - set cfg.Cgroups.Resources.Devices += specconv.AllowedDevices
	// - do pid "containment" (group so we can track utilization and kill later)

	// get file descriptor of the cgroup made for this task
	fd, fdCleanup, err := e.openCG(command)
	if err != nil {
		return nil, err
	}

	// configure child process to spawn in the cgroup
	e.childCmd.SysProcAttr.UseCgroupFD = true
	e.childCmd.SysProcAttr.CgroupFD = fd

	e.logger.Info("configured cgroup for executor", "pid", pid)

	// YOU ARE HERE - why does the child not spawn in the cgroup?

	// switch cgroupslib.GetMode() {
	// case cgroupslib.CG1:
	// case cgroupslib.CG2:
	// default:
	// 	return nil
	// }

	return fdCleanup, nil
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
