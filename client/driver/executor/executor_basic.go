// +build !linux

package executor

import (
	"path/filepath"
	"runtime"

	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"
)

func (e *UniversalExecutor) makeExecutable(binPath string) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	path := binPath
	if !filepath.IsAbs(binPath) {
		// The path must be relative the allocations directory.
		path = filepath.Join(e.taskDir, binPath)
	}
	return e.makeExecutablePosix(path)
}

func (e *UniversalExecutor) configureChroot() error {
	return nil
}

func DestroyCgroup(groups *cgroupConfig.Cgroup) error {
	return nil
}

func (e *UniversalExecutor) removeChrootMounts() error {
	return nil
}

func (e *UniversalExecutor) runAs(userid string) error {
	return nil
}

func (e *UniversalExecutor) applyLimits(pid int) error {
	return nil
}

func (e *UniversalExecutor) configureIsolation() error {
	return nil
}
