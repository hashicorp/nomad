// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package rawexec

import (
	"github.com/hashicorp/nomad/plugins/drivers"
)

func (tc *TaskConfig) Validate(driverCofig Config, cfg drivers.TaskConfig) error {
	// This is a noop on windows since the uid and gid cannot be checked against a range easily
	// We could eventually extend this functionality to check for individual users IDs strings
	// but that is not currently supported. See driverValidators.UserInRange for
	// unix logic
	return nil
}
