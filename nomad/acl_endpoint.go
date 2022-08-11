package nomad

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	policy "github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/state/paginator"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// aclDisabled is returned when an ACL endpoint is hit but ACLs are not enabled
	aclDisabled = structs.NewErrRPCCoded(400, "ACL support disabled")
)

const (
	// aclBootstrapReset is the file name to create in the data dir. It's only contents
	// should be the reset index
	aclBootstrapReset = "acl-bootstrap-reset"
)

// ACL endpoint is used for manipulating ACL tokens and policies
type ACL struct {
	srv    *Server
	logger log.Logger
}

// UpsertPolicies is used to create or update a set of policies
func (a *ACL) UpsertPolicies(args *structs.ACLPolicyUpsertRequest, reply *structs.GenericResponse) error {
	// Ensure ACLs are enabled, and always flow modification requests to the authoritative region
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}
	args.Region = a.srv.config.AuthoritativeRegion

	if done, err := a.srv.forward("ACL.UpsertPolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "upsert_policies"}, time.Now())

	// Check management level permissions
	if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Validate non-zero set of policies
	if len(args.Policies) == 0 {
		return structs.NewErrRPCCoded(400, "must specify as least one policy")
	}

	// Validate each policy, compute hash
	for idx, policy := range args.Policies {
		if err := policy.Validate(); err != nil {
			return structs.NewErrRPCCodedf(404, "policy %d invalid: %v", idx, err)
		}
		policy.SetHash()
	}

	// Update via Raft
	_, index, err := a.srv.raftApply(structs.ACLPolicyUpsertRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// DeletePolicies is used to delete policies
func (a *ACL) DeletePolicies(args *structs.ACLPolicyDeleteRequest, reply *structs.GenericResponse) error {
	// Ensure ACLs are enabled, and always flow modification requests to the authoritative region
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}
	args.Region = a.srv.config.AuthoritativeRegion

	if done, err := a.srv.forward("ACL.DeletePolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "delete_policies"}, time.Now())

	// Check management level permissions
	if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Validate non-zero set of policies
	if len(args.Names) == 0 {
		return structs.NewErrRPCCoded(400, "must specify as least one policy")
	}

	// Update via Raft
	_, index, err := a.srv.raftApply(structs.ACLPolicyDeleteRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// ListPolicies is used to list the policies
func (a *ACL) ListPolicies(args *structs.ACLPolicyListRequest, reply *structs.ACLPolicyListResponse) error {
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}

	if done, err := a.srv.forward("ACL.ListPolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "list_policies"}, time.Now())

	// Check management level permissions
	acl, err := a.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	} else if acl == nil {
		return structs.ErrPermissionDenied
	}

	// If it is not a management token determine the policies that may be listed
	mgt := acl.IsManagement()
	var policies map[string]struct{}
	if !mgt {
		token, err := a.requestACLToken(args.AuthToken)
		if err != nil {
			return err
		}
		if token == nil {
			return structs.ErrTokenNotFound
		}

		policies = make(map[string]struct{}, len(token.Policies))
		for _, p := range token.Policies {
			policies[p] = struct{}{}
		}
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Iterate over all the policies
			var err error
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = state.ACLPolicyByNamePrefix(ws, prefix)
			} else {
				iter, err = state.ACLPolicies(ws)
			}
			if err != nil {
				return err
			}

			// Convert all the policies to a list stub
			reply.Policies = nil
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				policy := raw.(*structs.ACLPolicy)
				if _, ok := policies[policy.Name]; ok || mgt {
					reply.Policies = append(reply.Policies, policy.Stub())
				}
			}

			// Use the last index that affected the policy table
			index, err := state.Index("acl_policy")
			if err != nil {
				return err
			}

			// Ensure we never set the index to zero, otherwise a blocking query cannot be used.
			// We floor the index at one, since realistically the first write must have a higher index.
			if index == 0 {
				index = 1
			}
			reply.Index = index
			return nil
		}}
	return a.srv.blockingRPC(&opts)
}

