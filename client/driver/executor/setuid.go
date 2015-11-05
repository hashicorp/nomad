// +build !windows

package executor

import (
	"fmt"
	"strconv"
	"syscall"
)

// SetUID changes the Uid for this command (must be set before starting)
func (c *cmd) SetUID(userid string) error {
	uid, err := strconv.ParseUint(userid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert userid to uint32: %s", err)
	}
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	if c.SysProcAttr.Credential == nil {
		c.SysProcAttr.Credential = &syscall.Credential{}
	}
	c.SysProcAttr.Credential.Uid = uint32(uid)
	return nil
}

// SetGID changes the Gid for this command (must be set before starting)
func (c *cmd) SetGID(groupid string) error {
	gid, err := strconv.ParseUint(groupid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert groupid to uint32: %s", err)
	}
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	if c.SysProcAttr.Credential == nil {
		c.SysProcAttr.Credential = &syscall.Credential{}
	}
	c.SysProcAttr.Credential.Gid = uint32(gid)
	return nil
}
