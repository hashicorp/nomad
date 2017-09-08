package sentinel

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/sentinel/lang/ast"
	"github.com/hashicorp/sentinel/lang/token"
	"github.com/hashicorp/sentinel/runtime/eval"
	"github.com/mitchellh/hashstructure"
)

// Policy represents a single policy.
//
// A policy can be either ready or not ready. A ready policy is a policy
// that has a loaded policy and configured imports. A policy can be readied
// by acquiring a lock, calling the Set* methods, and then calling the Ready
// method. The Ready method will mark the policy as ready and switch the write
// lock to a read lock.
//
// Policy methods are safe for concurrent access. A policy is protected with
// a reader/writer lock. The lock must be held for using the policy.
//
// Policies are not usually created directly. Instead, the Sentinel.Policy
// method is used to create and lock the policy. The returned value from this
// method always returns a locked policy. The policy should be unlocked upon
// completion.
type Policy struct {
	// Policy has a RWMutex. This is unexported since locking should be
	// completely handled by the Sentinel structure.
	rwmutex sync.RWMutex

	ready    uint32                   // atomic set, 0 == not ready, 1 == pending, 2 == ready
	compiled *eval.Compiled           // compiled policy to execute
	imports  map[string]*policyImport // available imports
	name     string                   // human-friendly name
	level    EnforcementLevel         // enforcement level

	// Ready state. This is only used/set if the policy is not ready.
	readyLock sync.Mutex
	file      *ast.File      // Parsed file to execute
	fileSet   *token.FileSet // FileSet for positional information
}

// EnforcementLevel controls the behavior of policy execution by allowing
// optional policies, overridable policies, etc. There are three enforcement
// levels documented in the enum.
type EnforcementLevel string

const (
	// Advisory means that the policy is allowed to fail. This is reported
	// to the host system which should then log this.
	//
	// SoftMandatory is a policy that is required, but can be overidden
	// on failure. The override is specified within EvalOpts and the mechanism
	// for setting it is determined by the host system.
	//
	// HardMandatory is a policy that is required and cannot be overidden.
	// This is the default enforcement level.
	Advisory      EnforcementLevel = "advisory"
	SoftMandatory EnforcementLevel = "soft-mandatory"
	HardMandatory EnforcementLevel = "hard-mandatory"
)

// policyImport is the structure used to store import configuration for
// a single policy.
type policyImport struct {
	Config map[string]interface{} // Config
	Hash   uint64                 // Unique hash for the configuration
}

//-------------------------------------------------------------------
// Getters only valid when Ready, they all assume a read lock is
// already held
//

// FileSet returns the FileSet associated with this Policy. This can be
// used to turn token.Pos values into full positions.
func (p *Policy) FileSet() *token.FileSet {
	return p.fileSet
}

// Name returns the name of this policy that was set with SetName.
// This will default to the ID given to Sentinel.Policy but can be
// overwritten.
func (p *Policy) Name() string {
	return p.name
}

// Doc returns the docstring value for this policy.
func (p *Policy) Doc() string {
	return p.file.Doc.Text()
}

// Level returns the enforcement level for this policy.
func (p *Policy) Level() EnforcementLevel {
	return p.level
}

//-------------------------------------------------------------------
// Readiness
//

const (
	readyNotReady uint32 = iota
	readyReady
)

// Ready returns whether the policy is ready or not. This can be called
// concurrently without any lock held.
func (p *Policy) Ready() bool {
	return atomic.LoadUint32(&p.ready) == readyReady
}

// SetReady marks a Policy as ready and swaps the write lock to a read lock,
// allowing waiters to begin using the policy.
//
// This should only be called if Ready() is false and after the other Set*
// methods are called to setup the state of the Policy.
//
// The error return value should be checked. This will be nil if the policy
// was successfuly set as ready. If it is non-nil, the policy is not
// ready. In either case, the policy must be explicitly unlocked still.
func (p *Policy) SetReady() error {
	// Compile the policy.
	if p.file == nil {
		return errors.New("policy file must be set to set policy ready")
	}
	cf, err := eval.Compile(&eval.CompileOpts{
		File:    p.file,
		FileSet: p.fileSet,
	})
	if err != nil {
		return err
	}
	p.compiled = cf

	// Set the default enforcement level
	if p.level == "" {
		p.level = HardMandatory
	}

	// Hash all the imports
	for k, v := range p.imports {
		if v.Hash == 0 {
			// New import configuration, set it up
			hash, err := hashstructure.Hash(v.Config, nil)
			if err != nil {
				return fmt.Errorf(
					"error setting policy configuration for import %q: %s",
					k, v)
			}

			v.Hash = hash
		}
	}

	// Mark as ready
	if !atomic.CompareAndSwapUint32(&p.ready, readyNotReady, readyReady) {
		// It was already ready, just return since there is no way we
		// hold a write lock. We really should panic in this scenario, but
		// I didn't want to introduce a potential crash case.
		return errors.New("unable to set policy ready, incorrect source state")
	}

	// Acquire read lock
	p.rwmutex.Unlock()
	p.rwmutex.RLock()

	// Unlock the ready lock
	p.readyLock.Unlock()

	return nil
}

