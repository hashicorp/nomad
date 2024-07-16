// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package capabilities

// DockerDefaults is a list of Windows capabilities enabled by Docker by default
// and is used to compute the set of capabilities to add/drop given docker driver
// configuration.
func DockerDefaults() *Set {
	defaults := NomadDefaults()
	return defaults
}
