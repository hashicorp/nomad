// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/nomad/structs"
)

// Plan endpoint is used for plan interactions
type Plan struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewPlanEndpoint(srv *Server, ctx *RPCContext) *Plan {
	return &Plan{srv: srv, ctx: ctx, logger: srv.logger.Named("plan")}
}

// Submit is used to submit a plan to the leader
func (p *Plan) Submit(args *structs.PlanRequest, reply *structs.PlanResponse) error {

	authErr := p.srv.Authenticate(p.ctx, args)

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(p.srv, p.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}
	if done, err := p.srv.forward("Plan.Submit", args, args, reply); done {
		return err
	}
	p.srv.MeasureRPCRate("plan", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "plan", "submit"}, time.Now())

	if args.Plan == nil {
		return fmt.Errorf("cannot submit nil plan")
	}

	// Pause the Nack timer for the eval as it is making progress as long as it
	// is in the plan queue. We resume immediately after we get a result to
	// handle the case that the receiving worker dies.
	plan := args.Plan
	id := plan.EvalID
	token := plan.EvalToken
	if err := p.srv.evalBroker.PauseNackTimeout(id, token); err != nil {
		return err
	}
	defer p.srv.evalBroker.ResumeNackTimeout(id, token)

	// Submit the plan to the queue
	future, err := p.srv.planQueue.Enqueue(plan)
	if err != nil {
		return err
	}

	// Wait for the results
	result, err := future.Wait()
	if err != nil {
		return err
	}

	// Package the result
	reply.Result = result
	reply.Index = result.AllocIndex
	return nil
}
