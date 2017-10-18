// +build !windows

package executor

import (
	"fmt"
	"os/user"
	"strconv"
	"syscall"
)

// runAs takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func (e *UniversalExecutor) runAs(userid string) error {
	u, err := user.Lookup(userid)
	if err != nil {
		return fmt.Errorf("Failed to identify user %v: %v", userid, err)
	}

	// Get the groups the user is a part of
	gidStrings, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("Unable to lookup user's group membership: %v", err)
	}

	gids := make([]uint32, len(gidStrings))
	for _, gidString := range gidStrings {
		u, err := strconv.Atoi(gidString)
		if err != nil {
			return fmt.Errorf("Unable to convert user's group to int %s: %v", gidString, err)
		}

		gids = append(gids, uint32(u))
	}

	// Convert the uid and gid
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert userid to uint32: %s", err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert groupid to uint32: %s", err)
	}

	// Set the command to run as that user and group.
	if e.cmd.SysProcAttr == nil {
		e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if e.cmd.SysProcAttr.Credential == nil {
		e.cmd.SysProcAttr.Credential = &syscall.Credential{}
	}
	e.cmd.SysProcAttr.Credential.Uid = uint32(uid)
	e.cmd.SysProcAttr.Credential.Gid = uint32(gid)
	e.cmd.SysProcAttr.Credential.Groups = gids

	e.logger.Printf("[DEBUG] executor: running as user:group %d:%d with group membership in %v", uid, gid, gids)

	return nil
}
