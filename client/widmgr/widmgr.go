// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package widmgr

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	cstate "github.com/hashicorp/nomad/client/state"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

// IdentityManager defines a manager responsible for signing and renewing
// signed identities. At runtime it is implemented by *widmgr.WIDMgr.
type IdentityManager interface {
	Run() error
	Get(structs.WIHandle) (*structs.SignedWorkloadIdentity, error)
	Watch(structs.WIHandle) (<-chan *structs.SignedWorkloadIdentity, func())
	Shutdown()
}

type WIDMgr struct {
	allocID                 string
	defaultSignedIdentities map[string]string // signed by the plan applier
	minIndex                uint64
	widSpecs                map[structs.WIHandle]*structs.WorkloadIdentity // workload handle -> WI
	signer                  IdentitySigner
	db                      cstate.StateDB

	// lastToken are the last retrieved signed workload identifiers keyed by
	// TaskIdentity
	lastToken     map[structs.WIHandle]*structs.SignedWorkloadIdentity
	lastTokenLock sync.RWMutex

	// watchers is a map of task identities to slices of channels (each identity
	// can have multiple watchers)
	watchers     map[structs.WIHandle][]chan *structs.SignedWorkloadIdentity
	watchersLock sync.Mutex

	// minWait is the minimum amount of time to wait before renewing. Settable to
	// ease testing.
	minWait time.Duration

	stopCtx context.Context
	stop    context.CancelFunc

	logger hclog.Logger
}

func NewWIDMgr(signer IdentitySigner, a *structs.Allocation, db cstate.StateDB, logger hclog.Logger, envBuilder *taskenv.Builder) *WIDMgr {
	widspecs := map[structs.WIHandle]*structs.WorkloadIdentity{}
	tg := a.Job.LookupTaskGroup(a.TaskGroup)

	allocEnv := envBuilder.Build()

	for _, service := range tg.Services {
		if service.Identity != nil {
			handle := *service.IdentityHandle(allocEnv.ReplaceEnv)
			widspecs[handle] = service.Identity
		}
	}

	for _, task := range tg.Tasks {
		// Omit default identity as it does not expire
		for _, id := range task.Identities {
			widspecs[*task.IdentityHandle(id)] = id
		}

		// update the builder for this task
		taskEnv := envBuilder.UpdateTask(a, task).Build()
		for _, service := range task.Services {
			if service.Identity != nil {
				handle := *service.IdentityHandle(taskEnv.ReplaceEnv)
				widspecs[handle] = service.Identity
			}
		}
	}

	// Create a context for the renew loop. This context will be canceled when
	// the allocation is stopped or agent is shutting down
	stopCtx, stop := context.WithCancel(context.Background())

	return &WIDMgr{
		allocID:                 a.ID,
		defaultSignedIdentities: a.SignedIdentities,
		minIndex:                a.CreateIndex,
		widSpecs:                widspecs,
		signer:                  signer,
		db:                      db,
		minWait:                 10 * time.Second,
		lastToken:               map[structs.WIHandle]*structs.SignedWorkloadIdentity{},
		watchers:                map[structs.WIHandle][]chan *structs.SignedWorkloadIdentity{},
		stopCtx:                 stopCtx,
		stop:                    stop,
		logger:                  logger.Named("widmgr"),
	}
}

// SetMinWait sets the minimum time for renewals
func (m *WIDMgr) SetMinWait(t time.Duration) {
	m.minWait = t
}

// Run blocks until identities are initially signed and then renews them in a
// goroutine. The goroutine is stopped when WIDMgr.Shutdown is called.
//
// If an error is returned the identities could not be fetched and the renewal
// goroutine was not started.
func (m *WIDMgr) Run() error {
	if len(m.widSpecs) == 0 && len(m.defaultSignedIdentities) == 0 {
		m.logger.Debug("no workload identities to retrieve or renew")
		return nil
	}

	m.logger.Debug("retrieving and renewing workload identities", "num_identities", len(m.widSpecs))

	hasExpired, err := m.restoreStoredIdentities()
	if err != nil {
		m.logger.Warn("failed to get signed identities from state DB, refreshing from server: %w", err)
	}
	if hasExpired {
		if err := m.getInitialIdentities(); err != nil {
			return fmt.Errorf("failed to fetch signed identities: %w", err)
		}
	}

	go m.renew()

	return nil
}

