// build !linux !darwin

package command

// No isolation on Windows.
func (c *SpawnDaemonCommand) isolateCmd() error { return nil }
func (c *SpawnDaemonCommand) configureChroot()  {}
