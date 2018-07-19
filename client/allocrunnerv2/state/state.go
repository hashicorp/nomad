package state

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// XXX Why its own package?
// State captures the state of the allocation runner.
type State struct {
	// ClientStatus captures the overall state of the allocation
	ClientStatus string

	// ClientDescription is an optional human readable description of the
	// allocations client state
	ClientDescription string

	// DeploymentStatus captures the status of the deployment
	DeploymentStatus *structs.AllocDeploymentStatus
}
