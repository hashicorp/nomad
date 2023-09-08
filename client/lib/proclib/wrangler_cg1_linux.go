// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package proclib

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"oss.indeed.com/go/libtime/decay"
)

// LinuxWranglerCG1 is an implementation of ProcessWrangler that leverages
// cgroups v1 on older Linux systems.
//
// e.g. Ubuntu 20.04 / RHEL 8 and previous versions.
type LinuxWranglerCG1 struct {
	task Task
	log  hclog.Logger
	cg   cgroupslib.Lifecycle
}

func newCG1(c *Configs) create {
	logger := c.Logger.Named("cg1")
	cgroupslib.Init(logger)
	return func(task Task) ProcessWrangler {
		return &LinuxWranglerCG1{
			task: task,
			log:  logger,
			cg:   cgroupslib.Factory(task.AllocID, task.Task),
		}
	}
}

func (w *LinuxWranglerCG1) Initialize() error {
	w.log.Trace("initialize cgroups", "task", w.task)
	return w.cg.Setup()
}

func (w *LinuxWranglerCG1) Kill() error {
	w.log.Trace("force kill processes in cgroup", "task", w.task)
	return w.cg.Kill()
}

func (w *LinuxWranglerCG1) Cleanup() error {
	w.log.Trace("remove cgroups", "task", w.task)

	// need to give the kernel an opportunity to cleanup procs; which could
	// take some time while the procs wake from being thawed only to find they
	// have been issued a kill signal and need to be reaped

	rm := func() (bool, error) {
		err := w.cg.Teardown()
		if err != nil {
			return true, err
		}
		return false, nil
	}

	go func() {
		if err := decay.Backoff(rm, decay.BackoffOptions{
			MaxSleepTime:   30 * time.Second,
			InitialGapSize: 1 * time.Second,
		}); err != nil {
			w.log.Debug("failed to cleanup cgroups", "alloc", w.task.AllocID, "task", w.task.Task, "error", err)
		}
	}()

	return nil
}
