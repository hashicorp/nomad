// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package proclib

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
)

// LinuxWranglerCG2 is an implementation of ProcessWrangler that leverages
// cgroups v2 on modern Linux systems.
//
// e.g. Ubuntu 22.04 / RHEL 9 and later versions.
type LinuxWranglerCG2 struct {
	task Task
	log  hclog.Logger
	cg   cgroupslib.Lifecycle
}

func newCG2(c *Configs) create {
	logger := c.Logger.Named("cg2")
	cgroupslib.Init(logger)
	return func(task Task) ProcessWrangler {
		return &LinuxWranglerCG2{
			task: task,
			log:  c.Logger,
			cg:   cgroupslib.Factory(task.AllocID, task.Task),
		}
	}
}

func (w LinuxWranglerCG2) Initialize() error {
	w.log.Trace("initialize cgroup", "task", w.task)
	return w.cg.Setup()
}

func (w *LinuxWranglerCG2) Kill() error {
	w.log.Trace("force kill processes in cgroup", "task", w.task)
	return w.cg.Kill()
}

func (w *LinuxWranglerCG2) Cleanup() error {
	w.log.Trace("remove cgroup", "task", w.task)
	return w.cg.Teardown()
}
