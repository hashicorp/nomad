// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestStateStore_WrappedRootKey_CRUD(t *testing.T) {
	ci.Parallel(t)
	store := testStateStore(t)
	index, err := store.LatestIndex()
	must.NoError(t, err)

	// create 3 default keys, one of which is active
	keyIDs := []string{}
	for i := 0; i < 3; i++ {
		key := structs.NewRootKeyMeta()
		keyIDs = append(keyIDs, key.KeyID)
		if i == 0 {
			key.State = structs.RootKeyStateActive
		}
		index++
		wrappedKeys := structs.NewRootKey(key)
		must.NoError(t, store.UpsertRootKey(index, wrappedKeys, false))
	}

	// retrieve the active key
	activeKey, err := store.GetActiveRootKey(nil)
	must.NoError(t, err)
	must.NotNil(t, activeKey)

	// update an inactive key to active and verify the rotation
	inactiveKey, err := store.RootKeyByID(nil, keyIDs[1])
	must.NoError(t, err)
	must.NotNil(t, inactiveKey)
	oldCreateIndex := inactiveKey.CreateIndex
	newlyActiveKey := inactiveKey.Copy()
	newlyActiveKey = inactiveKey.MakeActive()
	index++
	must.NoError(t, store.UpsertRootKey(index, newlyActiveKey, false))

	iter, err := store.RootKeys(nil)
	must.NoError(t, err)
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		key := raw.(*structs.RootKey)
		if key.KeyID == newlyActiveKey.KeyID {
			must.True(t, key.IsActive(), must.Sprint("expected updated key to be active"))
			must.Eq(t, oldCreateIndex, key.CreateIndex)
		} else {
			must.False(t, key.IsActive(), must.Sprint("expected other keys to be inactive"))
		}
	}

	// delete the active key and verify it's been deleted
	index++
	must.NoError(t, store.DeleteRootKey(index, keyIDs[1]))

	iter, err = store.RootKeys(nil)
	must.NoError(t, err)
	var found int
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		key := raw.(*structs.RootKey)
		must.NotEq(t, keyIDs[1], key.KeyID)
		must.False(t, key.IsActive(), must.Sprint("expected remaining keys to be inactive"))
		found++
	}
	must.Eq(t, 2, found, must.Sprint("expected only 2 keys remaining"))

	// deleting non-existent keys is safe
	must.NoError(t, store.DeleteRootKey(index, uuid.Generate()))
}

func TestStateStore_IsRootKeyInUse(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name string
		fn   func(*StateStore)
	}{
		{
			name: "in use by alloc",
			fn: func(store *StateStore) {

				keyID := uuid.Generate()

				mockAlloc := mock.Alloc()
				mockAlloc.SigningKeyID = keyID

				must.NoError(t, store.UpsertAllocs(
					structs.MsgTypeTestSetup,
					100,
					[]*structs.Allocation{mockAlloc},
				))

				isInUse, err := store.IsRootKeyInUse(keyID)
				must.NoError(t, err)
				must.True(t, isInUse)
			},
		},
		{
			name: "in use by variable",
			fn: func(store *StateStore) {

				keyID := uuid.Generate()

				mockVariable := mock.VariableEncrypted()
				mockVariable.KeyID = keyID

				stateResp := store.VarSet(110,
					&structs.VarApplyStateRequest{Var: mockVariable, Op: structs.VarOpSet},
				)

				must.NoError(t, stateResp.Error)
				must.Eq(t, structs.VarOpResultOk, stateResp.Result)

				isInUse, err := store.IsRootKeyInUse(keyID)
				must.NoError(t, err)
				must.True(t, isInUse)
			},
		},
		{
			name: "in use by node",
			fn: func(store *StateStore) {
				keyID := uuid.Generate()

				mockNode := mock.Node()
				mockNode.IdentitySigningKeyID = keyID

				must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 120, mockNode))

				isInUse, err := store.IsRootKeyInUse(keyID)
				must.NoError(t, err)
				must.True(t, isInUse)
			},
		},
		{
			name: "not in use",
			fn: func(store *StateStore) {

				// Generate a random key ID to use to sign all the state
				// objects.
				keyID := uuid.Generate()

				// Create a node, variable, and alloc that all use the same key
				// and write them to the store.
				mockNode := mock.Node()
				mockNode.IdentitySigningKeyID = keyID

				must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 130, mockNode))

				mockVariable := mock.VariableEncrypted()
				mockVariable.KeyID = keyID

				stateResp := store.VarSet(140,
					&structs.VarApplyStateRequest{Var: mockVariable, Op: structs.VarOpSet},
				)

				must.NoError(t, stateResp.Error)
				must.Eq(t, structs.VarOpResultOk, stateResp.Result)

				mockAlloc := mock.Alloc()
				mockAlloc.SigningKeyID = keyID

				must.NoError(t, store.UpsertAllocs(
					structs.MsgTypeTestSetup,
					150,
					[]*structs.Allocation{mockAlloc},
				))

				// Perform a check using a different key ID to ensure we get the
				// expected result.
				isInUse, err := store.IsRootKeyInUse(uuid.Generate())
				must.NoError(t, err)
				must.False(t, isInUse)
			},
		},
	}

	testStore := testStateStore(t)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.fn(testStore)
		})
	}
}
