package allocdir

import "github.com/hashicorp/nomad/nomad/structs"

func (r *AllocRunner) Build(tasks []*structs.Task) error {
	// TODO: Need to figure out how to do mounts on windows.
	return nil
}
