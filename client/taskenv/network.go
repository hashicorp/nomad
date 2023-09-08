// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskenv

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// InterpolateNetworks returns an interpolated copy of the task group networks
// with values from the task's environment.
//
// Current interoperable fields:
//   - Hostname
//   - DNS
func InterpolateNetworks(taskEnv *TaskEnv, networks structs.Networks) structs.Networks {

	// Guard against not having a valid taskEnv. This can be the case if the
	// PreKilling or Exited hook is run before Poststart.
	if taskEnv == nil || networks == nil {
		return nil
	}

	// Create a copy of the networks array, so we can manipulate the copy.
	interpolated := networks.Copy()

	// Iterate the copy and perform the interpolation.
	for i := range interpolated {
		interpolated[i].Hostname = taskEnv.ReplaceEnv(interpolated[i].Hostname)
		if interpolated[i].DNS != nil {
			interpolated[i].DNS.Servers = taskEnv.ParseAndReplace(interpolated[i].DNS.Servers)
			interpolated[i].DNS.Searches = taskEnv.ParseAndReplace(interpolated[i].DNS.Searches)
			interpolated[i].DNS.Options = taskEnv.ParseAndReplace(interpolated[i].DNS.Options)
		}
	}

	return interpolated
}
