// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package validators

import (
	"os/user"
)

// noop
func getUserID(*user.User) (UserID, error) {
	return 0, nil
}

// noop
func getGroupsID(*user.User) ([]GroupID, error) {
	return []GroupID{}, nil
}
