// +build !linux

package helper

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// This is an empty impl of ResourceConstrainer for platforms where we enforce
// resource constraints
type ResourceConstrainer struct {
}

// NewResourceConstrainer creates a cgroup which can be applied to a pid
func NewResourceConstrainer(resources *structs.Resources, pid int) (*ResourceConstrainer, error) {
	return &ResourceConstrainer{}, nil
}

// Apply will place the pid into the created cgroups.
func (r *ResourceConstrainer) Apply() error {
	return nil
}

// Destroy removes the cgroup created for constraining resources for the pid and
// if the pid is still alive then the pid is killed along with the cgroup
func (r *ResourceConstrainer) Destroy() error {
	return nil
}
