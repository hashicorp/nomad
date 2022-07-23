package nomad

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

// SecureVariables encapsulates the secure variables RPC endpoint which is
// callable via the SecureVariables RPCs and externally via the "/v1/var{s}"
// HTTP API.
type SecureVariables struct {
	srv       *Server
	logger    hclog.Logger
	encrypter *Encrypter
}

// Apply is used to apply a SV update request to the data store.
func (sv *SecureVariables) Apply(args *structs.SecureVariablesApplyRequest, reply *structs.SecureVariablesApplyResponse) error {
	if done, err := sv.srv.forward(structs.SecureVariablesApplyRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "apply"}, time.Now())

	// Check if the Namespace is explicitly set on the secure variable. If
	// not, use the RequestNamespace
	if args.Var == nil {
		return fmt.Errorf("variable must not be nil")
	}
	targetNS := args.Var.Namespace
	if targetNS == "" {
		targetNS = args.RequestNamespace()
		args.Var.Namespace = targetNS
	}

	canRead, err := svePreApply(sv, args, args.Var)
	if err != nil {
		return err
	}

	var ev *structs.SecureVariableEncrypted

	switch args.Op {
	case structs.SVOpSet, structs.SVOpCAS:
		ev, err = sv.encrypt(args.Var)
		if err != nil {
			return fmt.Errorf("secure variable error: encrypt: %w", err)
		}
	case structs.SVOpDelete, structs.SVOpDeleteCAS:
		ev = &structs.SecureVariableEncrypted{
			SecureVariableMetadata: structs.SecureVariableMetadata{
				Namespace:   args.Var.Namespace,
				Path:        args.Var.Path,
				ModifyIndex: args.Var.ModifyIndex,
			},
		}
	}

	// Make a SVEArgs
	sveArgs := structs.SVApplyStateRequest{
		Op:           args.Op,
		Var:          ev,
		WriteRequest: args.WriteRequest,
	}

	// Apply the update.
	out, index, err := sv.srv.raftApply(structs.SVApplyStateRequestType, sveArgs)
	if err != nil {
		return fmt.Errorf("raft apply failed: %w", err)
	}
	r, err := sv.makeSecureVariablesApplyResponse(args, out.(*structs.SVApplyStateResponse), canRead)
	if err != nil {
		return err
	}
	*reply = *r
	reply.Index = index
	return nil
}

func svePreApply(sv *SecureVariables, args *structs.SecureVariablesApplyRequest, vd *structs.SecureVariableDecrypted) (canRead bool, err error) {

	canRead = false
	var aclObj *acl.ACL

	// Perform the ACL token resolution.
	if aclObj, err = sv.srv.ResolveToken(args.AuthToken); err != nil {
		return
	} else if aclObj != nil {
		hasPerm := func(perm string) bool {
			return aclObj.AllowSecureVariableOperation(args.Var.Namespace,
				args.Var.Path, perm)
		}
		canRead = hasPerm(acl.SecureVariablesCapabilityRead)

		switch args.Op {
		case structs.SVOpSet, structs.SVOpCAS:
			if !hasPerm(acl.SecureVariablesCapabilityWrite) {
				err = structs.ErrPermissionDenied
				return
			}
		case structs.SVOpDelete, structs.SVOpDeleteCAS:
			if !hasPerm(acl.SecureVariablesCapabilityDestroy) {
				err = structs.ErrPermissionDenied
				return
			}
		default:
			err = fmt.Errorf("svPreApply: unexpected SVOp received: %q", args.Op)
			return
		}
	} else {
		// ACLs are not enabled.
		canRead = true
	}

	switch args.Op {
	case structs.SVOpSet, structs.SVOpCAS:
		args.Var.Canonicalize()
		if err = args.Var.Validate(); err != nil {
			return
		}

	case structs.SVOpDelete, structs.SVOpDeleteCAS:
		if args.Var == nil || args.Var.Path == "" {
			err = fmt.Errorf("delete requires a Path")
			return
		}
	}

	return
}

