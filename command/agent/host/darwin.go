// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build darwin
// +build darwin

package host

func mountedPaths() []string {
	return []string{"/"}
}
