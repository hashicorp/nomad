package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Alloc endpoint is used for manipulating allocations
type Alloc struct {
	srv    *Server
	logger log.Logger
}

// List is used to list the allocations in the system
func (a *Alloc) List(args *structs.AllocListRequest, reply *structs.AllocListResponse) error {
	if done, err := a.srv.forward("Alloc.List", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "alloc", "list"}, time.Now())

	if args.RequestNamespace() == structs.AllNamespacesSentinel {
		return a.listAllNamespaces(args, reply)
	}

	// Check namespace read-job permissions
	aclObj, err := a.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture all the allocations
			var err error
			var iter memdb.ResultIterator

			prefix := args.QueryOptions.Prefix
			if prefix != "" {
				iter, err = state.AllocsByIDPrefix(ws, args.RequestNamespace(), prefix)
			} else {
				iter, err = state.AllocsByNamespace(ws, args.RequestNamespace())
			}
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
				allocs = append(allocs, alloc.Stub(args.Fields))
			}
			reply.Allocations = allocs

			// Use the last index that affected the jobs table
			index, err := state.Index("allocs")
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

// listAllNamespaces lists all allocations across all namespaces
func (a *Alloc) listAllNamespaces(args *structs.AllocListRequest, reply *structs.AllocListResponse) error {
	// Check for read-job permissions
	aclObj, err := a.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	}
	prefix := args.QueryOptions.Prefix
	allow := func(ns string) bool {
		return aclObj.AllowNsOp(ns, acl.NamespaceCapabilityReadJob)
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// get list of accessible namespaces
			allowedNSes, err := allowedNSes(aclObj, state, allow)
			if err == structs.ErrPermissionDenied {
				// return empty allocations if token isn't authorized for any
				// namespace, matching other endpoints
				reply.Allocations = []*structs.AllocListStub{}
			} else if err != nil {
				return err
			} else {
				var iter memdb.ResultIterator
				var err error
				if prefix != "" {
					iter, err = state.AllocsByIDPrefixAllNSs(ws, prefix)
				} else {
					iter, err = state.Allocs(ws)
				}
				if err != nil {
					return err
				}

				var allocs []*structs.AllocListStub
				for raw := iter.Next(); raw != nil; raw = iter.Next() {
					alloc := raw.(*structs.Allocation)
					if allowedNSes != nil && !allowedNSes[alloc.Namespace] {
						continue
					}
					allocs = append(allocs, alloc.Stub(args.Fields))
				}
				reply.Allocations = allocs
			}

			// Use the last index that affected the jobs table
			index, err := state.Index("allocs")
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

	// Check namespace read-job permissions before performing blocking query.
	allowNsOp := acl.NamespaceValidator(acl.NamespaceCapabilityReadJob)
	aclObj, err := a.srv.ResolveToken(args.AuthToken)
	if err != nil {
		// If ResolveToken had an unexpected error return that
		if err != structs.ErrTokenNotFound {
			return err
		}

		// Attempt to lookup AuthToken as a Node.SecretID since nodes
		// call this endpoint and don't have an ACL token.
		node, stateErr := a.srv.fsm.State().NodeBySecretID(nil, args.AuthToken)
		if stateErr != nil {
			// Return the original ResolveToken error with this err
			var merr multierror.Error
			merr.Errors = append(merr.Errors, err, stateErr)
			return merr.ErrorOrNil()
		}

		// Not a node or a valid ACL token
		if node == nil {
			return structs.ErrTokenNotFound
		}
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Lookup the allocation
			out, err := state.AllocByID(ws, args.AllocID)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Alloc = out
			if out != nil {
				// Re-check namespace in case it differs from request.
				if !allowNsOp(aclObj, out.Namespace) {
					return structs.NewErrUnknownAllocation(args.AllocID)
				}

				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the allocs table
				index, err := state.Index("allocs")
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

// GetAllocs is used to lookup a set of allocations
func (a *Alloc) GetAllocs(args *structs.AllocsGetRequest,
	reply *structs.AllocsGetResponse) error {
	if done, err := a.srv.forward("Alloc.GetAllocs", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "alloc", "get_allocs"}, time.Now())

	allocs := make([]*structs.Allocation, len(args.AllocIDs))

	// Setup the blocking query. We wait for at least one of the requested
	// allocations to be above the min query index. This guarantees that the
	// server has received that index.
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Lookup the allocation
			thresholdMet := false
			maxIndex := uint64(0)
			for i, alloc := range args.AllocIDs {
				out, err := state.AllocByID(ws, alloc)
				if err != nil {
					return err
				}
				if out == nil {
					// We don't have the alloc yet
					thresholdMet = false
					break
				}

				// Store the pointer
				allocs[i] = out

				// Check if we have passed the minimum index
				if out.ModifyIndex > args.QueryOptions.MinQueryIndex {
					thresholdMet = true
				}

				if maxIndex < out.ModifyIndex {
					maxIndex = out.ModifyIndex
				}
			}

			// Setup the output
			if thresholdMet {
				reply.Allocs = allocs
				reply.Index = maxIndex
			} else {
				// Use the last index that affected the nodes table
				index, err := state.Index("allocs")
				if err != nil {
					return err
				}
				reply.Index = index
			}

			// Set the query response
			a.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		},
	}
	return a.srv.blockingRPC(&opts)
}

