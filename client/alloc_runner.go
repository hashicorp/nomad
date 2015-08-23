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
func (c *AllocRunner) Alloc() *structs.Allocation {
	return c.alloc
}

// Run is a long running goroutine used to manage an allocation
func (c *AllocRunner) Run() {
	c.logger.Printf("[DEBUG] client: starting context for alloc '%s'", c.alloc.ID)

	// TODO: Start
	for {
		select {
		case update := <-c.updateCh:
			// TODO: Update
			c.alloc = update
		case <-c.destroyCh:
			// TODO: Destroy
			return
		}
	}
}

// Update is used to update the allocation of the context
func (c *AllocRunner) Update(update *structs.Allocation) {
	select {
	case c.updateCh <- update:
	default:
		c.logger.Printf("[ERR] client: dropping update to alloc '%s'", update.ID)
	}
}

// Destroy is used to indicate that the allocation context should be destroyed
func (c *AllocRunner) Destroy() {
	c.destroyLock.Lock()
	defer c.destroyLock.Unlock()

	if c.destroy {
		return
	}
	c.destroy = true
	close(c.destroyCh)
}
