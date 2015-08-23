package client

import "github.com/hashicorp/nomad/nomad/structs"

// AllocContext is used to wrap an allocation and provide the execution context.
type AllocContext struct {
	client *Client
	alloc  *structs.Allocation
}

// NewAllocContext is used to create a new allocation context
func NewAllocContext(client *Client, alloc *structs.Allocation) *AllocContext {
	ctx := &AllocContext{
		client: client,
		alloc:  alloc,
	}
	return ctx
}

// Alloc returns the associated allocation
func (ctx *AllocContext) Alloc() *structs.Allocation {
	return ctx.alloc
}

// Run is a long running goroutine used to manage an allocation
func (ctx *AllocContext) Run() {
	for {
	}
}

// Update is used to update the allocation of the context
func (ctx *AllocContext) Update(update *structs.Allocation) {
}

// Destroy is used to indicate that the allocation context should be destroyed
func (ctx *AllocContext) Destroy() {
}
