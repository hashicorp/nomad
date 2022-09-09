package nomad

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Variables encapsulates the variables RPC endpoint which is
// callable via the Variables RPCs and externally via the "/v1/var{s}"
// HTTP API.
type Variables struct {
	srv       *Server
	logger    hclog.Logger
	encrypter *Encrypter
}

// Apply is used to apply a SV update request to the data store.
func (sv *Variables) Apply(args *structs.VariablesApplyRequest, reply *structs.VariablesApplyResponse) error {
	if done, err := sv.srv.forward(structs.VariablesApplyRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{
		"nomad", "variables", "apply", string(args.Op)}, time.Now())

	// Check if the Namespace is explicitly set on the variable. If
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

	var ev *structs.VariableEncrypted

	switch args.Op {
	case structs.VarOpSet, structs.VarOpCAS:
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
	out, index, err := sv.srv.raftApply(structs.VarApplyStateRequestType, sveArgs)
	if err != nil {
		return fmt.Errorf("raft apply failed: %w", err)
	}
	r, err := sv.makeVariablesApplyResponse(args, out.(*structs.VarApplyStateResponse), canRead)
	if err != nil {
		return err
	}
	*reply = *r
	reply.Index = index
	return nil
}

func svePreApply(sv *Variables, args *structs.VariablesApplyRequest, vd *structs.VariableDecrypted) (canRead bool, err error) {

	canRead = false
	var aclObj *acl.ACL

	// Perform the ACL token resolution.
	if aclObj, err = sv.srv.ResolveToken(args.AuthToken); err != nil {
		return
	} else if aclObj != nil {
		hasPerm := func(perm string) bool {
			return aclObj.AllowVariableOperation(args.Var.Namespace,
				args.Var.Path, perm)
		}
		canRead = hasPerm(acl.VariablesCapabilityRead)

		switch args.Op {
		case structs.VarOpSet, structs.VarOpCAS:
			if !hasPerm(acl.VariablesCapabilityWrite) {
				err = structs.ErrPermissionDenied
				return
			}
		case structs.VarOpDelete, structs.VarOpDeleteCAS:
			if !hasPerm(acl.VariablesCapabilityDestroy) {
				err = structs.ErrPermissionDenied
				return
			}
		default:
			err = fmt.Errorf("svPreApply: unexpected VarOp received: %q", args.Op)
			return
		}
	} else {
		// ACLs are not enabled.
		canRead = true
	}

	switch args.Op {
	case structs.VarOpSet, structs.VarOpCAS:
		args.Var.Canonicalize()
		if err = args.Var.Validate(); err != nil {
			return
		}

	case structs.VarOpDelete, structs.VarOpDeleteCAS:
		if args.Var == nil || args.Var.Path == "" {
			err = fmt.Errorf("delete requires a Path")
			return
		}
	}

	return
}

// MakeVariablesApplyResponse merges the output of this VarApplyStateResponse with the
// VariableDataItems
func (sv *Variables) makeVariablesApplyResponse(
	req *structs.VariablesApplyRequest, eResp *structs.VarApplyStateResponse,
	canRead bool) (*structs.VariablesApplyResponse, error) {

	out := structs.VariablesApplyResponse{
		Op:        eResp.Op,
		Input:     req.Var,
		Result:    eResp.Result,
		Error:     eResp.Error,
		WriteMeta: eResp.WriteMeta,
	}

	if eResp.IsOk() {
		if eResp.WrittenSVMeta != nil {
			// The writer is allowed to read their own write
			out.Output = &structs.VariableDecrypted{
				VariableMetadata: *eResp.WrittenSVMeta,
				Items:            req.Var.Items.Copy(),
			}
		}
		return &out, nil
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
	if done, err := sv.srv.forward(structs.VariablesReadRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "variables", "read"}, time.Now())

	_, err := sv.handleMixedAuthEndpoint(args.QueryOptions,
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

	if done, err := sv.srv.forward(structs.VariablesListRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "variables", "list"}, time.Now())

	// If the caller has requested to list variables across all namespaces, use
	// the custom function to perform this.
	if args.RequestNamespace() == structs.AllNamespacesSentinel {
		return sv.listAllVariables(args, reply)
	}

	aclObj, err := sv.handleMixedAuthEndpoint(args.QueryOptions,
		acl.PolicyList, args.Prefix)
	if err != nil {
		return err
	}

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
						sv := raw.(*structs.VariableEncrypted)
						return strings.HasPrefix(sv.Path, args.Prefix) &&
							(aclObj == nil || aclObj.AllowVariableOperation(sv.Namespace, sv.Path, acl.PolicyList)), nil
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
func (s *Variables) listAllVariables(
	args *structs.VariablesListRequest,
	reply *structs.VariablesListResponse) error {

	// Perform token resolution. The request already goes through forwarding
	// and metrics setup before being called.
	aclObj, err := s.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	}

	// allowFunc checks whether the caller has the read-job capability on the
	// passed namespace.
	allowFunc := func(ns string) bool {
		return aclObj.AllowVariableOperation(ns, "", acl.PolicyList)
	}

	// Set up and return the blocking query.
	return s.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			// Identify which namespaces the caller has access to. If they do
			// not have access to any, send them an empty response. Otherwise,
			// handle any error in a traditional manner.
			_, err := allowedNSes(aclObj, stateStore, allowFunc)
			switch err {
			case structs.ErrPermissionDenied:
				reply.Data = make([]*structs.VariableMetadata, 0)
				return nil
			case nil:
				// Fallthrough.
			default:
				return err
			}

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
				},
			)

			filters := []paginator.Filter{
				paginator.GenericFilter{
					Allow: func(raw interface{}) (bool, error) {
						sv := raw.(*structs.VariableEncrypted)
						return strings.HasPrefix(sv.Path, args.Prefix) &&
							(aclObj == nil || aclObj.AllowVariableOperation(sv.Namespace, sv.Path, acl.PolicyList)), nil
					},
				},
			}

			// Build the paginator. This includes the function that is
			// responsible for appending a variable to the stubs array.
			paginatorImpl, err := paginator.NewPaginator(iter, tokenizer, filters, args.QueryOptions,
				func(raw interface{}) error {
					sv := raw.(*structs.VariableEncrypted)
					svStub := sv.VariableMetadata
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
			return s.srv.setReplyQueryMeta(stateStore, state.TableVariables, &reply.QueryMeta)
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
func (sv *Variables) handleMixedAuthEndpoint(args structs.QueryOptions, cap, pathOrPrefix string) (*acl.ACL, error) {

	// Perform the initial token resolution.
	aclObj, err := sv.srv.ResolveToken(args.AuthToken)
	if err == nil {
		// Perform our ACL validation. If the object is nil, this means ACLs
		// are not enabled, otherwise trigger the allowed namespace function.
		if aclObj != nil {
			if !aclObj.AllowVariableOperation(args.RequestNamespace(), pathOrPrefix, cap) {
				return nil, structs.ErrPermissionDenied
			}
		}
		return aclObj, nil
	}
	if helper.IsUUID(args.AuthToken) {
		// early return for ErrNotFound or other errors if it's formed
		// like an ACLToken.SecretID
		return nil, err
	}

	// Attempt to verify the token as a JWT with a workload
	// identity claim
	claims, err := sv.srv.VerifyClaim(args.AuthToken)
	if err != nil {
		metrics.IncrCounter([]string{
			"nomad", "variables", "invalid_allocation_identity"}, 1)
		sv.logger.Trace("allocation identity was not valid", "error", err)
		return nil, structs.ErrPermissionDenied
	}

	// The workload identity gets access to paths that match its
	// identity, without having to go thru the ACL system
	err = sv.authValidatePrefix(claims, args.RequestNamespace(), pathOrPrefix)
	if err == nil {
		return aclObj, nil
	}

	// If the workload identity doesn't match the implicit permissions
	// given to paths, check for its attached ACL policies
	aclObj, err = sv.srv.ResolveClaims(claims)
	if err != nil {
		return nil, err // this only returns an error when the state store has gone wrong
	}
	if aclObj != nil && aclObj.AllowVariableOperation(
		args.RequestNamespace(), pathOrPrefix, cap) {
		return aclObj, nil
	}
	return nil, structs.ErrPermissionDenied
}

// authValidatePrefix asserts that the requested path is valid for
// this allocation
func (sv *Variables) authValidatePrefix(claims *structs.IdentityClaims, ns, pathOrPrefix string) error {

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
