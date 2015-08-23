package client

import (
	"log"
	"sync"

	"github.com/hashicorp/nomad/nomad/structs"
)

// AllocRunner is used to wrap an allocation and provide the execution context.
type AllocRunner struct {
	client *Client
	logger *log.Logger

	alloc *structs.Allocation

	updateCh chan *structs.Allocation

	destroy     bool
	destroyCh   chan struct{}
	destroyLock sync.Mutex
}

// NewAllocRunner is used to create a new allocation context
func NewAllocRunner(client *Client, alloc *structs.Allocation) *AllocRunner {
	ctx := &AllocRunner{
		client:    client,
		logger:    client.logger,
		alloc:     alloc,
		updateCh:  make(chan *structs.Allocation, 8),
		destroyCh: make(chan struct{}),
	}
	return ctx
}

// Alloc returns the associated allocation
func (r *AllocRunner) Alloc() *structs.Allocation {
	return r.alloc
}

// Run is a long running goroutine used to manage an allocation
func (r *AllocRunner) Run() {
	r.logger.Printf("[DEBUG] client: starting context for alloc '%s'", r.alloc.ID)

	// TODO: Start
	for {
		select {
		case update := <-r.updateCh:
			// TODO: Update
			r.alloc = update
		case <-r.destroyCh:
			// TODO: Destroy
			return
		}
	}
}

// Update is used to update the allocation of the context
func (r *AllocRunner) Update(update *structs.Allocation) {
	select {
	case r.updateCh <- update:
	default:
		r.logger.Printf("[ERR] client: dropping update to alloc '%s'", update.ID)
	}
}

// Destroy is used to indicate that the allocation context should be destroyed
func (r *AllocRunner) Destroy() {
	r.destroyLock.Lock()
	defer r.destroyLock.Unlock()

	if r.destroy {
		return
	}
	r.destroy = true
	close(r.destroyCh)
}
