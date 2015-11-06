// +build !windows

package command

import "syscall"

// isolateCmd sets the session id for the process and the umask.
func (c *SpawnDaemonCommand) isolateCmd() error {
	if c.config.Cmd.SysProcAttr == nil {
		c.config.Cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	c.config.Cmd.SysProcAttr.Setsid = true
	syscall.Umask(0)
	return nil
}
