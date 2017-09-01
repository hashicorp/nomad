package nomad

import (
	"fmt"
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

// GetDeployment is used to request information about a specific deployment
func (d *Deployment) GetDeployment(args *structs.DeploymentSpecificRequest,
	reply *structs.SingleDeploymentResponse) error {
	if done, err := d.srv.forward("Deployment.GetDeployment", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "get_deployment"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Verify the arguments
			if args.DeploymentID == "" {
				return fmt.Errorf("missing deployment ID")
			}

			// Look for the deployment
			out, err := state.DeploymentByID(ws, args.DeploymentID)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Deployment = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the deployments table
				index, err := state.Index("deployment")
				if err != nil {
					return err
				}
				reply.Index = index
			}

			// Set the query response
			d.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return d.srv.blockingRPC(&opts)
}

// Fail is used to force fail a deployment
func (d *Deployment) Fail(args *structs.DeploymentFailRequest, reply *structs.DeploymentUpdateResponse) error {
	if done, err := d.srv.forward("Deployment.Fail", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "fail"}, time.Now())

	// Validate the arguments
	if args.DeploymentID == "" {
		return fmt.Errorf("missing deployment ID")
	}

	// Lookup the deployment
	snap, err := d.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	deploy, err := snap.DeploymentByID(ws, args.DeploymentID)
	if err != nil {
		return err
	}
	if deploy == nil {
		return fmt.Errorf("deployment not found")
	}

	if !deploy.Active() {
		return fmt.Errorf("can't fail terminal deployment")
	}

	// Call into the deployment watcher
	return d.srv.deploymentWatcher.FailDeployment(args, reply)
}

// Pause is used to pause a deployment
func (d *Deployment) Pause(args *structs.DeploymentPauseRequest, reply *structs.DeploymentUpdateResponse) error {
	if done, err := d.srv.forward("Deployment.Pause", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "pause"}, time.Now())

	// Validate the arguments
	if args.DeploymentID == "" {
		return fmt.Errorf("missing deployment ID")
	}

	// Lookup the deployment
	snap, err := d.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	deploy, err := snap.DeploymentByID(ws, args.DeploymentID)
	if err != nil {
		return err
	}
	if deploy == nil {
		return fmt.Errorf("deployment not found")
	}

	if !deploy.Active() {
		if args.Pause {
			return fmt.Errorf("can't pause terminal deployment")
		}

		return fmt.Errorf("can't resume terminal deployment")
	}

	// Call into the deployment watcher
	return d.srv.deploymentWatcher.PauseDeployment(args, reply)
}

// Promote is used to promote canaries in a deployment
func (d *Deployment) Promote(args *structs.DeploymentPromoteRequest, reply *structs.DeploymentUpdateResponse) error {
	if done, err := d.srv.forward("Deployment.Promote", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "promote"}, time.Now())

	// Validate the arguments
	if args.DeploymentID == "" {
		return fmt.Errorf("missing deployment ID")
	}

	// Lookup the deployment
	snap, err := d.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	deploy, err := snap.DeploymentByID(ws, args.DeploymentID)
	if err != nil {
		return err
	}
	if deploy == nil {
		return fmt.Errorf("deployment not found")
	}

	if !deploy.Active() {
		return fmt.Errorf("can't promote terminal deployment")
	}

	// Call into the deployment watcher
	return d.srv.deploymentWatcher.PromoteDeployment(args, reply)
}

// SetAllocHealth is used to set the health of allocations that are part of the
// deployment.
func (d *Deployment) SetAllocHealth(args *structs.DeploymentAllocHealthRequest, reply *structs.DeploymentUpdateResponse) error {
	if done, err := d.srv.forward("Deployment.SetAllocHealth", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "set_alloc_health"}, time.Now())

	// Validate the arguments
	if args.DeploymentID == "" {
		return fmt.Errorf("missing deployment ID")
	}

	if len(args.HealthyAllocationIDs)+len(args.UnhealthyAllocationIDs) == 0 {
		return fmt.Errorf("must specify at least one healthy/unhealthy allocation ID")
	}

	// Lookup the deployment
	snap, err := d.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	deploy, err := snap.DeploymentByID(ws, args.DeploymentID)
	if err != nil {
		return err
	}
	if deploy == nil {
		return fmt.Errorf("deployment not found")
	}

	if !deploy.Active() {
		return fmt.Errorf("can't set health of allocations for a terminal deployment")
	}

	// Call into the deployment watcher
	return d.srv.deploymentWatcher.SetAllocHealth(args, reply)
}

// List returns the list of deployments in the system
func (d *Deployment) List(args *structs.DeploymentListRequest, reply *structs.DeploymentListResponse) error {
	if done, err := d.srv.forward("Deployment.List", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "list"}, time.Now())

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Capture all the deployments
			var err error
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = state.DeploymentsByIDPrefix(ws, args.RequestNamespace(), prefix)
			} else {
				iter, err = state.DeploymentsByNamespace(ws, args.RequestNamespace())
			}
			if err != nil {
				return err
			}

			var deploys []*structs.Deployment
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				deploy := raw.(*structs.Deployment)
				deploys = append(deploys, deploy)
			}
			reply.Deployments = deploys

			// Use the last index that affected the deployment table
			index, err := state.Index("deployment")
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

// Reap is used to cleanup terminal deployments
func (d *Deployment) Reap(args *structs.DeploymentDeleteRequest,
	reply *structs.GenericResponse) error {
	if done, err := d.srv.forward("Deployment.Reap", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "reap"}, time.Now())

	// Update via Raft
	_, index, err := d.srv.raftApply(structs.DeploymentDeleteRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}
