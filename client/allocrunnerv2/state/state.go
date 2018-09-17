package state

import (
	"sync"

	"github.com/hashicorp/nomad/nomad/structs"
)

// State captures the state of the allocation runner.
type State struct {
	sync.RWMutex

	// ClientState captures the overall state of the allocation
	ClientState string

	// ClientDesc is an optional human readable description of the
	// allocations client state
	ClientDesc string

	// DeploymentStatus captures the status of the deployment
	DeploymentStatus *structs.AllocDeploymentStatus
}

type PersistentState struct {
	ClientState string
}

func (s *State) PersistentState() *PersistentState {
	s.RLock()
	defer s.RUnlock()
	return &PersistentState{
		ClientState: s.ClientState,
	}
}
