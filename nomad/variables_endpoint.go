// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	maxAttemptsToRaftApply = 6
)

var (
	errVarAlreadyLocked = errors.New("variable already holds a lock")
	errVarNotFound      = errors.New("variable doesn't exist")
	errLockNotFound     = errors.New("variable doesn't hold a lock")
	errVarIsLocked      = errors.New("attempting to modify locked variable")
)

type variableTimers interface {
	CreateVariableLockTTLTimer(structs.VariableEncrypted)
	RemoveVariableLockTTLTimer(structs.VariableEncrypted)
	RenewTTLTimer(structs.VariableEncrypted) error
}

// Variables encapsulates the variables RPC endpoint which is
// callable via the Variables RPCs and externally via the "/v1/var{s}"
// HTTP API.
type Variables struct {
	srv    *Server
	ctx    *RPCContext
	logger hclog.Logger
	timers variableTimers

	encrypter *Encrypter
}

func NewVariablesEndpoint(srv *Server, ctx *RPCContext, enc *Encrypter) *Variables {
	return &Variables{srv: srv, ctx: ctx, logger: srv.logger.Named("variables"), encrypter: enc, timers: srv}
}

// Apply is used to apply a SV update request to the data store.
func (sv *Variables) Apply(args *structs.VariablesApplyRequest, reply *structs.VariablesApplyResponse) error {

	authErr := sv.srv.Authenticate(sv.ctx, args)
	if done, err := sv.srv.forward(structs.VariablesApplyRPCMethod, args, args, reply); done {
		return err
	}
	sv.srv.MeasureRPCRate("variables", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{
		"nomad", "variables", "apply", string(args.Op)}, time.Now())
	// TODO: Add metrics for acquire and release if the operation is lock related

	if args.Var == nil {
		return fmt.Errorf("variable must not be nil")
	}

	// Check if the Namespace is explicitly set on the variable. If
	// not, use the RequestNamespace
	targetNS := args.Var.Namespace
	if targetNS == "" {
		targetNS = args.RequestNamespace()
		args.Var.Namespace = targetNS
	}

	if !ServersMeetMinimumVersion(
		sv.srv.serf.Members(), sv.srv.Region(), minVersionKeyring, true) {
		return fmt.Errorf("all servers must be running version %v or later to apply variables", minVersionKeyring)
	}

	// Perform the ACL resolution.
	aclObj, err := sv.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	// IF ACL is being used,
	if aclObj != nil {
		err := hasOperationPermissions(aclObj, args.Var.Namespace, args.Var.Path, args.Op)
		if err != nil {
			return err
		}
	}

	err = canonicalizeAndValidate(args)
	if err != nil {
		return err
	}

	var ev *structs.VariableEncrypted

	switch args.Op {
	case structs.VarOpSet, structs.VarOpCAS, structs.VarOpLockAcquire,
		structs.VarOpLockRelease:
		ev, err = sv.encrypt(args.Var)
		if err != nil {
			return fmt.Errorf("variable error: encrypt: %w", err)
		}
		now := time.Now().UnixNano()
		ev.CreateTime = now // existing will override if it exists
		ev.ModifyTime = now

	case structs.VarOpDelete, structs.VarOpDeleteCAS:
		ev = &structs.VariableEncrypted{
			VariableMetadata: structs.VariableMetadata{
				Namespace:   args.Var.Namespace,
				Path:        args.Var.Path,
				ModifyIndex: args.Var.ModifyIndex,
			},
		}
	}

	// Make a SVEArgs
	sveArgs := structs.VarApplyStateRequest{
		Op:           args.Op,
		Var:          ev,
		WriteRequest: args.WriteRequest,
	}

	// Apply the update.
	o, index, err := sv.srv.raftApply(structs.VarApplyStateRequestType, sveArgs)
	if err != nil {
		return fmt.Errorf("raft apply failed: %w", err)
	}

	out, _ := o.(*structs.VarApplyStateResponse)

	// The return value depends on the operation results and the callers permissions
	r, err := sv.makeVariablesApplyResponse(args, out, aclObj)
	if err != nil {
		return err
	}

	*reply = *r
	reply.Index = index

	if !out.IsConflict() {
		switch args.Op {
		case structs.VarOpLockAcquire:
			sv.timers.CreateVariableLockTTLTimer(ev.Copy())
		case structs.VarOpLockRelease:
			sv.timers.RemoveVariableLockTTLTimer(ev.Copy())
		}
	}

	return nil
}

