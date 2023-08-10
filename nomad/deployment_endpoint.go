// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"net/http"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Deployment endpoint is used for manipulating deployments
type Deployment struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewDeploymentEndpoint(srv *Server, ctx *RPCContext) *Deployment {
	return &Deployment{srv: srv, ctx: ctx, logger: srv.logger.Named("deployment")}
}

// GetDeployment is used to request information about a specific deployment
func (d *Deployment) GetDeployment(args *structs.DeploymentSpecificRequest,
	reply *structs.SingleDeploymentResponse) error {

	authErr := d.srv.Authenticate(d.ctx, args)
	if done, err := d.srv.forward("Deployment.GetDeployment", args, args, reply); done {
		return err
	}
	d.srv.MeasureRPCRate("deployment", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "deployment", "get_deployment"}, time.Now())

	// Check namespace read-job permissions
	allowNsOp := acl.NamespaceValidator(acl.NamespaceCapabilityReadJob)
	aclObj, err := d.srv.ResolveACL(args)
	if err != nil {
		return err
	} else if !allowNsOp(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

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

			// Re-check namespace in case it differs from request.
			if out != nil && !allowNsOp(aclObj, out.Namespace) {
				// hide this deployment, caller is not authorized to view it
				out = nil
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

	authErr := d.srv.Authenticate(d.ctx, args)
	if done, err := d.srv.forward("Deployment.Fail", args, args, reply); done {
		return err
	}
	d.srv.MeasureRPCRate("deployment", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
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

	// Check namespace submit-job permissions
	if aclObj, err := d.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(deploy.Namespace, acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	if !deploy.Active() {
		return structs.ErrDeploymentTerminalNoFail
	}

	// Call into the deployment watcher
	return d.srv.deploymentWatcher.FailDeployment(args, reply)
}

// Pause is used to pause a deployment
func (d *Deployment) Pause(args *structs.DeploymentPauseRequest, reply *structs.DeploymentUpdateResponse) error {
	authErr := d.srv.Authenticate(d.ctx, args)
	if done, err := d.srv.forward("Deployment.Pause", args, args, reply); done {
		return err
	}
	d.srv.MeasureRPCRate("deployment", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
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

	// Check namespace submit-job permissions
	if aclObj, err := d.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(deploy.Namespace, acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	if !deploy.Active() {
		if args.Pause {
			return structs.ErrDeploymentTerminalNoPause
		}

		return structs.ErrDeploymentTerminalNoResume
	}

	// Call into the deployment watcher
	return d.srv.deploymentWatcher.PauseDeployment(args, reply)
}

// Promote is used to promote canaries in a deployment
func (d *Deployment) Promote(args *structs.DeploymentPromoteRequest, reply *structs.DeploymentUpdateResponse) error {
	authErr := d.srv.Authenticate(d.ctx, args)
	if done, err := d.srv.forward("Deployment.Promote", args, args, reply); done {
		return err
	}
	d.srv.MeasureRPCRate("deployment", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
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

	// Check namespace submit-job permissions
	if aclObj, err := d.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(deploy.Namespace, acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	if !deploy.Active() {
		return structs.ErrDeploymentTerminalNoPromote
	}

	// Call into the deployment watcher
	return d.srv.deploymentWatcher.PromoteDeployment(args, reply)
}

// Run is used to start a pending deployment
func (d *Deployment) Run(args *structs.DeploymentRunRequest, reply *structs.DeploymentUpdateResponse) error {
	authErr := d.srv.Authenticate(d.ctx, args)
	if done, err := d.srv.forward("Deployment.Run", args, args, reply); done {
		return err
	}
	d.srv.MeasureRPCRate("deployment", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "run"}, time.Now())

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

	// Check namespace submit-job permissions
	if aclObj, err := d.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(deploy.Namespace, acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	if !deploy.Active() {
		return structs.ErrDeploymentTerminalNoRun
	}

	// Call into the deployment watcher
	return d.srv.deploymentWatcher.RunDeployment(args, reply)
}

// Unblock is used to unblock a deployment
func (d *Deployment) Unblock(args *structs.DeploymentUnblockRequest, reply *structs.DeploymentUpdateResponse) error {
	authErr := d.srv.Authenticate(d.ctx, args)
	if done, err := d.srv.forward("Deployment.Unblock", args, args, reply); done {
		return err
	}
	d.srv.MeasureRPCRate("deployment", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "unblock"}, time.Now())

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

	// Check namespace submit-job permissions
	if aclObj, err := d.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(deploy.Namespace, acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	if !deploy.Active() {
		return structs.ErrDeploymentTerminalNoUnblock
	}

	// Call into the deployment watcher
	return d.srv.deploymentWatcher.UnblockDeployment(args, reply)
}

// Cancel is used to cancel a deployment
func (d *Deployment) Cancel(args *structs.DeploymentCancelRequest, reply *structs.DeploymentUpdateResponse) error {
	authErr := d.srv.Authenticate(d.ctx, args)
	if done, err := d.srv.forward("Deployment.Cancel", args, args, reply); done {
		return err
	}
	d.srv.MeasureRPCRate("deployment", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "cancel"}, time.Now())

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

	// Check namespace submit-job permissions
	if aclObj, err := d.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(deploy.Namespace, acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	if !deploy.Active() {
		return structs.ErrDeploymentTerminalNoCancel
	}

	// Call into the deployment watcher
	return d.srv.deploymentWatcher.CancelDeployment(args, reply)
}

// SetAllocHealth is used to set the health of allocations that are part of the
// deployment.
func (d *Deployment) SetAllocHealth(args *structs.DeploymentAllocHealthRequest, reply *structs.DeploymentUpdateResponse) error {
	authErr := d.srv.Authenticate(d.ctx, args)
	if done, err := d.srv.forward("Deployment.SetAllocHealth", args, args, reply); done {
		return err
	}
	d.srv.MeasureRPCRate("deployment", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
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

	// Check namespace submit-job permissions
	if aclObj, err := d.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(deploy.Namespace, acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	if !deploy.Active() {
		return structs.ErrDeploymentTerminalNoSetHealth
	}

	// Call into the deployment watcher
	return d.srv.deploymentWatcher.SetAllocHealth(args, reply)
}

// List returns the list of deployments in the system
func (d *Deployment) List(args *structs.DeploymentListRequest, reply *structs.DeploymentListResponse) error {
	authErr := d.srv.Authenticate(d.ctx, args)
	if done, err := d.srv.forward("Deployment.List", args, args, reply); done {
		return err
	}
	d.srv.MeasureRPCRate("deployment", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "list"}, time.Now())

	namespace := args.RequestNamespace()

	// Check namespace read-job permissions against request namespace since
	// results are filtered by request namespace.
	aclObj, err := d.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	if aclObj != nil && !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	allow := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityReadJob)

	// Setup the blocking query
	sort := state.SortOption(args.Reverse)
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			allowableNamespaces, err := allowedNSes(aclObj, store, allow)
			if err != nil {
				if err == structs.ErrPermissionDenied {
					reply.Deployments = make([]*structs.Deployment, 0)
					return nil
				}
				return err
			}

			// Capture all the deployments
			var iter memdb.ResultIterator
			var opts paginator.StructsTokenizerOptions

			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = store.DeploymentsByIDPrefix(ws, namespace, prefix, sort)
				opts = paginator.StructsTokenizerOptions{
					WithID: true,
				}
			} else if namespace != structs.AllNamespacesSentinel {
				iter, err = store.DeploymentsByNamespaceOrdered(ws, namespace, sort)
				opts = paginator.StructsTokenizerOptions{
					WithCreateIndex: true,
					WithID:          true,
				}
			} else {
				iter, err = store.Deployments(ws, sort)
				opts = paginator.StructsTokenizerOptions{
					WithCreateIndex: true,
					WithID:          true,
				}
			}
			if err != nil {
				return err
			}

			tokenizer := paginator.NewStructsTokenizer(iter, opts)

			filters := []paginator.Filter{
				paginator.NamespaceFilter{
					AllowableNamespaces: allowableNamespaces,
				},
			}

			var deploys []*structs.Deployment
			pnator, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
				func(raw interface{}) error {
					deploy := raw.(*structs.Deployment)
					deploys = append(deploys, deploy)
					return nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			nextToken, err := pnator.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to read result page: %v", err)
			}

			reply.QueryMeta.NextToken = nextToken
			reply.Deployments = deploys

			// Use the last index that affected the deployment table
			index, err := store.Index("deployment")
			if err != nil {
				return err
			}
			reply.Index = index

			// Set the query response
			d.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		},
	}

	return d.srv.blockingRPC(&opts)
}

// Allocations returns the list of allocations that are a part of the deployment
func (d *Deployment) Allocations(args *structs.DeploymentSpecificRequest, reply *structs.AllocListResponse) error {
	authErr := d.srv.Authenticate(d.ctx, args)
	if done, err := d.srv.forward("Deployment.Allocations", args, args, reply); done {
		return err
	}
	d.srv.MeasureRPCRate("deployment", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "deployment", "allocations"}, time.Now())

	// Check namespace read-job permissions against the request namespace.
	// Must re-check against the alloc namespace when they return to ensure
	// there's no namespace mismatch.
	allowNsOp := acl.NamespaceValidator(acl.NamespaceCapabilityReadJob)
	aclObj, err := d.srv.ResolveACL(args)
	if err != nil {
		return err
	} else if !allowNsOp(aclObj, args.RequestNamespace()) {
		return structs.ErrPermissionDenied
	}

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

			// Deployments do not span namespaces so just check the
			// first allocs namespace.
			if len(allocs) > 0 {
				ns := allocs[0].Namespace
				if ns != args.RequestNamespace() && !allowNsOp(aclObj, ns) {
					return structs.ErrPermissionDenied
				}
			}

			stubs := make([]*structs.AllocListStub, 0, len(allocs))
			for _, alloc := range allocs {
				stubs = append(stubs, alloc.Stub(nil))
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

	authErr := d.srv.Authenticate(d.ctx, args)

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(d.srv, d.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}
	if done, err := d.srv.forward("Deployment.Reap", args, args, reply); done {
		return err
	}
	d.srv.MeasureRPCRate("deployment", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
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
