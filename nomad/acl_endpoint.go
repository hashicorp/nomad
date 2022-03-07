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
		return structs.NewErrRPCCoded(400, "must specify as least one token")
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
			return structs.NewErrRPCCoded(400, "cannot upsert mixed global and non-global tokens")
		}

		// Force the request to the authoritative region if it has global
		args.Region = a.srv.config.AuthoritativeRegion
	}

	if done, err := a.srv.forward("ACL.UpsertTokens", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "upsert_tokens"}, time.Now())

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

	// Validate each token
	for idx, token := range args.Tokens {
		if err := token.Validate(); err != nil {
			return structs.NewErrRPCCodedf(400, "token %d invalid: %v", idx, err)
		}

		// Generate an accessor and secret ID if new
		if token.AccessorID == "" {
			token.AccessorID = uuid.Generate()
			token.SecretID = uuid.Generate()
			token.CreateTime = time.Now().UTC()

		} else {
			// Verify the token exists
			out, err := state.ACLTokenByAccessorID(nil, token.AccessorID)
			if err != nil {
				return structs.NewErrRPCCodedf(400, "token lookup failed: %v", err)
			}
			if out == nil {
				return structs.NewErrRPCCodedf(404, "cannot find token %s", token.AccessorID)
			}

			// Cannot toggle the "Global" mode
			if token.Global != out.Global {
				return structs.NewErrRPCCodedf(400, "cannot toggle global mode of %s", token.AccessorID)
			}
		}

		// Compute the token hash
		token.SetHash()
	}

	// Update via Raft
	_, index, err := a.srv.raftApply(structs.ACLTokenUpsertRequestType, args)
	if err != nil {
		return err
	}

	// Populate the response. We do a lookup against the state to
	// pickup the proper create / modify times.
	state, err = a.srv.State().Snapshot()
	if err != nil {
		return err
	}
	for _, token := range args.Tokens {
		out, err := state.ACLTokenByAccessorID(nil, token.AccessorID)
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
				iter, err = state.ACLTokenByAccessorIDPrefix(ws, prefix)
				opts = paginator.StructsTokenizerOptions{
					WithID: true,
				}
			} else if args.GlobalOnly {
				iter, err = state.ACLTokensByGlobal(ws, true)
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

	// Expire token via raft; because this is the only write in the RPC the
	// caller can safely retry with the same token if the raft write fails
	_, index, err := a.srv.raftApply(structs.OneTimeTokenExpireRequestType, args)
	if err != nil {
		return err
	}
	reply.Index = index
	return nil
}
