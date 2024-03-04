// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package rawexec

import (
	"github.com/hashicorp/nomad/drivers/shared/validators"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/plugins/drivers"
)

func (tc *TaskConfig) Validate(driverCofig Config, cfg drivers.TaskConfig) error {
	usernameToLookup := cfg.User

	// Uses the current user of the cleint agent process
	// if no override is given (differs from exec)
	if usernameToLookup == "" {
		current, err := users.Current()
		if err != nil {
			return err
		}

		usernameToLookup = current.Name
	}

	return validators.UserInRange(users.Lookup, usernameToLookup, driverCofig.DeniedHostUids, driverCofig.DeniedHostGids)
}
