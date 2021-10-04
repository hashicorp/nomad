package taskenv

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// InterpolateNetworks returns an interpolated copy of the task group networks
// with values from the task's environment.
//
// Current interoperable fields:
//   - Hostname
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
	}

	return interpolated
}
