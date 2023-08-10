// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-version"

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

var minVersionEvalDeleteByFilter = version.Must(version.NewVersion("1.4.3"))

// Eval endpoint is used for eval interactions
type Eval struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
}

func NewEvalEndpoint(srv *Server, ctx *RPCContext) *Eval {
	return &Eval{srv: srv, ctx: ctx, logger: srv.logger.Named("eval")}
}

// GetEval is used to request information about a specific evaluation
func (e *Eval) GetEval(args *structs.EvalSpecificRequest,
	reply *structs.SingleEvalResponse) error {

	authErr := e.srv.Authenticate(e.ctx, args)
	if done, err := e.srv.forward("Eval.GetEval", args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "get_eval"}, time.Now())

	// Check for read-job permissions before performing blocking query.
	allowNsOp := acl.NamespaceValidator(acl.NamespaceCapabilityReadJob)
	aclObj, err := e.srv.ResolveACL(args)
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
			var related []*structs.EvaluationStub

			// Look for the eval
			eval, err := state.EvalByID(ws, args.EvalID)
			if err != nil {
				return fmt.Errorf("failed to lookup eval: %v", err)
			}

			if eval != nil {
				// Re-check namespace in case it differs from request.
				if !allowNsOp(aclObj, eval.Namespace) {
					return structs.ErrPermissionDenied
				}

				// Lookup related evals if requested.
				if args.IncludeRelated {
					related, err = state.EvalsRelatedToID(ws, eval.ID)
					if err != nil {
						return fmt.Errorf("failed to lookup related evals: %v", err)
					}

					// Use a copy to avoid modifying the original eval.
					eval = eval.Copy()
					eval.RelatedEvals = related
				}
			}

			// Setup the output.
			reply.Eval = eval
			if eval != nil {
				reply.Index = eval.ModifyIndex
			} else {
				// Use the last index that affected the evals table
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

	authErr := e.srv.Authenticate(e.ctx, args)

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}
	if done, err := e.srv.forward("Eval.Dequeue", args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
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

	// If the eval broker is paused, attempt to block and wait for a state
	// change before returning. This avoids a tight loop and mimics the
	// behaviour where there are no evals to process.
	//
	// The call can return because either the timeout is reached or the broker
	// SetEnabled function was called to modify its state. It is possible this
	// is because of leadership transition, therefore the RPC should exit to
	// allow all safety checks and RPC forwarding to occur again.
	//
	// The log line is trace, because the default worker timeout is 500ms which
	// produces a large amount of logging.
	if !e.srv.evalBroker.Enabled() {
		message := e.srv.evalBroker.enabledNotifier.WaitForChange(args.Timeout)
		e.logger.Trace("eval broker wait for un-pause", "message", message)
		return nil
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

	authErr := e.srv.Authenticate(e.ctx, args)

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}
	if done, err := e.srv.forward("Eval.Ack", args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "ack"}, time.Now())

	// Ack the EvalID
	if err := e.srv.evalBroker.Ack(args.EvalID, args.Token); err != nil {
		return err
	}

	// Wake up the eval cancelation reaper. This never blocks; if the buffer is
	// full we know it's going to get picked up by the reaper so we don't need
	// another send on that channel.
	select {
	case e.srv.reapCancelableEvalsCh <- struct{}{}:
	default:
	}
	return nil
}