// ResetReady makes the policy not ready again. The lock must be held
// prior to calling this. If the write lock is already held, then this will
// return immediately. If a read lock is held, this will block until the
// write lock can be acquired.
//
// Once this returns, the Policy should be treated like a not ready policy.
// SetReady should be called, Unlock should be called, etc.
//
// This will not reset any of the data or configuration associated with
// a policy. You can call SetReady directly after this to retain the existing
// policy.
func (p *Policy) ResetReady() {
	// If we're not already ready, then just ignore this call. This is safe
	// because the precondition is that a lock MUST be held to call this.
	// If we have a read lock, this will be ready. If we have a write lock,
	// then we have an exclusive lock. In either case, we're safely handling
	// locks.
	if !p.Ready() {
		return
	}

	// Acquire readylock so only one writer can exist. If the policy
	// is alread not ready, then this will block waiting for the person
	// with the write lock to yield.
	p.readyLock.Lock()

	// We should have the read lock so unlock that first.
	p.rwmutex.RUnlock()

	// Grab a write lock on the rwmutex. This will only properly
	// happen once all the readers unlock.
	p.rwmutex.Lock()

	// Set not ready
	atomic.StoreUint32(&p.ready, readyNotReady)
}

// Lock locks the Policy. It automatically grabs a reader or writer lock
// based on the value of Ready().
func (p *Policy) Lock() {
	// Fast path: check if we're already Ready() and grab a reader lock.
	if p.Ready() {
		p.rwmutex.RLock()
		return
	}

	// Slow path: we're not ready. First, acquire the lock protecting ready
	// state. We have a secondary lock here so that we can grab it after
	// the read lock is acquired from SetReady.
	p.readyLock.Lock()

	// If it became ready, switch with a read lock. Otherwise, keep the writer
	if p.Ready() {
		p.readyLock.Unlock()
		p.rwmutex.RLock()
		return
	}

	// Not ready, grab the RW write lock to prevent any read locks.
	p.rwmutex.Lock()
}

// Unlock unlocks the Policy.
func (p *Policy) Unlock() {
	// If we're ready, it is always a reader unlock. This is always true
	// since the SetReady() function will swap a write lock with a read lock
	// and Lock won't allow a write lock unless its not ready.
	if p.Ready() {
		p.rwmutex.RUnlock()
	} else {
		p.rwmutex.Unlock()
		p.readyLock.Unlock()
	}
}

//-------------------------------------------------------------------
// Setters while not ready
//

// SetName sets a human-friendly name for this policy. This is used
// for tracing and debugging. This name will default to the ID given
// to Sentinel.Policy but can be changed to anything else.
//
// The write lock must be held for this.
func (p *Policy) SetName(name string) {
	p.name = name
}

// SetLevel sets the enforcement level for this policy. See EnforcementLevel
// for more documentation. The default enforcement level is HardMandatory.
func (p *Policy) SetLevel(level EnforcementLevel) {
	p.level = level
}

// SetPolicy sets the parsed policy.
//
// The write lock must be held for this.
func (p *Policy) SetPolicy(f *ast.File, fset *token.FileSet) {
	p.file = f
	p.fileSet = fset
}

// SetImport sets a custom configuration for a given import.
//
// Policies can always access all available configured imports on the
// Sentinel instance. This method only needs to be called to configure
// custom configuration.
//
// The import must be registered/configured with the Sentinel instance that
// executes this policy at the time of execution. You may set import
// configuration for imports that are not yet registered as long as they
// are registered by the time of execution.
//
// The write lock must be held for this.
func (p *Policy) SetImport(name string, config map[string]interface{}) {
	if p.imports == nil {
		p.imports = make(map[string]*policyImport)
	}

	p.imports[name] = &policyImport{
		Config: config,
	}
}

//-------------------------------------------------------------------
// encoding/json
//

// MarshalJSON implements json.Marshaler. A policy marshals only to its name.
func (p *Policy) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.name)
}
