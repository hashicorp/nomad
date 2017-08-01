package nomad

import (
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

type Resources struct {
	srv *Server
}

// List is used to list the jobs registered in the system
// TODO logic to determine context, to return only that context if needed
// TODO if no context, return all
// TODO refactor to prevent duplication
func (r *Resources) List(args *structs.ResourcesRequest,
	reply *structs.ResourcesResponse) error {

	resources := structs.ResourcesListStub{}
	resources.Matches = make(map[string][]string)

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {

			// return jobs matching given prefix
			var err error
			var iter memdb.ResultIterator
			iter, err = state.JobsByIDPrefix(ws, args.QueryOptions.Prefix)
			if err != nil {
				return err
			}

			var jobs []string
			for i := 0; i < 20; i++ {
				raw := iter.Next()
				if raw == nil {
					break
				}

				job := raw.(*structs.Job)
				jobs = append(jobs, job.ID)
			}

			resources.Matches["jobs"] = jobs
			reply.Resources = resources

			return nil
		}}
	return r.srv.blockingRPC(&opts)
}
