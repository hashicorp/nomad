// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"errors"
	"time"

	"github.com/hashicorp/cap/oidc"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

var (
	ErrNonceReuse = errors.New("nonce reuse detected")
	// ErrTooManyRequests is returned if the request cache is full.
	// Realistically, we expect this only to happen if the auth-url
	// API endpoint is being DOS'd.
	ErrTooManyRequests = errors.New("too many auth requests")
)

// MaxRequests is how many requests are allowed to be stored at a time.
// It needs to be large enough for legitimate user traffic, but small enough
// to prevent a DOS from eating up server memory.
const MaxRequests = 1000

// NewRequestCache creates a cache for OIDC requests.
// The JWT expiration time in the cap library is 5 minutes,
// so timeout should be around that long.
func NewRequestCache(timeout time.Duration) *RequestCache {
	return &RequestCache{
		c: expirable.NewLRU[string, *oidc.Req](MaxRequests, nil, timeout),
	}
}

type RequestCache struct {
	c *expirable.LRU[string, *oidc.Req]
}

// Store saves the request, to be Loaded later with its Nonce.
// If LoadAndDelete is not called, the stale request will be auto-deleted.
func (rc *RequestCache) Store(req *oidc.Req) error {
	if rc.c.Len() >= MaxRequests {
		return ErrTooManyRequests
	}

	if _, ok := rc.c.Get(req.Nonce()); ok {
		// we already had a request for this nonce (should never happen)
		return ErrNonceReuse
	}

	rc.c.Add(req.Nonce(), req)

	return nil
}

func (rc *RequestCache) Load(nonce string) *oidc.Req {
	if req, ok := rc.c.Get(nonce); ok {
		return req
	}
	return nil
}

func (rc *RequestCache) LoadAndDelete(nonce string) *oidc.Req {
	if req, ok := rc.c.Get(nonce); ok {
		rc.c.Remove(nonce)
		return req
	}
	return nil
}
