// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestKeyring_CRUD(t *testing.T) {
	testutil.Parallel(t)

	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	kr := c.Keyring()

	// Find the bootstrap key
	keys, qm, err := kr.List(nil)
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 1, keys)
	oldKeyID := keys[0].KeyID

	// Create a key by requesting a rotation
	key, wm, err := kr.Rotate(nil, nil)
	must.NoError(t, err)
	must.NotNil(t, key)
	assertWriteMeta(t, wm)

	// Read all the keys
	keys, qm, err = kr.List(&QueryOptions{WaitIndex: key.CreateIndex})
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 2, keys)

	// Delete the old key
	wm, err = kr.Delete(&KeyringDeleteOptions{KeyID: oldKeyID}, nil)
	must.NoError(t, err)
	assertWriteMeta(t, wm)

	// Read all the keys back
	keys, qm, err = kr.List(&QueryOptions{WaitIndex: key.CreateIndex})
	must.NoError(t, err)
	assertQueryMeta(t, qm)
	must.Len(t, 1, keys)
	must.Eq(t, key.KeyID, keys[0].KeyID)
	must.Eq(t, RootKeyState(RootKeyStateActive), keys[0].State)
}
