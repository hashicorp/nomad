// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/hashicorp/go-multierror"
)

const (
	lockLeaseRenewalFactor = 0.7
	lockRetryBackoffFactor = 1.1
)

var (
	errLockConflict = errors.New("lock is not held")

	//LockNoPathErr is returned when no path is provided in the variable to be
	// used for the lease mechanism
	LockNoPathErr = errors.New("the path for the lock provided")
)

// Locks returns a new handle on a lock for the given variable.
func (c *Client) Locks(wo WriteOptions, v *Variable, lease time.Duration) *Locks {
	l := &Locks{
		c:            c,
		WriteOptions: wo,
		Variable:     *v,
	}

	l.c.configureRetries(&retryOptions{
		maxToLastCall: lease,
	})

	return l
}

// Locks is used to maintain all the resources necessary to operate over a lock.
// It makes the calls to the http using an exponential retry mechanism that will
// try until it either reaches 5 attempts or the ttl of the lock expires.
type Locks struct {
	c *Client
	Variable
	ttl time.Duration

	WriteOptions
}

//	 Acquire will make the actual call to acquire the lock over the variable using
//	the ttl in the Locks to create the VariableLock. It will return the
//	path of the variable holding the lock.
//
//	callerID will be used to identify who is holding the lock in the future,
//	currently is only por testing purposes.
//
// Important: A conflict response from the server is not an execution error
// and should be handled differently
func (l *Locks) Acquire(ctx context.Context, callerID string) (string, error) {
	var out Variable

	l.Variable.Lock = &VariableLock{
		TTL: l.ttl.String(),
	}

	_, err := l.c.retryPut(ctx, "/v1/var/"+l.Path+"?lock-acquire", l.Variable, &out, &l.WriteOptions)
	if err != nil {
		var callErr UnexpectedResponseError
		ok := errors.As(err, &callErr)

		// http.StatusConflict means the lock is already held. This will happen
		// under the normal execution if multiple instances are fighting for the same lock and
		// doesn't disrupt the flow.
		if ok || callErr.statusCode != http.StatusConflict {
			return "", errLockConflict
		}

		return "", err
	}

	l.Variable = out

	return out.Path, nil
}

// Release makes the call to release the lock over a variable, even if the ttl
// has not yet passed.
func (l *Locks) Release(ctx context.Context) error {
	var out Variable

	_, err := l.c.retryPut(ctx, "/v1/var/"+l.Path+"?lock-release", l.Variable, &out, &l.WriteOptions)
	if err != nil {
		return err
	}

	l.Variable = out
	return nil
}

// Renew is used to extend the ttl of a lock. It can be used as a heartbeat or a
// lease to maintain the hold over the lock for longer periods or as a sync
// mechanism among multiple instances looking to acquire the same lock.
// Renew will return true if the renewal was successful.
//
// Important: A conflict response from the server is not an execution error
// and should be handled differently, it signals the leaser that the lock is
// lost and the execution of the protected
// function should be stopped, but the lock can be reacquired in the future.
func (l *Locks) Renew(ctx context.Context) error {
	var out VariableMetadata

	_, err := l.c.retryPut(ctx, "/v1/var/"+l.Path+"?lock-renew", l.Variable, &out, &l.WriteOptions)
	if err != nil {
		var callErr UnexpectedResponseError
		ok := errors.As(err, &callErr)

		if ok || callErr.statusCode != http.StatusConflict {
			return errLockConflict
		}

		return err
	}
	return nil
}

type locker interface {
	// Acquire will make the actual call to acquire the lock over the variable using
	// the ttl in the Locks to create the VariableLock.
	//
	// callerID will be used to identify who is holding the lock in the future,
	// currently is only por testing purposes.
	Acquire(ctx context.Context, callerID string) (string, error)
	// Release makes the call to release the lock over a variable, even if the ttl
	// has not yet passed.
	Release(ctx context.Context) error
	// Renew is used to extend the ttl of a lock. It can be used as a heartbeat or a
	// lease to maintain the hold over the lock for longer periods or as a sync
	// mechanism among multiple instances looking to acquire the same lock.
	Renew(ctx context.Context) error
}