// MakeSecureVariablesApplyResponse merges the output of this SVApplyStateResponse with the
// SecureVariableDataItems
func (sv *SecureVariables) makeSecureVariablesApplyResponse(
	req *structs.SecureVariablesApplyRequest, eResp *structs.SVApplyStateResponse,
	canRead bool) (*structs.SecureVariablesApplyResponse, error) {

	out := structs.SecureVariablesApplyResponse{
		Op:        eResp.Op,
		Input:     req.Var,
		Result:    eResp.Result,
		Error:     eResp.Error,
		WriteMeta: eResp.WriteMeta,
	}

	if eResp.IsOk() {
		if eResp.WrittenSVMeta != nil {
			// The writer is allowed to read their own write
			out.Output = &structs.SecureVariableDecrypted{
				SecureVariableMetadata: *eResp.WrittenSVMeta,
				Items:                  req.Var.Items.Copy(),
			}
		}
		return &out, nil
	}

	// At this point, the response is necessarily a conflict.
	// Prime output from the encrypted responses metadata
	out.Conflict = &structs.SecureVariableDecrypted{
		SecureVariableMetadata: eResp.Conflict.SecureVariableMetadata,
		Items:                  nil,
	}

	// If the caller can't read the conflicting value, return the
	// metadata, but no items and flag it as redacted
	if !canRead {
		out.Result = structs.SVOpResultRedacted
		return &out, nil
	}

	if eResp.Conflict == nil || eResp.Conflict.KeyID == "" {
		// zero-value conflicts can be returned for delete-if-set
		dv := &structs.SecureVariableDecrypted{}
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

// Upsert creates or updates secure variables held within Nomad. Due to ACL
// checking, every element in Data will be checked for namespace and targeted
// to the namespace in the SecureVariable. Therefore, the caller must ensure
// that the provided struct's Namespace is the desired destination. Unset
// Namespace values will default to `args.RequestNamespace`
func (sv *SecureVariables) Upsert(
	args *structs.SecureVariablesUpsertRequest,
	reply *structs.SecureVariablesUpsertResponse) error {

	if done, err := sv.srv.forward(structs.SecureVariablesUpsertRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "upsert"}, time.Now())

	// Use a multierror, so we can capture all validation errors and pass this
	// back so they can be addressed by the caller in a single pass.
	var mErr multierror.Error
	uArgs := structs.SecureVariablesEncryptedUpsertRequest{
		Data:         make([]*structs.SecureVariableEncrypted, len(args.Data)),
		WriteRequest: args.WriteRequest,
	}

	// Iterate the secure variables and validate them. Any error results in the
	// call failing.
	for i, v := range args.Data {

		// Check if the Namespace is explicitly set on the secure variable. If
		// not, use the RequestNamespace
		targetNS := v.Namespace
		if targetNS == "" {
			targetNS = args.RequestNamespace()
			v.Namespace = targetNS
		}

		// Perform the ACL token resolution.
		if aclObj, err := sv.srv.ResolveToken(args.AuthToken); err != nil {
			return err
		} else if aclObj != nil {
			for _, variable := range args.Data {
				if !aclObj.AllowSecureVariableOperation(targetNS,
					variable.Path, acl.PolicyWrite) {
					return structs.ErrPermissionDenied
				}
			}
		}

		v.Canonicalize()
		if err := v.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}

		ns, err := sv.srv.State().NamespaceByName(nil, v.Namespace)
		if err != nil {
			return err
		}
		if ns == nil {
			return fmt.Errorf("secure variable %q is in nonexistent namespace %q",
				v.Path, v.Namespace)
		}

		if args.CheckIndex != nil {
			var conflict *structs.SecureVariableDecrypted
			if err := sv.validateCASUpdate(*args.CheckIndex, v, &conflict); err != nil {
				if reply.Conflicts == nil {
					reply.Conflicts = make([]*structs.SecureVariableDecrypted, len(args.Data))
				}
				reply.Conflicts[i] = conflict
				continue
			}
		}
		ev, err := sv.encrypt(v)
		if err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}
		uArgs.Data[i] = ev
	}
	if len(reply.Conflicts) != 0 {
		// This is a reply with CAS conflicts so it needs to return here
		// "successfully". The caller needs to check to see if Conflicts
		// is non-Nil.
		return nil
	}
	if err := mErr.ErrorOrNil(); err != nil {
		return &mErr
	}

	// Update via Raft.
	out, index, err := sv.srv.raftApply(structs.SecureVariableUpsertRequestType, uArgs)
	if err != nil {
		return err
	}

	// Check if the FSM response, which is an interface, contains an error.
	if err, ok := out.(error); ok && err != nil {
		return err
	}

	// Update the index. There is no need to floor this as we are writing to
	// state and therefore will get a non-zero index response.
	reply.Index = index
	return nil
}