// Get retrieves the latest signed identity or returns an error. It must be
// called after Run and does not block.
//
// For retrieving tokens which might be renewed callers should use Watch
// instead to avoid missing new tokens retrieved by Run between Get and Watch
// calls.
func (m *WIDMgr) Get(id structs.WIHandle) (*structs.SignedWorkloadIdentity, error) {
	token := m.get(id)
	if token == nil {
		// This is an error as every identity should have a token by the time Get
		// is called.
		return nil, fmt.Errorf("unable to find token for workload %q and identity %q", id.WorkloadIdentifier, id.IdentityName)
	}

	return token, nil
}

func (m *WIDMgr) get(id structs.WIHandle) *structs.SignedWorkloadIdentity {
	m.lastTokenLock.RLock()
	defer m.lastTokenLock.RUnlock()

	return m.lastToken[id]
}

// Watch returns a channel that sends new signed identities until it is closed
// due to shutdown. Must be called after Run.
//
// The caller must call the returned func to stop watching and ensure the
// watched id actually exists, otherwise the channel never returns a result.
func (m *WIDMgr) Watch(id structs.WIHandle) (<-chan *structs.SignedWorkloadIdentity, func()) {
	// If Shutdown has been called return a closed chan
	if m.stopCtx.Err() != nil {
		c := make(chan *structs.SignedWorkloadIdentity)
		close(c)
		return c, func() {}
	}

	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()

	// Buffer of 1 so sends don't block on receives
	c := make(chan *structs.SignedWorkloadIdentity, 1)
	m.watchers[id] = make([]chan *structs.SignedWorkloadIdentity, 0)
	m.watchers[id] = append(m.watchers[id], c)

	// Create a cancel func for watchers to deregister when they exit.
	cancel := func() {
		m.watchersLock.Lock()
		defer m.watchersLock.Unlock()

		m.watchers[id] = slices.DeleteFunc(
			m.watchers[id],
			func(ch chan *structs.SignedWorkloadIdentity) bool { return ch == c },
		)
	}

	// Prime chan with latest token to avoid a race condition where consumers
	// could miss a token update between Get and Watch calls.
	if token := m.get(id); token != nil {
		c <- token
	}

	return c, cancel
}

// Shutdown stops renewal and closes all watch chans.
func (m *WIDMgr) Shutdown() {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()

	m.stop()

	for _, w := range m.watchers {
		for _, c := range w {
			close(c)
		}
	}

	// ensure it's safe to call Shutdown multiple times
	m.watchers = map[structs.WIHandle][]chan *structs.SignedWorkloadIdentity{}
}

// restoreStoredIdentities recreates the state of the WIDMgr from a previously
// saved state, so that we can avoid asking for all identities again after a
// client agent restart. It returns true if the caller should immediately call
// getIdentities because one or more of the identities is expired.
func (m *WIDMgr) restoreStoredIdentities() (bool, error) {
	storedIdentities, err := m.db.GetAllocIdentities(m.allocID)
	if err != nil {
		return true, err
	}
	if len(storedIdentities) == 0 {
		return true, nil
	}

	m.lastTokenLock.Lock()
	defer m.lastTokenLock.Unlock()

	var hasExpired bool

	for _, identity := range storedIdentities {
		if !identity.Expiration.IsZero() && identity.Expiration.Before(time.Now()) {
			hasExpired = true
		}
		m.lastToken[identity.WIHandle] = identity
	}

	return hasExpired, nil
}

// SignForTesting signs all the identities in m.widspec, typically with the mock
// signer. This should only be used for testing downstream hooks.
func (m *WIDMgr) SignForTesting() {
	m.getInitialIdentities()
}

// getInitialIdentities fetches all signed identities or returns an error. It
// should be run once when the WIDMgr first runs.
func (m *WIDMgr) getInitialIdentities() error {
	// get the default identity signed by the plan applier
	defaultTokens := map[structs.WIHandle]*structs.SignedWorkloadIdentity{}
	for taskName, signature := range m.defaultSignedIdentities {
		id := structs.WIHandle{
			WorkloadIdentifier: taskName,
			IdentityName:       "default",
		}
		widReq := structs.WorkloadIdentityRequest{
			AllocID: m.allocID,
			WIHandle: structs.WIHandle{
				WorkloadIdentifier: taskName,
				IdentityName:       "default",
			},
		}
		defaultTokens[id] = &structs.SignedWorkloadIdentity{
			WorkloadIdentityRequest: widReq,
			JWT:                     signature,
			Expiration:              time.Time{},
		}
	}

	if len(m.widSpecs) == 0 && len(defaultTokens) == 0 {
		return nil
	}

	m.lastTokenLock.Lock()
	defer m.lastTokenLock.Unlock()

	reqs := make([]*structs.WorkloadIdentityRequest, 0, len(m.widSpecs))
	for wiHandle := range m.widSpecs {
		reqs = append(reqs, &structs.WorkloadIdentityRequest{
			AllocID:  m.allocID,
			WIHandle: wiHandle,
		})
	}

	// Get signed workload identities
	signedWIDs := []*structs.SignedWorkloadIdentity{}
	if len(m.widSpecs) != 0 {
		var err error
		signedWIDs, err = m.signer.SignIdentities(m.minIndex, reqs)
		if err != nil {
			return err
		}
	}

	// Store default identity tokens
	for id, token := range defaultTokens {
		m.lastToken[id] = token
	}

	// Index initial workload identities by name
	for _, swid := range signedWIDs {
		m.lastToken[swid.WIHandle] = swid
	}

	return m.db.PutAllocIdentities(m.allocID, signedWIDs)
}

