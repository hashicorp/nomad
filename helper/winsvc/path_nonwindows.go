// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package winsvc

import "errors"

func ExpandPath(path string) (string, error) {
	return "", errors.New("Windows path expansion not supported on this platform")
}
