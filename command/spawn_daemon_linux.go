package command

import "syscall"

// configureChroot enters the user command into a chroot if specified in the
// config and on an OS that supports Chroots.
func (c *SpawnDaemonCommand) configureChroot() {
	if len(c.config.Chroot) != 0 {
		if c.config.Cmd.SysProcAttr == nil {
			c.config.Cmd.SysProcAttr = &syscall.SysProcAttr{}
		}

		c.config.Cmd.SysProcAttr.Chroot = c.config.Chroot
		c.config.Cmd.Dir = "/"
	}
}
