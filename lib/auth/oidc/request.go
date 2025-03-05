// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"context"
	"sync"
	"time"

	//"github.com/coreos/go-oidc/v3/oidc"
	"github.com/hashicorp/cap/oidc"
)

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
func (rc *RequestCache) Store(ctx context.Context, req *oidc.Req) {
	ctx, cancel := context.WithTimeout(ctx, rc.timeout)
	er := &expiringRequest{
		req:    req,
		ctx:    ctx,
		cancel: cancel,
	}
	if _, loaded := rc.m.LoadOrStore(req.Nonce(), er); loaded {
		// we already had this one, so don't need a new timeout
		cancel()
		return
	}
	// auto-delete after timeout
	go func() {
		<-ctx.Done()
		rc.m.Delete(req.Nonce())
	}()
}

func (rc *RequestCache) Load(nonce string) *oidc.Req {
	if er, ok := rc.m.Load(nonce); ok {
		return er.(*expiringRequest).req
	}
	return nil
}

func (rc *RequestCache) LoadAndDelete(nonce string) *oidc.Req {
	if er, loaded := rc.m.LoadAndDelete(nonce); loaded {
		// there is a tiny race condition here, if by massive coincidence,
		// or a bug, the same nonce makes its way in here, because
		// this cancel() also triggers a Delete()
		er.(*expiringRequest).cancel()
		return er.(*expiringRequest).req
	}
	return nil
}
