// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/cap/oidc"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestRequestCache(t *testing.T) {
	// using a top-level cache and running each sub-test in parallel exercises
	// a little bit of thread safety.
	rc := NewRequestCache()

	t.Run("reuse nonce", func(t *testing.T) {
		t.Parallel()
		req := getRequest(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		must.NoError(t, rc.Store(ctx, req))
		must.ErrorIs(t, rc.Store(ctx, req), ErrNonceReuse)
	})

	t.Run("cancel parent ctx", func(t *testing.T) {
		t.Parallel()
		req := getRequest(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		must.NoError(t, rc.Store(ctx, req))
		must.Eq(t, req, rc.Load(req.Nonce()))

		cancel() // triggers delete
		waitUntilGone(t, rc, req.Nonce())
	})

	t.Run("load and delete", func(t *testing.T) {
		t.Parallel()
		req := getRequest(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		must.NoError(t, rc.Store(ctx, req))
		must.Eq(t, req, rc.Load(req.Nonce()))

		must.Eq(t, req, rc.LoadAndDelete(req.Nonce())) // triggers delete
		waitUntilGone(t, rc, req.Nonce())
		must.Nil(t, rc.LoadAndDelete(req.Nonce()))
	})

	t.Run("timeout", func(t *testing.T) {
		// this test needs its own cache to reduce the timeout
		// without affecting any other tests.
		rc := NewRequestCache()
		rc.timeout = time.Millisecond

		req := getRequest(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		must.NoError(t, rc.Store(ctx, req))

		// timeout triggers delete behind the scenes
		waitUntilGone(t, rc, req.Nonce())
	})
}

func getRequest(t *testing.T) *oidc.Req {
	t.Helper()
	nonce := t.Name()
	req, err := oidc.NewRequest(time.Minute, "test-redirect-url",
		oidc.WithNonce(nonce))
	must.NoError(t, err)
	return req
}

func waitUntilGone(t *testing.T, rc *RequestCache, nonce string) {
	t.Helper()
	must.Wait(t,
		wait.InitialSuccess(
			wait.Timeout(100*time.Millisecond), // should be much faster
			wait.Gap(10*time.Millisecond),
			wait.BoolFunc(func() bool {
				return rc.Load(nonce) == nil
			}),
		),
		must.Sprint("request should have gone away"),
	)
}