// LockLeaser is a helper used to run a protected function that should only be
// active if the instance that runs it is currently holding the lock.
// Can be used to provide synchrony among multiple independent instances.
//
// It includes the lease renewal mechanism and tracking in case the protected
// function returns an error. Internally it uses an exponential retry mechanism
// for the api calls.
type LockLeaser struct {
	ID            string
	lease         time.Duration
	renewalPeriod time.Duration
	waitPeriod    time.Duration
	randomDelay   time.Duration

	locker
}

// NewLockLeaser returns an instance of LockLeaser. callerID
// is optional, in case they it is not provided, internal one will be created.
// The variable doesn't need to exist, if it doesn't, one will be created,
// but the path for it is mandatory.
//
// Important: It will be on the user to remove the variable created for the lock.
func (c *Client) NewLockLeaser(wo WriteOptions, variable *Variable, lease time.Duration,
	callerID string) (*LockLeaser, error) {

	if variable == nil || variable.Path == "" {
		return nil, LockNoPathErr
	}

	rn := rand.New(rand.NewSource(time.Now().Unix())).Intn(100)

	variable.Lock = &VariableLock{
		TTL: lease.String(),
	}

	ll := LockLeaser{
		lease:         lease,
		renewalPeriod: time.Duration(float64(lease) * lockLeaseRenewalFactor),
		waitPeriod:    time.Duration(float64(lease) * lockRetryBackoffFactor),
		ID:            callerID,
		randomDelay:   time.Duration(rn) * time.Millisecond,
		locker:        c.Locks(wo, variable, lease),
	}

	return &ll, nil
}

// Start wraps the start function in charge of executing the protected
// function and maintain the lease but is in charge of releasing the
// lock before exiting. It is a blocking function.
func (ll *LockLeaser) Start(ctx context.Context, protectedFunc func(ctx context.Context) error) error {
	var mErr multierror.Error

	err := ll.start(ctx, protectedFunc)
	if err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	err = ll.Release(ctx)
	if err != nil {
		mErr.Errors = append(mErr.Errors, err)
	}

	return mErr.ErrorOrNil()
}

// start starts the process of maintaining the lease and executes the protected
// function in an independent go routine. It is a blocking function
// that will only return in case of an error or context cancellation.
func (ll *LockLeaser) start(ctx context.Context, protectedFunc func(ctx context.Context) error) error {
	innerCtx, innerCancel := context.WithCancel(ctx)
	defer innerCancel()

	// Channel to monitor the possible errors on execution
	errChannel := make(chan error, 1)

	// To avoid collisions if all the instances start at the same time, wait
	// a random time before making the first call.
	waitWithContext(innerCtx, ll.randomDelay)

	waitTicker := time.NewTicker(ll.waitPeriod)
	defer waitTicker.Stop()

	for {
		lockID, err := ll.locker.Acquire(innerCtx, ll.ID)
		if err != nil && err != errLockConflict {
			errChannel <- fmt.Errorf("error acquiring the lock: %w", err)
		}

		if lockID != "" {
			funcCtx, funcCancel := context.WithCancel(innerCtx)
			defer funcCancel()

			// Execute the lock protected function.
			go func() {
				err := protectedFunc(funcCtx)
				if err != nil {
					errChannel <- fmt.Errorf("error executing the protected function: %w", err)
				}
				// innerCancel will force the maintainLease to return once the protectedFunc
				// is done.
				innerCancel()
			}()

			// Maintain lease is a blocking function, will only return in case
			// the lock is lost or the context is canceled.
			err := ll.maintainLease(innerCtx)
			if err != nil && err != errLockConflict {
				errChannel <- fmt.Errorf("error renewing the lease: %w", err)
			}

			funcCancel()
		}

		waitTicker.Stop()
		waitTicker = time.NewTicker(ll.waitPeriod)

		select {
		case err := <-errChannel:
			return fmt.Errorf("locks: %w", err)

		case <-ctx.Done():
			return nil

		case <-waitTicker.C:
		}
	}
}

func (ll *LockLeaser) maintainLease(ctx context.Context) error {
	renewTicker := time.NewTicker(ll.renewalPeriod)
	defer renewTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil

		case <-renewTicker.C:
			err := ll.locker.Renew(ctx)
			if err != nil {
				return err
			}
		}
	}
}

func waitWithContext(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
