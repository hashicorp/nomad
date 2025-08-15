// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package winsvc

import (
	"errors"
)

// NewWindowsServiceManager returns an error
func NewWindowsServiceManager() (WindowsServiceManager, error) {
	return nil, errors.New("Windows service manager is not supported on this platform")
}
