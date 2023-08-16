package allocrunner

/* TODO HEY
 * the problem with this is that lock-maintenance goes away with the client...
 */

import (
	"context"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// errNilLock should only happen when restoring terminal allocations,
// so really it's not much of an error...
var errNilLock = errors.New("lock is nil")

func newLockHook(
	logger hclog.Logger,
	alloc *structs.Allocation,
	rpc client.RPCer,
	nodeSecret string,
	shutdownCtx context.Context,
) *lockHook {
	return &lockHook{
		logger: logger.Named("lock_hook"),
		alloc:  alloc,
		rpc:    rpc,
		token:  nodeSecret,

		shutdownCtx: shutdownCtx,
	}
}

// lockHook manages the lifecycle of one or more variable locks
type lockHook struct {
	logger hclog.Logger
	alloc  *structs.Allocation
	rpc    client.RPCer
	token  string

	shutdownCtx context.Context

	locks []*rpcLocker // TODO: protect this?
	stop  context.CancelFunc
}

var _ interfaces.RunnerPrerunHook = &lockHook{}
var _ interfaces.RunnerPostrunHook = &lockHook{}
var _ interfaces.RunnerDestroyHook = &lockHook{}

//var _ interfaces.RunnerPreKillHook = &lockHook{}
//var _ interfaces.ShutdownHook = &lockHook{}

func (h *lockHook) Name() string {
	return "lock_hook"
}

func (h *lockHook) Prerun() error {
	if len(h.alloc.Locks) == 0 {
		//h.logger.Debug("no locks found on alloc")
		return nil
	}

	ctx, cancel := context.WithCancel(h.shutdownCtx)
	h.stop = cancel

	for _, l := range h.alloc.Locks {
		lock := newRPCLocker(h.logger, h.alloc, h.rpc, h.token, l.Path)
		h.locks = append(h.locks, lock)
		go lock.hold(ctx)
	}

	return nil
}

func (h *lockHook) Postrun() error {
	if h.stop != nil {
		h.stop()
	}

	// this also runs during restore of terminal allocs
	if h.alloc.TerminalStatus() {
		return nil
	}
	if len(h.locks) == 0 {
		return nil
	}

	// release all the locks
	var errs multierror.Error
	for _, l := range h.locks {
		if err := l.release(); err != nil {
			multierror.Append(&errs, err)
		}
	}
	return errs.ErrorOrNil()
}

func (h *lockHook) Destroy() error {
	if len(h.locks) == 0 {
		return nil
	}

	// we'll sleep the max TTL * 2 to avoid conflicting with ourself. TODO: ???
	//var maxTTL time.Duration
	//for _, l := range h.locks {
	//	if l.ttl > maxTTL {
	//		maxTTL = l.ttl
	//	}
	//}
	//time.Sleep(maxTTL * 2) // TODO: only if shutdownCtx is not Done()?

	// delete all the locks TODO: do we want to??
	var errs multierror.Error
	for _, l := range h.locks {
		if err := l.delete(); err != nil {
			multierror.Append(&errs, err)
		}
	}
	return errs.ErrorOrNil()
}

func newRPCLocker(l hclog.Logger, a *structs.Allocation, rpc client.RPCer, token string, path string) *rpcLocker {
	locker := &rpcLocker{
		alloc: a,
		rpc:   rpc,
		token: token,
		path:  path,
		ttl:   time.Second * 10, // min for demo purposes
		//ttl:   api.DefaultLockTTL, // TODO: not api pkg?
	}
	locker.logger = l.Named("rpc_locker").
		With("path", locker.varPath())
	return locker
}

// rpcLocker manages the lifecycle of a single variable lock
type rpcLocker struct {
	logger hclog.Logger
	alloc  *structs.Allocation
	rpc    client.RPCer
	token  string

	path string
	ttl  time.Duration

	//lock   *atomic.Value
	lock    *structs.VariableLock // TODO: protect this?
	meta    *structs.VariableMetadata
	lastIdx uint64

	stop context.CancelFunc
}

func (l *rpcLocker) withTTL(d time.Duration) *rpcLocker { // TODO: unused
	l.ttl = d
	return l
}

func (l *rpcLocker) hold(ctx context.Context) {
	l.logger.Debug("hold")

	ctx, cancel := context.WithCancel(ctx)
	l.stop = cancel

	// we reset this to the lock's actual TTL after we acquire it
	refreshRate := time.Duration(time.Second * 15) // default

	timer, stop := helper.NewSafeTimer(0) // run ~immediately at first
	defer stop()

	for {
		select {
		case <-ctx.Done():
			l.logger.Info("hold context done")
			return
		case <-timer.C:
			timer.Reset(refreshRate)
		}

		if l.lock == nil {
			if err := l.acquire(); err != nil {
				// Debug because this is totally expected.
				l.logger.Debug("unable to acquire lock", "error", err)
			} else {
				refreshRate = l.lock.TTL
			}
			continue
		}

		if err := l.renew(); err != nil {
			// we do expect to be able to renew a lock we've acquired, so Warn.
			l.logger.Warn("unable to renew lock", "error", err)
		}
	}
}

func (l *rpcLocker) acquire() error {
	l.logger.Debug("acquire") // TODO: trace these debugs...?

	req := &structs.VariablesApplyRequest{
		Op:  structs.VarOpLockAcquire,
		Var: l.variable(""),

		WriteRequest: l.writeRequest(),
	}
	var resp structs.VariablesApplyResponse
	err := l.rpc.RPC(structs.VariablesApplyRPCMethod, req, &resp)
	if err != nil {
		return err
	}
	if err = l.handleApplyResponse("acquire", &resp); err != nil {
		return err
	}

	if resp.Output.Lock == nil {
		return errors.New("got nil Output")
	}

	l.lock = resp.Output.Lock
	l.lastIdx = resp.WriteMeta.Index
	l.meta = &resp.Output.VariableMetadata
	return nil
}

func (l *rpcLocker) renew() error {
	l.logger.Debug("renew")

	if l.lock == nil {
		return errNilLock
	}

	req := &structs.VariablesRenewLockRequest{
		Path:   l.varPath(),
		LockID: l.lock.ID,

		WriteRequest: l.writeRequest(),
	}
	var resp structs.VariablesRenewLockResponse
	err := l.rpc.RPC(structs.VariablesRenewLockRPCMethod, req, &resp)
	if err != nil {
		return err
	}

	l.lastIdx = resp.WriteMeta.Index
	l.meta = resp.VarMeta
	return nil
}

func (l *rpcLocker) release() error {
	l.logger.Debug("release")

	if l.stop != nil {
		l.stop()
	}
	if l.lock == nil {
		return errNilLock
	}

	v := l.variable(l.lock.ID)
	//v.Items = nil
	req := &structs.VariablesApplyRequest{
		Op:  structs.VarOpLockRelease,
		Var: v,

		WriteRequest: l.writeRequest(),
	}

	var resp structs.VariablesApplyResponse
	err := l.rpc.RPC(structs.VariablesApplyRPCMethod, req, &resp)
	if err != nil {
		return err
	}
	return l.handleApplyResponse("release", &resp)
}

func (l *rpcLocker) delete() error {
	l.logger.Debug("delete")

	if l.stop != nil {
		l.stop()
	}
	if l.lock == nil {
		return errNilLock
	}

	v := l.variable(l.lock.ID)

	v.ModifyIndex = l.lastIdx
	req := &structs.VariablesApplyRequest{
		Op:  structs.VarOpDeleteCAS,
		Var: v,

		WriteRequest: l.writeRequest(),
	}

	var resp structs.VariablesApplyResponse
	err := l.rpc.RPC(structs.VariablesApplyRPCMethod, req, &resp)
	if err != nil {
		return err
	}
	return l.handleApplyResponse("delete", &resp)
}

func (l *rpcLocker) handleApplyResponse(op string, r *structs.VariablesApplyResponse) error {
	if r.IsError() {
		return fmt.Errorf("%s apply error: %s", op, r.Error)
	}
	if r.IsConflict() {
		return fmt.Errorf("%s lock error: %s", op, r.Result)
	}
	return nil
}

func (l *rpcLocker) varPath() string {
	if l.path == "" {
		return path.Join("nomad", "jobs", l.alloc.JobID, l.alloc.TaskGroup, "fancy-lock") // TODO: blarg...
	}
	return l.path
}

func (l *rpcLocker) variable(lockID string) *structs.VariableDecrypted {
	meta := l.meta
	if meta == nil {
		meta = &structs.VariableMetadata{
			Namespace: l.alloc.Namespace,
			Path:      l.varPath(),
			Lock: &structs.VariableLock{
				ID:        lockID,
				TTL:       l.ttl,
				LockDelay: l.ttl, // TODO: same as ttl?
			},
		}
	}
	return &structs.VariableDecrypted{
		Items: map[string]string{
			"alloc": l.alloc.ID,
		},
		VariableMetadata: *meta,
		//VariableMetadata: structs.VariableMetadata{
		//	Namespace: l.alloc.Namespace,
		//	Path:      l.varPath(),
		//	Lock: &structs.VariableLock{
		//		ID:        lockID,
		//		TTL:       l.ttl,
		//		LockDelay: l.ttl, // TODO: same as ttl?
		//	},
		//},
	}
}

func (l *rpcLocker) writeRequest() structs.WriteRequest {
	return structs.WriteRequest{
		Region:    l.alloc.Job.Region,
		Namespace: l.alloc.Job.Namespace,
		AuthToken: l.token,
	}
}

//func (h *lockHook) retryError(fn func() error) error {
//	ctx, cancel := context.WithDeadline(h.shutdownCtx, time.Now().Add(time.Second*30))
//	defer cancel()
//
//	var err error
//	for {
//		select {
//		case <-ctx.Done():
//			return ctx.Err()
//		default:
//		}
//		if err = fn(); err == nil {
//			break
//		}
//		h.logger.Warn("lockHook.retryError", "error", err)
//	}
//	return err
//}
