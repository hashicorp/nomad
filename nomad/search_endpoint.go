package nomad

import (
	"fmt"
	"strings"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// truncateLimit is the maximum number of matches that will be returned for a
	// prefix for a specific context
	truncateLimit = 20
)

var (
	// allContexts are the available contexts which are searched to find matches
	// for a given prefix
	allContexts = []structs.Context{structs.Allocs, structs.Jobs, structs.Nodes,
		structs.Evals, structs.Deployments}
)

// Search endpoint is used to look up matches for a given prefix and context
type Search struct {
	srv *Server
}

// getMatches extracts matches for an iterator, and returns a list of ids for
// these matches.
func (s *Search) getMatches(iter memdb.ResultIterator, prefix string) ([]string, bool) {
	var matches []string

	for i := 0; i < truncateLimit; i++ {
		raw := iter.Next()
		if raw == nil {
			break
		}

		var id string
		switch t := raw.(type) {
		case *structs.Job:
			id = raw.(*structs.Job).ID
		case *structs.Evaluation:
			id = raw.(*structs.Evaluation).ID
		case *structs.Allocation:
			id = raw.(*structs.Allocation).ID
		case *structs.Node:
			id = raw.(*structs.Node).ID
		case *structs.Deployment:
			id = raw.(*structs.Deployment).ID
		default:
			s.srv.logger.Printf("[ERR] nomad.resources: unexpected type for resources context: %T", t)
			continue
		}

		if !strings.HasPrefix(id, prefix) {
			continue
		}

		matches = append(matches, id)
	}

	return matches, iter.Next() != nil
}

// getResourceIter takes a context and returns a memdb iterator specific to
// that context
func getResourceIter(context structs.Context, prefix string, ws memdb.WatchSet, state *state.StateStore) (memdb.ResultIterator, error) {
	switch context {
	case structs.Jobs:
		return state.JobsByIDPrefix(ws, prefix)
	case structs.Evals:
		return state.EvalsByIDPrefix(ws, prefix)
	case structs.Allocs:
		return state.AllocsByIDPrefix(ws, prefix)
	case structs.Nodes:
		return state.NodesByIDPrefix(ws, prefix)
	case structs.Deployments:
		return state.DeploymentsByIDPrefix(ws, prefix)
	default:
		return nil, fmt.Errorf("context must be one of %v; got %q", allContexts, context)
	}
}

// If the length of a prefix is odd, return a subset to the last even character
// This only applies to UUIDs, jobs are excluded
func roundUUIDDownIfOdd(prefix string, context structs.Context) string {
	if context == structs.Jobs {
		return prefix
	}

	l := len(prefix)
	if l%2 == 0 {
		return prefix
	}
	return prefix[:l-1]
}

// PrefixSearch is used to list matches for a given prefix, and returns
// matching jobs, evaluations, allocations, and/or nodes.
func (s *Search) PrefixSearch(args *structs.SearchRequest,
	reply *structs.SearchResponse) error {
	reply.Matches = make(map[structs.Context][]string)
	reply.Truncations = make(map[structs.Context]bool)

	// Setup the blocking query
	opts := blockingOptions{
		queryMeta: &reply.QueryMeta,
		queryOpts: &structs.QueryOptions{},
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			iters := make(map[structs.Context]memdb.ResultIterator)

			contexts := allContexts
			if args.Context != structs.All {
				contexts = []structs.Context{args.Context}
			}

			for _, ctx := range contexts {
				iter, err := getResourceIter(ctx, roundUUIDDownIfOdd(args.Prefix, args.Context), ws, state)

				if err != nil {
					// Searching other contexts with job names raises an error, which in
					// this case we want to ignore.
					if !strings.Contains(err.Error(), "Invalid UUID: encoding/hex") {
						return err
					}
				} else {
					iters[ctx] = iter
				}
			}

			// Return matches for the given prefix
			for k, v := range iters {
				res, isTrunc := s.getMatches(v, args.Prefix)
				reply.Matches[k] = res
				reply.Truncations[k] = isTrunc
			}

			// Set the index for the context. If the context has been specified, it
			// will be used as the index of the response. Otherwise, the
			// maximum index from all resources will be used.
			for _, ctx := range contexts {
				index, err := state.Index(string(ctx))
				if err != nil {
					return err
				}
				if index > reply.Index {
					reply.Index = index
				}
			}

			s.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return s.srv.blockingRPC(&opts)
}
