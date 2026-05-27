// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux
// +build !linux

package allocdir

// currently a noop on non-Linux platforms
func (t *TaskDir) unmountSpecialDirs() error {
	return nil
}
