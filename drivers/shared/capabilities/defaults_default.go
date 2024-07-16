// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package capabilities

// DockerDefaults is a list of Linux capabilities enabled by Docker by default
// and is used to compute the set of capabilities to add/drop given docker driver
// configuration.
//
// https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities
func DockerDefaults() *Set {
	defaults := NomadDefaults()
	defaults.Add("NET_RAW")
	return defaults
}
