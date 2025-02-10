// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package capabilities

import (
	"strings"

	"github.com/docker/docker/api/types"
)

// DockerDefaults is a list of Windows capabilities enabled by Docker by default
// and is used to compute the set of capabilities to add/drop given docker driver
// configuration.
//
// Doing this on windows is somewhat tricky, because capabilities differ by
// runtime, so we have to perform some extra checks.
func DockerDefaults(ver types.Version) *Set {
	defaults := NomadDefaults()

	// Docker CE doesn't include NET_RAW on Windows, Mirantis (aka Docker EE) does
	if strings.Contains(ver.Platform.Name, "Mirantis") {
		defaults.Add("NET_RAW")
	}

	return defaults
}
