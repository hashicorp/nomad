// Package sentinel contains the high-level API for embedding Sentinel.
//
// The APIs in this package manage many of the complexities of resource
// management and execution for policies to simplify embedding Sentinel
// into a new system. While lower-level API packages could be used, this is
// the recommended starting point for embedding Sentinel.
package sentinel

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/mitchellh/hashstructure"
)

// Sentinel is the primary structure for managing resources and executing
// policies. It is created with New and should be cleaned with Close when
// it is no longer needed.
type Sentinel struct {
	// imports is the set of imports and their runtime information.
	imports       map[string]*sentinelImport
	importsLock   sync.RWMutex
	importReapTTL time.Duration

	evalTimeout time.Duration

	policies     map[string]*Policy
	policiesLock sync.RWMutex

	// cancelFunc is the function that should be called to stop any background
	// tasks running.
	cancelFunc context.CancelFunc
}

// Config is the configuration for creating a Sentinel structure with New.
//
// The configuration for Sentinel can be updated at any time with SetConfig.
type Config struct {
	// EvalTimeout is the timeout for a single policy execution.
	// This must be set to some non-zero value or it will default to
	// 1 second.
	EvalTimeout time.Duration

	// Imports are the available imports.
	Imports map[string]*Import

	// ImportReapTTL is the duration an import remains allocated and
	// configured before being reaped. This should be at least as long as
	// the maxium time a policy can execute so an import isn't reaped
	// mid-execution.
	ImportReapTTL time.Duration
}

// New creates a new Sentinel object.
//
// The Sentinel object is the primary structure for managing resources and
// executing policies. All methods on the structure are safe for concurrent
// access unless otherwise noted.
//
// A host system usually only needs a single instance for the entire process.
// This allows Sentinel to best manage the resources necessary for policy
// enforcement.
//
// The configuration for initializing is optional. If nil is given, a default
// configuration is used. If a non-nil configuration is given, it must not
// be modified or reused after this function call.
func New(cfg *Config) *Sentinel {
	// Build the context we'll use
	ctx, ctxCancel := context.WithCancel(context.Background())

	result := &Sentinel{
		imports:     make(map[string]*sentinelImport),
		policies:    make(map[string]*Policy),
		evalTimeout: 1 * time.Second,
		cancelFunc:  ctxCancel,
	}

	// Update import information
	if cfg != nil {
		result.importReapTTL = cfg.ImportReapTTL
		result.evalTimeout = cfg.EvalTimeout
		if err := result.SetConfig(cfg); err != nil {
			panic(err)
		}
	}

	// Set defaults
	if result.importReapTTL == 0 {
		result.importReapTTL = ImportReapTTL
	}

	// Start the import reaper
	go result.importReaper(ctx)

	return result
}

// Close cleans up resources used by a Sentinel structure that won't be
// cleaned up with a standard Go GC. This should be called for a clean exit.
//
// Sentinel may start external binary processes (to serve plugins). These
// won't exit unless the parent process exits or Close is called.
func (s *Sentinel) Close() error {
	// Stop the reaper
	s.cancelFunc()

	// Grab a lock so we can stop all imports
	s.importsLock.Lock()
	defer s.importsLock.Unlock()
	for _, impt := range s.imports {
		impt.Close()
	}

	return nil
}

//-------------------------------------------------------------------
// Configuration

// SetConfig updates the configuration for Sentinel.
//
// This is an expensive operation depending on what keys in the configuration
// are updated. Updating imports for example can cause a stop-the-world
// event. Configuration should not be changed frequently.
//
// Currently only Imports can be updated.
//
// This can be called concurrently.
func (s *Sentinel) SetConfig(c *Config) error {
	s.importsLock.Lock()
	defer s.importsLock.Unlock()

	// Close any imports that have been removed
	for k, impt := range s.imports {
		if _, ok := c.Imports[k]; !ok {
			// Import is no longer available
			delete(s.imports, k)

			// If this has a plugin binary, kill it
			if err := impt.Close(); err != nil {
				return err
			}
		}
	}

	// Update all the new/changed imports
	for k, impt := range c.Imports {
		if old, ok := s.imports[k]; ok {
			if reflect.DeepEqual(old, impt) {
				// Identical, do nothing
				continue
			}

			// Changed the old, kill it. In the future we should do a more
			// fine-grained update here.
			if err := old.Close(); err != nil {
				return err
			}

			delete(s.imports, k)
		}

		// Setup the new import
		hash, err := hashstructure.Hash(impt.Config, nil)
		if err != nil {
			return err
		}

		s.imports[k] = &sentinelImport{
			Import: impt,
			Hash:   hash,
		}
	}

	return nil
}

