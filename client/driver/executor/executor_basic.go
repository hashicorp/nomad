// +build darwin dragonfly freebsd netbsd openbsd solaris windows

package executor

import hclog "github.com/hashicorp/go-hclog"

func NewExecutorWithIsolation(logger hclog.Logger) Executor {
	logger = logger.Named("executor")
	logger.Error("isolation executor is not supported on this platform, using default")
	return NewExecutor(logger)
}
