package nomad

import (
	"time"

	metrics "github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Deployment endpoint is used for manipulating deployments
type Deployment struct {
	srv *Server
}

// TODO http endpoint and api
// Allocations returns the list of allocations that are a part of the deployment
func (d *Deployment) Allocations(args *structs.DeploymentSpecificRequest, reply *structs.AllocListResponse) error {
	if done, err := d.srv.forward("Deployment.Allocations", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "allocations"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture all the allocations
			allocs, err := state.AllocsByDeployment(ws, args.DeploymentID)
			if err != nil {
				return err
			}

			stubs := make([]*structs.AllocListStub, 0, len(allocs))
			for _, alloc := range allocs {
				stubs = append(stubs, alloc.Stub())
			}
			reply.Allocations = stubs

			// Use the last index that affected the jobs table
			index, err := state.Index("allocs")
			if err != nil {
				return err
			}
			reply.Index = index

			// Set the query response
			d.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return d.srv.blockingRPC(&opts)
}
