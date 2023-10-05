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
	widSpecs                map[string][]*structs.WorkloadIdentity // task -> WI
	signer                  IdentitySigner

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

func NewWIDMgr(signer IdentitySigner, a *structs.Allocation, logger hclog.Logger) *WIDMgr {
	widspecs := map[string][]*structs.WorkloadIdentity{}
	tg := a.Job.LookupTaskGroup(a.TaskGroup)
	for _, task := range tg.Tasks {
		// Omit default identity as it does not expire
		if len(task.Identities) > 0 {
			widspecs[task.Name] = helper.CopySlice(task.Identities)
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

	if err := m.getIdentities(); err != nil {
		return fmt.Errorf("failed to fetch signed identities: %w", err)
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
		return nil, fmt.Errorf("unable to find token for task %q and identity %q", id.WorkloadIdentifier, id.IdentityName)
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
}

// getIdentities fetches all signed identities or returns an error.
func (m *WIDMgr) getIdentities() error {
	// get the default identity signed by the plan applier
	defaultTokens := map[structs.WIHandle]*structs.SignedWorkloadIdentity{}
	for taskName, signature := range m.defaultSignedIdentities {
		id := structs.WIHandle{
			WorkloadIdentifier: taskName,
			IdentityName:       "default",
		}
		widReq := structs.WorkloadIdentityRequest{
			AllocID:      m.allocID,
			TaskName:     taskName,
			IdentityName: "default",
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
	for taskName, widspecs := range m.widSpecs {
		for _, widspec := range widspecs {
			reqs = append(reqs, &structs.WorkloadIdentityRequest{
				AllocID:      m.allocID,
				TaskName:     taskName,
				IdentityName: widspec.Name,
			})
		}
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
		id := structs.WIHandle{
			WorkloadIdentifier: swid.TaskName,
			IdentityName:       swid.IdentityName,
		}

		m.lastToken[id] = swid
	}

	// TODO: Persist signed identity token to client state
	return nil
}

// renew fetches new signed workload identity tokens before the existing tokens
// expire.
func (m *WIDMgr) renew() {
	if len(m.widSpecs) == 0 {
		return
	}

	reqs := make([]*structs.WorkloadIdentityRequest, 0, len(m.widSpecs))
	for taskName, widspecs := range m.widSpecs {
		for _, widspec := range widspecs {
			if widspec.TTL == 0 {
				continue
			}
			reqs = append(reqs, &structs.WorkloadIdentityRequest{
				AllocID:      m.allocID,
				TaskName:     taskName,
				IdentityName: widspec.Name,
			})
		}
	}

	if len(reqs) == 0 {
		m.logger.Trace("no workload identities expire")
		return
	}

	renewNow := false
	minExp := time.Time{}

	for taskName, wids := range m.widSpecs {
		for _, wid := range wids {
			if wid.TTL == 0 {
				// No ttl, so no need to renew it
				continue
			}

			//FIXME make this less ugly
			token := m.get(structs.WIHandle{
				WorkloadIdentifier: taskName,
				IdentityName:       wid.Name,
			})
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
		// FIXME this will have to be revisited once we support identity change modes
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
			id := structs.WIHandle{
				WorkloadIdentifier: token.TaskName,
				IdentityName:       token.IdentityName,
			}

			// Set for getters
			m.lastTokenLock.Lock()
			m.lastToken[id] = token
			m.lastTokenLock.Unlock()

			// Send to watchers
			m.watchersLock.Lock()
			m.send(id, token)
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