// GetPolicy is used to get a specific policy
func (a *ACL) GetPolicy(args *structs.ACLPolicySpecificRequest, reply *structs.SingleACLPolicyResponse) error {
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}

	if done, err := a.srv.forward("ACL.GetPolicy", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "get_policy"}, time.Now())

	// Check management level permissions
	acl, err := a.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	} else if acl == nil {
		return structs.ErrPermissionDenied
	}

	// If the policy is the anonymous one, anyone can get it
	// If it is not a management token determine if it can get this policy
	mgt := acl.IsManagement()
	if !mgt && args.Name != "anonymous" {
		token, err := a.requestACLToken(args.AuthToken)
		if err != nil {
			return err
		}
		if token == nil {
			return structs.ErrTokenNotFound
		}

		found := false
		for _, p := range token.Policies {
			if p == args.Name {
				found = true
				break
			}
		}

		if !found {
			return structs.ErrPermissionDenied
		}
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for the policy
			out, err := state.ACLPolicyByName(ws, args.Name)
			if err != nil {
				return err
			}

			// Setup the output
			reply.Policy = out
			if out != nil {
				reply.Index = out.ModifyIndex
				rules, err := policy.Parse(out.Rules)

				if err != nil {
					return err
				}
				reply.Policy.RulesJSON = rules
			} else {
				// Use the last index that affected the policy table
				index, err := state.Index("acl_policy")
				if err != nil {
					return err
				}
				reply.Index = index
			}
			return nil
		}}
	return a.srv.blockingRPC(&opts)
}

func (a *ACL) requestACLToken(secretID string) (*structs.ACLToken, error) {
	if secretID == "" {
		return structs.AnonymousACLToken, nil
	}

	snap, err := a.srv.fsm.State().Snapshot()
	if err != nil {
		return nil, err
	}

	return snap.ACLTokenBySecretID(nil, secretID)
}

// GetPolicies is used to get a set of policies
func (a *ACL) GetPolicies(args *structs.ACLPolicySetRequest, reply *structs.ACLPolicySetResponse) error {
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}
	if done, err := a.srv.forward("ACL.GetPolicies", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "get_policies"}, time.Now())

	// For client typed tokens, allow them to query any policies associated with that token.
	// This is used by clients which are resolving the policies to enforce. Any associated
	// policies need to be fetched so that the client can determine what to allow.
	token, err := a.requestACLToken(args.AuthToken)
	if err != nil {
		return err
	}

	if token == nil {
		return structs.ErrTokenNotFound
	}
	if token.Type != structs.ACLManagementToken && !token.PolicySubset(args.Names) {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Setup the output
			reply.Policies = make(map[string]*structs.ACLPolicy, len(args.Names))

			// Look for the policy
			for _, policyName := range args.Names {
				out, err := state.ACLPolicyByName(ws, policyName)
				if err != nil {
					return err
				}
				if out != nil {
					reply.Policies[policyName] = out
				}
			}

			// Use the last index that affected the policy table
			index, err := state.Index("acl_policy")
			if err != nil {
				return err
			}
			reply.Index = index
			return nil
		}}
	return a.srv.blockingRPC(&opts)
}