// Nack is used to negative acknowledge completion of a dequeued evaluation.
func (e *Eval) Nack(args *structs.EvalAckRequest,
	reply *structs.GenericResponse) error {

	authErr := e.srv.Authenticate(e.ctx, args)

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}
	if done, err := e.srv.forward("Eval.Nack", args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
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

	authErr := e.srv.Authenticate(e.ctx, args)

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}
	if done, err := e.srv.forward("Eval.Update", args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
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

	authErr := e.srv.Authenticate(e.ctx, args)

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}
	if done, err := e.srv.forward("Eval.Create", args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
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

	authErr := e.srv.Authenticate(e.ctx, args)

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}
	if done, err := e.srv.forward("Eval.Reblock", args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
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
func (e *Eval) Reap(args *structs.EvalReapRequest,
	reply *structs.GenericResponse) error {

	authErr := e.srv.Authenticate(e.ctx, args)

	// Ensure the connection was initiated by another server if TLS is used.
	err := validateTLSCertificateLevel(e.srv, e.ctx, tlsCertificateLevelServer)
	if err != nil {
		return err
	}
	if done, err := e.srv.forward("Eval.Reap", args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
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

// Delete is used by operators to delete evaluations during severe outages. It
// differs from Reap while duplicating some behavior to ensure we have the
// correct controls for user initiated deletions.
func (e *Eval) Delete(
	args *structs.EvalDeleteRequest,
	reply *structs.EvalDeleteResponse) error {

	authErr := e.srv.Authenticate(e.ctx, args)
	if done, err := e.srv.forward(structs.EvalDeleteRPCMethod, args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "delete"}, time.Now())

	// This RPC endpoint is very destructive and alters Nomad's core state,
	// meaning only those with management tokens can call it.
	if aclObj, err := e.srv.ResolveACL(args); err != nil {
		return err
	} else if aclObj != nil && !aclObj.IsManagement() {
		return structs.ErrPermissionDenied
	}

	if args.Filter != "" && !ServersMeetMinimumVersion(
		e.srv.Members(), e.srv.Region(), minVersionEvalDeleteByFilter, true) {
		return fmt.Errorf(
			"all servers must be running version %v or later to delete evals by filter",
			minVersionEvalDeleteByFilter)
	}
	if args.Filter != "" && len(args.EvalIDs) > 0 {
		return fmt.Errorf("evals cannot be deleted by both ID and filter")
	}
	if args.Filter == "" && len(args.EvalIDs) == 0 {
		return fmt.Errorf("evals must be deleted by either ID or filter")
	}

	// The eval broker must be disabled otherwise Nomad's state will likely get
	// wild in a very un-fun way.
	if e.srv.evalBroker.Enabled() {
		return errors.New("eval broker is enabled; eval broker must be paused to delete evals")
	}

	if args.Filter != "" {
		count, index, err := e.deleteEvalsByFilter(args)
		if err != nil {
			return err
		}

		// Update the index and return.
		reply.Index = index
		reply.Count = count
		return nil
	}

	// Grab the state snapshot, so we can look up relevant eval information.
	serverStateSnapshot, err := e.srv.State().Snapshot()
	if err != nil {
		return fmt.Errorf("failed to lookup state snapshot: %v", err)
	}
	ws := memdb.NewWatchSet()

	count := 0

	// Iterate the evaluations and ensure they are safe to delete. It is
	// possible passed evals are not safe to delete and would make Nomads state
	// a little wonky. The nature of the RPC return error, means a single
	// unsafe eval ID fails the whole call.
	for _, evalID := range args.EvalIDs {

		evalInfo, err := serverStateSnapshot.EvalByID(ws, evalID)
		if err != nil {
			return fmt.Errorf("failed to lookup eval: %v", err)
		}
		if evalInfo == nil {
			return errors.New("eval not found")
		}
		ok, err := serverStateSnapshot.EvalIsUserDeleteSafe(ws, evalInfo)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("eval %s is not safe to delete", evalInfo.ID)
		}
		count++
	}

	// Generate the Raft request object using the reap request object. This
	// avoids adding new Raft messages types and follows the existing reap
	// flow.
	raftReq := structs.EvalReapRequest{
		Evals:         args.EvalIDs,
		UserInitiated: true,
		WriteRequest:  args.WriteRequest,
	}

	// Update via Raft.
	_, index, err := e.srv.raftApply(structs.EvalDeleteRequestType, &raftReq)
	if err != nil {
		return err
	}

	// Update the index and return.
	reply.Index = index
	reply.Count = count
	return nil
}

// deleteEvalsByFilter deletes evaluations in batches based on the filter. It
// returns a count, the index, and any error
func (e *Eval) deleteEvalsByFilter(args *structs.EvalDeleteRequest) (int, uint64, error) {
	count := 0
	index := uint64(0)

	filter, err := bexpr.CreateEvaluator(args.Filter)
	if err != nil {
		return count, index, err
	}

	// Note that deleting evals by filter is imprecise: For sets of evals larger
	// than a single batch eval inserts may occur behind the cursor and therefore
	// be missed. This imprecision is not considered to hurt this endpoint's
	// purpose of reducing pressure on servers during periods of heavy scheduling
	// activity.
	snap, err := e.srv.State().Snapshot()
	if err != nil {
		return count, index, fmt.Errorf("failed to lookup state snapshot: %v", err)
	}

	iter, err := snap.Evals(nil, state.SortDefault)
	if err != nil {
		return count, index, err
	}

	// We *can* send larger raft logs but rough benchmarks for deleting 1M evals
	// show that a smaller page size strikes a balance between throughput and
	// time we block the FSM apply for other operations
	perPage := structs.MaxUUIDsPerWriteRequest / 10

	raftReq := structs.EvalReapRequest{
		Filter:        args.Filter,
		PerPage:       int32(perPage),
		UserInitiated: true,
		WriteRequest:  args.WriteRequest,
	}

	// Note: Paginator is designed around fetching a single page for a single
	// RPC call and finalizes its state after that page. So we're doing our own
	// pagination here.
	pageCount := 0
	lastToken := ""

	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		eval := raw.(*structs.Evaluation)
		deleteOk, err := snap.EvalIsUserDeleteSafe(nil, eval)
		if !deleteOk || err != nil {
			continue
		}
		match, err := filter.Evaluate(eval)
		if !match || err != nil {
			continue
		}
		pageCount++
		lastToken = eval.ID

		if pageCount >= perPage {
			raftReq.PerPage = int32(pageCount)
			_, index, err = e.srv.raftApply(structs.EvalDeleteRequestType, &raftReq)
			if err != nil {
				return count, index, err
			}
			count += pageCount

			pageCount = 0
			raftReq.NextToken = lastToken
		}
	}

	// send last batch if it's partial
	if pageCount > 0 {
		raftReq.PerPage = int32(pageCount)
		_, index, err = e.srv.raftApply(structs.EvalDeleteRequestType, &raftReq)
		if err != nil {
			return count, index, err
		}
		count += pageCount
	}

	return count, index, nil
}

