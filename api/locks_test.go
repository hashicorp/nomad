// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

var testLease = 10 * time.Millisecond

type mockLock struct {
	locked        bool
	acquireCalls  map[string]int
	renewsCounter int
	ownerID       string
	mu            sync.Mutex

	leaseStartTime time.Time
}

func (ml *mockLock) LockTTL() time.Duration {
	return testLease
}

func (ml *mockLock) Acquire(_ context.Context, callerID string) (string, error) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	ml.acquireCalls[callerID] += 1
	if ml.locked {
		return "", nil
	}

	ml.locked = true
	ml.leaseStartTime = time.Now()
	ml.renewsCounter = 0
	return "lockPath", nil
}

func (ml *mockLock) Release(_ context.Context) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	if !ml.locked {
		return errors.New("lock not locked")
	}

	ml.locked = false
	ml.renewsCounter = 0
	return nil
}

// The behavior of renew is not an exact replication of
// the lock work, its intended to test the behavior of the
// multiple instances running.
func (ml *mockLock) Renew(_ context.Context) error {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	if !ml.locked {
		return errors.New("error")
	}

	if time.Since(ml.leaseStartTime) > testLease {
		ml.locked = false
		return ErrLockConflict
	}

	ml.leaseStartTime = time.Now()
	ml.renewsCounter += 1
	return nil
}

func (ml *mockLock) getLockState() mockLock {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	return mockLock{
		locked:        ml.locked,
		acquireCalls:  copyMap(ml.acquireCalls),
		renewsCounter: ml.renewsCounter,
	}
}

type mockService struct {
	mockLock

	mu            sync.Mutex
	startsCounter int
	starterID     string
}

func (ms *mockService) Run(callerID string, _ context.Context) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		ms.mu.Lock()
		ms.startsCounter += 1
		ms.starterID = callerID
		ms.mu.Unlock()

		<-ctx.Done()

		ms.mu.Lock()
		ms.starterID = ""
		ms.mu.Unlock()

		return nil
	}
}

func (ms *mockService) getServiceState() mockService {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	return mockService{
		startsCounter: ms.startsCounter,
		starterID:     ms.starterID,
	}
}