// Bootstrap is used to bootstrap the initial token
func (a *ACL) Bootstrap(args *structs.ACLTokenBootstrapRequest, reply *structs.ACLTokenUpsertResponse) error {
	// Ensure ACLs are enabled, and always flow modification requests to the authoritative region
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}
	args.Region = a.srv.config.AuthoritativeRegion
	providedTokenID := args.BootstrapSecret

	if done, err := a.srv.forward("ACL.Bootstrap", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "bootstrap"}, time.Now())

	// Always ignore the reset index from the arguments
	args.ResetIndex = 0

	// Snapshot the state
	state, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	// Verify bootstrap is possible. The state store method re-verifies this,
	// but we do an early check to avoid raft transactions when possible.
	ok, resetIdx, err := state.CanBootstrapACLToken()
	if err != nil {
		return err
	}
	if !ok {
		// Check if there is a reset index specified
		specifiedIndex := a.fileBootstrapResetIndex()
		if specifiedIndex == 0 {
			return structs.NewErrRPCCodedf(400, "ACL bootstrap already done (reset index: %d)", resetIdx)
		} else if specifiedIndex != resetIdx {
			return structs.NewErrRPCCodedf(400, "Invalid bootstrap reset index (specified %d, reset index: %d)", specifiedIndex, resetIdx)
		}

		// Setup the reset index to allow bootstrapping again
		args.ResetIndex = resetIdx
	}

	// Create a new global management token, override any parameter
	args.Token = &structs.ACLToken{
		AccessorID: uuid.Generate(),
		SecretID:   uuid.Generate(),
		Name:       "Bootstrap Token",
		Type:       structs.ACLManagementToken,
		Global:     true,
		CreateTime: time.Now().UTC(),
	}

	// if a token has been passed in from the API overwrite the generated one.
	if providedTokenID != "" {
		if helper.IsUUID(providedTokenID) {
			args.Token.SecretID = providedTokenID
		} else {
			return structs.NewErrRPCCodedf(400, "invalid acl token")
		}
	}

	args.Token.SetHash()

	// Update via Raft
	_, index, err := a.srv.raftApply(structs.ACLTokenBootstrapRequestType, args)
	if err != nil {
		return err
	}

	// Populate the response. We do a lookup against the state to
	// pickup the proper create / modify times.
	state, err = a.srv.State().Snapshot()
	if err != nil {
		return err
	}
	out, err := state.ACLTokenByAccessorID(nil, args.Token.AccessorID)
	if err != nil {
		return structs.NewErrRPCCodedf(400, "token lookup failed: %v", err)
	}
	reply.Tokens = append(reply.Tokens, out)

	// Update the index
	reply.Index = index
	return nil
}

// fileBootstrapResetIndex is used to read the reset file from <data-dir>/acl-bootstrap-reset
func (a *ACL) fileBootstrapResetIndex() uint64 {
	// Determine the file path to check
	path := filepath.Join(a.srv.config.DataDir, aclBootstrapReset)

	// Read the file
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			a.logger.Error("failed to read bootstrap file", "path", path, "error", err)
		}
		return 0
	}

	// Attempt to parse the file
	var resetIdx uint64
	if _, err := fmt.Sscanf(string(raw), "%d", &resetIdx); err != nil {
		a.logger.Error("failed to parse bootstrap file", "path", path, "error", err)
		return 0
	}

	// Return the reset index
	a.logger.Warn("bootstrap file parsed", "path", path, "reset_index", resetIdx)
	return resetIdx
}

// UpsertTokens is used to create or update a set of tokens
func (a *ACL) UpsertTokens(args *structs.ACLTokenUpsertRequest, reply *structs.ACLTokenUpsertResponse) error {
	// Ensure ACLs are enabled, and always flow modification requests to the authoritative region
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}

	// Validate non-zero set of tokens
	if len(args.Tokens) == 0 {
		return structs.NewErrRPCCoded(http.StatusBadRequest, "must specify as least one token")
	}

	// Force the request to the authoritative region if we are creating global tokens
	hasGlobal := false
	allGlobal := true
	for _, token := range args.Tokens {
		if token.Global {
			hasGlobal = true
		} else {
			allGlobal = false
		}
	}

	// Disallow mixed requests with global and non-global tokens since we forward
	// the entire request as a single batch.
	if hasGlobal {
		if !allGlobal {
			return structs.NewErrRPCCoded(http.StatusBadRequest,
				"cannot upsert mixed global and non-global tokens")
		}

		// Force the request to the authoritative region if it has global
		args.Region = a.srv.config.AuthoritativeRegion
	}

	if done, err := a.srv.forward(structs.ACLUpsertTokensRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "upsert_tokens"}, time.Now())

	// Check management level permissions
	if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Snapshot the state so we can perform lookups against the accessor ID if
	// needed. Do it here, so we only need to do this once no matter how many
	// tokens we are upserting.
	stateSnapshot, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	// Validate each token
	for idx, token := range args.Tokens {

		// Store any existing token found, so we can perform the correct update
		// validation.
		var existingToken *structs.ACLToken

		// If the token is being updated, perform a lookup so can can validate
		// the new changes against the old.
		if token.AccessorID != "" {
			out, err := stateSnapshot.ACLTokenByAccessorID(nil, token.AccessorID)
			if err != nil {
				return structs.NewErrRPCCodedf(http.StatusInternalServerError, "token lookup failed: %v", err)
			}
			if out == nil {
				return structs.NewErrRPCCodedf(http.StatusBadRequest, "cannot find token %s", token.AccessorID)
			}
			existingToken = out
		}

		// Canonicalize sets information needed by the validation function, so
		// this order must be maintained.
		token.Canonicalize()

		if err := token.Validate(a.srv.config.ACLTokenMinExpirationTTL,
			a.srv.config.ACLTokenMaxExpirationTTL, existingToken); err != nil {
			return structs.NewErrRPCCodedf(http.StatusBadRequest, "token %d invalid: %v", idx, err)
		}

		// Compute the token hash
		token.SetHash()
	}

	// Update via Raft
	_, index, err := a.srv.raftApply(structs.ACLTokenUpsertRequestType, args)
	if err != nil {
		return err
	}

	// Populate the response. We do a lookup against the state to pick up the
	// proper create / modify times.
	stateSnapshot, err = a.srv.State().Snapshot()
	if err != nil {
		return err
	}
	for _, token := range args.Tokens {
		out, err := stateSnapshot.ACLTokenByAccessorID(nil, token.AccessorID)
		if err != nil {
			return structs.NewErrRPCCodedf(400, "token lookup failed: %v", err)
		}
		reply.Tokens = append(reply.Tokens, out)
	}

	// Update the index
	reply.Index = index
	return nil
}

