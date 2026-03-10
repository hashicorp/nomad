// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build linux && !cgo

package executor

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	"github.com/hashicorp/nomad/drivers/shared/executor/procstats"
)

// NewExecutorWithIsolation returns universal executor if CGO is disabled. This
// is only to prevent compilation issues, if CGO is disabled, task drivers that
// depend on resource isolation (exec/java) are disabled anyway.
func NewExecutorWithIsolation(logger hclog.Logger, compute cpustats.Compute) Executor {
	ue := &UniversalExecutor{
		logger:         logger.Named("executor"),
		processExited:  make(chan interface{}),
		totalCpuStats:  cpustats.New(compute),
		userCpuStats:   cpustats.New(compute),
		systemCpuStats: cpustats.New(compute),
	}
	ue.processStats = procstats.New(compute, ue)
	return ue
}
