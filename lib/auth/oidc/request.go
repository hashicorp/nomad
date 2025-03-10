// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"context"
	"errors"
	"sync"
	"time"

	//"github.com/coreos/go-oidc/v3/oidc"
	"github.com/hashicorp/cap/oidc"
)

var ErrNonceReuse = errors.New("nonce reuse detected")

// expiringRequest ensures that OIDC requests that are only partially fulfilled
// do not get stuck in memory forever.
type expiringRequest struct {
	// req is what we actually care about
	req *oidc.Req
	// ctx lets us clean up stale requests automatically
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRequestCache creates a cache for OIDC requests.
func NewRequestCache() *RequestCache {
	return &RequestCache{
		m: sync.Map{},
		// the JWT expiration time in cap library is 5 minutes,
		// so auto-delete from our request cache after 6.
		timeout: 6 * time.Minute,
	}
}

type RequestCache struct {
	m       sync.Map
	timeout time.Duration
}

// Store saves the request, to be Loaded later with its Nonce.
// If LoadAndDelete is not called, the stale request will be auto-deleted.
func (rc *RequestCache) Store(ctx context.Context, req *oidc.Req) error {
	ctx, cancel := context.WithTimeout(ctx, rc.timeout)
	er := &expiringRequest{
		req:    req,
		ctx:    ctx,
		cancel: cancel,
	}
	if _, loaded := rc.m.LoadOrStore(req.Nonce(), er); loaded {
		// we already had a request for this nonce, which should never happen,
		// so cancel the new request and error to notify caller of a bug.
		cancel()
		return ErrNonceReuse
	}
	// auto-delete after timeout or context canceled
	go func() {
		<-ctx.Done()
		rc.m.Delete(req.Nonce())
	}()
	return nil
}

func (rc *RequestCache) Load(nonce string) *oidc.Req {
	if er, ok := rc.m.Load(nonce); ok {
		return er.(*expiringRequest).req
	}
	return nil
}

func (rc *RequestCache) LoadAndDelete(nonce string) *oidc.Req {
	if er, loaded := rc.m.LoadAndDelete(nonce); loaded {
		// there is a tiny race condition here. if by massive coincidence,
		// or a bug, the same nonce makes its way in here, this cancel()
		// triggers a map Delete() up in Store(), which could delete a request
		// out from under a subsequent Store()
		er.(*expiringRequest).cancel()
		return er.(*expiringRequest).req
	}
	return nil
}