// DeleteTokens is used to delete tokens
func (a *ACL) DeleteTokens(args *structs.ACLTokenDeleteRequest, reply *structs.GenericResponse) error {
	// Ensure ACLs are enabled, and always flow modification requests to the authoritative region
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}

	// Validate non-zero set of tokens
	if len(args.AccessorIDs) == 0 {
		return structs.NewErrRPCCoded(400, "must specify as least one token")
	}

	if done, err := a.srv.forward("ACL.DeleteTokens", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "delete_tokens"}, time.Now())

	// Check management level permissions
	if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Snapshot the state
	state, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	// Determine if we are deleting local or global tokens
	hasGlobal := false
	allGlobal := true
	nonexistentTokens := make([]string, 0)
	for _, accessor := range args.AccessorIDs {
		token, err := state.ACLTokenByAccessorID(nil, accessor)
		if err != nil {
			return structs.NewErrRPCCodedf(400, "token lookup failed: %v", err)
		}
		if token == nil {
			nonexistentTokens = append(nonexistentTokens, accessor)
			continue
		}
		if token.Global {
			hasGlobal = true
		} else {
			allGlobal = false
		}
	}

	if len(nonexistentTokens) != 0 {
		return structs.NewErrRPCCodedf(400, "Cannot delete nonexistent tokens: %v", strings.Join(nonexistentTokens, ", "))
	}

	// Disallow mixed requests with global and non-global tokens since we forward
	// the entire request as a single batch.
	if hasGlobal {
		if !allGlobal {
			return structs.NewErrRPCCoded(400, "cannot delete mixed global and non-global tokens")
		}

		// Force the request to the authoritative region if it has global
		if a.srv.config.Region != a.srv.config.AuthoritativeRegion {
			args.Region = a.srv.config.AuthoritativeRegion
			_, err := a.srv.forward("ACL.DeleteTokens", args, args, reply)
			return err
		}
	}

	// Update via Raft
	_, index, err := a.srv.raftApply(structs.ACLTokenDeleteRequestType, args)
	if err != nil {
		return err
	}

	// Update the index
	reply.Index = index
	return nil
}

// ListTokens is used to list the tokens
func (a *ACL) ListTokens(args *structs.ACLTokenListRequest, reply *structs.ACLTokenListResponse) error {
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}
	if done, err := a.srv.forward("ACL.ListTokens", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "list_tokens"}, time.Now())

	// Check management level permissions
	if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	sort := state.SortOption(args.Reverse)
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Iterate over all the tokens
			var err error
			var iter memdb.ResultIterator
			var opts paginator.StructsTokenizerOptions

			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = state.ACLTokenByAccessorIDPrefix(ws, prefix, sort)
				opts = paginator.StructsTokenizerOptions{
					WithID: true,
				}
			} else if args.GlobalOnly {
				iter, err = state.ACLTokensByGlobal(ws, true, sort)
				opts = paginator.StructsTokenizerOptions{
					WithID: true,
				}
			} else {
				iter, err = state.ACLTokens(ws, sort)
				opts = paginator.StructsTokenizerOptions{
					WithCreateIndex: true,
					WithID:          true,
				}
			}
			if err != nil {
				return err
			}

			tokenizer := paginator.NewStructsTokenizer(iter, opts)

			var tokens []*structs.ACLTokenListStub
			paginator, err := paginator.NewPaginator(iter, tokenizer, nil, args.QueryOptions,
				func(raw interface{}) error {
					token := raw.(*structs.ACLToken)
					tokens = append(tokens, token.Stub())
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
			reply.Tokens = tokens

			// Use the last index that affected the token table
			index, err := state.Index("acl_token")
			if err != nil {
				return err
			}
			reply.Index = index

			return nil
		}}
	return a.srv.blockingRPC(&opts)
}

