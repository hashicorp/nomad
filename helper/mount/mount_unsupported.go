// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux
// +build !linux

package mount

import (
	"errors"
)

// mounter provides the default implementation of mount.Mounter
// for unsupported platforms.
type mounter struct {
}

// New returns a Mounter for the current system.
func New() Mounter {
	return &mounter{}
}

func (m *mounter) IsNotAMountPoint(path string) (bool, error) {
	return false, errors.New("Unsupported platform")
}

func (m *mounter) Mount(device, target, mountType, options string) error {
	return errors.New("Unsupported platform")
}
