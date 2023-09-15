// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package widmgr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

type RPCer interface {
	RPC(method string, args any, reply any) error
}

// SignerConfig wraps the configuration parameters the workload identity manager
// needs.
type SignerConfig struct {
	// NodeSecret is the node's secret token
	NodeSecret string

	// Region of the node
	Region string

	RPC RPCer
}

// Signer fetches and validates workload identities.
// TODO Move to widmgr/signer.go
type Signer struct {
	nodeSecret string
	region     string
	rpc        RPCer
}

// NewSigner workload identity manager.
func NewSigner(c SignerConfig) *Signer {
	return &Signer{
		nodeSecret: c.NodeSecret,
		region:     c.Region,
		rpc:        c.RPC,
	}
}

// SignIdentities wraps the Alloc.SignIdentities RPC and retrieves signed
// workload identities. The minIndex should be set to the lowest allocation
// CreateIndex to ensure that the server handling the request isn't so stale
// that it doesn't know the allocation exist (and therefore rejects the signing
// requests).
//
// Since a single rejection causes an error to be returned, SignIdentities
// should currently only be used when requesting signed identities for a single
// allocation.
func (s *Signer) SignIdentities(minIndex uint64, req []*structs.WorkloadIdentityRequest) ([]*structs.SignedWorkloadIdentity, error) {
	args := structs.AllocIdentitiesRequest{
		Identities: req,
		QueryOptions: structs.QueryOptions{
			Region:        s.region,
			MinQueryIndex: minIndex - 1,
			AllowStale:    true,
			AuthToken:     s.nodeSecret,
		},
	}
	reply := structs.AllocIdentitiesResponse{}
	if err := s.rpc.RPC("Alloc.SignIdentities", &args, &reply); err != nil {
		return nil, err
	}

	if n := len(reply.Rejections); n == 1 {
		return nil, fmt.Errorf("%d/%d signing request was rejected", n, len(req))
	} else if n > 1 {
		return nil, fmt.Errorf("%d/%d signing requests were rejected", n, len(req))
	}

	if len(reply.SignedIdentities) == 0 {
		return nil, fmt.Errorf("empty signed identity response")
	}

	if exp, act := len(reply.SignedIdentities), len(req); exp != act {
		return nil, fmt.Errorf("expected %d signed identities but received %d", exp, act)
	}

	return reply.SignedIdentities, nil
}

// IdentitySigner is the interface needed to retrieve signed identities for
// workload identities. At runtime it is implemented by *widmgr.WIDMgr.
type IdentitySigner interface {
	SignIdentities(minIndex uint64, req []*structs.WorkloadIdentityRequest) ([]*structs.SignedWorkloadIdentity, error)
}

type WIDMgr struct {
	allocID  string
	minIndex uint64
	widSpecs map[string][]*structs.WorkloadIdentity // task -> WI
	signer   IdentitySigner

	// tokens are signed workload identifiers keyed by TaskIdentity
	tokens     map[cstructs.TaskIdentity]*structs.SignedWorkloadIdentity
	tokensLock sync.Mutex

	watchers     map[cstructs.TaskIdentity]chan *structs.SignedWorkloadIdentity
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
		widspecs[task.Name] = helper.CopySlice(task.Identities)
	}

	// Create a context for the renew loop. This context will be canceled when
	// the allocation is stopped or agent is shutting down
	stopCtx, stop := context.WithCancel(context.Background())

	return &WIDMgr{
		allocID:  a.ID,
		minIndex: a.CreateIndex,
		widSpecs: widspecs,
		signer:   signer,
		minWait:  10 * time.Second,
		watchers: map[cstructs.TaskIdentity]chan *structs.SignedWorkloadIdentity{},
		stopCtx:  stopCtx,
		stop:     stop,
		logger:   logger, //TODO: sublogger?
	}
}

// Run blocks until identities are initially signed and then renews them in a
// goroutine. The goroutine is stopped when WIDMgr.Shutdown is called.
//
// If an error is returned the identities could not be fetched and the renewal
// goroutine was not started.
func (m *WIDMgr) Run() error {
	if err := m.getIdentities(); err != nil {
		return fmt.Errorf("failed to fetch signed identities: %w", err)
	}

	go m.renew()

	return nil
}

// Get retrieves the latest signed identity or returns an error. It must be
// called after Run and does not block.
func (m *WIDMgr) Get(id cstructs.TaskIdentity) (*structs.SignedWorkloadIdentity, error) {
	token := m.tokens[id]
	if token == nil {
		// This is an error as every identity should have a token by the time Get
		// is called.
		return nil, fmt.Errorf("unable to find token for task %q and identity %q", id.TaskName, id.IdentityName)
	}

	return token, nil
}

