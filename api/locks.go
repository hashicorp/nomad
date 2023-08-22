// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

//
import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
)

const (
	leaseRenewalFactor = 0.7
	retryBackoffFactor = 1.1
)

var (
	// ErrLockHeld is returned if we attempt to double lock
	ErrLockHeld = fmt.Errorf("Lock already held")

	// ErrLockNotHeld is returned if we attempt to unlock a lock
	// that we do not hold.
	ErrLockNotHeld = fmt.Errorf("Lock not held")

	// ErrLockInUse is returned if we attempt to destroy a lock
	// that is in use.
	ErrLockInUse = fmt.Errorf("Lock in use")

	// ErrLockConflict is returned if the flags on a key
	// used for a lock do not match expectation
	ErrLockConflict = fmt.Errorf("Existing key does not match lock holder")
)

// Variables returns a new handle on the variables.
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

type Locks struct {
	c *Client
	Variable
	lease time.Duration

	WriteOptions
}

func (l *Locks) Acquire(ctx context.Context, callerID string) (string, error) {
	var out Variable

	l.Variable.Lock = &VariableLock{
		TTL: l.lease.String(),
	}

	_, err := l.c.retryPut(ctx, "/v1/var/"+l.Path+"?lock-acquire", l.Variable, &out, &l.WriteOptions)
	if err != nil {
		return "", err
	}

	l.Variable = out

	return out.Lock.ID, nil
}

func (l *Locks) Release(ctx context.Context) error {
	var out Variable

	_, err := l.c.retryPut(ctx, "/v1/var/"+l.Path+"?lock-release", l.Variable, &out, &l.WriteOptions)
	if err != nil {
		return err
	}

	l.Variable = out
	return nil
}

func (l *Locks) Renew(ctx context.Context) error {
	var out VariableMetadata

	_, err := l.c.retryPut(ctx, "/v1/var/"+l.Path+"?lock-renew", l.Variable, &out, &l.WriteOptions)
	if err != nil {
		return err
	}
	return nil
}

type locker interface {
	Acquire(ctx context.Context, callerID string) (string, error)
	Release(ctx context.Context) error
	Renew(ctx context.Context) error
}

type LockLeaser struct {
	ID            string
	lease         time.Duration
	renewalPeriod time.Duration
	waitPeriod    time.Duration
	randomDelay   time.Duration

	locker
}

func (c *Client) NewLockLeaser(wo WriteOptions, v *Variable, lease time.Duration,
	callerID string) *LockLeaser {
	if callerID == "" {
		callerID = uuid.Generate()
	}

	rn := rand.New(rand.NewSource(time.Now().Unix())).Intn(100)

	if v == nil {
		v = &Variable{
			Namespace: wo.Namespace,
			Path:      "", // TO BE DETERMINED, any ideas?
			Lock: &VariableLock{
				TTL: lease.String(),
			},
		}
	}

	ll := LockLeaser{
		lease:         lease,
		renewalPeriod: time.Duration(float64(lease) * leaseRenewalFactor),
		waitPeriod:    time.Duration(float64(lease) * retryBackoffFactor),
		ID:            callerID,
		randomDelay:   time.Duration(rn) * time.Millisecond,
		locker:        c.Locks(wo, v, lease),
	}

	return &ll
}

func (ll *LockLeaser) Start(ctx context.Context, protectedFunc func(ctx context.Context) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer ll.locker.Release(ctx)

	// Channel to monitor the possible errors on the protected function
	errChannel := make(chan error)

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

			// Start running the lock protected function
			go func() {
				err := protectedFunc(funcCtx)
				if err != nil {
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
			return fmt.Errorf("locks: unable to start protected function: %w", err)

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
