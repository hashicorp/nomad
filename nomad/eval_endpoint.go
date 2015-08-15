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
	reply *structs.EvalDequeueResponse) error {
	if done, err := e.srv.forward("Eval.Dequeue", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "dequeue"}, time.Now())

	// Ensure there is at least one scheduler
	if len(args.Schedulers) == 0 {
		return fmt.Errorf("dequeue requires at least one scheduler type")
	}

	// Ensure there is a default timeout
	if args.Timeout <= 0 {
		args.Timeout = DefaultDequeueTimeout
	}

	// Attempt the dequeue
	eval, token, err := e.srv.evalBroker.Dequeue(args.Schedulers, args.Timeout)
	if err != nil {
		return err
	}

	// Provide the output if any
	if eval != nil {
		reply.Eval = eval
		reply.Token = token
	}

	// Set the query response
	e.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// Ack is used to acknowledge completion of a dequeued evaluation
func (e *Eval) Ack(args *structs.EvalAckRequest,
	reply *structs.GenericResponse) error {
	if done, err := e.srv.forward("Eval.Ack", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "ack"}, time.Now())

	// Ack the EvalID
	if err := e.srv.evalBroker.Ack(args.EvalID, args.Token); err != nil {
		return err
	}
	return nil
}

// NAck is used to negative acknowledge completion of a dequeued evaluation
func (e *Eval) Nack(args *structs.EvalAckRequest,
	reply *structs.GenericResponse) error {
	if done, err := e.srv.forward("Eval.Nack", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "nack"}, time.Now())

	// Nack the EvalID
	if err := e.srv.evalBroker.Nack(args.EvalID, args.Token); err != nil {
		return err
	}
	return nil
}

// Update is used to perform an update of an Eval if it is outstanding.
func (e *Eval) Update(args *structs.EvalUpdateRequest,
	reply *structs.GenericResponse) error {
	if done, err := e.srv.forward("Eval.Update", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "update"}, time.Now())

	// Ensure there is only a single update with token
	if len(args.Evals) != 1 {
		return fmt.Errorf("only a single eval can be updated")
	}
	eval := args.Evals[0]

	// Verify the evaluation is outstanding, and that the tokens match.
	token, ok := e.srv.evalBroker.Outstanding(eval.ID)
	if !ok {
		return fmt.Errorf("evaluation is not outstanding")
	}
	if args.Token != token {
		return fmt.Errorf("evaluation token does not match")
	}

	// Update via Raft
	_, index, err := e.srv.raftApply(structs.EvalUpdateRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}
