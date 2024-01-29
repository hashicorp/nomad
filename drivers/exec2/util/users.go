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
// value.
func LookupUser(username string) (int, int, string, error) {
	// if we can succusfully parse username as an anonymous user, use that
	ugid, err := anonymous.Parse(username)
	if err == nil {
		return int(ugid), int(ugid), anonymous.Home, nil
	}

	// otherwise lookup the user using nomad's user lookup cache
	return users.LookupUnix(username)
}
