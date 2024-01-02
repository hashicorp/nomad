// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build darwin
// +build darwin

package host

func mountedPaths() []string {
	return []string{"/"}
}
