package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/watch"
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

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		watch:     watch.NewItems(watch.Item{Table: "allocs"}),
		run: func() error {
			// Capture all the allocations
			snap, err := a.srv.fsm.State().Snapshot()
			if err != nil {
				return err
			}
			iter, err := snap.Allocs()
			if err != nil {
				return err
			}

			var allocs []*structs.AllocListStub
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				alloc := raw.(*structs.Allocation)
				allocs = append(allocs, alloc.Stub())
			}
			reply.Allocations = allocs

			// Use the last index that affected the jobs table
			index, err := snap.Index("allocs")
			if err != nil {
				return err
			}
			reply.Index = index

			// Set the query response
			a.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return a.srv.blockingRPC(&opts)
}

// GetAlloc is used to lookup a particular allocation
func (a *Alloc) GetAlloc(args *structs.AllocSpecificRequest,
	reply *structs.SingleAllocResponse) error {
	if done, err := a.srv.forward("Alloc.GetAlloc", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "alloc", "get_alloc"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		watch:     watch.NewItems(watch.Item{Alloc: args.AllocID}),
		run: func() error {
			// Lookup the allocation
			snap, err := a.srv.fsm.State().Snapshot()
			if err != nil {
				return err
			}

			var out *structs.Allocation

			// Exact lookup if the identifier length is 36 (full UUID)
			if len(args.AllocID) == 36 {
				out, err = snap.AllocByID(args.AllocID)
				if err != nil {
					return err
				}
			} else {
				iter, err := snap.AllocByIDPrefix(args.AllocID)
				if err != nil {
					return err
				}

				// Gather all matching nodes
				var allocs []*structs.Allocation
				var allocIds []string
				for {
					raw := iter.Next()
					if raw == nil {
						break
					}
					alloc := raw.(*structs.Allocation)
					allocIds = append(allocIds, alloc.ID)
					allocs = append(allocs, alloc)
				}

				if len(allocs) == 1 {
					// Return unique allocation
					out = allocs[0]
				} else if len(allocs) > 1 {
					return fmt.Errorf("Ambiguous identifier: %+v", allocIds)
				}
			}

			// Setup the output
			reply.Alloc = out
			if out != nil {
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
		}}
	return a.srv.blockingRPC(&opts)
}
