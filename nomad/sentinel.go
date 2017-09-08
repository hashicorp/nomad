// +build ent

package nomad

import (
	"errors"
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/sentinel/sentinel"
)

// sentinelDataCallback materializes the Sentinel data
type sentinelDataCallback func() map[string]interface{}

// enforceScope is to enforce any Sentinel policies for a given scope.
// Returns either a set of warnings or errors.
func (s *Server) enforceScope(override bool, scope string, dataCB sentinelDataCallback) (warn, err error) {
	// Fast-path if ACLs are disabled
	if !s.config.ACLEnabled {
		return nil, nil
	}

	// Gather the applicable policies
	registered, err := s.sentinelPoliciesByScope(scope)
	if err != nil {
		return nil, err
	}

	// Hot-path when we have no policies
	if len(registered) == 0 {
		return nil, nil
	}
	// TODO: Use labeled version
	defer metrics.MeasureSince([]string{"nomad", "sentinel", "enforce_scope", scope}, time.Now())

	// Prepare the policies for execution
	prepared, err := prepareSentinelPolicies(s.sentinel, registered)
	if err != nil {
		return nil, err
	}

	// Materialize the data if we have a callback
	var data map[string]interface{}
	if dataCB != nil {
		data = dataCB()
	}

	// Evaluate the policy
	result := s.sentinel.Eval(prepared, &sentinel.EvalOpts{
		Data:     data,
		Override: override,
		Trace:    true,
	})

	// Unlock all the policies
	for _, p := range prepared {
		p.Unlock()
	}

	// Convert the result into warnings or errors
	return sentinelResultToWarnErr(result)
}

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
		p.SetLevel(sentinel.EnforcementLevel(inp.EnforcementLevel))
		p.SetPolicy(f, fset)
		p.SetReady()
	}
	return out, nil
}

// sentinelResultToWarnErr is used to convert a sentinel evaluation result
// into either a set of warnings or a set of errors.
func sentinelResultToWarnErr(result *sentinel.EvalResult) (warn, err error) {
	// Check for an error
	if result.Error != nil {
		return nil, errors.New(result.String())
	}

	// Collect all the warnings / errors
	var mWarn multierror.Error
	var mErr multierror.Error
	for _, policyResult := range result.Policies {
		if !policyResult.Result {
			msg := fmt.Errorf("%s : %s", policyResult.Policy.Name(),
				policyResult.String())
			if policyResult.AllowedFailure {
				mWarn.Errors = append(mWarn.Errors, msg)
			} else {
				mErr.Errors = append(mErr.Errors, msg)
			}
		}
	}
	return mWarn.ErrorOrNil(), mErr.ErrorOrNil()
}

// gcSentinelPolicies is a long running routine which garbage collects unused
// policies that are cached.
func (s *Server) gcSentinelPolicies(stopCh chan struct{}) {
	ticker := time.NewTicker(s.config.SentinelGCInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			// Snapshot the current state
			snap, err := s.State().Snapshot()
			if err != nil {
				s.logger.Printf("[ERR] sentinel.gc: failed to snapshot state: %v", err)
				continue
			}

			// Invalidate the unused policies
			if err := invalidateUnusedPolicies(s.sentinel, snap); err != nil {
				s.logger.Printf("[ERR] sentinel.gc: failed to GC sentinel policies: %v", err)
			}
		}
	}
}

// invalidateUnusedPolicies is used to invalidate any policies that are cached but no longer needed
func invalidateUnusedPolicies(sentinel *sentinel.Sentinel, snap *state.StateSnapshot) error {
	// Get all the cached policies
	cachedPolicies := sentinel.Policies()

	// Hot path if nothing is cached
	if len(cachedPolicies) == 0 {
		return nil
	}

	// Iterate over all registered policies
	iter, err := snap.SentinelPolicies(nil)
	if err != nil {
		return err
	}

	// Collect all the live policies by cache key
	livePolicies := make(map[string]struct{})
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policy := raw.(*structs.SentinelPolicy)
		livePolicies[policy.CacheKey()] = struct{}{}
	}

	// Invalidate any policies that are no longer live
	for _, cached := range cachedPolicies {
		if _, ok := livePolicies[cached]; !ok {
			sentinel.InvalidatePolicy(cached)
		}
	}
	return nil
}
