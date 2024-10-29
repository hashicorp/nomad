// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package validators

import (
	"fmt"
	"os/user"
	"strconv"

	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

func getUserID(user *user.User) (hw.UserID, error) {
	id, err := strconv.ParseUint(user.Uid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("unable to convert userid %s to integer", user.Uid)
	}

	return hw.UserID(id), nil
}

func getGroupID(user *user.User) ([]hw.GroupID, error) {
	gidStrings, err := user.GroupIds()
	if err != nil {
		return []hw.GroupID{}, fmt.Errorf("unable to lookup user's group membership: %w", err)
	}

	gids := make([]hw.GroupID, len(gidStrings))

	for _, gidString := range gidStrings {
		u, err := strconv.ParseUint(gidString, 10, 32)
		if err != nil {
			return []hw.GroupID{}, fmt.Errorf("unable to convert user's group %q to integer: %w", gidString, err)
		}

		gids = append(gids, hw.GroupID(u))
	}

	return gids, nil
}