// Delete removes a single secure variable, as specified by its namespace and
// path from Nomad.
func (sv *SecureVariables) Delete(
	args *structs.SecureVariablesDeleteRequest,
	reply *structs.SecureVariablesDeleteResponse) error {

	if done, err := sv.srv.forward(structs.SecureVariablesDeleteRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "delete"}, time.Now())

	// Perform the ACL token resolution.
	if aclObj, err := sv.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil {
		if !aclObj.AllowSecureVariableOperation(args.RequestNamespace(), args.Path, acl.PolicyWrite) {
			return structs.ErrPermissionDenied
		}
	}
	if args.CheckIndex != nil {

		if err := sv.validateCASDelete(*args.CheckIndex, args.Namespace, args.Path, &reply.Conflict); err != nil {

			// If the validateCASDelete func sends back the conflict sentinel
			// error value then it will have put the conflict into the reply,
			// and we need to "succeed".
			if err.Error() == "conflict" {
				reply.Index = reply.Conflict.ModifyIndex
				return nil
			}

			// There are a few cases where validateCASDelete can error that
			// aren't conflicts.
			return err
		}
	}
	// Update via Raft.
	out, index, err := sv.srv.raftApply(structs.SecureVariableDeleteRequestType, args)
	if err != nil {
		return err
	}

	// Check if the FSM response, which is an interface, contains an error.
	if err, ok := out.(error); ok && err != nil {
		return err
	}

	// Update the index. There is no need to floor this as we are writing to
	// state and therefore will get a non-zero index response.
	reply.Index = index
	return nil
}

// Read is used to get a specific secure variable
func (sv *SecureVariables) Read(args *structs.SecureVariablesReadRequest, reply *structs.SecureVariablesReadResponse) error {
	if done, err := sv.srv.forward(structs.SecureVariablesReadRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "read"}, time.Now())

	// FIXME: Temporary ACL Test policy. Update once implementation complete
	err := sv.handleMixedAuthEndpoint(args.QueryOptions,
		acl.PolicyRead, args.Path)
	if err != nil {
		return err
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {
			out, err := s.GetSecureVariable(ws, args.RequestNamespace(), args.Path)
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
				reply.Data = &ov
				reply.Index = out.ModifyIndex
			} else {
				sv.srv.replySetIndex(state.TableSecureVariables, &reply.QueryMeta)
			}
			return nil
		}}
	return sv.srv.blockingRPC(&opts)
}

// List is used to list secure variables held within state. It supports single
// and wildcard namespace listings.
func (sv *SecureVariables) List(
	args *structs.SecureVariablesListRequest,
	reply *structs.SecureVariablesListResponse) error {

	if done, err := sv.srv.forward(structs.SecureVariablesListRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "list"}, time.Now())

	// If the caller has requested to list secure variables across all namespaces, use
	// the custom function to perform this.
	if args.RequestNamespace() == structs.AllNamespacesSentinel {
		return sv.listAllSecureVariables(args, reply)
	}

	// FIXME: Temporary ACL Test policy. Update once implementation complete
	err := sv.handleMixedAuthEndpoint(args.QueryOptions,
		acl.PolicyList, args.Prefix)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}

	// Set up and return the blocking query.
	return sv.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			// Perform the state query to get an iterator.
			iter, err := stateStore.GetSecureVariablesByNamespaceAndPrefix(ws, args.RequestNamespace(), args.Prefix)
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

			// Set up our output after we have checked the error.
			var svs []*structs.SecureVariableMetadata

			// Build the paginator. This includes the function that is
			// responsible for appending a variable to the secure variables
			// stubs slice.
			paginatorImpl, err := paginator.NewPaginator(iter, tokenizer, nil, args.QueryOptions,
				func(raw interface{}) error {
					sv := raw.(*structs.SecureVariableEncrypted)
					svStub := sv.SecureVariableMetadata
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
			return sv.srv.setReplyQueryMeta(stateStore, state.TableSecureVariables, &reply.QueryMeta)
		},
	})
}

