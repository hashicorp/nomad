// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package rawexec

import (
	"fmt"

	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func (d *Driver) Validate(cfg drivers.TaskConfig) error {
	usernameToLookup := cfg.User

	// Uses the current user of the client agent process
	// if no override is given (differs from exec)
	if usernameToLookup == "" {
		user, err := users.Current()
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}

		usernameToLookup = user.Username
	}

	return d.userIDValidator.HasValidIDs(usernameToLookup)
}