func hasReadPermission(aclObj *acl.ACL, namespace, path string) bool {
	return aclObj.AllowVariableOperation(namespace,
		path, acl.VariablesCapabilityRead, nil)
}

func hasOperationPermissions(aclObj *acl.ACL, namespace, path string, op structs.VarOp) error {

	hasPerm := func(perm string) bool {
		return aclObj.AllowVariableOperation(namespace,
			path, perm, nil)
	}

	switch op {
	case structs.VarOpSet, structs.VarOpCAS, structs.VarOpLockAcquire,
		structs.VarOpLockRelease:
		if !hasPerm(acl.VariablesCapabilityWrite) {
			return structs.ErrPermissionDenied
		}

	case structs.VarOpDelete, structs.VarOpDeleteCAS:
		if !hasPerm(acl.VariablesCapabilityDestroy) {
			return structs.ErrPermissionDenied
		}
	default:
		return fmt.Errorf("svPreApply: unexpected VarOp received: %q", op)
	}

	return nil
}

func canonicalizeAndValidate(args *structs.VariablesApplyRequest) error {
	switch args.Op {
	case structs.VarOpLockAcquire:
		// In case the user wants to use the default values so no lock data was provided.
		if args.Var.VariableMetadata.Lock == nil {
			args.Var.VariableMetadata.Lock = &structs.VariableLock{}
		}

		args.Var.Canonicalize()
		return args.Var.ValidateForLock()

	case structs.VarOpSet, structs.VarOpCAS:
		args.Var.Canonicalize()

		return args.Var.Validate()

	case structs.VarOpDelete, structs.VarOpDeleteCAS:
		if args.Var == nil || args.Var.Path == "" {
			return fmt.Errorf("delete requires a Path")
		}

	case structs.VarOpLockRelease:
		if args.Var == nil || args.Var.Lock == nil ||
			args.Var.Lock.ID == "" {
			return errors.New("release requires all lock information")
		}

		return structs.ValidatePath(args.Var.Path)
	}

	return nil
}

// MakeVariablesApplyResponse merges the output of this VarApplyStateResponse with the
// VariableDataItems
func (sv *Variables) makeVariablesApplyResponse(
	req *structs.VariablesApplyRequest, eResp *structs.VarApplyStateResponse,
	aclObj *acl.ACL) (*structs.VariablesApplyResponse, error) {

	out := structs.VariablesApplyResponse{
		Op:        eResp.Op,
		Input:     req.Var,
		Result:    eResp.Result,
		Error:     eResp.Error,
		WriteMeta: eResp.WriteMeta,
	}

	// The read permission modify the way the response is populated. If ACL is not
	// used, read permission is granted by default and every call is treated as management.
	var canRead bool = true
	var isManagement = true
	if aclObj != nil {
		canRead = hasReadPermission(aclObj, req.Var.Namespace, req.Var.Path)
		isManagement = aclObj.IsManagement()
	}

	if eResp.IsOk() {
		if eResp.WrittenSVMeta != nil {
			// The writer is allowed to read their own write
			out.Output = &structs.VariableDecrypted{
				VariableMetadata: *eResp.WrittenSVMeta,
				Items:            req.Var.Items.Copy(),
			}

			// Verify the caller is providing the correct lockID, meaning it is the
			// lock holder and has access to the lock information or is a management call.
			// If locked, remove the lock information from response.
			if !(isCallerOwner(req, eResp.WrittenSVMeta) || isManagement) {
				out.Output.VariableMetadata.Lock = nil
			}
		}

		return &out, nil
	}

	if eResp.IsError() {
		return &out, eResp.Error
	}

	// At this point, the response is necessarily a conflict.
	// Prime output from the encrypted responses metadata
	out.Conflict = &structs.VariableDecrypted{
		VariableMetadata: eResp.Conflict.VariableMetadata,
		Items:            nil,
	}

	// If the caller can't read the conflicting value, return the
	// metadata, but no items and flag it as redacted
	if !canRead {
		out.Result = structs.VarOpResultRedacted
		return &out, nil
	}

	if eResp.Conflict == nil || eResp.Conflict.KeyID == "" {
		// zero-value conflicts can be returned for delete-if-set
		dv := &structs.VariableDecrypted{}
		dv.Namespace = eResp.Conflict.Namespace
		dv.Path = eResp.Conflict.Path
		out.Conflict = dv
	} else {
		// At this point, the caller has read access to the conflicting
		// value so we can return it in the output; decrypt it.
		dv, err := sv.decrypt(eResp.Conflict)
		if err != nil {
			return nil, err
		}
		out.Conflict = dv
	}

	return &out, nil
}

