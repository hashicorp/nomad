package nomad

import (
	"errors"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/sentinel/sentinel"
)

// sentinelPoliciesByScope returns all the applicable policies by scope
func (s *Server) sentinelPoliciesByScope(scope string) ([]*structs.SentinelPolicy, error) {
	// Snapshot the current state
	snap, err := s.State().Snapshot()
	if err != nil {
		return nil, err
	}

	// Gather the applicable policies
	iter, err := snap.SentinelPoliciesByScope(nil, scope)
	if err != nil {
		return nil, err
	}
	var registered []*structs.SentinelPolicy
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		registered = append(registered, raw.(*structs.SentinelPolicy))
	}
	return registered, nil
}

// prepareSentinelPolicies converts all the raw policies into compiled
// policies. The caller must unlock all the policies when complete.
func prepareSentinelPolicies(sent *sentinel.Sentinel, policies []*structs.SentinelPolicy) ([]*sentinel.Policy, error) {
	// Convert the policies to sentinel policies
	var out []*sentinel.Policy
	for _, inp := range policies {
		// Get the policy by a unique ID
		p := sent.Policy(inp.CacheKey())
		out = append(out, p)

		// If the policy is ready, then we have nothing more to do. Store
		// it and continue. This allows policies to only have to be parsed
		// once. Once a policy is "ready" multiple readers can evaluate it
		// concurrently.
		if p.Ready() {
			continue
		}

		// Compile the policy
		f, fset, err := inp.Compile()
		if err != nil {
			// Release all the locks
			for _, r := range out {
				r.Unlock()
			}
			return nil, err
		}

		// Set the policy and declare it is ready
		p.SetName(inp.Name)
		p.SetLevel(sentinel.EnforcementLevel(inp.Type))
		p.SetPolicy(f, fset)
		p.SetReady()
	}
	return out, nil
}

// sentinelResultToWarnErr is used to convert a sentinel evaluation result
// into either a set of warnings or a set of errors.
func sentinelResultToWarnErr(result *sentinel.EvalResult) (error, error) {
	// Check for an error
	if result.Error != nil {
		return nil, errors.New(result.String())
	}

	// Collect all the warnings / errors
	var mWarn multierror.Error
	var mErr multierror.Error
	for _, policyResult := range result.Policies {
		if !policyResult.Result {
			msg := errors.New(policyResult.String())
			if policyResult.AllowedFailure {
				mWarn.Errors = append(mWarn.Errors, msg)
			} else {
				mErr.Errors = append(mErr.Errors, msg)
			}
		}
	}
	return mWarn.ErrorOrNil(), mErr.ErrorOrNil()
}
