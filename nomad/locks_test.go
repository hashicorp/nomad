package nomad

import (
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
	testServer.invalidateVariableLock(mockVar1.LockID(), mockVar1)

	// Ensure the TTL timer has been removed.
	must.Nil(t, testServer.lockTTLTimer.Get(mockVar1.LockID()))

	// Pull the variable out of state and check that the lock ID has been
	// removed.
	_, varGetResp, err := testServer.fsm.State().VarGet(nil, mockVar1.Namespace, mockVar1.Path)
	must.NoError(t, err)
	must.NotNil(t, varGetResp.Lock)
	must.Eq(t, "", varGetResp.LockID())
}
