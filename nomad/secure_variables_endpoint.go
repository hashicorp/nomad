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

// Upsert creates or updates secure variables held within Nomad.
func (sv *SecureVariables) Upsert(
	args *structs.SecureVariablesUpsertRequest,
	reply *structs.SecureVariablesUpsertResponse) error {

	if done, err := sv.srv.forward(structs.SecureVariablesUpsertRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "upsert"}, time.Now())

	// Perform the ACL token resolution.
	if aclObj, err := sv.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil {
		// FIXME: Temporary ACL Test policy. Update once implementation complete
		if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
			return structs.ErrPermissionDenied
		}
	}

	// Use a multierror, so we can capture all validation errors and pass this
	// back so they can be addressed by the caller in a single pass.
	var mErr multierror.Error

	// Iterate the secure variables and validate them. Any error results in the
	// call failing.
	for _, i := range args.Data {
		i.Canonicalize()
		if err := i.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}
		if err := sv.encrypt(i); err != nil {
			mErr.Errors = append(mErr.Errors, err)
			continue
		}
	}
	if err := mErr.ErrorOrNil(); err != nil {
		return err
	}

	// Update via Raft.
	out, index, err := sv.srv.raftApply(structs.SecureVariableUpsertRequestType, args)
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
		// FIXME: Temporary ACL Test policy. Update once implementation complete
		if !aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob) {
			return structs.ErrPermissionDenied
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
		acl.NamespaceCapabilitySubmitJob, args.Path)
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
				reply.Data = out.Copy()
				if err := sv.decrypt(reply.Data); err != nil {
					return err
				}
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
		acl.NamespaceCapabilitySubmitJob, args.Prefix)
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
			var svs []*structs.SecureVariableStub

			// Build the paginator. This includes the function that is
			// responsible for appending a variable to the secure variables
			// stubs slice.
			paginatorImpl, err := paginator.NewPaginator(iter, tokenizer, nil, args.QueryOptions,
				func(raw interface{}) error {
					sv := raw.(*structs.SecureVariable)
					svStub := sv.Stub()
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
		// FIXME: Temporary ACL Test policy. Update once implementation complete
		return aclObj.AllowNsOp(ns, acl.NamespaceCapabilityReadJob)
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
				reply.Data = make([]*structs.SecureVariableStub, 0)
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

			var svs []*structs.SecureVariableStub

			// Generate the tokenizer to use for pagination using namespace and
			// ID to ensure complete uniqueness.
			tokenizer := paginator.NewStructsTokenizer(iter,
				paginator.StructsTokenizerOptions{
					WithNamespace: true,
					WithID:        true,
				},
			)

			// Build the paginator. This includes the function that is
			// responsible for appending a variable to the stubs array.
			paginatorImpl, err := paginator.NewPaginator(iter, tokenizer, nil, args.QueryOptions,
				func(raw interface{}) error {
					sv := raw.(*structs.SecureVariable)
					if allowedNSes != nil && !allowedNSes[sv.Namespace] {
						return nil
					}
					svStub := sv.Stub()
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

func (sv *SecureVariables) encrypt(v *structs.SecureVariable) error {
	b, err := json.Marshal(v.UnencryptedData)
	if err != nil {
		return err
	}
	ed := &structs.SecureVariableData{
		Data:  b,
		KeyID: "TODO",
	}
	v.EncryptedData = ed
	v.UnencryptedData = nil
	return nil
}

func (sv *SecureVariables) decrypt(v *structs.SecureVariable) error {
	if v.EncryptedData == nil {
		return fmt.Errorf("secure variable %q.%q not encrypted", v.Namespace, v.Path)
	}

	v.UnencryptedData = make(map[string]string)
	err := json.Unmarshal(v.EncryptedData.Data, &v.UnencryptedData)
	if err != nil {
		return err
	}
	v.EncryptedData = nil
	return nil
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
			if !aclObj.AllowNsOp(args.RequestNamespace(), cap) {
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
	claim, err := sv.srv.ResolveClaim(args.AuthToken)
	if err != nil {
		return structs.ErrPermissionDenied
	}

	alloc, err := sv.authValidateAllocationIdentity(claim.AllocationID, args.RequestNamespace())
	if err != nil {
		metrics.IncrCounter([]string{
			"nomad", "secure_variables", "invalid_allocation_identity"}, 1)
		sv.logger.Trace("allocation identity was not valid", "error", err)
		return structs.ErrPermissionDenied
	}

	err = sv.authValidatePrefix(alloc, claim.TaskName, pathOrPrefix)
	if err != nil {
		sv.logger.Trace("allocation identity did not have permission for path", "error", err)
		return structs.ErrPermissionDenied
	}

	return nil
}

// authValidateAllocationIdentity asserts that the allocation ID
// belongs to a non-terminal Allocation in the requested namespace
func (sv *SecureVariables) authValidateAllocationIdentity(allocID, ns string) (*structs.Allocation, error) {

	store, err := sv.srv.fsm.State().Snapshot()
	if err != nil {
		return nil, err
	}
	alloc, err := store.AllocByID(nil, allocID)
	if err != nil {
		return nil, err
	}
	if alloc == nil || alloc.Job == nil {
		return nil, fmt.Errorf("allocation does not exist")
	}

	// the claims for terminal allocs are always treated as expired
	if alloc.TerminalStatus() {
		return nil, fmt.Errorf("allocation is terminal")
	}

	if alloc.Job.Namespace != ns {
		return nil, fmt.Errorf("allocation is in another namespace")
	}

	return alloc, nil
}

// authValidatePrefix asserts that the requested path is valid for
// this allocation
func (sv *SecureVariables) authValidatePrefix(alloc *structs.Allocation, taskName, pathOrPrefix string) error {

	parts := strings.Split(pathOrPrefix, "/")
	expect := []string{"jobs", alloc.Job.ID, alloc.TaskGroup, taskName}
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
