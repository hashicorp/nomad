package nomad

import (
	"fmt"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	st "github.com/hashicorp/nomad/nomad/structs"
)

// truncateLimit is the maximum number of matches that will be returned for a
// prefix for a specific context
const (
	truncateLimit = 20
)

// allContexts are the available contexts which are searched to find matches
// for a given prefix
var (
	allContexts = []st.Context{st.Allocs, st.Jobs, st.Nodes, st.Evals}
)

// Search endpoint is used to look up matches for a given prefix and context
type Search struct {
	srv *Server
}

func isSubset(prefix, id string) bool {
	return id[0:len(prefix)] == prefix
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
		case *st.Job:
			id = raw.(*st.Job).ID
		case *st.Evaluation:
			id = raw.(*st.Evaluation).ID
		case *st.Allocation:
			id = raw.(*st.Allocation).ID
		case *st.Node:
			id = raw.(*st.Node).ID
		default:
			s.srv.logger.Printf("[ERR] nomad.resources: unexpected type for resources context: %T", t)
			continue
		}

		if !isSubset(prefix, id) {
			continue
		}

		matches = append(matches, id)
	}

	return matches, iter.Next() != nil
}

// getResourceIter takes a context and returns a memdb iterator specific to
// that context
func getResourceIter(context st.Context, prefix string, ws memdb.WatchSet, state *state.StateStore) (memdb.ResultIterator, error) {
	switch context {
	case st.Jobs:
		return state.JobsByIDPrefix(ws, prefix)
	case st.Evals:
		return state.EvalsByIDPrefix(ws, prefix)
	case st.Allocs:
		return state.AllocsByIDPrefix(ws, prefix)
	case st.Nodes:
		return state.NodesByIDPrefix(ws, prefix)
	default:
		return nil, fmt.Errorf("context must be one of %v; got %q", allContexts, context)
	}
}

// If the length of a prefix is odd, return a subset to the last even character
// This only applies to UUIDs, jobs are excluded
func roundUUIDDownIfOdd(prefix string, context st.Context) string {
	if context == st.Jobs {
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
func (s *Search) PrefixSearch(args *st.SearchRequest,
	reply *st.SearchResponse) error {
	reply.Matches = make(map[st.Context][]string)
	reply.Truncations = make(map[st.Context]bool)

	// Setup the blocking query
	opts := blockingOptions{
		queryMeta: &reply.QueryMeta,
		queryOpts: &st.QueryOptions{},
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			iters := make(map[st.Context]memdb.ResultIterator)

			contexts := allContexts
			if args.Context != st.All {
				contexts = []st.Context{args.Context}
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
				res, isTrunc := s.getMatches(v, args.Prefix)
				reply.Matches[k] = res
				reply.Truncations[k] = isTrunc
			}

			// Set the index for the context. If the context has been specified, it
			// will be used as the index of the response. Otherwise, the
			// maximum index from all resources will be used.
			for _, e := range contexts {
				index, err := state.Index(string(e))
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
