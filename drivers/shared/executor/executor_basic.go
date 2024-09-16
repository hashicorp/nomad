// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package executor

import (
	"os/exec"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	"github.com/hashicorp/nomad/drivers/shared/executor/procstats"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func NewExecutorWithIsolation(logger hclog.Logger, compute cpustats.Compute) Executor {
	logger = logger.Named("executor")
	logger.Error("isolation executor is not supported on this platform, using default")
	return NewExecutor(logger, compute)
}

func (e *UniversalExecutor) configureResourceContainer(_ *ExecCommand, _ int) (func(), error) {
	nothing := func() {}
	return nothing, nil
}

func (e *UniversalExecutor) start(command *ExecCommand) error {
	return e.childCmd.Start()
}

func withNetworkIsolation(f func() error, _ *drivers.NetworkIsolationSpec) error {
	return f()
}

func setCmdUser(*exec.Cmd, string) error { return nil }

func (e *UniversalExecutor) ListProcesses() set.Collection[int] {
	return procstats.List(e.childCmd.Process.Pid)
}

func (e *UniversalExecutor) setSubCmdCgroup(*exec.Cmd, string) (func(), error) {
	return func() {}, nil
}
