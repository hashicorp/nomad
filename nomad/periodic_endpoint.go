// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Periodic endpoint is used for periodic job interactions
type Periodic struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewPeriodicEndpoint(srv *Server, ctx *RPCContext) *Periodic {
	return &Periodic{srv: srv, ctx: ctx, logger: srv.logger.Named("periodic")}
}

// Force is used to force a new instance of a periodic job
func (p *Periodic) Force(args *structs.PeriodicForceRequest, reply *structs.PeriodicForceResponse) error {

	authErr := p.srv.Authenticate(p.ctx, args)
	if done, err := p.srv.forward("Periodic.Force", args, args, reply); done {
		return err
	}
	p.srv.MeasureRPCRate("periodic", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "periodic", "force"}, time.Now())

	// Check for write-job permissions
	if aclObj, err := p.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilityDispatchJob) && !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
		return structs.ErrPermissionDenied
	}

	// Validate the arguments
	if args.JobID == "" {
		return fmt.Errorf("missing job ID for evaluation")
	}

	// Lookup the job
	snap, err := p.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	job, err := snap.JobByID(ws, args.RequestNamespace(), args.JobID)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	if !job.IsPeriodic() {
		return fmt.Errorf("can't force launch non-periodic job")
	}

	// Force run the job.
	eval, err := p.srv.periodicDispatcher.ForceEval(args.RequestNamespace(), job.ID)
	if err != nil {
		return fmt.Errorf("force launch for job %q failed: %v", job.ID, err)
	}

	reply.EvalID = eval.ID
	reply.EvalCreateIndex = eval.CreateIndex
	reply.Index = eval.CreateIndex
	return nil
}
