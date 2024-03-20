// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package allocdir

// cleanTestMounts is a noop helper for non-Linux OSes
func cleanTestMounts() {}