// GetToken is used to get a specific token
func (a *ACL) GetToken(args *structs.ACLTokenSpecificRequest, reply *structs.SingleACLTokenResponse) error {
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}
	if done, err := a.srv.forward("ACL.GetToken", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "get_token"}, time.Now())

	acl, err := a.srv.ResolveToken(args.AuthToken)
	if err != nil {
		return err
	}

	// Ensure ACLs are enabled and this call is made with one
	if acl == nil {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Look for the token
			out, err := state.ACLTokenByAccessorID(ws, args.AccessorID)
			if err != nil {
				return err
			}

			if out == nil {
				// If the token doesn't resolve, only allow management tokens to
				// block.
				if !acl.IsManagement() {
					return structs.ErrPermissionDenied
				}

				// Check management level permissions or that the secret ID matches the
				// accessor ID
			} else if !acl.IsManagement() && out.SecretID != args.AuthToken {
				return structs.ErrPermissionDenied
			}

			// Setup the output
			reply.Token = out
			if out != nil {
				reply.Index = out.ModifyIndex
			} else {
				// Use the last index that affected the token table
				index, err := state.Index("acl_token")
				if err != nil {
					return err
				}
				reply.Index = index
			}
			return nil
		}}
	return a.srv.blockingRPC(&opts)
}

// GetTokens is used to get a set of token
func (a *ACL) GetTokens(args *structs.ACLTokenSetRequest, reply *structs.ACLTokenSetResponse) error {
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}
	if done, err := a.srv.forward("ACL.GetTokens", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "get_tokens"}, time.Now())

	// Check management level permissions
	if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Setup the output
			reply.Tokens = make(map[string]*structs.ACLToken, len(args.AccessorIDS))

			// Look for the token
			for _, accessor := range args.AccessorIDS {
				out, err := state.ACLTokenByAccessorID(ws, accessor)
				if err != nil {
					return err
				}
				if out != nil {
					reply.Tokens[out.AccessorID] = out
				}
			}

			// Use the last index that affected the token table
			index, err := state.Index("acl_token")
			if err != nil {
				return err
			}
			reply.Index = index
			return nil
		}}
	return a.srv.blockingRPC(&opts)
}

// ResolveToken is used to lookup a specific token by a secret ID. This is used for enforcing ACLs by clients.
func (a *ACL) ResolveToken(args *structs.ResolveACLTokenRequest, reply *structs.ResolveACLTokenResponse) error {
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}
	if done, err := a.srv.forward("ACL.ResolveToken", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "resolve_token"}, time.Now())

	// Setup the query meta
	a.srv.setQueryMeta(&reply.QueryMeta)

	// Snapshot the state
	state, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	// Look for the token
	out, err := state.ACLTokenBySecretID(nil, args.SecretID)
	if err != nil {
		return err
	}

	// Setup the output
	reply.Token = out
	if out != nil {
		reply.Index = out.ModifyIndex
	} else {
		// Use the last index that affected the token table
		index, err := state.Index("acl_token")
		if err != nil {
			return err
		}
		reply.Index = index
	}
	return nil
}