func (m *WIDMgr) get(id cstructs.TaskIdentity) *structs.SignedWorkloadIdentity {
	m.tokensLock.Lock()
	defer m.tokensLock.Unlock()

	return m.tokens[id]
}

// Watch sends new signed identities until it is closed due to shutdown. Must
// be called after Run.
//
// The caller must call the returned func to stop watching.
func (m *WIDMgr) Watch(id cstructs.TaskIdentity) (<-chan *structs.SignedWorkloadIdentity, func()) {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()

	if existing, ok := m.watchers[id]; ok {
		//TODO I think we only want 1 watcher per id, so let's just close on double watches?
		close(existing)
	}

	// Buffer of 1 so sends don't block on receives
	c := make(chan *structs.SignedWorkloadIdentity, 1)
	m.watchers[id] = c

	cancel := func() {
		m.watchersLock.Lock()
		defer m.watchersLock.Unlock()

		delete(m.watchers, id)

		//TODO should we close(c) here? seems sketchy... caller shouldn't try to
		//recv on c after calling cancel
	}

	return c, cancel
}

// Shutdown stops renewal and closes all watch chans.
func (m *WIDMgr) Shutdown() {
}

// getIdentities fetches all signed identities or returns an error.
func (m *WIDMgr) getIdentities() error {
	m.tokensLock.Lock()
	defer m.tokensLock.Unlock()

	if len(m.widSpecs) == 0 {
		return nil
	}

	reqs := make([]*structs.WorkloadIdentityRequest, len(m.widSpecs))
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
	signedWIDs, err := m.signer.SignIdentities(m.minIndex, reqs)
	if err != nil {
		return err
	}

	// Index initial workload identities by name
	m.tokens = make(map[cstructs.TaskIdentity]*structs.SignedWorkloadIdentity, len(signedWIDs))
	for _, swid := range signedWIDs {
		id := cstructs.TaskIdentity{
			TaskName:     swid.TaskName,
			IdentityName: swid.IdentityName,
		}

		m.tokens[id] = swid
	}

	return nil
}

// renew fetches new signed workload identity tokens before the existing tokens
// expire.
func (m *WIDMgr) renew() {
	if len(m.widSpecs) == 0 {
		return
	}

	reqs := make([]*structs.WorkloadIdentityRequest, len(m.widSpecs))
	for taskName, widspecs := range m.widSpecs {
		for _, widspec := range widspecs {
			reqs = append(reqs, &structs.WorkloadIdentityRequest{
				AllocID:      m.allocID,
				TaskName:     taskName,
				IdentityName: widspec.Name,
			})
		}
	}

	renewNow := false
	minExp := time.Now().Add(30 * time.Hour) // set high default expiration

	for taskName, wids := range m.widSpecs {
		for _, wid := range wids {
			if wid.TTL == 0 {
				// No ttl, so no need to renew it
				continue
			}

			//FIXME make this less ugly
			token := m.get(cstructs.TaskIdentity{
				TaskName:     taskName,
				IdentityName: wid.Name,
			})
			if token == nil {
				// Missing a signature, treat this case as already expired so we get a
				// token ASAP
				m.logger.Debug("missing token for identity", "identity", wid.Name)
				renewNow = true
				continue
			}

			if token.Expiration.Before(minExp) {
				minExp = token.Expiration
			}
		}
	}

	if len(reqs) == 0 {
		m.logger.Trace("no workload identities expire")
		return
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
			//TODO Close watchers
			return
		}

		m.logger.Debug("waiting to renew identities", "num", len(reqs), "wait", wait)
		timer.Reset(wait)
		select {
		case <-timer.C:
			m.logger.Trace("getting new signed identities", "num", len(reqs))
		case <-m.stopCtx.Done():
			//TODO Close watchers
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
			id := cstructs.TaskIdentity{
				TaskName:     token.TaskName,
				IdentityName: token.IdentityName,
			}

			// Set for getters
			m.tokensLock.Lock()
			m.tokens[id] = token
			m.tokensLock.Unlock()

			// Send to watchers
			m.send(id, token)

			// Set next expiration time
			if minExp.IsZero() {
				minExp = token.Expiration
			} else if token.Expiration.Before(minExp) {
				minExp = token.Expiration
			}
		}

		// Success! Set next renewal and reset retries
		wait = helper.ExpiryToRenewTime(minExp, time.Now, m.minWait)
		retry = 0
	}
}

func (m *WIDMgr) send(id cstructs.TaskIdentity, token *structs.SignedWorkloadIdentity) {
	m.watchersLock.Lock()
	defer m.watchersLock.Unlock()

	c, ok := m.watchers[id]
	if !ok {
		// No watchers
		return
	}

	// Pop any unreceived tokens
	select {
	case <-c:
	default:
	}

	// Send new token, should never block since this is the only sender and watchersLock is held
	c <- token
}
