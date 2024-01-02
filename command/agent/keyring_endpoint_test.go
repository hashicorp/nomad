// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestHTTP_Keyring_CRUD(t *testing.T) {
	ci.Parallel(t)

	httpTest(t, nil, func(s *TestAgent) {

		respW := httptest.NewRecorder()

		// List (get bootstrap key)

		req, err := http.NewRequest(http.MethodGet, "/v1/operator/keyring/keys", nil)
		require.NoError(t, err)
		obj, err := s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)
		listResp := obj.([]*structs.RootKeyMeta)
		require.Len(t, listResp, 1)
		oldKeyID := listResp[0].KeyID

		// Rotate

		req, err = http.NewRequest(http.MethodPut, "/v1/operator/keyring/rotate", nil)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
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
		listResp = obj.([]*structs.RootKeyMeta)
		require.Len(t, listResp, 2)
		for _, key := range listResp {
			if key.KeyID == newID1 {
				require.True(t, key.Active(), "new key should be active")
			} else {
				require.False(t, key.Active(), "initial key should be inactive")
			}
		}

		// Delete the old key and verify its gone

		req, err = http.NewRequest(http.MethodDelete, "/v1/operator/keyring/key/"+oldKeyID, nil)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)

		req, err = http.NewRequest(http.MethodGet, "/v1/operator/keyring/keys", nil)
		require.NoError(t, err)
		obj, err = s.Server.KeyringRequest(respW, req)
		require.NoError(t, err)
		listResp = obj.([]*structs.RootKeyMeta)
		require.Len(t, listResp, 1)
		require.Equal(t, newID1, listResp[0].KeyID)
		require.True(t, listResp[0].Active())
		require.Len(t, listResp, 1)
	})
}
