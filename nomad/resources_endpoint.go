package nomad

import (
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type Resources struct {
	srv *Server
}

func getMatches(iter memdb.ResultIterator, context, prefix string) ([]string, bool) {
	var matches []string
	isTruncated := false

	for i := 0; i < 20; i++ {
		raw := iter.Next()
		if raw == nil {
			break
		}

		getID := func(i interface{}) string {
			switch i.(type) {
			case *structs.Job:
				return i.(*structs.Job).ID
			case *structs.Evaluation:
				return i.(*structs.Evaluation).ID
			default:
				return ""
			}
		}

		id := getID(raw)
		if id == "" {
			continue
		}

		matches = append(matches, id)
	}

	if iter.Next() != nil {
		isTruncated = true
	}

	return matches, isTruncated
}

// List is used to list the jobs registered in the system
// TODO if no context, return all
func (r *Resources) List(args *structs.ResourcesRequest,
	reply *structs.ResourcesResponse) error {
	reply.Matches = make(map[string][]string)
	reply.Truncations = make(map[string]bool)

	// Setup the blocking query
	opts := blockingOptions{
		queryMeta: &reply.QueryMeta,
		queryOpts: &structs.QueryOptions{},
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			// return jobs matching given prefix
			var err error
			var iter memdb.ResultIterator
			res := make([]string, 0)
			isTrunc := false

			if args.Context == "job" {
				iter, err = state.JobsByIDPrefix(ws, args.Prefix)
			} else if args.Context == "eval" {
				iter, err = state.EvalsByIDPrefix(ws, args.Prefix)
			}

			if err != nil {
				return err
			}
			res, isTrunc = getMatches(iter, args.Context, args.Prefix)
			reply.Matches[args.Context] = res
			reply.Truncations[args.Context] = isTrunc

			return nil
		}}
	return r.srv.blockingRPC(&opts)
}
