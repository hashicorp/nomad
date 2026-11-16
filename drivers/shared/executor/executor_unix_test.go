// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

//go:build unix

package executor

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
	"testing"

	"github.com/shoenig/test/must"
)

func TestSetCmdUser(t *testing.T) {
	u, err := user.Lookup("nobody")
	must.NoError(t, err)

	cmd := exec.Command("true")
	must.NoError(t, setCmdUser(cmd, u.Username))

	var credential *syscall.Credential
	if cmd.SysProcAttr != nil {
		credential = cmd.SysProcAttr.Credential
	}

	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	must.NoError(t, err)
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	must.NoError(t, err)

	must.NotNil(t, credential)
	must.Eq(t, uint32(uid), credential.Uid)
	must.Eq(t, uint32(gid), credential.Gid)

	must.SliceContains(t, cmd.Env, fmt.Sprintf("USER=%s", u.Username))
	must.SliceContains(t, cmd.Env, fmt.Sprintf("LOGNAME=%s", u.Username))
	must.SliceContains(t, cmd.Env, fmt.Sprintf("HOME=%s", u.HomeDir))
}

func TestSetCmdUser_UnknownUser(t *testing.T) {
	cmd := exec.Command("true")
	err := setCmdUser(cmd, "nomad-test-user-that-does-not-exist")
	must.ErrorContains(t, err, "failed to identify user")
}