//-------------------------------------------------------------------
// Policy Management

// Policy creates or reads the Policy with the given unique ID. The boolean
// value notes whether the policy is ready or not.
//
// The unique ID can be any string. It is an opaque value to Sentinel and
// is not used for anything other than storage and lookup.
//
// The returned Policy is locked. If the Policy is ready, a read lock is
// held. If the Policy is not ready, a write lock is held. In either case,
// Unlock must be called on the Policy when the user is done with it. Unlock
// may only be called once. To relock a Policy, you must call this method again.
func (s *Sentinel) Policy(id string) *Policy {
	// Fast path, acquire read lock and assume the policy exists
	s.policiesLock.RLock()
	result, ok := s.policies[id]
	if ok {
		// Unlock right away so that we don't block other Policy requests
		s.policiesLock.RUnlock()

		// Lock the result
		result.Lock()
		return result
	}

	// Slow path: policy didn't exist, grab a write lock so we can insert it
	s.policiesLock.RUnlock()
	s.policiesLock.Lock()

	// If it exists now (race), then return it
	if result, ok = s.policies[id]; ok {
		// Unlock so we don't block Policy access
		s.policiesLock.Unlock()

		// Acquire read lock for same reasons as above fast path.
		result.Lock()

		// Return
		return result
	}

	// Create new policy and lock it
	result = &Policy{}
	result.Lock()

	// Update the name
	result.SetName(id)

	// Insert and unlock
	s.policies[id] = result
	s.policiesLock.Unlock()

	// Return
	return result
}

// Policies returns the list of policies that have been registered with
// this Sentinel instance. The returned IDs may represent policies that
// are not yet "ready" but have been requested via Policy().
//
// The results are not in any specified order.
func (s *Sentinel) Policies() []string {
	s.policiesLock.RLock()
	defer s.policiesLock.RUnlock()

	result := make([]string, 0, len(s.policies))
	for k := range s.policies {
		result = append(result, k)
	}

	return result
}

// InvalidatePolicy removes a single policy from Sentinel, releasing
// any currently used resources with it. If the policy didn't exist, then
// this does nothing.
//
// The host system must take care that a stale policy isn't written after
// InvalidatePolicy is called. This is possible under a single scenario:
//
//   t=0 - Host reads policy from physical storage
//   t=1 - Host (another thread) writes new policy and calls sentinel.Invalidate
//   t=2 - Host calls sentinel.Policy to read policy from cache
//   t=3 - Policy is not ready
//   t=4 - Policy is made ready from read at t=0 with old policy
//
// Instead, at t=4, the host system should reload the value that it read
// earlier and use that value. This will ensure that only the new value is
// written.
//
// InvalidatePolicy will block while any write lock is held on the policy
// being invalidated. This ensures that the race above is the only possible
// race.
func (s *Sentinel) InvalidatePolicy(id string) {
	s.policiesLock.Lock()
	defer s.policiesLock.Unlock()

	// Get the old policy. If it exists, grab a lock. This lock will only
	// let us grab it under two circumstances:
	//
	//   1.) The policy is ready. In this case, its fine and we can invalidate.
	//       Any call to Policy will block on us.
	//
	//   2.) The policy is not ready. In this case, we have the exclusive
	//       write lock and we can continue to invalidate since the next
	//       call to Policy will block on us.
	if p, ok := s.policies[id]; ok {
		// We can safely unlock right away since we have the write lock
		// to s.policiesLock.
		p.Lock()
		p.Unlock()
	}

	// Just delete the policy. The plugin TTL will eventually expire which
	// will cause any active imports from this policy to be deallocated.
	delete(s.policies, id)
}