func (a *ACL) UpsertOneTimeToken(args *structs.OneTimeTokenUpsertRequest, reply *structs.OneTimeTokenUpsertResponse) error {
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}
	if done, err := a.srv.forward(
		"ACL.UpsertOneTimeToken", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince(
		[]string{"nomad", "acl", "upsert_one_time_token"}, time.Now())

	if !ServersMeetMinimumVersion(a.srv.Members(), minOneTimeAuthenticationTokenVersion, false) {
		return fmt.Errorf("All servers should be running version %v or later to use one-time authentication tokens", minAutopilotVersion)
	}

	// Snapshot the state
	state, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	// Look up the token; there's no capability check as you can only
	// request a OTT for your own ACL token
	aclToken, err := state.ACLTokenBySecretID(nil, args.AuthToken)
	if err != nil {
		return err
	}
	if aclToken == nil {
		return structs.ErrPermissionDenied
	}

	ott := &structs.OneTimeToken{
		OneTimeSecretID: uuid.Generate(),
		AccessorID:      aclToken.AccessorID,
		ExpiresAt:       time.Now().Add(10 * time.Minute),
	}

	// Update via Raft
	_, index, err := a.srv.raftApply(structs.OneTimeTokenUpsertRequestType, ott)
	if err != nil {
		return err
	}

	ott.ModifyIndex = index
	ott.CreateIndex = index
	reply.OneTimeToken = ott
	reply.Index = index
	return nil
}

// ExchangeOneTimeToken provides a one-time token's secret ID to exchange it
// for the ACL token that created that one-time token
func (a *ACL) ExchangeOneTimeToken(args *structs.OneTimeTokenExchangeRequest, reply *structs.OneTimeTokenExchangeResponse) error {
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}
	if done, err := a.srv.forward(
		"ACL.ExchangeOneTimeToken", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince(
		[]string{"nomad", "acl", "exchange_one_time_token"}, time.Now())

	if !ServersMeetMinimumVersion(a.srv.Members(), minOneTimeAuthenticationTokenVersion, false) {
		return fmt.Errorf("All servers should be running version %v or later to use one-time authentication tokens", minAutopilotVersion)
	}

	// Snapshot the state
	state, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	ott, err := state.OneTimeTokenBySecret(nil, args.OneTimeSecretID)
	if err != nil {
		return err
	}
	if ott == nil {
		return structs.ErrPermissionDenied
	}
	if ott.ExpiresAt.Before(time.Now()) {
		// we return early and leave cleaning up the expired token for GC
		return structs.ErrPermissionDenied
	}

	// Look for the token; it may have been deleted, in which case, 403
	aclToken, err := state.ACLTokenByAccessorID(nil, ott.AccessorID)
	if err != nil {
		return err
	}
	if aclToken == nil {
		return structs.ErrPermissionDenied
	}

	// Expire token via raft; because this is the only write in the RPC the
	// caller can safely retry with the same token if the raft write fails
	_, index, err := a.srv.raftApply(structs.OneTimeTokenDeleteRequestType,
		&structs.OneTimeTokenDeleteRequest{
			AccessorIDs: []string{ott.AccessorID},
		})
	if err != nil {
		return err
	}

	reply.Token = aclToken
	reply.Index = index
	return nil
}

// ExpireOneTimeTokens removes all expired tokens from the state store. It is
// called only by garbage collection
func (a *ACL) ExpireOneTimeTokens(args *structs.OneTimeTokenExpireRequest, reply *structs.GenericResponse) error {

	if done, err := a.srv.forward(
		"ACL.ExpireOneTimeTokens", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince(
		[]string{"nomad", "acl", "expire_one_time_tokens"}, time.Now())

	if !ServersMeetMinimumVersion(a.srv.Members(), minOneTimeAuthenticationTokenVersion, false) {
		return fmt.Errorf("All servers should be running version %v or later to use one-time authentication tokens", minAutopilotVersion)
	}

	// Check management level permissions
	if a.srv.config.ACLEnabled {
		if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
			return err
		} else if acl == nil || !acl.IsManagement() {
			return structs.ErrPermissionDenied
		}
	}

	args.Timestamp = time.Now() // use the leader's timestamp

	// Expire token via raft; because this is the only write in the RPC the
	// caller can safely retry with the same token if the raft write fails
	_, index, err := a.srv.raftApply(structs.OneTimeTokenExpireRequestType, args)
	if err != nil {
		return err
	}
	reply.Index = index
	return nil
}

