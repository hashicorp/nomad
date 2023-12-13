// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package executor

import (
	"os/exec"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/resources"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func NewExecutorWithIsolation(logger hclog.Logger, cpuTotalTicks uint64) Executor {
	logger = logger.Named("executor")
	logger.Error("isolation executor is not supported on this platform, using default")
	return NewExecutor(logger, cpuTotalTicks)
}

func (e *UniversalExecutor) configureResourceContainer(_ int) error { return nil }

func (e *UniversalExecutor) getAllPids() (resources.PIDs, error) {
	return getAllPidsByScanning()
}

func (e *UniversalExecutor) start(command *ExecCommand) error {
	return e.childCmd.Start()
}

func withNetworkIsolation(f func() error, _ *drivers.NetworkIsolationSpec) error {
	return f()
}

func setCmdUser(*exec.Cmd, string) error { return nil }
