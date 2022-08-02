package agent

import (
	"encoding/base64"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestHTTP_Keyring_CRUD(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, nil, func(s *TestAgent) {

		respW := httptest.NewRecorder()

		// Rotate

		req, err := http.NewRequest(http.MethodPut, "/v1/operator/keyring/rotate", nil)
		require.NoError(t, err)
		obj, err := s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)
		require.NotZero(t, respW.HeaderMap.Get("X-Nomad-Index"))
		rotateResp := obj.(structs.KeyringRotateRootKeyResponse)
		require.NotNil(t, rotateResp.Key)
		require.True(t, rotateResp.Key.Active())
		newID1 := rotateResp.Key.KeyID

		// List

		req, err = http.NewRequest(http.MethodGet, "/v1/operator/keyring/keys", nil)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)
		listResp := obj.([]*structs.RootKeyMeta)
		require.Len(t, listResp, 2)
		for _, key := range listResp {
			if key.KeyID == newID1 {
				require.True(t, key.Active(), "new key should be active")
			} else {
				require.False(t, key.Active(), "initial key should be inactive")
			}
		}

		// Update

		keyMeta := rotateResp.Key
		keyBuf := make([]byte, 32)
		rand.Read(keyBuf)
		encodedKey := base64.StdEncoding.EncodeToString(keyBuf)

		newID2 := uuid.Generate()

		key := &api.RootKey{
			Meta: &api.RootKeyMeta{
				State:     api.RootKeyStateActive,
				KeyID:     newID2,
				Algorithm: api.EncryptionAlgorithm(keyMeta.Algorithm),
			},
			Key: encodedKey,
		}
		reqBuf := encodeReq(key)

		req, err = http.NewRequest(http.MethodPut, "/v1/operator/keyring/keys", reqBuf)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)
		updateResp := obj.(structs.KeyringUpdateRootKeyResponse)
		require.NotNil(t, updateResp)

		// Delete the old key and verify its gone

		id := rotateResp.Key.KeyID
		req, err = http.NewRequest(http.MethodDelete, "/v1/operator/keyring/key/"+id, nil)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)

		req, err = http.NewRequest(http.MethodGet, "/v1/operator/keyring/keys", nil)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)
		listResp = obj.([]*structs.RootKeyMeta)
		require.Len(t, listResp, 2)

		for _, key := range listResp {
			require.NotEqual(t, newID1, key.KeyID)
			if key.KeyID == newID2 {
				require.True(t, key.Active(), "new key should be active")
			} else {
				require.False(t, key.Active(), "initial key should be inactive")
			}
		}
	})
}
