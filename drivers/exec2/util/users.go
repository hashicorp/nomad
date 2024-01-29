// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package util

import (
	"github.com/hashicorp/nomad/client/anonymous"
	"github.com/hashicorp/nomad/helper/users"
)

// LookupUser will return the UID, GID, and home directory associated with the
// given username. If username is of the form 'nomad:xxx' this indicates Nomad
// has synthesized an anonymous user for the task and the UID/GID are the xxx
// value. This home directory for such users is /dev/null.
func LookupUser(username string) (uint32, uint32, string, error) {
	// if we can succusfully parse username as an anonymous user, use that
	ugid, err := anonymous.Parse(username)
	if err == nil {
		return uint32(ugid), uint32(ugid), "/dev/null", nil
	}

	// otherwise lookup the user using nomad's user lookup cache
	u, err := users.Lookup(username)
	if err != nil {
		return 0, 0, "", err
	}
	// YOU ARE HERE (ugh refactor UIDforUser)
	return uint32(u.Uid), uint32(u.Gid), u.HomeDir, nil
}
