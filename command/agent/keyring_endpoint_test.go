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
		require.True(t, rotateResp.Key.Active)

		// List

		req, err = http.NewRequest(http.MethodGet, "/v1/operator/keyring/keys", nil)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)
		listResp := obj.([]*structs.RootKeyMeta)
		require.Len(t, listResp, 1)
		require.True(t, listResp[0].Active)

		// Update

		keyMeta := rotateResp.Key
		keyBuf := make([]byte, 128)
		rand.Read(keyBuf)
		encodedKey := make([]byte, base64.StdEncoding.EncodedLen(128))
		base64.StdEncoding.Encode(encodedKey, keyBuf)

		newID := uuid.Generate()

		key := &api.RootKey{
			Meta: &api.RootKeyMeta{
				Active:           true,
				KeyID:            newID,
				Algorithm:        api.EncryptionAlgorithm(keyMeta.Algorithm),
				EncryptionsCount: 500,
			},
			Key: string(encodedKey),
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
		require.Len(t, listResp, 1)
		require.True(t, listResp[0].Active)
		require.Equal(t, newID, listResp[0].KeyID)

	})
}
