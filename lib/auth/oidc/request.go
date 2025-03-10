// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hashicorp/cap/oidc"
)

var (
	ErrNonceReuse      = errors.New("nonce reuse detected")
	ErrTooManyRequests = errors.New("too many auth requests")
)

// MaxRequests is how many requests are allowed to be stored at a time.
// It needs to be large enough for legitimate user traffic, but small enough
// to prevent a DOS from eating up server memory.
const MaxRequests = 10000

// expiringRequest ensures that OIDC requests that are only partially fulfilled
// do not get stuck in memory forever.
type expiringRequest struct {
	// req is what we actually care about
	req *oidc.Req

	// cancel finishes the context so the cleanup goroutine can be cleaned up
	cancel context.CancelFunc

	// loadAndDeleted is set to true when LoadAndDelete deletes the request,
	// so if by some terrible coincidence or mishandling, someone tries to
	// Store() a request with the same nonce during a very narrow but
	// not-impossible timeframe between LoadAndDelete and the delete()
	// that follows the context being canceled.
	loadAndDeleted bool
}

// NewRequestCache creates a cache for OIDC requests.
func NewRequestCache() *RequestCache {
	return &RequestCache{
		m: make(map[string]*expiringRequest),
		// the JWT expiration time in cap library is 5 minutes,
		// so auto-delete from our request cache after 6.
		timeout: 6 * time.Minute,
	}
}

type RequestCache struct {
	l       sync.Mutex
	m       map[string]*expiringRequest
	timeout time.Duration
}

// Store saves the request, to be Loaded later with its Nonce.
// If LoadAndDelete is not called, the stale request will be auto-deleted.
func (rc *RequestCache) Store(ctx context.Context, req *oidc.Req) error {
	rc.l.Lock()
	defer rc.l.Unlock()

	if _, ok := rc.m[req.Nonce()]; ok {
		// we already had a request for this nonce (should never happen)
		return ErrNonceReuse
	}

	if len(rc.m) > MaxRequests {
		return ErrTooManyRequests
	}

	ctx, cancel := context.WithTimeout(ctx, rc.timeout)
	rc.m[req.Nonce()] = &expiringRequest{
		req:    req,
		cancel: cancel,
	}

	// auto-delete after timeout or context canceled
	go func() {
		<-ctx.Done()
		rc.l.Lock()
		defer rc.l.Unlock()
		// only delete if it was not already LoadAndDelete()d
		if deleteMe, ok := rc.m[req.Nonce()]; ok && !deleteMe.loadAndDeleted {
			delete(rc.m, req.Nonce())
		}
	}()

	return nil
}

func (rc *RequestCache) Load(nonce string) *oidc.Req {
	rc.l.Lock()
	defer rc.l.Unlock()
	if er, ok := rc.m[nonce]; ok {
		return er.req
	}
	return nil
}

func (rc *RequestCache) LoadAndDelete(nonce string) *oidc.Req {
	rc.l.Lock()
	defer rc.l.Unlock()
	if er, ok := rc.m[nonce]; ok {
		delete(rc.m, nonce)
		er.loadAndDeleted = true
		er.cancel()
		return er.req
	}
	return nil
}