// Read is used to get a specific variable
func (sv *Variables) Read(args *structs.VariablesReadRequest, reply *structs.VariablesReadResponse) error {

	authErr := sv.srv.Authenticate(sv.ctx, args)
	if done, err := sv.srv.forward(structs.VariablesReadRPCMethod, args, args, reply); done {
		return err
	}
	sv.srv.MeasureRPCRate("variables", structs.RateMetricRead, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "variables", "read"}, time.Now())

	aclObj, err := sv.handleMixedAuthEndpoint(args.QueryOptions,
		acl.PolicyRead, args.Path)
	if err != nil {
		return err
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {
			out, err := s.GetVariable(ws, args.RequestNamespace(), args.Path)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Data = nil
			if out != nil {
				dv, err := sv.decrypt(out)
				if err != nil {
					return err
				}

				ov := dv.Copy()
				if aclObj != nil && !aclObj.IsManagement() {
					ov.Lock = nil
				}

				reply.Data = &ov
				reply.Index = out.ModifyIndex
			} else {
				sv.srv.setReplyQueryMeta(s, state.TableVariables, &reply.QueryMeta)
			}
			return nil
		}}
	return sv.srv.blockingRPC(&opts)
}

// List is used to list variables held within state. It supports single
// and wildcard namespace listings.
func (sv *Variables) List(
	args *structs.VariablesListRequest,
	reply *structs.VariablesListResponse) error {

	authErr := sv.srv.Authenticate(sv.ctx, args)
	if done, err := sv.srv.forward(structs.VariablesListRPCMethod, args, args, reply); done {
		return err
	}
	sv.srv.MeasureRPCRate("variables", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "variables", "list"}, time.Now())

	// If the caller has requested to list variables across all namespaces, use
	// the custom function to perform this.
	if args.RequestNamespace() == structs.AllNamespacesSentinel {
		return sv.listAllVariables(args, reply)
	}

	var aclObj *acl.ACL
	var err error
	aclToken := args.GetIdentity().GetACLToken()
	if aclToken != nil {
		aclObj, err = sv.srv.ResolveACLForToken(aclToken)
		if err != nil {
			return err
		}
	}
	claims := args.GetIdentity().GetClaims()

	// Set up and return the blocking query.
	return sv.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			// Perform the state query to get an iterator.
			iter, err := stateStore.GetVariablesByNamespaceAndPrefix(ws, args.RequestNamespace(), args.Prefix)
			if err != nil {
				return err
			}

			// Generate the tokenizer to use for pagination using namespace and
			// ID to ensure complete uniqueness.
			tokenizer := paginator.NewStructsTokenizer(iter,
				paginator.StructsTokenizerOptions{
					WithNamespace: true,
					WithID:        true,
				},
			)

			filters := []paginator.Filter{
				paginator.GenericFilter{
					Allow: func(raw interface{}) (bool, error) {
						v := raw.(*structs.VariableEncrypted)
						if !strings.HasPrefix(v.Path, args.Prefix) {
							return false, nil
						}
						err := sv.authorize(aclObj, claims, v.Namespace, acl.PolicyList, v.Path)
						return err == nil, nil
					},
				},
			}

			// Set up our output after we have checked the error.
			var svs []*structs.VariableMetadata

			// Build the paginator. This includes the function that is
			// responsible for appending a variable to the variables
			// stubs slice.
			paginatorImpl, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
				func(raw interface{}) error {
					sv := raw.(*structs.VariableEncrypted)
					svStub := sv.VariableMetadata

					if aclObj != nil && !aclObj.IsManagement() {
						svStub.Lock = nil
					}

					svs = append(svs, &svStub)
					return nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			// Calling page populates our output variable stub array as well as
			// returns the next token.
			nextToken, err := paginatorImpl.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to read result page: %v", err)
			}

			// Populate the reply.
			reply.Data = svs
			reply.NextToken = nextToken

			// Use the index table to populate the query meta as we have no way
			// of tracking the max index on deletes.
			return sv.srv.setReplyQueryMeta(stateStore, state.TableVariables, &reply.QueryMeta)
		},
	})
}