// renew fetches new signed workload identity tokens before the existing tokens
// expire.
func (m *WIDMgr) renew() {
	if len(m.widSpecs) == 0 {
		return
	}

	reqs := make([]*structs.WorkloadIdentityRequest, 0, len(m.widSpecs))
	for workloadHandle, widspec := range m.widSpecs {
		if widspec.TTL == 0 {
			continue
		}
		reqs = append(reqs, &structs.WorkloadIdentityRequest{
			AllocID:  m.allocID,
			WIHandle: workloadHandle,
		})
	}

	if len(reqs) == 0 {
		m.logger.Trace("no workload identities expire")
		return
	}

	renewNow := false
	minExp := time.Time{}

	for workloadHandle, wid := range m.widSpecs {
		if wid.TTL == 0 {
			// No ttl, so no need to renew it
			continue
		}

		token := m.get(workloadHandle)
		if token == nil {
			// Missing a signature, treat this case as already expired so
			// we get a token ASAP
			m.logger.Debug("missing token for identity", "identity", wid.Name)
			renewNow = true
			continue
		}

		if minExp.IsZero() || token.Expiration.Before(minExp) {
			minExp = token.Expiration
		}
	}

	var wait time.Duration
	if !renewNow {
		wait = helper.ExpiryToRenewTime(minExp, time.Now, m.minWait)
	}

	timer, timerStop := helper.NewStoppedTimer()
	defer timerStop()

	var retry uint64

	for {
		// we need to handle stopCtx.Err() and manually stop the subscribers
		if err := m.stopCtx.Err(); err != nil {
			// close watchers and shutdown
			m.Shutdown()
			return
		}

		m.logger.Debug("waiting to renew identities", "num", len(reqs), "wait", wait)
		timer.Reset(wait)
		select {
		case <-timer.C:
			m.logger.Trace("getting new signed identities", "num", len(reqs))
		case <-m.stopCtx.Done():
			// close watchers and shutdown
			m.Shutdown()
			return
		}

		// Renew all tokens together since its cheap
		tokens, err := m.signer.SignIdentities(m.minIndex, reqs)
		if err != nil {
			retry++
			wait = helper.Backoff(m.minWait, time.Hour, retry) + helper.RandomStagger(m.minWait)
			m.logger.Error("error renewing workload identities", "error", err, "next", wait)
			continue
		}

		if len(tokens) == 0 {
			retry++
			wait = helper.Backoff(m.minWait, time.Hour, retry) + helper.RandomStagger(m.minWait)
			m.logger.Error("error renewing workload identities", "error", "no tokens", "next", wait)
			continue
		}

		// Reset next expiration time
		minExp = time.Time{}

		for _, token := range tokens {
			// Set for getters
			m.lastTokenLock.Lock()
			m.lastToken[token.WIHandle] = token
			m.lastTokenLock.Unlock()

			// Send to watchers
			m.watchersLock.Lock()
			m.send(token.WIHandle, token)
			m.watchersLock.Unlock()

			// Set next expiration time
			if minExp.IsZero() || token.Expiration.Before(minExp) {
				minExp = token.Expiration
			}
		}

		// Success! Set next renewal and reset retries
		wait = helper.ExpiryToRenewTime(minExp, time.Now, m.minWait)
		retry = 0
	}
}

// send must be called while holding the m.watchersLock
func (m *WIDMgr) send(id structs.WIHandle, token *structs.SignedWorkloadIdentity) {
	w, ok := m.watchers[id]
	if !ok {
		// No watchers
		return
	}

	for _, c := range w {
		// Pop any unreceived tokens
		select {
		case <-c:
		default:
		}

		// Send new token, should never block since this is the only sender and
		// watchersLock is held
		c <- token
	}
}
