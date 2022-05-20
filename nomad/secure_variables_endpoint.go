package nomad

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/acl"
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

	// ctx provides context regarding the underlying connection, so we can
	// perform TLS certificate validation on internal only endpoints.
	ctx *RPCContext
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
	aclObj, err := sv.srv.ResolveToken(args.AuthToken)

	switch err {
	case nil:
		// If ACLs are enabled, ensure the caller has the submit-job namespace
		// capability.
		if aclObj != nil {
			hasSubmitJob := aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob)
			if !hasSubmitJob {
				return structs.ErrPermissionDenied
			}
		}
	default:
		// This endpoint is generally called by Nomad nodes, so we want to
		// perform this check, unless the token resolution gave us a terminal
		// error.
		if err != structs.ErrTokenNotFound {
			return err
		}

		// Attempt to lookup AuthToken as a Node.SecretID and return any error
		// wrapped along with the original.
		node, stateErr := sv.srv.fsm.State().NodeBySecretID(nil, args.AuthToken)
		if stateErr != nil {
			var mErr multierror.Error
			mErr.Errors = append(mErr.Errors, err, stateErr)
			return mErr.ErrorOrNil()
		}

		// At this point, we do not have a valid ACL token, nor are we being
		// called, or able to confirm via the state store, by a node.
		if node == nil {
			return structs.ErrTokenNotFound
		}
	}

	// Use a multierror, so we can capture all validation errors and pass this
	// back so fixing in a single swoop.
	var mErr multierror.Error

	// Iterate the secure variables and validate them. Any error results in the
	// call failing.
	for _, i := range args.Data {
		if err := i.Validate(); err != nil {
			mErr.Errors = append(mErr.Errors, err)
		}
		if err := sv.encrypt(i); err != nil {
			mErr.Errors = append(mErr.Errors, err)
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

// Delete removes a singlesecure variable, as specified by its namespace and
// path from Nomad. This is typically called by Nomad nodes, however, in extreme
// situations can be used via the CLI and API by operators.
func (sv *SecureVariables) Delete(
	args *structs.SecureVariablesDeleteRequest,
	reply *structs.SecureVariablesDeleteResponse) error {

	if done, err := sv.srv.forward(structs.SecureVariablesDeleteRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "delete"}, time.Now())

	// Perform the ACL token resolution.
	aclObj, err := sv.srv.ResolveToken(args.AuthToken)

	switch err {
	case nil:
		// If ACLs are enabled, ensure the caller has the submit-job namespace
		// capability.
		if aclObj != nil {
			hasSubmitJob := aclObj.AllowNsOp(args.RequestNamespace(), acl.NamespaceCapabilitySubmitJob)
			if !hasSubmitJob {
				return structs.ErrPermissionDenied
			}
		}
	default:
		// This endpoint is generally called by Nomad nodes, so we want to
		// perform this check, unless the token resolution gave us a terminal
		// error.
		if err != structs.ErrTokenNotFound {
			return err
		}

		// Attempt to lookup AuthToken as a Node.SecretID and return any error
		// wrapped along with the original.
		node, stateErr := sv.srv.fsm.State().NodeBySecretID(nil, args.AuthToken)
		if stateErr != nil {
			var mErr multierror.Error
			mErr.Errors = append(mErr.Errors, err, stateErr)
			return mErr.ErrorOrNil()
		}

		// At this point, we do not have a valid ACL token, nor are we being
		// called, or able to confirm via the state store, by a node.
		if node == nil {
			return structs.ErrTokenNotFound
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

// GetNamespace is used to get a specific namespace
func (sv *SecureVariables) Read(args *structs.SecureVariablesReadRequest, reply *structs.SecureVariablesReadResponse) error {
	if done, err := sv.srv.forward(structs.SecureVariablesReadRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "secure_variables", "read"}, time.Now())

	// Perform our mixed auth handling.
	if err := sv.handleMixedAuthEndpoint(args.QueryOptions, acl.NamespaceCapabilityReadJob); err != nil {
		return err
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, s *state.StateStore) error {
			// Look for the namespace
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
				// Use the last index that affected the secure table
				index, err := s.Index(state.TableSecureVariables)
				if err != nil {
					return err
				}

				// Ensure we never set the index to zero, otherwise a blocking query cannot be used.
				// We floor the index at one, since realistically the first write must have a higher index.
				if index == 0 {
					index = 1
				}
				reply.Index = index
			}
			return nil
		}}
	return sv.srv.blockingRPC(&opts)
}

// handleMixedAuthEndpoint is a helper to handle auth on RPC endpoints that can
// either be called by Nomad nodes, or by external clients.
func (sv *SecureVariables) handleMixedAuthEndpoint(args structs.QueryOptions, cap string) error {

	// Perform the initial token resolution.
	aclObj, err := sv.srv.ResolveToken(args.AuthToken)

	switch err {
	case nil:
		// Perform our ACL validation. If the object is nil, this means ACLs
		// are not enabled, otherwise trigger the allowed namespace function.
		if aclObj != nil {
			if !aclObj.AllowNsOp(args.RequestNamespace(), cap) {
				return structs.ErrPermissionDenied
			}
		}
	default:
		// In the event we got any error other than notfound, consider this
		// terminal.
		if err != structs.ErrTokenNotFound {
			return err
		}

		// Attempt to lookup AuthToken as a Node.SecretID and return any error
		// wrapped along with the original.
		node, stateErr := sv.srv.fsm.State().NodeBySecretID(nil, args.AuthToken)
		if stateErr != nil {
			var mErr multierror.Error
			mErr.Errors = append(mErr.Errors, err, stateErr)
			return mErr.ErrorOrNil()
		}

		// At this point, we do not have a valid ACL token, nor are we being
		// called, or able to confirm via the state store, by a node.
		if node == nil {
			return structs.ErrTokenNotFound
		}
	}

	return nil
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

	// Perform our mixed auth handling.
	if err := sv.handleMixedAuthEndpoint(args.QueryOptions, acl.NamespaceCapabilityReadJob); err != nil {
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
			// responsible for appending a registration to the secure variables
			// array.
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