func TestAcquireLock_MultipleInstances(t *testing.T) {
	l := mockLock{
		acquireCalls: map[string]int{},
	}

	s := mockService{}

	testCtx := context.Background()

	// Set up independent contexts to test the switch when one controller stops
	hac1Ctx, hac1Cancel := context.WithCancel(testCtx)
	defer hac1Cancel()

	// Wait time on hac1 is 0, it should always get the lock.
	hac1 := LockLeaser{
		ID:            "hac1",
		locker:        &l,
		renewalPeriod: time.Duration(float64(testLease) * lockLeaseRenewalFactor),
		waitPeriod:    time.Duration(float64(testLease) * lockRetryBackoffFactor),
		randomDelay:   0,
	}

	hac2 := LockLeaser{
		ID:            "hac2",
		locker:        &l,
		renewalPeriod: time.Duration(float64(testLease) * lockLeaseRenewalFactor),
		waitPeriod:    time.Duration(float64(testLease) * lockRetryBackoffFactor),
		randomDelay:   6 * time.Millisecond,
	}

	lock := l.getLockState()
	must.False(t, lock.locked)

	go func() {
		err := hac1.Start(hac1Ctx, s.Run(hac1.ID, testCtx))
		must.NoError(t, err)
	}()

	go func() {
		err := hac2.Start(testCtx, s.Run(hac2.ID, testCtx))
		must.NoError(t, err)
	}()

	time.Sleep(4 * time.Millisecond)
	/*
		After 4 ms more (4 ms total):
		* hac2 should  not have tried to acquire the lock because it has an initial delay of 6ms.
		* hac1 should have the lock and the service should be running.
		* The first lease is not over yet, no calls to renew should have been made.
	*/

	lock = l.getLockState()
	service := s.getServiceState()

	must.True(t, lock.locked)
	must.Eq(t, 1, lock.acquireCalls[hac1.ID])
	must.Eq(t, 0, lock.acquireCalls[hac2.ID])

	must.Eq(t, 0, lock.renewsCounter)

	must.Eq(t, 1, service.startsCounter)
	must.StrContains(t, hac1.ID, service.starterID)

	time.Sleep(6 * time.Millisecond)
	/*
		After 6 ms more (10 ms total):
		* hac2 should have tried to acquire the lock at least once, after the 6ms
			initial delay has passed.
		* hc1 should have renewed once the lease and still hold the lock.
	*/
	lock = l.getLockState()
	service = s.getServiceState()
	must.True(t, lock.locked)
	must.Eq(t, 1, lock.acquireCalls[hac1.ID])
	must.Eq(t, 1, lock.acquireCalls[hac2.ID])

	must.One(t, lock.renewsCounter)

	must.One(t, service.startsCounter)
	must.StrContains(t, hac1.ID, service.starterID)

	time.Sleep(5 * time.Millisecond)

	/*
		After 5 ms more (15 ms total):
		* hac2 should have tried to acquire the lock still just once:
				initialDelay(6) + waitTime(11) = 18.
		* hac1 should have renewed the lease 2 times and still hold the lock:
				initialDelay(0) + renewals(2) * renewalPeriod(7) = 14.
	*/

	lock = l.getLockState()
	service = s.getServiceState()
	must.Eq(t, 1, lock.acquireCalls[hac1.ID])
	must.Eq(t, 1, lock.acquireCalls[hac2.ID])

	must.True(t, lock.locked)

	must.Eq(t, 2, lock.renewsCounter)
	must.Eq(t, 1, service.startsCounter)
	must.StrContains(t, hac1.ID, service.starterID)

	time.Sleep(15 * time.Millisecond)

	/*
		After 15 ms more (30 ms total):
		* hac2 should have tried to acquire the lock 3 times:
				initialDelay(6) + calls(2)* waitTime(11) = 28.
		* hac1 should have renewed the lease 4 times and still hold the lock:
				initialDelay(0) + renewals(4) * renewalPeriod(7) = 28.
	*/

	lock = l.getLockState()
	service = s.getServiceState()
	must.Eq(t, 1, lock.acquireCalls[hac1.ID])
	must.Eq(t, 3, lock.acquireCalls[hac2.ID])

	must.True(t, lock.locked)

	must.Eq(t, 4, lock.renewsCounter)
	must.Eq(t, 1, service.startsCounter)
	must.StrContains(t, hac1.ID, service.starterID)

	// Start a new instance of the service with ha running, initial delay of 1ms
	hac3 := LockLeaser{
		ID:            "hac3",
		locker:        &l,
		renewalPeriod: time.Duration(float64(testLease) * lockLeaseRenewalFactor),
		waitPeriod:    time.Duration(float64(testLease) * lockRetryBackoffFactor),
		randomDelay:   1 * time.Millisecond,
	}

	go func() {
		err := hac3.Start(testCtx, s.Run(hac3.ID, testCtx))
		must.NoError(t, err)
	}()

	time.Sleep(15 * time.Millisecond)

	/*
		After 15 ms more (45 ms total):
		* hac3 should have tried to acquire the lock twice, once on start and
			once after waitTime(11).
		* hac2 should have tried to acquire the lock 4 times:
				initialDelay(6) + calls(3) * waitTime(11) = 39.
		* hac1 should have renewed the lease 4 times and still hold the lock:
				initialDelay(0) + renewals(6) * renewalPeriod(7) = 42.
	*/

	lock = l.getLockState()
	service = s.getServiceState()
	must.Eq(t, 1, lock.acquireCalls[hac1.ID])
	must.Eq(t, 4, lock.acquireCalls[hac2.ID])
	must.Eq(t, 2, lock.acquireCalls[hac3.ID])

	must.True(t, lock.locked)

	must.Eq(t, 6, lock.renewsCounter)
	must.Eq(t, 1, service.startsCounter)
	must.StrContains(t, hac1.ID, service.starterID)

	// Stop hac1 and release the lock
	hac1Cancel()

	time.Sleep(10 * time.Millisecond)

	/*
		After 10 ms more (55 ms total):
		* hac3 should have tried to acquire the lock 3 times.
		* hac2 should have tried to acquire the lock 5 times and succeeded on the
		 the fifth, is currently holding the lock and Run the service, no renewals.
		* hc1 is stopped.
	*/

	lock = l.getLockState()
	service = s.getServiceState()
	must.Eq(t, 1, lock.acquireCalls[hac1.ID])
	must.Eq(t, 5, lock.acquireCalls[hac2.ID])
	must.Eq(t, 3, lock.acquireCalls[hac3.ID])

	must.True(t, lock.locked)

	must.Eq(t, 0, lock.renewsCounter)
	must.Eq(t, 2, service.startsCounter)
	must.StrContains(t, hac2.ID, service.starterID)

	time.Sleep(5 * time.Millisecond)

	/*
		After 5 ms more (60 ms total):
		* hac3 should have tried to acquire the lock 3 times.
		* hac2 should have renewed the lock once.
		* hc1 is stopped.
	*/

	lock = l.getLockState()
	service = s.getServiceState()
	must.Eq(t, 1, lock.acquireCalls[hac1.ID])
	must.Eq(t, 5, lock.acquireCalls[hac2.ID])
	must.Eq(t, 3, lock.acquireCalls[hac3.ID])

	must.True(t, lock.locked)

	must.Eq(t, 1, lock.renewsCounter)
	must.Eq(t, 2, service.startsCounter)
	must.StrContains(t, hac2.ID, service.starterID)
}

