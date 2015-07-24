package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// DefaultDequeueTimeout is used if no dequeue timeout is provided
	DefaultDequeueTimeout = time.Second
)

// Eval endpoint is used for eval interactions
type Eval struct {
	srv *Server
}

// GetEval is used to request information about a specific evaluation
func (e *Eval) GetEval(args *structs.EvalSpecificRequest,
	reply *structs.SingleEvalResponse) error {
	if done, err := e.srv.forward("Eval.GetEval", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "get_eval"}, time.Now())

	// Look for the job
	snap, err := e.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	out, err := snap.GetEvalByID(args.EvalID)
	if err != nil {
		return err
	}

	// Setup the output
	if out != nil {
		reply.Eval = out
		reply.Index = out.ModifyIndex
	} else {
		// Use the last index that affected the nodes table
		index, err := snap.GetIndex("evals")
		if err != nil {
			return err
		}
		reply.Index = index
	}

	// Set the query response
	e.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// Dequeue is used to dequeue a pending evaluation
func (e *Eval) Dequeue(args *structs.EvalDequeueRequest,
	reply *structs.SingleEvalResponse) error {
	if done, err := e.srv.forward("Eval.GetEval", args, args, reply); done {
		return err
	}

	// Ensure there is at least one scheduler
	if len(args.Schedulers) == 0 {
		return fmt.Errorf("dequeue requires at least one scheduler type")
	}

	// Ensure there is a default timeout
	if args.Timeout <= 0 {
		args.Timeout = DefaultDequeueTimeout
	}

	// Attempt the dequeue
	eval, err := e.srv.evalBroker.Dequeue(args.Schedulers, args.Timeout)
	if err != nil {
		return err
	}

	// Provide the output if any
	if eval != nil {
		reply.Eval = eval
	}

	// Set the query response
	e.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}
