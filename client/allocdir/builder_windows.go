// +build !linux

package allocdir

import "github.com/hashicorp/nomad/nomad/structs"

func (r *AllocRunner) Build(tasks []*structs.Task) error {
	// TODO: Need to figure out how to do mounts on windows.
	return nil
}

func (r *AllocDir) Embed(task string, dirs map[string]string) error {
	return nil
}
