// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package validators

import (
	"os/user"

	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

// noop
func getUserID(user *user.User) (hw.UserID, error) {
	return 0, nil
}

// noop
func getGroupID(user *user.User) ([]hw.GroupID, error) {
	return []hw.GroupID{}, nil
}
