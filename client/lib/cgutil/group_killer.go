// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgutil

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// freezer is the name of the cgroup subsystem used for stopping / starting
// a group of processes
const freezer = "freezer"

// thawed and frozen are the two states we put a cgroup in when trying to remove it
var (
	thawed = &configs.Resources{Freezer: configs.Thawed}
	frozen = &configs.Resources{Freezer: configs.Frozen}
)

// GroupKiller is used for SIGKILL-ing the process tree[s] of a cgroup by leveraging
// the freezer cgroup subsystem.
type GroupKiller interface {
	KillGroup(cgroup *configs.Cgroup) error
}

// NewGroupKiller creates a GroupKiller with executor PID pid.
func NewGroupKiller(logger hclog.Logger, pid int) GroupKiller {
	return &killer{
		logger: logger.Named("group_killer"),
		pid:    pid,
	}
}

type killer struct {
	logger hclog.Logger
	pid    int
}

// KillGroup will SIGKILL the process tree present in cgroup, using the freezer
// subsystem to prevent further forking, etc.
func (d *killer) KillGroup(cgroup *configs.Cgroup) error {
	if UseV2 {
		return d.v2(cgroup)
	}
	return d.v1(cgroup)
}

func (d *killer) v1(cgroup *configs.Cgroup) error {
	if cgroup == nil {
		return errors.New("missing cgroup")
	}

	// the actual path to our tasks freezer cgroup
	path := cgroup.Path

	d.logger.Trace("killing processes", "cgroup_path", path, "cgroup_version", "v1", "executor_pid", d.pid)

	// move executor PID into the init freezer cgroup so we can kill the task
	// pids without killing the executor (which is the process running this code,
	// doing the killing)
	initPath, err := cgroups.GetInitCgroupPath(freezer)
	if err != nil {
		return fmt.Errorf("failed to find init cgroup: %w", err)
	}
	m := map[string]string{freezer: initPath}
	if err = cgroups.EnterPid(m, d.pid); err != nil {
		return fmt.Errorf("failed to add executor pid to init cgroup: %w", err)
	}

	// ability to freeze the cgroup
	freeze := func() {
		_ = new(fs.FreezerGroup).Set(path, frozen)
	}

	// ability to thaw the cgroup
	thaw := func() {
		_ = new(fs.FreezerGroup).Set(path, thawed)
	}

	// do the common kill logic
	if err = d.kill(path, freeze, thaw); err != nil {
		return err
	}

	// remove the cgroup from disk
	return cgroups.RemovePath(path)
}

func (d *killer) v2(cgroup *configs.Cgroup) error {
	if cgroup == nil || cgroup.Path == "" {
		return errors.New("missing cgroup")
	}

	// move executor (d.PID) into init.scope
	editSelf := &editor{"init.scope"}
	if err := editSelf.write("cgroup.procs", strconv.Itoa(d.pid)); err != nil {
		return err
	}

	// write "1" to cgroup.kill
	editTask := &editor{cgroup.Path}
	if err := editTask.write("cgroup.kill", "1"); err != nil {
		return err
	}

	// note: do NOT remove the cgroup from disk; leave that to the Client, at
	// least until #14375 is implemented.
	return nil
}

// kill is used to SIGKILL all processes in cgroup
//
// The order of operations is
// 0. before calling this method, the executor pid has been moved outside of cgroup
// 1. freeze cgroup (so processes cannot fork further)
// 2. scan the cgroup to collect all pids
// 3. issue SIGKILL to each pid found
// 4. thaw the cgroup so processes can go die
// 5. wait on each processes until it is confirmed dead
func (d *killer) kill(cgroup string, freeze func(), thaw func()) error {
	// freeze the cgroup stopping further forking
	freeze()

	d.logger.Trace("search for pids in", "cgroup", cgroup)

	// find all the pids we intend to kill
	pids, err := cgroups.GetPids(cgroup)
	if err != nil {
		// if we fail to get pids, re-thaw before bailing so there is at least
		// a chance the processes can go die out of band
		thaw()
		return fmt.Errorf("failed to find pids: %w", err)
	}

	d.logger.Trace("send sigkill to frozen processes", "cgroup", cgroup, "pids", pids)

	var processes []*os.Process

	// kill the processes in cgroup
	for _, pid := range pids {
		p, findErr := os.FindProcess(pid)
		if findErr != nil {
			d.logger.Trace("failed to find process of pid to kill", "pid", pid, "error", findErr)
			continue
		}
		processes = append(processes, p)
		if killErr := p.Kill(); killErr != nil {
			d.logger.Trace("failed to kill process", "pid", pid, "error", killErr)
			continue
		}
	}

	// thawed the cgroup so we can wait on each process
	thaw()

	// wait on each process
	for _, p := range processes {
		// do not capture error; errors are normal here
		pState, _ := p.Wait()
		d.logger.Trace("return from wait on process", "pid", p.Pid, "state", pState)
	}

	// cgroups are not atomic, the OS takes a moment to un-mark the cgroup as in-use;
	// a tiny sleep here goes a long way for not creating noisy (but functionally benign)
	// errors about removing busy cgroup
	//
	// alternatively we could do the removal in a loop and silence the interim errors, but meh
	time.Sleep(50 * time.Millisecond)

	return nil
}
