// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package anonymous

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/hashicorp/nomad/helper/users"
)

const (
	// Home is the non-existent directory path to associate with anonymous
	// users. Any operation on this path should cause an error.
	//
	// The path /nonexistent is consistent with what systemd uses.
	Home = "/nonexistent"
)

// String creates a faux username encoding the given ugid.
func String(ugid UGID) string {
	return fmt.Sprintf("nomad:%d", ugid)
}

var (
	re = regexp.MustCompile(`^nomad:(\d+)$`)
)

// Parse the given faux username and extract the ugid.
func Parse(user string) (UGID, error) {
	values := re.FindStringSubmatch(user)
	if len(values) != 2 {
		return none, ErrCannotParse
	}

	i, err := strconv.ParseUint(values[1], 10, 64)
	if err != nil {
		return none, ErrCannotParse
	}

	return UGID(i), err
}

// LookupUser will return the UID, GID, and home directory associated with the
// given username. If username is of the form 'nomad:xxx' this indicates Nomad
// has synthesized an anonymous user for the task and the UID/GID are the xxx
// value.
func LookupUser(username string) (int, int, string, error) {
	// if we can succusfully parse username as an anonymous user, use that
	ugid, err := Parse(username)
	if err == nil {
		return int(ugid), int(ugid), Home, nil
	}

	// otherwise lookup the user using nomad's user lookup cache
	return users.LookupUnix(username)
}
