package executor

// SetUID changes the Uid for this command (must be set before starting)
func (c *cmd) SetUID(userid string) error {
	// TODO implement something for windows
	return nil
}

// SetGID changes the Gid for this command (must be set before starting)
func (c *cmd) SetGID(groupid string) error {
	// TODO implement something for windows
	return nil
}