// listAllVariables is used to list variables held within
// state where the caller has used the namespace wildcard identifier.
func (sv *Variables) listAllVariables(
	args *structs.VariablesListRequest,
	reply *structs.VariablesListResponse) error {

	// Perform token resolution. The request already goes through forwarding
	// and metrics setup before being called.
	var aclObj *acl.ACL
	var err error
	aclToken := args.GetIdentity().GetACLToken()
	if aclToken != nil {
		aclObj, err = sv.srv.ResolveACLForToken(aclToken)
		if err != nil {
			return err
		}
	}
	claims := args.GetIdentity().GetClaims()

	// Set up and return the blocking query.
	return sv.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			// Get all the variables stored within state.
			iter, err := stateStore.Variables(ws)
			if err != nil {
				return err
			}

			var svs []*structs.VariableMetadata

			// Generate the tokenizer to use for pagination using namespace and
			// ID to ensure complete uniqueness.
			tokenizer := paginator.NewStructsTokenizer(iter,
				paginator.StructsTokenizerOptions{
					WithNamespace: true,
					WithID:        true,
				})

			filters := []paginator.Filter{
				paginator.GenericFilter{
					Allow: func(raw interface{}) (bool, error) {
						v := raw.(*structs.VariableEncrypted)
						if !strings.HasPrefix(v.Path, args.Prefix) {
							return false, nil
						}
						err := sv.authorize(aclObj, claims, v.Namespace, acl.PolicyList, v.Path)
						return err == nil, nil
					},
				},
			}

			// Build the paginator. This includes the function that is
			// responsible for appending a variable to the stubs array.
			paginatorImpl, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
				func(raw interface{}) error {
					v := raw.(*structs.VariableEncrypted)
					svStub := v.VariableMetadata
					svs = append(svs, &svStub)
					return nil
				})
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to create result paginator: %v", err)
			}

			// Calling page populates our output variable stubs array as well as
			// returns the next token.
			nextToken, err := paginatorImpl.Page()
			if err != nil {
				return structs.NewErrRPCCodedf(
					http.StatusBadRequest, "failed to read result page: %v", err)
			}

			// Populate the reply.
			reply.Data = svs
			reply.NextToken = nextToken

			// Use the index table to populate the query meta as we have no way
			// of tracking the max index on deletes.
			return sv.srv.setReplyQueryMeta(stateStore, state.TableVariables, &reply.QueryMeta)
		},
	})
}

