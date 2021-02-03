package nomad

import (
	"fmt"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// truncateLimit is the maximum number of matches that will be returned for a
	// prefix for a specific context
	truncateLimit = 20
)

var (
	// ossContexts are the oss contexts which are searched to find matches
	// for a given prefix
	ossContexts = []structs.Context{
		structs.Allocs,
		structs.Jobs,
		structs.Nodes,
		structs.Evals,
		structs.Deployments,
		structs.Plugins,
		structs.Volumes,
		structs.ScalingPolicies,
		structs.Namespaces,
	}
)

// Search endpoint is used to look up matches for a given prefix and context
type Search struct {
	srv    *Server
	logger log.Logger
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
			id = t.ID
		case *structs.Evaluation:
			id = t.ID
		case *structs.Allocation:
			id = t.ID
		case *structs.Node:
			id = t.ID
		case *structs.Deployment:
			id = t.ID
		case *structs.CSIPlugin:
			id = t.ID
		case *structs.CSIVolume:
			id = t.ID
		case *structs.ScalingPolicy:
			id = t.ID
		case *structs.Namespace:
			id = t.Name
		default:
			matchID, ok := getEnterpriseMatch(raw)
			if !ok {
				s.logger.Error("unexpected type for resources context", "type", fmt.Sprintf("%T", t))
				continue
			}

			id = matchID
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
func getResourceIter(context structs.Context, aclObj *acl.ACL, namespace, prefix string, ws memdb.WatchSet, state *state.StateStore) (memdb.ResultIterator, error) {
	switch context {
	case structs.Jobs:
		return state.JobsByIDPrefix(ws, namespace, prefix)
	case structs.Evals:
		return state.EvalsByIDPrefix(ws, namespace, prefix)
	case structs.Allocs:
		return state.AllocsByIDPrefix(ws, namespace, prefix)
	case structs.Nodes:
		return state.NodesByIDPrefix(ws, prefix)
	case structs.Deployments:
		return state.DeploymentsByIDPrefix(ws, namespace, prefix)
	case structs.Plugins:
		return state.CSIPluginsByIDPrefix(ws, prefix)
	case structs.ScalingPolicies:
		return state.ScalingPoliciesByIDPrefix(ws, namespace, prefix)
	case structs.Volumes:
		return state.CSIVolumesByIDPrefix(ws, namespace, prefix)
	case structs.Namespaces:
		iter, err := state.NamespacesByNamePrefix(ws, prefix)
		if err != nil {
			return nil, err
		}
		if aclObj == nil {
			return iter, nil
		}
		return memdb.NewFilterIterator(iter, namespaceFilter(aclObj)), nil
	default:
		return getEnterpriseResourceIter(context, aclObj, namespace, prefix, ws, state)
	}
}

// namespaceFilter wraps a namespace iterator with a filter for removing
// namespaces the ACL can't access.
func namespaceFilter(aclObj *acl.ACL) memdb.FilterFunc {
	return func(v interface{}) bool {
		return !aclObj.AllowNamespace(v.(*structs.Namespace).Name)
	}
}

// If the length of a prefix is odd, return a subset to the last even character
// This only applies to UUIDs, jobs are excluded
func roundUUIDDownIfOdd(prefix string, context structs.Context) string {
	if context == structs.Jobs {
		return prefix
	}

	// We ignore the count of hyphens when calculating if the prefix is even:
	// E.g "e3671fa4-21"
	numHyphens := strings.Count(prefix, "-")
	l := len(prefix) - numHyphens
	if l%2 == 0 {
		return prefix
	}
	return prefix[:len(prefix)-1]
}

// PrefixSearch is used to list matches for a given prefix, and returns
// matching jobs, evaluations, allocations, and/or nodes.
func (s *Search) PrefixSearch(args *structs.SearchRequest, reply *structs.SearchResponse) error {
	if done, err := s.srv.forward("Search.PrefixSearch", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "search", "prefix_search"}, time.Now())

	aclObj, err := s.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	}

	namespace := args.RequestNamespace()

	// Require either node:read or namespace:read-job
	if !anySearchPerms(aclObj, namespace, args.Context) {
		return structs.ErrPermissionDenied
	}

	reply.Matches = make(map[structs.Context][]string)
	reply.Truncations = make(map[structs.Context]bool)

	// Setup the blocking query
	opts := blockingOptions{
		queryMeta: &reply.QueryMeta,
		queryOpts: &structs.QueryOptions{},
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			iters := make(map[structs.Context]memdb.ResultIterator)

			contexts := searchContexts(aclObj, namespace, args.Context)

			for _, ctx := range contexts {
				iter, err := getResourceIter(ctx, aclObj, namespace, roundUUIDDownIfOdd(args.Prefix, args.Context), ws, state)
				if err != nil {
					e := err.Error()
					switch {
					// Searching other contexts with job names raises an error, which in
					// this case we want to ignore.
					case strings.Contains(e, "Invalid UUID: encoding/hex"):
					case strings.Contains(e, "UUID have 36 characters"):
					case strings.Contains(e, "must be even length"):
					case strings.Contains(e, "UUID should have maximum of 4"):
					default:
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
				index, err := state.Index(contextToIndex(ctx))
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
