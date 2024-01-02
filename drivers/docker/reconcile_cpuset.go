// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package docker

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/cgutil"
	"github.com/hashicorp/nomad/helper"
)

const (
	cpusetReconcileInterval = 1 * time.Second
)

type CpusetFixer interface {
	Start()
}

// cpusetFixer adjusts the cpuset.cpus cgroup value to the assigned value by Nomad.
//
// Due to Docker not allowing the configuration of the full cgroup path, we must
// manually fix the cpuset values for all docker containers continuously, as the
// values will change as tasks of any driver using reserved cores are started and
// stopped, changing the size of the remaining shared cpu pool.
//
// The exec/java, podman, and containerd runtimes let you specify the cgroup path,
// making use of the cgroup Nomad creates and manages on behalf of the task.
type cpusetFixer struct {
	ctx      context.Context
	logger   hclog.Logger
	interval time.Duration
	once     sync.Once
	tasks    func() map[coordinate]struct{}
}

func newCpusetFixer(d *Driver) CpusetFixer {
	return &cpusetFixer{
		interval: cpusetReconcileInterval,
		ctx:      d.ctx,
		logger:   d.logger,
		tasks:    d.trackedTasks,
	}
}

// Start will start the background cpuset reconciliation until the cf context is
// cancelled for shutdown.
//
// Only runs if cgroups.v2 is in use.
func (cf *cpusetFixer) Start() {
	cf.once.Do(func() {
		if cgutil.UseV2 {
			go cf.loop()
		}
	})
}

func (cf *cpusetFixer) loop() {
	timer, cancel := helper.NewSafeTimer(0)
	defer cancel()

	for {
		select {
		case <-cf.ctx.Done():
			return
		case <-timer.C:
			timer.Stop()
			cf.apply()
			timer.Reset(cf.interval)
		}
	}
}

func (cf *cpusetFixer) apply() {
	coordinates := cf.tasks()
	for c := range coordinates {
		cf.fix(c)
	}
}

func (cf *cpusetFixer) fix(c coordinate) {
	source := c.NomadCgroup()
	destination := c.DockerCgroup()
	if err := cgutil.CopyCpuset(source, destination); err != nil {
		cf.logger.Debug("failed to copy cpuset", "error", err)
	}
}

type coordinate struct {
	containerID string
	allocID     string
	task        string
	path        string
}

func (c coordinate) NomadCgroup() string {
	parent, _ := cgutil.SplitPath(c.path)
	return filepath.Join(cgutil.CgroupRoot, parent, cgutil.CgroupScope(c.allocID, c.task))
}

func (c coordinate) DockerCgroup() string {
	parent, _ := cgutil.SplitPath(c.path)
	return filepath.Join(cgutil.CgroupRoot, parent, fmt.Sprintf("docker-%s.scope", c.containerID))
}

func (d *Driver) trackedTasks() map[coordinate]struct{} {
	d.tasks.lock.RLock()
	defer d.tasks.lock.RUnlock()

	m := make(map[coordinate]struct{}, len(d.tasks.store))
	for _, h := range d.tasks.store {
		m[coordinate{
			containerID: h.containerID,
			allocID:     h.task.AllocID,
			task:        h.task.Name,
			path:        h.task.Resources.LinuxResources.CpusetCgroupPath,
		}] = struct{}{}
	}
	return m
}