// UpsertRoles creates or updates ACL roles held within Nomad.
func (a *ACL) UpsertRoles(
	args *structs.ACLRolesUpsertRequest,
	reply *structs.ACLRolesUpsertResponse) error {

	// Only allow operators to upsert ACL roles when ACLs are enabled.
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}

	// This endpoint always forwards to the authoritative region as ACL roles
	// are global.
	args.Region = a.srv.config.AuthoritativeRegion

	if done, err := a.srv.forward(structs.ACLUpsertRolesRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "upsert_roles"}, time.Now())

	// Only tokens with management level permissions can create ACL roles.
	if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Snapshot the state so we can perform lookups against the ID and policy
	// links if needed. Do it here, so we only need to do this once no matter
	// how many roles we are upserting.
	stateSnapshot, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}

	// Validate each role.
	for idx, role := range args.ACLRoles {

		// Perform all the static validation of the ACL role object. Use the
		// array index as we cannot be sure the error was caused by a missing
		// name.
		if err := role.Validate(); err != nil {
			return structs.NewErrRPCCodedf(http.StatusBadRequest, "role %d invalid: %v", idx, err)
		}

		policyNames := make(map[string]struct{})
		var policiesLinks []*structs.ACLRolePolicyLink

		// We need to deduplicate the ACL policy links within this role as well
		// as ensure the policies exist within state.
		for _, policyLink := range role.Policies {

			// Perform a state look up for the policy. An error or not being
			// able to find the policy is terminal. We can include the name in
			// the error message as it has previously been validated.
			existing, err := stateSnapshot.ACLPolicyByName(nil, policyLink.Name)
			if err != nil {
				return structs.NewErrRPCCodedf(http.StatusInternalServerError, "policy lookup failed: %v", err)
			}
			if existing == nil {
				return structs.NewErrRPCCodedf(http.StatusBadRequest, "cannot find policy %s", policyLink.Name)
			}

			// If the policy name is not found within our map, this means we
			// have not seen it previously. We need to add this to our
			// deduplicated array and also mark the policy name as seen, so we
			// skip any future policies of the same name.
			if _, ok := policyNames[policyLink.Name]; !ok {
				policiesLinks = append(policiesLinks, policyLink)
				policyNames[policyLink.Name] = struct{}{}
			}
		}

		// Stored the potentially updated policy links within our role.
		role.Policies = policiesLinks

		// If the caller has passed a role ID, this call is considered an
		// update to an existing role. We should therefore ensure it is found
		// within state.
		if role.ID != "" {
			out, err := stateSnapshot.GetACLRoleByID(nil, role.ID)
			if err != nil {
				return structs.NewErrRPCCodedf(http.StatusBadRequest, "role lookup failed: %v", err)
			}
			if out == nil {
				return structs.NewErrRPCCodedf(http.StatusBadRequest, "cannot find role %s", role.ID)
			}
		}

		role.Canonicalize()
		role.SetHash()
	}

	// Update via Raft.
	out, index, err := a.srv.raftApply(structs.ACLRolesUpsertRequestType, args)
	if err != nil {
		return err
	}

	// Check if the FSM response, which is an interface, contains an error.
	if err, ok := out.(error); ok && err != nil {
		return err
	}

	// Populate the response. We do a lookup against the state to pick up the
	// proper create / modify times.
	stateSnapshot, err = a.srv.State().Snapshot()
	if err != nil {
		return err
	}
	for _, role := range args.ACLRoles {
		lookupACLRole, err := stateSnapshot.GetACLRoleByName(nil, role.Name)
		if err != nil {
			return structs.NewErrRPCCodedf(400, "ACL role lookup failed: %v", err)
		}
		reply.ACLRoles = append(reply.ACLRoles, lookupACLRole)
	}

	// Update the index. There is no need to floor this as we are writing to
	// state and therefore will get a non-zero index response.
	reply.Index = index
	return nil
}

// DeleteRolesByID is used to batch delete ACL roles using the ID as the
// deletion key.
func (a *ACL) DeleteRolesByID(
	args *structs.ACLRolesDeleteByIDRequest,
	reply *structs.ACLRolesDeleteByIDResponse) error {

	// Only allow operators to delete ACL roles when ACLs are enabled.
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}

	// This endpoint always forwards to the authoritative region as ACL roles
	// are global.
	args.Region = a.srv.config.AuthoritativeRegion

	if done, err := a.srv.forward(structs.ACLDeleteRolesByIDRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "delete_roles"}, time.Now())

	// Only tokens with management level permissions can create ACL roles.
	if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Update via Raft.
	out, index, err := a.srv.raftApply(structs.ACLRolesDeleteByIDRequestType, args)
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

