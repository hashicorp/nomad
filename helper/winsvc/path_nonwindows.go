// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package winsvc

import "errors"

func NewWindowsPaths() WindowsPaths {
	return &windowsPaths{}
}

type windowsPaths struct{}

func (w *windowsPaths) Expand(string) (string, error) {
	return "", errors.New("Windows path expansion not supported on this platform")
}

func (w *windowsPaths) CreateDirectory(string, bool) error {
	return errors.New("Windows directory creation not supported on this platform")
}
