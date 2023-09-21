// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func TestServer_restoreLockTTLTimers(t *testing.T) {
	ci.Parallel(t)

	testServer, testServerCleanup := TestServer(t, nil)
	defer testServerCleanup()
	testutil.WaitForLeader(t, testServer.RPC)

	// Generate two variables, one with and one without a lock and upsert these
	// into state.
	mockVar1 := mock.VariableEncrypted()

	mockVar2 := mock.VariableEncrypted()
	mockVar2.Lock = &structs.VariableLock{
		ID:        uuid.Generate(),
		TTL:       15 * time.Second,
		LockDelay: 15 * time.Second,
	}

	upsertResp1 := testServer.fsm.State().VarSet(10, &structs.VarApplyStateRequest{Var: mockVar1, Op: structs.VarOpSet})
	must.NoError(t, upsertResp1.Error)

	upsertResp2 := testServer.fsm.State().VarSet(20, &structs.VarApplyStateRequest{Var: mockVar2, Op: structs.VarOpLockAcquire})
	must.NoError(t, upsertResp2.Error)

	// Call the server function that restores the lock TTL timers. This would
	// usually happen on leadership transition.
	must.NoError(t, testServer.restoreLockTTLTimers())

	// Ensure the TTL timer tracking has the expected entries.
	must.Nil(t, testServer.lockTTLTimer.Get(mockVar1.LockID()))
	must.NotNil(t, testServer.lockTTLTimer.Get(mockVar2.LockID()))
}

func TestServer_invalidateVariableLock(t *testing.T) {
	ci.Parallel(t)

	testServer, testServerCleanup := TestServer(t, nil)
	defer testServerCleanup()
	testutil.WaitForLeader(t, testServer.RPC)

	// Generate a variable that includes a lock entry and upsert this into our
	// state.
	mockVar1 := mock.VariableEncrypted()
	mockVar1.Lock = &structs.VariableLock{
		ID:        uuid.Generate(),
		TTL:       15 * time.Second,
		LockDelay: 15 * time.Second,
	}

	upsertResp1 := testServer.fsm.State().VarSet(10, &structs.VarApplyStateRequest{Var: mockVar1, Op: structs.VarOpLockAcquire})
	must.NoError(t, upsertResp1.Error)

	// Create the timer manually, so we can control the invalidation for
	// testing.
	testServer.lockTTLTimer.Create(mockVar1.LockID(), mockVar1.Lock.TTL, func() {})

	// Perform the invalidation call.
	testServer.invalidateVariableLock(*mockVar1)

	// Pull the variable out of state and check that the lock ID has been
	// removed.
	varGetResp, err := testServer.fsm.State().GetVariable(nil, mockVar1.Namespace, mockVar1.Path)
	must.NoError(t, err)
	must.Nil(t, varGetResp.Lock)
}

func TestServer_createVariableLockTimer(t *testing.T) {
	ci.Parallel(t)

	testServer, testServerCleanup := TestServer(t, nil)
	defer testServerCleanup()
	testutil.WaitForLeader(t, testServer.RPC)

	// Generate a variable that includes a lock entry and upsert this into our
	// state.
	mockVar1 := mock.VariableEncrypted()
	mockVar1.Lock = &structs.VariableLock{
		ID:        uuid.Generate(),
		TTL:       10 * time.Millisecond,
		LockDelay: 10 * time.Millisecond,
	}

	upsertResp1 := testServer.fsm.State().VarSet(10, &structs.VarApplyStateRequest{Var: mockVar1, Op: structs.VarOpLockAcquire})
	must.NoError(t, upsertResp1.Error)

	testServer.CreateVariableLockTTLTimer(*mockVar1)

	time.Sleep(10 * time.Millisecond)

	// Check the timer is still present, meaning it didn't expired and the variable
	// still holds a lock
	must.NotNil(t, testServer.lockTTLTimer.Get(mockVar1.LockID()))
	varGetResp, err := testServer.fsm.State().GetVariable(nil, mockVar1.Namespace, mockVar1.Path)
	must.NoError(t, err)
	must.Eq(t, mockVar1.LockID(), varGetResp.LockID())

	// After 15ms more, the TTL has expired, no timer should be running but the variable
	// must still hold the lock.
	time.Sleep(15 * time.Millisecond)
	must.Nil(t, testServer.lockTTLTimer.Get(mockVar1.LockID()))
	varGetResp, err = testServer.fsm.State().GetVariable(nil, mockVar1.Namespace, mockVar1.Path)
	must.NoError(t, err)
	must.Eq(t, mockVar1.LockID(), varGetResp.LockID())

	// After 10ms more, the delay should have expired as well and the variable
	// should not hold the lock
	time.Sleep(10 * time.Millisecond)
	must.Nil(t, testServer.lockTTLTimer.Get(mockVar1.LockID()))
	varGetResp, err = testServer.fsm.State().GetVariable(nil, mockVar1.Namespace, mockVar1.Path)
	must.NoError(t, err)
	must.Nil(t, varGetResp.Lock)
}