// listAllSecureVariables is used to list secure variables held within
// state where the caller has used the namespace wildcard identifier.
func (s *SecureVariables) listAllSecureVariables(
	args *structs.SecureVariablesListRequest,
	reply *structs.SecureVariablesListResponse) error {

	// Perform token resolution. The request already goes through forwarding
	// and metrics setup before being called.
	aclObj, err := s.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	}

	// allowFunc checks whether the caller has the read-job capability on the
	// passed namespace.
	allowFunc := func(ns string) bool {
		return aclObj.AllowSecureVariableOperation(ns, "", acl.PolicyList)
	}

	// Set up and return the blocking query.
	return s.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			// Identify which namespaces the caller has access to. If they do
			// not have access to any, send them an empty response. Otherwise,
			// handle any error in a traditional manner.
			allowedNSes, err := allowedNSes(aclObj, stateStore, allowFunc)
			switch err {
			case structs.ErrPermissionDenied:
				reply.Data = make([]*structs.SecureVariableMetadata, 0)
				return nil
			case nil:
				// Fallthrough.
			default:
				return err
			}

			// Get all the secure variables stored within state.
			iter, err := stateStore.SecureVariables(ws)
			if err != nil {
				return err
			}

			var svs []*structs.SecureVariableMetadata

			// Generate the tokenizer to use for pagination using namespace and
			// ID to ensure complete uniqueness.
			tokenizer := paginator.NewStructsTokenizer(iter,
				paginator.StructsTokenizerOptions{
					WithNamespace: true,
					WithID:        true,
				},
			)

			// Wrap the SecureVariables iterator with a FilterIterator to
			// eliminate invalid values before sending them to the paginator.
			fltrIter := memdb.NewFilterIterator(iter, func(raw interface{}) bool {

				// Values are filtered when the func returns true.
				sv := raw.(*structs.SecureVariableEncrypted)
				if allowedNSes != nil && !allowedNSes[sv.Namespace] {
					return true
				}
				if !strings.HasPrefix(sv.Path, args.Prefix) {
					return true
				}
				return false
			})

			// Build the paginator. This includes the function that is
			// responsible for appending a variable to the stubs array.
			paginatorImpl, err := paginator.NewPaginator(fltrIter, tokenizer, nil, args.QueryOptions,
				func(raw interface{}) error {
					sv := raw.(*structs.SecureVariableEncrypted)
					svStub := sv.SecureVariableMetadata
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
			return s.srv.setReplyQueryMeta(stateStore, state.TableSecureVariables, &reply.QueryMeta)
		},
	})
}

