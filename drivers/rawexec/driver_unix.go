// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package rawexec

import (
	"fmt"

	"github.com/hashicorp/nomad/v2/helper/users"
	"github.com/hashicorp/nomad/v2/plugins/drivers"
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
