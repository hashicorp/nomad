package nomad

import (
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Plan endpoint is used for plan interactions
type Plan struct {
	srv *Server
}

// Submit is used to submit a plan to the leader
func (p *Plan) Submit(args *structs.PlanRequest, reply *structs.PlanResponse) error {
	if done, err := p.srv.forward("Plan.Submit", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "plan", "submit"}, time.Now())

	// Submit the plan to the queue
	future, err := p.srv.planQueue.Enqueue(args.Plan)
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
