// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
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
