// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package resources

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/cgutil"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs2"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type containment struct {
	lock   sync.RWMutex
	cgroup *configs.Cgroup
	logger hclog.Logger
}

func Contain(logger hclog.Logger, cgroup *configs.Cgroup) *containment {
	return &containment{
		cgroup: cgroup,
		logger: logger.Named("containment"),
	}
}

func (c *containment) Apply(pid int) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.logger.Trace("create containment for", "cgroup", c.cgroup, "pid", pid)

	// for v2 use manager to create and enter the cgroup
	if cgutil.UseV2 {
		mgr, err := fs2.NewManager(c.cgroup, "")
		if err != nil {
			return fmt.Errorf("failed to create v2 cgroup manager for containment: %w", err)
		}

		// add the pid to the cgroup
		if err = mgr.Apply(pid); err != nil {
			return fmt.Errorf("failed to apply v2 cgroup containment: %w", err)
		}

		// in v2 it is important to set the device resource configuration
		if err = mgr.Set(c.cgroup.Resources); err != nil {
			return fmt.Errorf("failed to set v2 cgroup resources: %w", err)
		}

		return nil
	}

	// for v1 a random cgroup was created already; just enter it
	if err := cgroups.EnterPid(map[string]string{"freezer": c.cgroup.Path}, pid); err != nil {
		return fmt.Errorf("failed to add pid to v1 cgroup: %w", err)
	}

	return nil
}

func (c *containment) Cleanup() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// the current pid is of the executor, who manages the task process cleanup
	executorPID := os.Getpid()
	c.logger.Trace("cleanup on", "cgroup", c.cgroup, "executor_pid", executorPID)

	// destroy the task processes
	destroyer := cgutil.NewGroupKiller(c.logger, executorPID)
	return destroyer.KillGroup(c.cgroup)
}

func (c *containment) GetPIDs() PIDs {
	c.lock.Lock()
	defer c.lock.Unlock()

	m := make(PIDs)
	if c.cgroup == nil {
		return m
	}

	// get the cgroup path under containment
	var path string
	if cgutil.UseV2 {
		path = filepath.Join(cgutil.CgroupRoot, c.cgroup.Path)
	} else {
		path = c.cgroup.Path
	}

	// find the pids in the cgroup under containment
	pids, err := cgroups.GetAllPids(path)
	if err != nil {
		c.logger.Debug("failed to get pids", "cgroup", c.cgroup, "error", err)
		return m
	}

	for _, pid := range pids {
		m[pid] = NewPID(pid)
	}

	return m
}
