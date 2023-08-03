// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type client interface {
	put(endpoint string, in, out any, q *WriteOptions) (*WriteMeta, error)
}

type retryClient struct {
	c client
	retryOptions
}

// LockOptions is used to parameterize the Lock behavior.
type retryOptions struct {
	MaxRetries uint64 // Optional, defaults to 3
	// MaxBetweenCalls sets a capping value for the delay between calls, to avoid it growing infinitely
	MaxBetweenCalls time.Duration // Optional, defaults to 0, meaning no time cap
	// MaxToLastCall sets a capping value for all the retry process, in case there is a deadline to make the call.
	MaxToLastCall time.Duration // Optional, defaults to 0, meaning no time cap
	// FixedDelay is used in case an uniform distribution of the calls is preferred.
	FixedDelay time.Duration // Optional, defaults to 0, meaning Delay is exponential, starting at 1sec
	// DelayBase is used to calculate the starting value at which the delay starts to grow,
	// When left empty, a value of 1 sec will be used as base and then the delays will
	// grow exponentially with every attempt: starting at 1s, then 2s, 4s, 8s...
	DelayBase time.Duration // Optional, defaults to 1sec
}

func newRetryClient(c client, opts retryOptions) *retryClient {
	rc := &retryClient{
		c: c,
		retryOptions: retryOptions{
			MaxRetries: 3,
			DelayBase:  time.Second,
		},
	}

	if opts.DelayBase != 0 {
		rc.DelayBase = opts.DelayBase
	}

	if opts.MaxRetries != 0 {
		rc.MaxRetries = opts.MaxRetries
	}

	if opts.MaxBetweenCalls != 0 {
		rc.MaxBetweenCalls = opts.MaxBetweenCalls
	}

	if opts.MaxToLastCall != 0 {
		rc.MaxToLastCall = opts.MaxToLastCall
	}

	if opts.FixedDelay != 0 {
		rc.FixedDelay = opts.FixedDelay
	}

	return rc
}

func (rc *retryClient) retryPut(ctx context.Context, endpoint string, in, out any, q *WriteOptions) (*WriteMeta, error) {
	var err error
	var wm *WriteMeta

	attDelay := time.Duration(0)
	startTime := time.Now()

	t := time.NewTimer(attDelay)
	defer t.Stop()

	for attempt := uint64(0); attempt < rc.MaxRetries; attempt++ {
		attDelay = rc.calculateDelay(attempt)

		if !t.Stop() {
			<-t.C
		}
		t.Reset(attDelay)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.C:

		}

		wm, err = rc.c.put(endpoint, in, out, q)

		// Maximum retry period is up, don't retry
		if rc.MaxToLastCall != 0 && time.Now().Sub(startTime) > rc.MaxToLastCall {
			break
		}

		// The put function only returns WriteMetadata if the call was successful
		// don't retry
		if wm != nil {
			break
		}

		// If WriteMetadata is nil, we need to process the error to decide if a retry is
		// necessary or not
		var callErr *UnexpectedResponseError
		ok := errors.As(err, &callErr)

		// If is not UnexpectedResponseError, it is an error while performing the call
		// don't retry
		if !ok {
			break
		}

		// Only 500+ or 429 status calls may be retried, otherwise
		// don't retry
		if !isCallRetriable(callErr.StatusCode()) {
			break
		}
	}

	return wm, err
}

// According to the HTTP protocol, it only makes sense to retry calls
// when the error is caused by a temporary situation, like a server being down
// (500s+) or the call being rate limited (429), this function checks if the
// statusCode is between the errors worth retrying.
func isCallRetriable(statusCode int) bool {
	return statusCode > http.StatusInternalServerError &&
		statusCode < http.StatusNetworkAuthenticationRequired ||
		statusCode == http.StatusTooManyRequests
}

func (rc *retryClient) calculateDelay(attempt uint64) time.Duration {
	if rc.FixedDelay != 0 {
		return rc.FixedDelay
	}

	if attempt == 0 {
		return 0
	}

	newDelay := rc.DelayBase << (attempt - 1)

	if rc.MaxBetweenCalls != 0 && newDelay > rc.MaxBetweenCalls {
		return rc.MaxBetweenCalls
	}

	return newDelay
}
