// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/cap/oidc"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func TestRequestCache(t *testing.T) {
	// using a top-level cache and running each sub-test in parallel exercises
	// a little bit of thread safety.
	rc := NewRequestCache(time.Minute)

	t.Run("reuse nonce", func(t *testing.T) {
		t.Parallel()
		req := getRequest(t)

		must.NoError(t, rc.store(req))
		must.ErrorIs(t, rc.store(req), ErrNonceReuse)
	})

	t.Run("load and delete", func(t *testing.T) {
		t.Parallel()
		req := getRequest(t)

		must.NoError(t, rc.store(req))
		must.Eq(t, req, rc.Load(req.Nonce()))

		must.Eq(t, req, rc.LoadAndDelete(req.Nonce()))

		waitUntilGone(t, rc, req.Nonce())
		must.Nil(t, rc.LoadAndDelete(req.Nonce()))
	})

	t.Run("load or add", func(t *testing.T) {
		t.Parallel()
		nonce := t.Name()
		fn := func() (*oidc.Req, error) {
			return oidc.NewRequest(time.Millisecond, "test-redirect-url",
				oidc.WithNonce(nonce))
		}

		// create new
		req, err := rc.LoadOrAdd(nonce, fn)
		must.NoError(t, err)
		must.Eq(t, nonce, req.Nonce())

		// fetch from cache
		req, err = rc.LoadOrAdd(nonce, fn)
		must.NoError(t, err)
		must.Eq(t, nonce, req.Nonce())

		// clear the cache
		must.NotNil(t, rc.LoadAndDelete(req.Nonce()))

		// create new again
		req, err = rc.LoadOrAdd(nonce, fn)
		must.NoError(t, err)
		must.Eq(t, nonce, req.Nonce())
	})

	t.Run("timeout", func(t *testing.T) {
		// this test needs its own cache to reduce the timeout
		// without affecting any other tests.
		rc := NewRequestCache(50 * time.Millisecond)

		req := getRequest(t)

		must.NoError(t, rc.store(req))

		// timeout triggers delete behind the scenes
		waitUntilGone(t, rc, req.Nonce())
	})

	t.Run("too many requests", func(t *testing.T) {
		// not Parallel, would make other tests flaky
		defer rc.c.Purge()

		var gotErr error
		for i := range MaxRequests + 5 {
			req, err := oidc.NewRequest(time.Minute, "test-redirect-url",
				oidc.WithNonce(fmt.Sprintf("too-many-cooks-%d", i)))
			must.NoError(t, err)

			if err := rc.store(req); err != nil {
				gotErr = err
				break
			}
		}
		must.ErrorIs(t, gotErr, ErrTooManyRequests)
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