// Stop is used to stop an allocation and migrate it to another node.
func (a *Alloc) Stop(args *structs.AllocStopRequest, reply *structs.AllocStopResponse) error {
	if done, err := a.srv.forward("Alloc.Stop", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "alloc", "stop"}, time.Now())

	alloc, err := getAlloc(a.srv.State(), args.AllocID)
	if err != nil {
		return err
	}

	// Check for namespace alloc-lifecycle permissions.
	allowNsOp := acl.NamespaceValidator(acl.NamespaceCapabilityAllocLifecycle)
	aclObj, err := a.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	} else if !allowNsOp(aclObj, alloc.Namespace) {
		return structs.ErrPermissionDenied
	}

	now := time.Now().UTC().UnixNano()
	eval := &structs.Evaluation{
		ID:             uuid.Generate(),
		Namespace:      alloc.Namespace,
		Priority:       alloc.Job.Priority,
		Type:           alloc.Job.Type,
		TriggeredBy:    structs.EvalTriggerAllocStop,
		JobID:          alloc.Job.ID,
		JobModifyIndex: alloc.Job.ModifyIndex,
		Status:         structs.EvalStatusPending,
		CreateTime:     now,
		ModifyTime:     now,
	}

	transitionReq := &structs.AllocUpdateDesiredTransitionRequest{
		Evals: []*structs.Evaluation{eval},
		Allocs: map[string]*structs.DesiredTransition{
			args.AllocID: {
				Migrate:         helper.BoolToPtr(true),
				NoShutdownDelay: helper.BoolToPtr(args.NoShutdownDelay),
			},
		},
	}

	// Commit this update via Raft
	_, index, err := a.srv.raftApply(structs.AllocUpdateDesiredTransitionRequestType, transitionReq)
	if err != nil {
		a.logger.Error("AllocUpdateDesiredTransitionRequest failed", "error", err)
		return err
	}

	// Setup the response
	reply.Index = index
	reply.EvalID = eval.ID
	return nil
}

// UpdateDesiredTransition is used to update the desired transitions of an
// allocation.
func (a *Alloc) UpdateDesiredTransition(args *structs.AllocUpdateDesiredTransitionRequest, reply *structs.GenericResponse) error {
	if done, err := a.srv.forward("Alloc.UpdateDesiredTransition", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "alloc", "update_desired_transition"}, time.Now())

	// Check that it is a management token.
	if aclObj, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Ensure at least a single alloc
	if len(args.Allocs) == 0 {
		return fmt.Errorf("must update at least one allocation")
	}

	// Commit this update via Raft
	_, index, err := a.srv.raftApply(structs.AllocUpdateDesiredTransitionRequestType, args)
	if err != nil {
		a.logger.Error("AllocUpdateDesiredTransitionRequest failed", "error", err)
		return err
	}

	// Setup the response
	reply.Index = index
	return nil
}
