package api

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/api/internal/testutil"
)

func TestKeyring_CRUD(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	kr := c.Keyring()

	// Find the bootstrap key
	keys, qm, err := kr.List(nil)
	require.NoError(t, err)
	assertQueryMeta(t, qm)
	require.Len(t, keys, 1)
	oldKeyID := keys[0].KeyID

	// Create a key by requesting a rotation
	key, wm, err := kr.Rotate(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, key)
	assertWriteMeta(t, wm)

	// Read all the keys
	keys, qm, err = kr.List(&QueryOptions{WaitIndex: key.CreateIndex})
	require.NoError(t, err)
	assertQueryMeta(t, qm)
	require.Len(t, keys, 2)

	// Delete the old key
	wm, err = kr.Delete(&KeyringDeleteOptions{KeyID: oldKeyID}, nil)
	require.NoError(t, err)
	assertWriteMeta(t, wm)

	// Read all the keys back
	keys, qm, err = kr.List(&QueryOptions{WaitIndex: key.CreateIndex})
	require.NoError(t, err)
	assertQueryMeta(t, qm)
	require.Len(t, keys, 1)
	require.Equal(t, key.KeyID, keys[0].KeyID)
	require.Equal(t, RootKeyState(RootKeyStateActive), keys[0].State)
}
