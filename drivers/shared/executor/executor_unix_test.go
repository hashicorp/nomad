// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

//go:build unix

package executor

import (
	"os/exec"
	"os/user"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSetCmdUser_GroupList ensures setCmdUser builds the supplementary
// group list from the user's real groups only: the list must have exactly
// one entry per group and must not contain a spurious gid 0.
//
// Regression test for https://github.com/hashicorp/nomad/issues/28762:
// the list was allocated with make([]uint32, len(gidStrings)) and then
// appended to, so a user in N groups got N zero entries followed by the
// N real groups. On Linux every task gained supplementary group 0 (root);
// on macOS the doubled list could exceed the 16-group limit and the task
// failed to start with EINVAL.
func TestSetCmdUser_GroupList(t *testing.T) {
	u, err := user.Current()
	require.NoError(t, err)

	expected, err := u.GroupIds()
	require.NoError(t, err)

	cmd := exec.Command("true")
	require.NoError(t, setCmdUser(cmd, u.Username))
	require.NotNil(t, cmd.SysProcAttr)
	require.NotNil(t, cmd.SysProcAttr.Credential)

	groups := cmd.SysProcAttr.Credential.Groups
	require.Len(t, groups, len(expected),
		"group list must contain exactly the user's groups, got %v for %v", groups, expected)

	// The list must not contain gid 0 unless the user is really in group 0.
	if !slices.Contains(expected, "0") {
		for _, gid := range groups {
			require.NotEqual(t, uint32(0), gid, "spurious gid 0 in group list %v", groups)
		}
	}

	// Every entry must be one of the user's groups.
	for _, gid := range groups {
		require.Contains(t, expected, strconv.FormatUint(uint64(gid), 10))
	}
}