func (sv *SecureVariables) encrypt(v *structs.SecureVariableDecrypted) (*structs.SecureVariableEncrypted, error) {
	b, err := json.Marshal(v.Items)
	if err != nil {
		return nil, err
	}
	ev := structs.SecureVariableEncrypted{
		SecureVariableMetadata: v.SecureVariableMetadata,
	}
	ev.Data, ev.KeyID, err = sv.encrypter.Encrypt(b)
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

func (sv *SecureVariables) decrypt(v *structs.SecureVariableEncrypted) (*structs.SecureVariableDecrypted, error) {
	b, err := sv.encrypter.Decrypt(v.Data, v.KeyID)
	if err != nil {
		return nil, err
	}
	dv := structs.SecureVariableDecrypted{
		SecureVariableMetadata: v.SecureVariableMetadata,
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
func (sv *SecureVariables) handleMixedAuthEndpoint(args structs.QueryOptions, cap, pathOrPrefix string) error {

	// Perform the initial token resolution.
	aclObj, err := sv.srv.ResolveToken(args.AuthToken)
	if err == nil {
		// Perform our ACL validation. If the object is nil, this means ACLs
		// are not enabled, otherwise trigger the allowed namespace function.
		if aclObj != nil {
			if !aclObj.AllowSecureVariableOperation(args.RequestNamespace(), pathOrPrefix, cap) {
				return structs.ErrPermissionDenied
			}
		}
		return nil
	}
	if helper.IsUUID(args.AuthToken) {
		// early return for ErrNotFound or other errors if it's formed
		// like an ACLToken.SecretID
		return err
	}

	// Attempt to verify the token as a JWT with a workload
	// identity claim
	claims, err := sv.srv.VerifyClaim(args.AuthToken)
	if err != nil {
		metrics.IncrCounter([]string{
			"nomad", "secure_variables", "invalid_allocation_identity"}, 1)
		sv.logger.Trace("allocation identity was not valid", "error", err)
		return structs.ErrPermissionDenied
	}

	// The workload identity gets access to paths that match its
	// identity, without having to go thru the ACL system
	err = sv.authValidatePrefix(claims, args.RequestNamespace(), pathOrPrefix)
	if err == nil {
		return nil
	}

	// If the workload identity doesn't match the implicit permissions
	// given to paths, check for its attached ACL policies
	aclObj, err = sv.srv.ResolveClaims(claims)
	if err != nil {
		return err // this only returns an error when the state store has gone wrong
	}
	if aclObj != nil && aclObj.AllowSecureVariableOperation(
		args.RequestNamespace(), pathOrPrefix, cap) {
		return nil
	}
	return structs.ErrPermissionDenied
}

// authValidatePrefix asserts that the requested path is valid for
// this allocation
func (sv *SecureVariables) authValidatePrefix(claims *structs.IdentityClaims, ns, pathOrPrefix string) error {

	store, err := sv.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	alloc, err := store.AllocByID(nil, claims.AllocationID)
	if err != nil {
		return err
	}
	if alloc == nil || alloc.Job == nil {
		return fmt.Errorf("allocation does not exist")
	}
	if alloc.Job.Namespace != ns {
		return fmt.Errorf("allocation is in another namespace")
	}

	parts := strings.Split(pathOrPrefix, "/")
	expect := []string{"nomad", "jobs", claims.JobID, alloc.TaskGroup, claims.TaskName}
	if len(parts) > len(expect) {
		return structs.ErrPermissionDenied
	}

	for idx, part := range parts {
		if part != expect[idx] {
			return structs.ErrPermissionDenied
		}
	}
	return nil
}

func (s *SecureVariables) validateCASUpdate(cidx uint64, sv *structs.SecureVariableDecrypted, conflict **structs.SecureVariableDecrypted) error {
	return s.validateCAS(cidx, sv.Namespace, sv.Path, conflict)
}

func (s *SecureVariables) validateCASDelete(cidx uint64, namespace, path string, conflict **structs.SecureVariableDecrypted) error {
	return s.validateCAS(cidx, namespace, path, conflict)
}

func (s *SecureVariables) validateCAS(cidx uint64, namespace, path string, conflictOut **structs.SecureVariableDecrypted) error {
	casConflict := errors.New("conflict")
	// lookup any existing key and validate the update
	snap, err := s.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	ws := memdb.NewWatchSet()
	exist, err := snap.GetSecureVariable(ws, namespace, path)
	if err != nil {
		return fmt.Errorf("cas error: %w", err)
	}
	if exist == nil && cidx != 0 {
		// return a zero value with the namespace and path applied
		zeroVal := &structs.SecureVariableDecrypted{
			SecureVariableMetadata: structs.SecureVariableMetadata{
				Namespace:   namespace,
				Path:        path,
				CreateIndex: 0,
				CreateTime:  0,
				ModifyIndex: 0,
				ModifyTime:  0,
			},
			Items: nil,
		}
		*conflictOut = zeroVal
		return casConflict
	}
	if exist != nil && exist.ModifyIndex != cidx {
		dec, err := s.decrypt(exist)
		if err != nil {
			// we can't return the conflict and we will have to bail out
			decErrStr := fmt.Sprintf(". Additional error decrypting conflict: %s", err)
			return fmt.Errorf("cas error: requested index %v; found index %v%s", cidx, exist.ModifyIndex, decErrStr)
		}
		*conflictOut = dec
		return casConflict
	}

	return nil
}
