// +build !linux

package allocdir

import (
	"errors"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (d *AllocDir) Build(tasks []*structs.Task) error {
	// TODO: Need to figure out how to do mounts on windows.
	return errors.New("Not implemented")
}

func (d *AllocDir) Embed(task string, dirs map[string]string) error {
	return errors.New("Not implemented")
}
