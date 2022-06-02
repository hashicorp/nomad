package api

import (
	"encoding/base64"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/api/internal/testutil"
)

func TestKeyring_CRUD(t *testing.T) {
	testutil.Parallel(t)
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	kr := c.Keyring()

	// Create a key by requesting a rotation
	key, wm, err := kr.Rotate(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, key)
	assertWriteMeta(t, wm)

	// Read all the keys
	keys, qm, err := kr.List(&QueryOptions{WaitIndex: key.CreateIndex})
	require.NoError(t, err)
	assertQueryMeta(t, qm)
	require.Len(t, keys, 2)

	// Write a new active key, forcing a rotation
	id := "fd77c376-9785-4c80-8e62-4ec3ab5f8b9a"
	buf := make([]byte, 32)
	rand.Read(buf)
	encodedKey := base64.StdEncoding.EncodeToString(buf)

	wm, err = kr.Update(&RootKey{
		Key: encodedKey,
		Meta: &RootKeyMeta{
			KeyID:     id,
			Active:    true,
			Algorithm: EncryptionAlgorithmAES256GCM,
		}}, nil)
	require.NoError(t, err)
	assertWriteMeta(t, wm)

	// Delete the old key
	wm, err = kr.Delete(&KeyringDeleteOptions{KeyID: keys[0].KeyID}, nil)
	require.NoError(t, err)
	assertWriteMeta(t, wm)

	// Read all the keys back
	keys, qm, err = kr.List(&QueryOptions{WaitIndex: key.CreateIndex})
	require.NoError(t, err)
	assertQueryMeta(t, qm)
	require.Len(t, keys, 2)
	for _, key := range keys {
		if key.KeyID == id {
			require.True(t, key.Active, "new key should be active")
		} else {
			require.False(t, key.Active, "initial key should be inactive")
		}
	}
}
