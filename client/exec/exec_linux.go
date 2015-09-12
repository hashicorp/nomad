package exec

import "github.com/hashicorp/nomad/nomad/structs"

type LinuxExecutor struct {
	cmd
}

func (e *LinuxExecutor) Limit(resources structs.Resources) {

}

func (e *LinuxExecutor) RunAs(userid string) {

}

func (e *LinuxExecutor) Start() error {
	return nil
}

func (e *LinuxExecutor) Open(pid int) error {
	return nil
}

func (e *LinuxExecutor) Shutdown() error {
	return nil
}

func (e *LinuxExecutor) ForceStop() error {
	return nil
}

func (e *LinuxExecutor) Command() *cmd {
	return &e.cmd
}