// ListRoles is used to list ACL roles within state. If not prefix is supplied,
// all ACL roles are listed, otherwise a prefix search is performed on the ACL
// role name.
func (a *ACL) ListRoles(
	args *structs.ACLRolesListRequest,
	reply *structs.ACLRolesListResponse) error {

	// Only allow operators to list ACL roles when ACLs are enabled.
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}

	if done, err := a.srv.forward(structs.ACLListRolesRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "list_roles"}, time.Now())

	// TODO (jrasell) allow callers to list role associated to their token.
	if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Set up and return the blocking query.
	return a.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			var (
				err  error
				iter memdb.ResultIterator
			)

			// If the operator supplied a prefix, perform a prefix search.
			// Otherwise, list all ACL roles in state.
			switch args.QueryOptions.Prefix {
			case "":
				iter, err = stateStore.GetACLRoles(ws)
			default:
				iter, err = stateStore.GetACLRoleByIDPrefix(ws, args.QueryOptions.Prefix)
			}
			if err != nil {
				return err
			}

			// Iterate all the results and add these to our reply object. There
			// is no stub object for an ACL role and the hash is needed by the
			// replication process.
			for raw := iter.Next(); raw != nil; raw = iter.Next() {
				reply.ACLRoles = append(reply.ACLRoles, raw.(*structs.ACLRole))
			}

			// Use the index table to populate the query meta as we have no way
			// of tracking the max index on deletes.
			return a.srv.setReplyQueryMeta(stateStore, state.TableACLRoles, &reply.QueryMeta)
		},
	})
}

// GetRoleByID is used to look up an individual ACL role using its ID.
func (a *ACL) GetRoleByID(
	args *structs.ACLRoleByIDRequest,
	reply *structs.ACLRoleByIDResponse) error {

	// Only allow operators to read an ACL role when ACLs are enabled.
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}

	if done, err := a.srv.forward(structs.ACLGetRoleByIDRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "get_role_id"}, time.Now())

	// TODO (jrasell) allow callers to detail a role associated to their token.
	if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Set up and return the blocking query.
	return a.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			// Perform a lookup for the ACL role.
			out, err := stateStore.GetACLRoleByID(ws, args.RoleID)
			if err != nil {
				return err
			}

			// Set the index correctly depending on whether the ACL role was
			// found.
			switch out {
			case nil:
				index, err := stateStore.Index(state.TableACLRoles)
				if err != nil {
					return err
				}
				reply.Index = index
			default:
				reply.Index = out.ModifyIndex
			}

			// We didn't encounter an error looking up the index; set the ACL
			// role on the reply and exit successfully.
			reply.ACLRole = out
			return nil
		},
	})
}

// GetRoleByName is used to look up an individual ACL role using its name.
func (a *ACL) GetRoleByName(
	args *structs.ACLRoleByNameRequest,
	reply *structs.ACLRoleByNameResponse) error {

	// Only allow operators to read an ACL role when ACLs are enabled.
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}

	if done, err := a.srv.forward(structs.ACLGetRoleByNameRPCMethod, args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "get_role_name"}, time.Now())

	// TODO (jrasell) allow callers to detail a role associated to their token.
	if acl, err := a.srv.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Set up and return the blocking query.
	return a.srv.blockingRPC(&blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, stateStore *state.StateStore) error {

			// Perform a lookup for the ACL role.
			out, err := stateStore.GetACLRoleByName(ws, args.RoleName)
			if err != nil {
				return err
			}

			// Set the index correctly depending on whether the ACL role was
			// found.
			switch out {
			case nil:
				index, err := stateStore.Index(state.TableACLRoles)
				if err != nil {
					return err
				}
				reply.Index = index
			default:
				reply.Index = out.ModifyIndex
			}

			// We didn't encounter an error looking up the index; set the ACL
			// role on the reply and exit successfully.
			reply.ACLRole = out
			return nil
		},
	})
}
