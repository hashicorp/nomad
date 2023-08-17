// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux
// +build !linux

package allocdir

// currently a noop on non-Linux platforms
func (t *TaskDir) unmountSpecialDirs() error {
	return nil
}
