// +build !linux

package executor

import (
	hclog "github.com/hashicorp/go-hclog"
)

func NewExecutorWithIsolation(logger hclog.Logger) Executor {
	logger = logger.Named("executor")
	logger.Error("isolation executor is not supported on this platform, using default")
	return NewExecutor(logger)
}

func (e *UniversalExecutor) configureResourceContainer(_ int) error { return nil }

func (e *UniversalExecutor) runAs(_ string) error { return nil }