// List is used to get a list of the evaluations in the system
func (e *Eval) List(args *structs.EvalListRequest, reply *structs.EvalListResponse) error {

	authErr := e.srv.Authenticate(e.ctx, args)
	if done, err := e.srv.forward("Eval.List", args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "list"}, time.Now())

	namespace := args.RequestNamespace()

	// Check for read-job permissions
	aclObj, err := e.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}
	allow := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityReadJob)

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

			// Get the namespaces the user is allowed to access.
			allowableNamespaces, err := allowedNSes(aclObj, store, allow)
			if err == structs.ErrPermissionDenied {
				// return empty evals if token isn't authorized for any
				// namespace, matching other endpoints
				reply.Evaluations = make([]*structs.Evaluation, 0)
			} else if err != nil {
				return err
			} else {
				if prefix := args.QueryOptions.Prefix; prefix != "" {
					iter, err = store.EvalsByIDPrefix(ws, namespace, prefix, sort)
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
				filters := []paginator.Filter{
					paginator.NamespaceFilter{
						AllowableNamespaces: allowableNamespaces,
					},
				}

				var evals []*structs.Evaluation
				paginator, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
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
			}

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

// Count is used to get a list of the evaluations in the system
func (e *Eval) Count(args *structs.EvalCountRequest, reply *structs.EvalCountResponse) error {

	authErr := e.srv.Authenticate(e.ctx, args)
	if done, err := e.srv.forward("Eval.Count", args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "count"}, time.Now())
	namespace := args.RequestNamespace()

	// Check for read-job permissions
	aclObj, err := e.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	if !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadJob) {
		return structs.ErrPermissionDenied
	}
	allow := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityReadJob)

	var filter *bexpr.Evaluator
	if args.Filter != "" {
		filter, err = bexpr.CreateEvaluator(args.Filter)
		if err != nil {
			return err
		}
	}

	// Setup the blocking query. This is only superficially like Eval.List,
	// because we don't any concerns about pagination, sorting, and legacy
	// filter fields.
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			// Scan all the evaluations
			var err error
			var iter memdb.ResultIterator

			// Get the namespaces the user is allowed to access.
			allowableNamespaces, err := allowedNSes(aclObj, store, allow)
			if err != nil {
				return err
			}

			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = store.EvalsByIDPrefix(ws, namespace, prefix, state.SortDefault)
			} else if namespace != structs.AllNamespacesSentinel {
				iter, err = store.EvalsByNamespace(ws, namespace)
			} else {
				iter, err = store.Evals(ws, state.SortDefault)
			}
			if err != nil {
				return err
			}

			count := 0

			iter = memdb.NewFilterIterator(iter, func(raw interface{}) bool {
				if raw == nil {
					return true
				}
				eval := raw.(*structs.Evaluation)
				if allowableNamespaces != nil && !allowableNamespaces[eval.Namespace] {
					return true
				}
				if filter != nil {
					ok, err := filter.Evaluate(eval)
					if err != nil {
						return true
					}
					return !ok
				}
				return false
			})

			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				count++
			}

			// Use the last index that affected the jobs table
			index, err := store.Index("evals")
			if err != nil {
				return err
			}
			reply.Index = index
			reply.Count = count

			// Set the query response
			e.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}

	return e.srv.blockingRPC(&opts)
}

// Allocations is used to list the allocations for an evaluation
func (e *Eval) Allocations(args *structs.EvalSpecificRequest,
	reply *structs.EvalAllocationsResponse) error {

	authErr := e.srv.Authenticate(e.ctx, args)
	if done, err := e.srv.forward("Eval.Allocations", args, args, reply); done {
		return err
	}
	e.srv.MeasureRPCRate("eval", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}
	defer metrics.MeasureSince([]string{"nomad", "eval", "allocations"}, time.Now())

	// Check for read-job permissions
	allowNsOp := acl.NamespaceValidator(acl.NamespaceCapabilityReadJob)
	aclObj, err := e.srv.ResolveACL(args)
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