func TestFailedRenewal(t *testing.T) {
	l := mockLock{
		acquireCalls: map[string]int{},
	}

	s := mockService{}

	testCtx, testCancel := context.WithCancel(context.Background())
	defer testCancel()

	// Set the renewal period to 1.5  * testLease (15 ms) to force and error.
	hac := LockLeaser{
		ID:            "hac1",
		locker:        &l,
		renewalPeriod: time.Duration(float64(testLease) * 1.5),
		waitPeriod:    time.Duration(float64(testLease) * lockRetryBackoffFactor),
		randomDelay:   0,
	}

	lock := l.getLockState()
	must.False(t, lock.locked)

	go hac.Start(testCtx, s.Run(hac.ID, testCtx))

	time.Sleep(5 * time.Millisecond)
	/*
		After 5ms, the service should be running and the lock held,
		no renewals needed or performed yet.
	*/

	lock = l.getLockState()
	service := s.getServiceState()
	must.Eq(t, 1, lock.acquireCalls[hac.ID])
	must.True(t, lock.locked)

	must.Eq(t, 0, lock.renewsCounter)
	must.Eq(t, 1, service.startsCounter)
	must.StrContains(t, hac.ID, service.starterID)

	time.Sleep(15 * time.Millisecond)

	/*
		After 15ms (20ms total) hac should have tried and failed at renewing the
		lock, causing the service to return, no new calls to acquire the lock yet
		either.
	*/

	lock = l.getLockState()
	service = s.getServiceState()
	must.Eq(t, 1, lock.acquireCalls[hac.ID])
	must.False(t, lock.locked)

	must.Eq(t, 0, lock.renewsCounter)
	must.Eq(t, 1, service.startsCounter)
	must.StrContains(t, hac.ID, "")

	time.Sleep(10 * time.Millisecond)

	/*
		After 10ms (30ms total) hac should have tried and succeeded at getting
		the lock and the service should be running again.
	*/

	lock = l.getLockState()
	service = s.getServiceState()
	must.Eq(t, 2, lock.acquireCalls[hac.ID])
	must.True(t, lock.locked)

	must.Eq(t, 0, lock.renewsCounter)
	must.Eq(t, 2, service.startsCounter)
	must.StrContains(t, hac.ID, service.starterID)
}

func TestStart_ProtectedFunctionError(t *testing.T) {
	l := mockLock{
		acquireCalls: map[string]int{},
	}

	testCtx := context.Background()

	hac := LockLeaser{
		locker:        &l,
		renewalPeriod: time.Duration(float64(testLease) * lockLeaseRenewalFactor),
		waitPeriod:    time.Duration(float64(testLease) * lockRetryBackoffFactor),
	}

	lock := l.getLockState()
	must.False(t, lock.locked)

	err := hac.Start(testCtx, func(ctx context.Context) error {
		return errors.New("error")
	})

	must.Error(t, err)

	lock = l.getLockState()
	must.False(t, lock.locked)
	must.Zero(t, lock.renewsCounter)
}

func copyMap(originalMap map[string]int) map[string]int {
	newMap := map[string]int{}
	for k, v := range originalMap {
		newMap[k] = v
	}
	return newMap
}
