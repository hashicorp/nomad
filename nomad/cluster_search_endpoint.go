package nomad

import (
	"fmt"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// truncateLimit is the maximum number of matches that will be returned for a
// prefix for a specific context
const (
	truncateLimit = 20
)

// allContexts are the available contexts which searched to find matches for a
// given prefix
var (
	allContexts = []string{"allocs", "nodes", "jobs", "evals"}
)

// ClusterSearch endpoint is used to lookup matches for a given prefix and context
type ClusterSearch struct {
	srv *Server
}

func isSubset(prefix, id string) bool {
	return id[0:len(prefix)] == prefix
}

// getMatches extracts matches for an iterator, and returns a list of ids for
// these matches.
func (c *ClusterSearch) getMatches(iter memdb.ResultIterator, prefix string) ([]string, bool) {
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
		default:
			c.srv.logger.Printf("[ERR] nomad.resources: unexpected type for resources context: %T", t)
			continue
		}

		if !isSubset(prefix, id) {
			break
		}

		matches = append(matches, id)
	}

	return matches, iter.Next() != nil
}

// getResourceIter takes a context and returns a memdb iterator specific to
// that context
func getResourceIter(context, prefix string, ws memdb.WatchSet, state *state.StateStore) (memdb.ResultIterator, error) {
	switch context {
	case "jobs":
		return state.JobsByIDPrefix(ws, prefix)
	case "evals":
		return state.EvalsByIDPrefix(ws, prefix)
	case "allocs":
		return state.AllocsByIDPrefix(ws, prefix)
	case "nodes":
		return state.NodesByIDPrefix(ws, prefix)
	default:
		return nil, fmt.Errorf("context must be one of %v; got %q", allContexts, context)
	}
}

// If the length of a prefix is odd, return a subset to the last even character
// This only applies to UUIDs, Job names are excluded
func roundUUIDDownIfOdd(prefix, context string) string {
	if context == "job" {
		return prefix
	}

	if len(prefix)%2 == 0 {
		return prefix
	}
	return prefix[:len(prefix)-1]
}

// List is used to list matches for a given prefix. ClusterSearch returns jobs,
// evaluations, allocations, and/or nodes.
func (c *ClusterSearch) List(args *structs.ClusterSearchRequest,
	reply *structs.ClusterSearchResponse) error {
	reply.Matches = make(map[string][]string)
	reply.Truncations = make(map[string]bool)

	// Setup the blocking query
	opts := blockingOptions{
		queryMeta: &reply.QueryMeta,
		queryOpts: &structs.QueryOptions{},
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			iters := make(map[string]memdb.ResultIterator)

			contexts := allContexts
			if args.Context != "" {
				contexts = []string{args.Context}
			}

			for _, e := range contexts {
				iter, err := getResourceIter(e, roundUUIDDownIfOdd(args.Prefix, args.Context), ws, state)
				if err != nil {
					return err
				}
				iters[e] = iter
			}

			// Return matches for the given prefix
			for k, v := range iters {
				res, isTrunc := c.getMatches(v, args.Prefix)
				reply.Matches[k] = res
				reply.Truncations[k] = isTrunc
			}

			// Set the index for the context. If the context has been specified, it
			// will be used as the index of the response. Otherwise, the
			// maximum index from all resources will be used.
			for _, e := range contexts {
				index, err := state.Index(e)
				if err != nil {
					return err
				}
				if index > reply.Index {
					reply.Index = index
				}
			}

			c.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return c.srv.blockingRPC(&opts)
}
