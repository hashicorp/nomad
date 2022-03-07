package nomad

import (
	"fmt"
	"net/http"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
)

const (
	// DefaultDequeueTimeout is used if no dequeue timeout is provided
	DefaultDequeueTimeout = time.Second
)

// Eval endpoint is used for eval interactions
type Eval struct {
	srv    *Server
	logger log.Logger

	// ctx provides context regarding the underlying connection
	ctx *RPCContext
}

// GetEval is used to request information about a specific evaluation
func (e *Eval) GetEval(args *structs.EvalSpecificRequest,
	reply *structs.SingleEvalResponse) error {
	if done, err := e.srv.forward("Eval.GetEval", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "get_eval"}, time.Now())

	// Check for read-job permissions before performing blocking query.
	allowNsOp := acl.NamespaceValidator(acl.NamespaceCapabilityReadJob)
	aclObj, err := e.srv.ResolveToken(args.AuthToken)
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
			// Look for the job
			out, err := state.EvalByID(ws, args.EvalID)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Eval = out
			if out != nil {
				// Re-check namespace in case it differs from request.
				if !allowNsOp(aclObj, out.Namespace) {
					return structs.ErrPermissionDenied
				}

				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the nodes table
				index, err := state.Index("evals")
				if err != nil {
					return err
				}
				reply.Index = index
			}

			// Set the query response
			e.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return e.srv.blockingRPC(&opts)
}

// Dequeue is used to dequeue a pending evaluation
func (e *Eval) Dequeue(args *structs.EvalDequeueRequest,
	reply *structs.EvalDequeueResponse) error {

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}

	if done, err := e.srv.forward("Eval.Dequeue", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "dequeue"}, time.Now())

	// Ensure there is at least one scheduler
	if len(args.Schedulers) == 0 {
		return fmt.Errorf("dequeue requires at least one scheduler type")
	}

	// Check that there isn't a scheduler version mismatch
	if args.SchedulerVersion != scheduler.SchedulerVersion {
		return fmt.Errorf("dequeue disallowed: calling scheduler version is %d; leader version is %d",
			args.SchedulerVersion, scheduler.SchedulerVersion)
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
		// Get the index that the worker should wait until before scheduling.
		waitIndex, err := e.getWaitIndex(eval.Namespace, eval.JobID, eval.ModifyIndex)
		if err != nil {
			var mErr multierror.Error
			_ = multierror.Append(&mErr, err)

			// We have dequeued the evaluation but won't be returning it to the
			// worker so Nack the eval.
			if err := e.srv.evalBroker.Nack(eval.ID, token); err != nil {
				_ = multierror.Append(&mErr, err)
			}

			return &mErr
		}

		reply.Eval = eval
		reply.Token = token
		reply.WaitIndex = waitIndex
	}

	// Set the query response
	e.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}

// getWaitIndex returns the wait index that should be used by the worker before
// invoking the scheduler. The index should be the highest modify index of any
// evaluation for the job. This prevents scheduling races for the same job when
// there are blocked evaluations.
func (e *Eval) getWaitIndex(namespace, job string, evalModifyIndex uint64) (uint64, error) {
	snap, err := e.srv.State().Snapshot()
	if err != nil {
		return 0, err
	}

	evals, err := snap.EvalsByJob(nil, namespace, job)
	if err != nil {
		return 0, err
	}

	// Since dequeueing evals is concurrent with applying Raft messages to
	// the state store, initialize to the currently dequeued eval's index
	// in case it isn't in the snapshot used by EvalsByJob yet.
	max := evalModifyIndex
	for _, eval := range evals {
		if max < eval.ModifyIndex {
			max = eval.ModifyIndex
		}
	}

	return max, nil
}

// Ack is used to acknowledge completion of a dequeued evaluation
func (e *Eval) Ack(args *structs.EvalAckRequest,
	reply *structs.GenericResponse) error {

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}

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

// Nack is used to negative acknowledge completion of a dequeued evaluation.
func (e *Eval) Nack(args *structs.EvalAckRequest,
	reply *structs.GenericResponse) error {

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}

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

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}

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
	if err := e.srv.evalBroker.OutstandingReset(eval.ID, args.EvalToken); err != nil {
		return err
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

// Create is used to make a new evaluation
func (e *Eval) Create(args *structs.EvalUpdateRequest,
	reply *structs.GenericResponse) error {

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}

	if done, err := e.srv.forward("Eval.Create", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "create"}, time.Now())

	// Ensure there is only a single update with token
	if len(args.Evals) != 1 {
		return fmt.Errorf("only a single eval can be created")
	}
	eval := args.Evals[0]

	// Verify the parent evaluation is outstanding, and that the tokens match.
	if err := e.srv.evalBroker.OutstandingReset(eval.PreviousEval, args.EvalToken); err != nil {
		return err
	}

	// Look for the eval
	snap, err := e.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	out, err := snap.EvalByID(ws, eval.ID)
	if err != nil {
		return err
	}
	if out != nil {
		return fmt.Errorf("evaluation already exists")
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

// Reblock is used to reinsert an existing blocked evaluation into the blocked
// evaluation tracker.
func (e *Eval) Reblock(args *structs.EvalUpdateRequest, reply *structs.GenericResponse) error {
	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}

	if done, err := e.srv.forward("Eval.Reblock", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "reblock"}, time.Now())

	// Ensure there is only a single update with token
	if len(args.Evals) != 1 {
		return fmt.Errorf("only a single eval can be reblocked")
	}
	eval := args.Evals[0]

	// Verify the evaluation is outstanding, and that the tokens match.
	if err := e.srv.evalBroker.OutstandingReset(eval.ID, args.EvalToken); err != nil {
		return err
	}

	// Look for the eval
	snap, err := e.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}

	ws := memdb.NewWatchSet()
	out, err := snap.EvalByID(ws, eval.ID)
	if err != nil {
		return err
	}
	if out == nil {
		return fmt.Errorf("evaluation does not exist")
	}
	if out.Status != structs.EvalStatusBlocked {
		return fmt.Errorf("evaluation not blocked")
	}

	// Reblock the eval
	e.srv.blockedEvals.Reblock(eval, args.EvalToken)
	return nil
}