func (sv *Variables) encrypt(v *structs.VariableDecrypted) (*structs.VariableEncrypted, error) {
	b, err := json.Marshal(v.Items)
	if err != nil {
		return nil, err
	}
	ev := structs.VariableEncrypted{
		VariableMetadata: v.VariableMetadata,
	}

	ev.Data, ev.KeyID, err = sv.encrypter.Encrypt(b)
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

func (sv *Variables) decrypt(v *structs.VariableEncrypted) (*structs.VariableDecrypted, error) {
	b, err := sv.encrypter.Decrypt(v.Data, v.KeyID)
	if err != nil {
		return nil, err
	}
	dv := structs.VariableDecrypted{
		VariableMetadata: v.VariableMetadata,
	}
	dv.Items = make(map[string]string)
	err = json.Unmarshal(b, &dv.Items)
	if err != nil {
		return nil, err
	}
	return &dv, nil
}

// handleMixedAuthEndpoint is a helper to handle auth on RPC endpoints that can
// either be called by external clients or by workload identity
func (sv *Variables) handleMixedAuthEndpoint(args structs.QueryOptions, policy, pathOrPrefix string) (*acl.ACL, error) {

	var aclObj *acl.ACL
	var err error
	aclToken := args.GetIdentity().GetACLToken()
	if aclToken != nil {
		aclObj, err = sv.srv.ResolveACLForToken(aclToken)
		if err != nil {
			return nil, err
		}
	}
	claims := args.GetIdentity().GetClaims()

	err = sv.authorize(aclObj, claims, args.RequestNamespace(), policy, pathOrPrefix)
	if err != nil {
		return aclObj, err
	}

	return aclObj, nil
}

func (sv *Variables) authorize(aclObj *acl.ACL, claims *structs.IdentityClaims, ns, policy, pathOrPrefix string) error {

	if aclObj == nil && claims == nil {
		return nil // ACLs aren't enabled
	}

	// Perform normal ACL validation. If the ACL object is nil, that means we're
	// working with an identity claim.
	if aclObj != nil {
		allowed := aclObj.AllowVariableOperation(ns, pathOrPrefix, policy, nil)
		if !allowed {
			return structs.ErrPermissionDenied
		}
		return nil
	}

	// Check the workload-associated policies and automatic task access to
	// variables.
	if claims != nil {
		aclObj, err := sv.srv.ResolveClaims(claims)
		if err != nil {
			return err // returns internal errors only
		}
		if aclObj != nil {
			group, err := sv.groupForAlloc(claims)
			if err != nil {
				// returns ErrPermissionDenied for claims from terminal
				// allocations, otherwise only internal errors
				return err
			}
			allowed := aclObj.AllowVariableOperation(
				ns, pathOrPrefix, policy, &acl.ACLClaim{
					Namespace: claims.Namespace,
					Job:       claims.JobID,
					Group:     group,
					Task:      claims.TaskName,
				})
			if allowed {
				return nil
			}
		}
	}
	return structs.ErrPermissionDenied
}

func (sv *Variables) groupForAlloc(claims *structs.IdentityClaims) (string, error) {
	store, err := sv.srv.fsm.State().Snapshot()
	if err != nil {
		return "", err
	}
	alloc, err := store.AllocByID(nil, claims.AllocationID)
	if err != nil {
		return "", err
	}
	if alloc == nil || alloc.Job == nil {
		return "", structs.ErrPermissionDenied
	}
	return alloc.TaskGroup, nil
}

// RenewLock is used to apply a SV renew lock operation on a variable to maintain the lease.
func (sv *Variables) RenewLock(args *structs.VariablesRenewLockRequest, reply *structs.VariablesRenewLockResponse) error {
	authErr := sv.srv.Authenticate(sv.ctx, args)
	if done, err := sv.srv.forward(structs.VariablesRenewLockRPCMethod, args, args, reply); done {
		return err
	}

	sv.srv.MeasureRPCRate("variables", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{
		"nomad", "variables", "lock", "renew"}, time.Now())

	aclObj, err := sv.srv.ResolveACL(args)
	if err != nil {
		return err
	}

	// ACLs are enabled, check for the correct permissions
	if aclObj != nil {
		if !aclObj.AllowVariableOperation(args.WriteRequest.Namespace, args.Path,
			acl.VariablesCapabilityWrite, nil) {
			return structs.ErrPermissionDenied
		}
	}

	if err := args.Validate(); err != nil {
		return err
	}

	// Get the variable from the SS to verify it exists and is currently lock
	stateSnapshot, err := sv.srv.State().Snapshot()
	if err != nil {
		return err
	}

	_, encryptedVar, err := stateSnapshot.VarGet(nil, args.WriteRequest.Namespace, args.Path)
	if err != nil {
		return err
	}

	if encryptedVar == nil {
		return errVarNotFound
	}

	if encryptedVar.Lock == nil {
		return errLockNotFound
	}

	// Verify the caller is providing the correct lockID, meaning it is the lock holder and
	// can renew the lock.
	if encryptedVar.Lock.ID != args.LockID {
		return errVarIsLocked
	}

	// if the lock exists in the variable, but not in the timer, it means
	// it expired and cant be renewed anymore. The delay will take care of
	// removing the lock from the variable when it expires.
	err = sv.timers.RenewTTLTimer(encryptedVar.Copy())
	if err != nil {
		return errVarIsLocked
	}

	updatedVar := encryptedVar.Copy()
	reply.VarMeta = &updatedVar.VariableMetadata
	reply.Index = encryptedVar.ModifyIndex
	return nil
}

func isCallerOwner(req *structs.VariablesApplyRequest, respVarMeta *structs.VariableMetadata) bool {
	reqLock := req.Var.VariableMetadata.Lock
	savedLock := respVarMeta.Lock

	return reqLock != nil &&
		savedLock != nil &&
		reqLock.ID == savedLock.ID
}
