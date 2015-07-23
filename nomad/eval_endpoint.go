package nomad

import (
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
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