func TestServer_createAndRenewVariableLockTimer(t *testing.T) {

	ci.Parallel(t)

	testServer, testServerCleanup := TestServer(t, nil)
	defer testServerCleanup()
	testutil.WaitForLeader(t, testServer.RPC)

	// Generate a variable that includes a lock entry and upsert this into our
	// state.
	mockVar1 := mock.VariableEncrypted()
	mockVar1.Lock = &structs.VariableLock{
		ID:        uuid.Generate(),
		TTL:       10 * time.Millisecond,
		LockDelay: 10 * time.Millisecond,
	}

	// Attempt to renew a lock that has no timer yet
	err := testServer.RenewTTLTimer(*mockVar1)

	if !errors.Is(errTimerNotFound, err) {
		t.Fatalf("expected error, got %s", err)
	}

	upsertResp1 := testServer.fsm.State().VarSet(10, &structs.VarApplyStateRequest{Var: mockVar1, Op: structs.VarOpLockAcquire})
	must.NoError(t, upsertResp1.Error)

	testServer.CreateVariableLockTTLTimer(*mockVar1)

	time.Sleep(10 * time.Millisecond)

	// Check the timer is still present, meaning it didn't expired and the variable
	// still holds a lock
	must.NotNil(t, testServer.lockTTLTimer.Get(mockVar1.LockID()))
	varGetResp, err := testServer.fsm.State().GetVariable(nil, mockVar1.Namespace, mockVar1.Path)
	must.NoError(t, err)
	must.Eq(t, mockVar1.LockID(), varGetResp.LockID())

	for i := 0; i < 3; i++ {
		// Renew the lock
		err = testServer.RenewTTLTimer(*mockVar1)
		must.NoError(t, err)

		// After 10 ms the timer is must still present, meaning it was correctly renewed
		// and the variable still holds a lock.
		time.Sleep(10 * time.Millisecond)

		must.NotNil(t, testServer.lockTTLTimer.Get(mockVar1.LockID()))
		varGetResp, err = testServer.fsm.State().GetVariable(nil, mockVar1.Namespace, mockVar1.Path)
		must.NoError(t, err)
		must.Eq(t, mockVar1.LockID(), varGetResp.LockID())
	}

	// After 15ms more, the TTL has expired, no timer should be running but the variable
	// must still hold the lock.
	time.Sleep(15 * time.Millisecond)

	must.Nil(t, testServer.lockTTLTimer.Get(mockVar1.LockID()))
	varGetResp, err = testServer.fsm.State().GetVariable(nil, mockVar1.Namespace, mockVar1.Path)
	must.NoError(t, err)
	must.Eq(t, mockVar1.LockID(), varGetResp.LockID())

	// After 10ms more, the delay should have expired as well and the variable
	// should not hold the lock
	time.Sleep(10 * time.Millisecond)

	must.Nil(t, testServer.lockTTLTimer.Get(mockVar1.LockID()))
	varGetResp, err = testServer.fsm.State().GetVariable(nil, mockVar1.Namespace, mockVar1.Path)
	must.NoError(t, err)
	must.Nil(t, varGetResp.Lock)
}
