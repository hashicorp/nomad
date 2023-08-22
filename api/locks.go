// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/go-multierror"
)

const (
	lockLeaseRenewalFactor = 0.7
	lockRetryBackoffFactor = 1.1
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

// Acquire will make the actual call to acquire the lock over the variable using
// the ttl in the Locks to create the VariableLock. It will return the
// path of the variable holding the lock.
//
// callerID will be used to identify who is holding the lock in the future,
// currently is only por testing purposes.
func (l *Locks) Acquire(ctx context.Context, callerID string) (string, error) {
	var out Variable

	l.Variable.Lock = &VariableLock{
		TTL: l.ttl.String(),
	}

	_, err := l.c.retryPut(ctx, "/v1/var/"+l.Path+"?lock-acquire", l.Variable, &out, &l.WriteOptions)
	if err != nil {
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
func (l *Locks) Renew(ctx context.Context) error {
	var out VariableMetadata

	_, err := l.c.retryPut(ctx, "/v1/var/"+l.Path+"?lock-renew", l.Variable, &out, &l.WriteOptions)
	if err != nil {
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

// NewLockLeaser returns an instance of LockLeaser. Both variable and callerID
// are optional, in case they are not provided, internal ones will be created.
// Important: It will be on the user to remove the internal variable created for the lock.
func (c *Client) NewLockLeaser(wo WriteOptions, variable *Variable, lease time.Duration,
	callerID string) *LockLeaser {

	rn := rand.New(rand.NewSource(time.Now().Unix())).Intn(100)

	if variable == nil {
		variable = &Variable{
			Namespace: wo.Namespace,
			Path:      "", // TO BE DETERMINED, any ideas?
			Lock: &VariableLock{
				TTL: lease.String(),
			},
		}
	}

	ll := LockLeaser{
		lease:         lease,
		renewalPeriod: time.Duration(float64(lease) * lockLeaseRenewalFactor),
		waitPeriod:    time.Duration(float64(lease) * lockRetryBackoffFactor),
		ID:            callerID,
		randomDelay:   time.Duration(rn) * time.Millisecond,
		locker:        c.Locks(wo, variable, lease),
	}

	return &ll
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
// function in an independent go routine.
func (ll *LockLeaser) start(ctx context.Context, protectedFunc func(ctx context.Context) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Channel to monitor the possible errors on the protected function
	errChannel := make(chan error, 1)

	// To avoid collisions if all the instances start at the same time, wait
	// a random time before making the first call.
	ll.wait(ctx)

	waitTicker := time.NewTicker(ll.waitPeriod)
	defer waitTicker.Stop()

	for {

		lockID, err := ll.locker.Acquire(ctx, ll.ID)
		if err != nil {
			return err
		}

		if lockID != "" {

			funcCtx, funcCancel := context.WithCancel(ctx)
			defer funcCancel()

			// Start running the lock protected function.
			go func() {
				err := protectedFunc(funcCtx)
				if err != nil {
					// Cancel will force the maintainLease to return.
					cancel()
					errChannel <- err
				}
			}()

			// Maintain lease is a blocking function, will only return in case
			// the lock is lost or the context is canceled.
			err := ll.maintainLease(ctx)
			if err != nil {

				funcCancel()
				// Give the protected function some time to return before potentially
				// running it again.
				ll.wait(ctx)
			}

		}

		waitTicker.Stop()
		waitTicker = time.NewTicker(ll.waitPeriod)

		select {
		case err := <-errChannel:
			return fmt.Errorf("locks: error executing the protected function: %w", err)

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

func (ll *LockLeaser) wait(ctx context.Context) {
	t := time.NewTimer(ll.randomDelay)
	defer t.Stop()

	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
