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
	// TODO: there'll be 2 keys here once we get bootstrapping done
	require.Len(t, keys, 1)

	// Write a new active key, forcing a rotation
	id := "fd77c376-9785-4c80-8e62-4ec3ab5f8b9a"
	buf := make([]byte, 128)
	rand.Read(buf)
	encodedKey := make([]byte, base64.StdEncoding.EncodedLen(128))
	base64.StdEncoding.Encode(encodedKey, buf)

	wm, err = kr.Update(&RootKey{
		Key: string(encodedKey),
		Meta: &RootKeyMeta{
			KeyID:            id,
			Active:           true,
			Algorithm:        EncryptionAlgorithmAES256GCM,
			EncryptionsCount: 100,
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
	// TODO: there'll be 2 keys here once we get bootstrapping done
	require.Len(t, keys, 1)
	require.Equal(t, id, keys[0].KeyID)

}
