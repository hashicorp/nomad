// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package winsvc

import (
	"errors"
	"io"
)

// NewEventLogger is a stub for non-Windows platforms to generate
// and error when used.
func NewEventLogger(_ string) (io.WriteCloser, error) {
	return nil, errors.New("EventLogger is not supported on this platform")
}
