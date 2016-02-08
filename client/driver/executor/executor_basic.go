// +build !linux

package executor

func (e *UniversalExecutor) configureChroot() error {
	return nil
}

func (e *UniversalExecutor) destroyCgroup() error {
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
