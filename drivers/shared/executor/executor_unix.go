// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build unix

package executor

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/hashicorp/nomad/helper/users"
)

// configure new process group for child process
func (e *UniversalExecutor) setNewProcessGroup() error {
	if e.childCmd.SysProcAttr == nil {
		e.childCmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	e.childCmd.SysProcAttr.Setpgid = true
	return nil
}

// SIGKILL the process group starting at process.Pid
func (e *UniversalExecutor) killProcessTree(process *os.Process) error {
	pid := process.Pid
	negative := -pid // tells unix to kill entire process group
	signal := syscall.SIGKILL

	// If new process group was created upon command execution
	// we can kill the whole process group now to cleanup any leftovers.
	if e.childCmd.SysProcAttr != nil && e.childCmd.SysProcAttr.Setpgid {
		e.logger.Trace("sending sigkill to process group", "pid", pid, "negative", negative, "signal", signal)
		if err := syscall.Kill(negative, signal); err != nil && err.Error() != noSuchProcessErr {
			return err
		}
		return nil
	}
	return process.Kill()
}

// Only send the process a shutdown signal (default INT), doesn't
// necessarily kill it.
func (e *UniversalExecutor) shutdownProcess(sig os.Signal, proc *os.Process) error {
	if sig == nil {
		sig = os.Interrupt
	}

	if err := proc.Signal(sig); err != nil && err.Error() != finishedErr {
		return fmt.Errorf("executor shutdown error: %v", err)
	}

	return nil
}

// setCmdUser takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func setCmdUser(cmd *exec.Cmd, userid string) error {
	u, err := users.Lookup(userid)
	if err != nil {
		return fmt.Errorf("failed to identify user %v: %v", userid, err)
	}

	// Get the groups the user is a part of
	gidStrings, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("unable to lookup user's group membership: %v", err)
	}

	gids := make([]uint32, len(gidStrings))
	for _, gidString := range gidStrings {
		u, err := strconv.ParseUint(gidString, 10, 32)
		if err != nil {
			return fmt.Errorf("unable to convert user's group to uint32 %s: %v", gidString, err)
		}

		gids = append(gids, uint32(u))
	}

	// Convert the uid and gid
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("unable to convert userid to uint32: %s", err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("unable to convert groupid to uint32: %s", err)
	}

	// Set the command to run as that user and group.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if cmd.SysProcAttr.Credential == nil {
		cmd.SysProcAttr.Credential = &syscall.Credential{}
	}
	cmd.SysProcAttr.Credential.Uid = uint32(uid)
	cmd.SysProcAttr.Credential.Gid = uint32(gid)
	cmd.SysProcAttr.Credential.Groups = gids

	// Override HOME and USER environment variables
	cmd.Env = append(cmd.Env, fmt.Sprintf("USER=%s", u.Username))
	cmd.Env = append(cmd.Env, fmt.Sprintf("HOME=%s", u.HomeDir))

	return nil
}