// Reap is used to cleanup dead evaluations and allocations
func (e *Eval) Reap(args *structs.EvalDeleteRequest,
	reply *structs.GenericResponse) error {

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}

	if done, err := e.srv.forward("Eval.Reap", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "reap"}, time.Now())

	// Update via Raft
	_, index, err := e.srv.raftApply(structs.EvalDeleteRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// List is used to get a list of the evaluations in the system
func (e *Eval) List(args *structs.EvalListRequest, reply *structs.EvalListResponse) error {
	if done, err := e.srv.forward("Eval.List", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "list"}, time.Now())

	namespace := args.RequestNamespace()

	// Check for read-job permissions
	if aclObj, err := e.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}

	if args.Filter != "" {
		// Check for incompatible filtering.
		hasLegacyFilter := args.FilterJobID != "" || args.FilterEvalStatus != ""
		if hasLegacyFilter {
			return structs.ErrIncompatibleFiltering
		}
	}

	// Setup the blocking query
	sort := state.SortOption(args.Reverse)
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			// Scan all the evaluations
			var err error
			var iter memdb.ResultIterator
			var opts paginator.StructsTokenizerOptions

			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = store.EvalsByIDPrefix(ws, namespace, prefix)
				opts = paginator.StructsTokenizerOptions{
					WithID: true,
				}
			} else if namespace != structs.AllNamespacesSentinel {
				iter, err = store.EvalsByNamespaceOrdered(ws, namespace, sort)
				opts = paginator.StructsTokenizerOptions{
					WithCreateIndex: true,
					WithID:          true,
				}
			} else {
				iter, err = store.Evals(ws, sort)
				opts = paginator.StructsTokenizerOptions{
					WithCreateIndex: true,
					WithID:          true,
				}
			}
			if err != nil {
				return err
			}

			iter = memdb.NewFilterIterator(iter, func(raw interface{}) bool {
				if eval := raw.(*structs.Evaluation); eval != nil {
					return args.ShouldBeFiltered(eval)
				}
				return false
			})

			tokenizer := paginator.NewStructsTokenizer(iter, opts)

			var evals []*structs.Evaluation
			paginator, err := paginator.NewPaginator(iter, tokenizer, nil, args.QueryOptions,
				func(raw interface{}) error {
					eval := raw.(*structs.Evaluation)
					evals = append(evals, eval)
					return nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			nextToken, err := paginator.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to read result page: %v", err)
			}

			reply.QueryMeta.NextToken = nextToken
			reply.Evaluations = evals

			// Use the last index that affected the jobs table
			index, err := store.Index("evals")
			if err != nil {
				return err
			}
			reply.Index = index

			// Set the query response
			e.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return e.srv.blockingRPC(&opts)
}

// Allocations is used to list the allocations for an evaluation
func (e *Eval) Allocations(args *structs.EvalSpecificRequest,
	reply *structs.EvalAllocationsResponse) error {
	if done, err := e.srv.forward("Eval.Allocations", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "allocations"}, time.Now())

	// Check for read-job permissions
	allowNsOp := acl.NamespaceValidator(acl.NamespaceCapabilityReadJob)
	aclObj, err := e.srv.ResolveToken(args.AuthToken)
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
			// Capture the allocations
			allocs, err := state.AllocsByEval(ws, args.EvalID)
			if err != nil {
				return err
			}

			// Convert to a stub
			if len(allocs) > 0 {
				// Evaluations do not span namespaces so just check the
				// first allocs namespace.
				ns := allocs[0].Namespace
				if ns != args.RequestNamespace() && !allowNsOp(aclObj, ns) {
					return structs.ErrPermissionDenied
				}

				reply.Allocations = make([]*structs.AllocListStub, 0, len(allocs))
				for _, alloc := range allocs {
					reply.Allocations = append(reply.Allocations, alloc.Stub(nil))
				}
			}

			// Use the last index that affected the allocs table
			index, err := state.Index("allocs")
			if err != nil {
				return err
			}
			reply.Index = index

			// Set the query response
			e.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return e.srv.blockingRPC(&opts)
}
