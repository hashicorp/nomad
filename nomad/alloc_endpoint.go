package nomad

import (
	"time"

	"github.com/hashicorp/nomad/Godeps/_workspace/src/github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Alloc endpoint is used for manipulating allocations
type Alloc struct {
	srv *Server
}

// List is used to list the allocations in the system
func (a *Alloc) List(args *structs.AllocListRequest, reply *structs.AllocListResponse) error {
	if done, err := a.srv.forward("Alloc.List", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "alloc", "list"}, time.Now())

	// Capture all the allocations
	snap, err := a.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	iter, err := snap.Allocs()
	if err != nil {
		return err
	}

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		alloc := raw.(*structs.Allocation)
		reply.Allocations = append(reply.Allocations, alloc.Stub())
	}

	// Use the last index that affected the jobs table
	index, err := snap.Index("allocs")
	if err != nil {
		return err
	}
	reply.Index = index

	// Set the query response
	a.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// GetAlloc is used to lookup a particular allocation
func (a *Alloc) GetAlloc(args *structs.AllocSpecificRequest,
	reply *structs.SingleAllocResponse) error {
	if done, err := a.srv.forward("Alloc.GetAlloc", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "alloc", "get_alloc"}, time.Now())

	// Lookup the allocation
	snap, err := a.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	out, err := snap.AllocByID(args.AllocID)
	if err != nil {
		return err
	}

	// Setup the output
	if out != nil {
		reply.Alloc = out
		reply.Index = out.ModifyIndex
	} else {
		// Use the last index that affected the nodes table
		index, err := snap.Index("allocs")
		if err != nil {
			return err
		}
		reply.Index = index
	}

	// Set the query response
	a.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}
