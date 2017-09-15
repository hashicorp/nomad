package nomad

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	metrics "github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	// aclDisabled is returned when an ACL endpoint is hit but ACLs are not enabled
	aclDisabled = fmt.Errorf("ACL support disabled")
)

const (
	// aclBootstrapReset is the file name to create in the data dir. It's only contents
	// should be the reset index
	aclBootstrapReset = "acl-bootstrap-reset"
)

// ACL endpoint is used for manipulating ACL tokens and policies
type ACL struct {
	srv *Server
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
	if acl, err := a.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Validate non-zero set of policies
	if len(args.Policies) == 0 {
		return fmt.Errorf("must specify as least one policy")
	}

	// Validate each policy, compute hash
	for idx, policy := range args.Policies {
		if err := policy.Validate(); err != nil {
			return fmt.Errorf("policy %d invalid: %v", idx, err)
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
	if acl, err := a.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Validate non-zero set of policies
	if len(args.Names) == 0 {
		return fmt.Errorf("must specify as least one policy")
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
	if acl, err := a.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
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
				reply.Policies = append(reply.Policies, policy.Stub())
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
	if acl, err := a.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
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
	token, err := a.srv.State().ACLTokenBySecretID(nil, args.SecretID)
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

	// Always ignore the reset index from the arguements
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
			return fmt.Errorf("ACL bootstrap already done (reset index: %d)", resetIdx)
		} else if specifiedIndex != resetIdx {
			return fmt.Errorf("Invalid bootstrap reset index (specified %d, reset index: %d)", specifiedIndex, resetIdx)
		}

		// Setup the reset index to allow bootstrapping again
		args.ResetIndex = resetIdx
	}

	// Create a new global management token, override any parameter
	args.Token = &structs.ACLToken{
		AccessorID: structs.GenerateUUID(),
		SecretID:   structs.GenerateUUID(),
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
		return fmt.Errorf("token lookup failed: %v", err)
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
			a.srv.logger.Printf("[ERR] acl.bootstrap: failed to read %q: %v", path, err)
		}
		return 0
	}

	// Attempt to parse the file
	var resetIdx uint64
	if _, err := fmt.Sscanf(string(raw), "%d", &resetIdx); err != nil {
		a.srv.logger.Printf("[ERR] acl.bootstrap: failed to parse %q: %v", path, err)
		return 0
	}

	// Return the reset index
	a.srv.logger.Printf("[WARN] acl.bootstrap: parsed %q: reset index %d", path, resetIdx)
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
		return fmt.Errorf("must specify as least one token")
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
			return fmt.Errorf("cannot upsert mixed global and non-global tokens")
		}

		// Force the request to the authoritative region if it has global
		args.Region = a.srv.config.AuthoritativeRegion
	}

	if done, err := a.srv.forward("ACL.UpsertTokens", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "upsert_tokens"}, time.Now())

	// Check management level permissions
	if acl, err := a.srv.resolveToken(args.SecretID); err != nil {
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
			return fmt.Errorf("token %d invalid: %v", idx, err)
		}

		// Generate an accessor and secret ID if new
		if token.AccessorID == "" {
			token.AccessorID = structs.GenerateUUID()
			token.SecretID = structs.GenerateUUID()
			token.CreateTime = time.Now().UTC()

		} else {
			// Verify the token exists
			out, err := state.ACLTokenByAccessorID(nil, token.AccessorID)
			if err != nil {
				return fmt.Errorf("token lookup failed: %v", err)
			}
			if out == nil {
				return fmt.Errorf("cannot find token %s", token.AccessorID)
			}

			// Cannot toggle the "Global" mode
			if token.Global != out.Global {
				return fmt.Errorf("cannot toggle global mode of %s", token.AccessorID)
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
			return fmt.Errorf("token lookup failed: %v", err)
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
		return fmt.Errorf("must specify as least one token")
	}

	if done, err := a.srv.forward("ACL.DeleteTokens", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "delete_tokens"}, time.Now())

	// Check management level permissions
	if acl, err := a.srv.resolveToken(args.SecretID); err != nil {
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
	for _, accessor := range args.AccessorIDs {
		token, err := state.ACLTokenByAccessorID(nil, accessor)
		if err != nil {
			return fmt.Errorf("token lookup failed: %v", err)
		}
		if token == nil {
			continue
		}
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
			return fmt.Errorf("cannot delete mixed global and non-global tokens")
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
	if acl, err := a.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
		return structs.ErrPermissionDenied
	}

	// Setup the blocking query
	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, state *state.StateStore) error {
			// Iterate over all the tokens
			var err error
			var iter memdb.ResultIterator
			if prefix := args.QueryOptions.Prefix; prefix != "" {
				iter, err = state.ACLTokenByAccessorIDPrefix(ws, prefix)
			} else if args.GlobalOnly {
				iter, err = state.ACLTokensByGlobal(ws, true)
			} else {
				iter, err = state.ACLTokens(ws)
			}
			if err != nil {
				return err
			}

			// Convert all the tokens to a list stub
			reply.Tokens = nil
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				token := raw.(*structs.ACLToken)
				reply.Tokens = append(reply.Tokens, token.Stub())
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

// GetToken is used to get a specific token
func (a *ACL) GetToken(args *structs.ACLTokenSpecificRequest, reply *structs.SingleACLTokenResponse) error {
	if !a.srv.config.ACLEnabled {
		return aclDisabled
	}
	if done, err := a.srv.forward("ACL.GetToken", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "get_token"}, time.Now())

	// Check management level permissions
	if acl, err := a.srv.resolveToken(args.SecretID); err != nil {
		return err
	} else if acl == nil || !acl.IsManagement() {
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
	if acl, err := a.srv.resolveToken(args.SecretID); err != nil {
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
