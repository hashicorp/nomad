// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package validators

import (
	"fmt"
	"os/user"
	"strconv"
)

// HasValidIds is used when running a task to ensure the
// given user is in the ID range defined in the task config
func HasValidIds(user *user.User, deniedHostUIDs, deniedHostGIDs []IDRange) error {
	uid, err := strconv.ParseUint(user.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("unable to convert userid %s to integer", user.Uid)
	}

	// check uids

	for _, uidRange := range deniedHostUIDs {
		if uid >= uidRange.Lower && uid <= uidRange.Upper {
			return fmt.Errorf("running as uid %d is disallowed", uid)
		}
	}

	// check gids

	gidStrings, err := user.GroupIds()
	if err != nil {
		return fmt.Errorf("unable to lookup user's group membership: %w", err)
	}
	gids := make([]uint64, len(gidStrings))

	for _, gidString := range gidStrings {
		u, err := strconv.ParseUint(gidString, 10, 32)
		if err != nil {
			return fmt.Errorf("unable to convert user's group %q to integer: %w", gidString, err)
		}

		gids = append(gids, u)
	}

	for _, gidRange := range deniedHostGIDs {
		for _, gid := range gids {
			if gid >= gidRange.Lower && gid <= gidRange.Upper {
				return fmt.Errorf("running as gid %d is disallowed", gid)
			}
		}
	}

	return nil
}