package interfaces

import "github.com/hashicorp/nomad/nomad/structs"

type Client interface {
	AllocStateHandler
}

// AllocStateHandler exposes a handler to be called when a allocation's state changes
type AllocStateHandler interface {
	// AllocStateUpdated is used to emit an updated allocation. This allocation
	// is stripped to only include client settable fields.
	AllocStateUpdated(alloc *structs.Allocation) error
}
