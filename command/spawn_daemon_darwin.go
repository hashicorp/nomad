package command

// No chroot on darwin.
func (c *SpawnDaemonCommand) configureChroot() {}
