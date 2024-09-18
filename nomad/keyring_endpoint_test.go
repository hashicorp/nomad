// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

// TestKeyringEndpoint_CRUD exercises the basic keyring operations
func TestKeyringEndpoint_CRUD(t *testing.T) {

	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForKeyring(t, srv.RPC, "global")
	codec := rpcClient(t, srv)

	// Upsert a new key

	key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	require.NoError(t, err)
	id := key.Meta.KeyID
	key = key.MakeActive()

	updateReq := &structs.KeyringUpdateRootKeyRequest{
		RootKey:      key,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var updateResp structs.KeyringUpdateRootKeyResponse

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.EqualError(t, err, structs.ErrPermissionDenied.Error())

	updateReq.AuthToken = rootToken.SecretID
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), updateResp.Index)

	// Get doesn't need a token here because it uses mTLS role verification
	getReq := &structs.KeyringGetRootKeyRequest{
		KeyID:        id,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var getResp structs.KeyringGetRootKeyResponse

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Get", getReq, &getResp)
	require.NoError(t, err)
	require.Equal(t, updateResp.Index, getResp.Index)
	require.Equal(t, structs.EncryptionAlgorithmAES256GCM, getResp.Key.Meta.Algorithm)

	// Make a blocking query for List and wait for an Update. Note
	// that Get queries don't need ACL tokens in the test server
	// because they always pass the mTLS check

	errCh := make(chan error, 1)
	var listResp structs.KeyringListRootKeyMetaResponse

	go func() {
		defer close(errCh)
		codec := rpcClient(t, srv) // not safe to share across goroutines
		listReq := &structs.KeyringListRootKeyMetaRequest{
			QueryOptions: structs.QueryOptions{
				Region:        "global",
				MinQueryIndex: getResp.Index,
				AuthToken:     rootToken.SecretID,
			},
		}
		errCh <- msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
	}()

	updateReq.RootKey.Meta.CreateTime = time.Now().UTC().UnixNano()
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), updateResp.Index)

	// wait for the blocking query to complete and check the response
	require.NoError(t, <-errCh)
	require.Equal(t, listResp.Index, updateResp.Index)
	require.Len(t, listResp.Keys, 2) // bootstrap + new one

	// Delete the key and verify that it's gone

	delReq := &structs.KeyringDeleteRootKeyRequest{
		KeyID:        id,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var delResp structs.KeyringDeleteRootKeyResponse

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Delete", delReq, &delResp)
	require.EqualError(t, err, structs.ErrPermissionDenied.Error())

	delReq.AuthToken = rootToken.SecretID
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Delete", delReq, &delResp)
	require.EqualError(t, err, "active root key cannot be deleted - call rotate first")

	// set inactive
	updateReq.RootKey = updateReq.RootKey.MakeInactive()
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Delete", delReq, &delResp)
	require.NoError(t, err)
	require.Greater(t, delResp.Index, getResp.Index)

	listReq := &structs.KeyringListRootKeyMetaRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: rootToken.SecretID,
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
	require.NoError(t, err)
	require.Greater(t, listResp.Index, getResp.Index)
	require.Len(t, listResp.Keys, 1) // just the bootstrap key
}

// TestKeyringEndpoint_validateUpdate exercises all the various
// validations we make for the update RPC
func TestKeyringEndpoint_InvalidUpdates(t *testing.T) {

	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForKeyring(t, srv.RPC, "global")
	codec := rpcClient(t, srv)

	// Setup an existing key
	key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	require.NoError(t, err)
	id := key.Meta.KeyID
	key = key.MakeActive()

	updateReq := &structs.KeyringUpdateRootKeyRequest{
		RootKey: key,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: rootToken.SecretID,
		},
	}
	var updateResp structs.KeyringUpdateRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)

	testCases := []struct {
		key            *structs.UnwrappedRootKey
		expectedErrMsg string
	}{
		{
			key:            &structs.UnwrappedRootKey{},
			expectedErrMsg: "root key metadata is required",
		},
		{
			key:            &structs.UnwrappedRootKey{Meta: &structs.RootKeyMeta{}},
			expectedErrMsg: "root key UUID is required",
		},
		{
			key:            &structs.UnwrappedRootKey{Meta: &structs.RootKeyMeta{KeyID: "invalid"}},
			expectedErrMsg: "root key UUID is required",
		},
		{
			key: &structs.UnwrappedRootKey{Meta: &structs.RootKeyMeta{
				KeyID:     id,
				Algorithm: structs.EncryptionAlgorithmAES256GCM,
			}},
			expectedErrMsg: "root key state \"\" is invalid",
		},
		{
			key: &structs.UnwrappedRootKey{Meta: &structs.RootKeyMeta{
				KeyID:     id,
				Algorithm: structs.EncryptionAlgorithmAES256GCM,
				State:     structs.RootKeyStateActive,
			}},
			expectedErrMsg: "root key material is required",
		},

		{
			key: &structs.UnwrappedRootKey{
				Key: []byte{0x01},
				Meta: &structs.RootKeyMeta{
					KeyID:     id,
					Algorithm: "whatever",
					State:     structs.RootKeyStateActive,
				}},
			expectedErrMsg: "root key algorithm cannot be changed after a key is created",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.expectedErrMsg, func(t *testing.T) {
			updateReq := &structs.KeyringUpdateRootKeyRequest{
				RootKey: tc.key,
				WriteRequest: structs.WriteRequest{
					Region:    "global",
					AuthToken: rootToken.SecretID,
				},
			}
			var updateResp structs.KeyringUpdateRootKeyResponse
			err := msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
			require.EqualError(t, err, tc.expectedErrMsg)
		})
	}

}

// TestKeyringEndpoint_Rotate exercises the key rotation logic
func TestKeyringEndpoint_Rotate(t *testing.T) {

	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForKeyring(t, srv.RPC, "global")
	codec := rpcClient(t, srv)

	store := srv.fsm.State()
	key0, err := store.GetActiveRootKey(nil)
	must.NoError(t, err)

	// Setup an existing key
	key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	must.NoError(t, err)
	key1 := key.Meta

	updateReq := &structs.KeyringUpdateRootKeyRequest{
		RootKey: key,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: rootToken.SecretID,
		},
	}
	var updateResp structs.KeyringUpdateRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	must.NoError(t, err)

	// Rotate the key

	rotateReq := &structs.KeyringRotateRootKeyRequest{
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}
	var rotateResp structs.KeyringRotateRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	must.EqError(t, err, structs.ErrPermissionDenied.Error())

	rotateReq.AuthToken = rootToken.SecretID
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	must.NoError(t, err)
	must.Greater(t, updateResp.Index, rotateResp.Index)
	key2 := rotateResp.Key

	// Verify we have a new key and the old one is inactive

	listReq := &structs.KeyringListRootKeyMetaRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: rootToken.SecretID,
		},
	}
	var listResp structs.KeyringListRootKeyMetaResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
	must.NoError(t, err)
	must.Greater(t, updateResp.Index, listResp.Index)
	must.Len(t, 3, listResp.Keys) // bootstrap + old + new

	for _, keyMeta := range listResp.Keys {
		switch keyMeta.KeyID {
		case key0.KeyID, key1.KeyID:
			must.True(t, keyMeta.IsInactive(), must.Sprint("older keys must be inactive"))
		case key2.KeyID:
			must.True(t, keyMeta.IsActive(), must.Sprint("expected new key to be active"))
		}
	}

	getReq := &structs.KeyringGetRootKeyRequest{
		KeyID: key2.KeyID,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var getResp structs.KeyringGetRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Get", getReq, &getResp)
	must.NoError(t, err)
	must.Len(t, 32, getResp.Key.Key)

	// Rotate the key with prepublishing

	publishTime := time.Now().Add(24 * time.Hour).UnixNano()
	rotateResp = structs.KeyringRotateRootKeyResponse{}
	rotateReq = &structs.KeyringRotateRootKeyRequest{
		PublishTime: publishTime,
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: rootToken.SecretID,
		},
	}
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	must.NoError(t, err)
	must.Greater(t, updateResp.Index, rotateResp.Index)
	key3 := rotateResp.Key

	listResp = structs.KeyringListRootKeyMetaResponse{}
	err = msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
	must.NoError(t, err)
	must.Greater(t, updateResp.Index, listResp.Index)
	must.Len(t, 4, listResp.Keys) // bootstrap + old + new + prepublished

	for _, keyMeta := range listResp.Keys {
		switch keyMeta.KeyID {
		case key0.KeyID, key1.KeyID:
			must.True(t, keyMeta.IsInactive(), must.Sprint("older keys must be inactive"))
		case key2.KeyID:
			must.True(t, keyMeta.IsActive(), must.Sprint("expected active key to remain active"))
		case key3.KeyID:
			must.True(t, keyMeta.IsPrepublished(), must.Sprint("expected new key to be prepublished"))
		}
	}
}

// TestKeyringEndpoint_ListPublic asserts the Keyring.ListPublic RPC returns
// all keys which may be in use for active crytpographic material (variables,
// valid JWTs).
func TestKeyringEndpoint_ListPublic(t *testing.T) {

	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForKeyring(t, srv.RPC, "global")
	codec := rpcClient(t, srv)

	// Assert 1 key exists and normal fields are set
	req := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: "ignored!",
		},
	}
	var resp structs.KeyringListPublicResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Keyring.ListPublic", &req, &resp))
	must.Eq(t, srv.config.RootKeyRotationThreshold, resp.RotationThreshold)
	must.Len(t, 1, resp.PublicKeys)
	must.NonZero(t, resp.Index)

	// Rotate keys and assert there are now 2 keys
	rotateReq := &structs.KeyringRotateRootKeyRequest{
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: rootToken.SecretID,
		},
	}
	var rotateResp structs.KeyringRotateRootKeyResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp))
	must.NotEq(t, resp.Index, rotateResp.Index)

	// Verify we have a new key and the old one is inactive
	var resp2 structs.KeyringListPublicResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Keyring.ListPublic", &req, &resp2))
	must.Eq(t, srv.config.RootKeyRotationThreshold, resp2.RotationThreshold)
	must.Len(t, 2, resp2.PublicKeys)
	must.NonZero(t, resp2.Index)

	found := false
	for _, pk := range resp2.PublicKeys {
		if pk.KeyID == resp.PublicKeys[0].KeyID {
			must.False(t, found, must.Sprint("found the original public key twice"))
			found = true
			must.Eq(t, resp.PublicKeys[0], pk)
			break
		}
	}
	must.True(t, found, must.Sprint("original public key missing after rotation"))
}

// TestKeyringEndpoint_GetConfig_Issuer asserts that GetConfig returns OIDC
// Discovery Configuration if an issuer is configured.
func TestKeyringEndpoint_GetConfig_Issuer(t *testing.T) {
	ci.Parallel(t)

	// Set OIDCIssuer to a valid looking (but fake) issuer
	const testIssuer = "https://oidc.test.nomadproject.io/"

	srv, _, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue

		c.OIDCIssuer = testIssuer
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	req := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: "ignored!",
		},
	}
	var resp structs.KeyringGetConfigResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Keyring.GetConfig", &req, &resp))
	must.NotNil(t, resp.OIDCDiscovery)
	must.Eq(t, testIssuer, resp.OIDCDiscovery.Issuer)
	must.StrHasPrefix(t, testIssuer, resp.OIDCDiscovery.JWKS)
}

// TestKeyringEndpoint_GetConfig_Disabled asserts that GetConfig returns
// nothing if an issuer is NOT configured. OIDC Discovery cannot work without
// an issuer set, and there's no sensible default for Nomad to choose.
func TestKeyringEndpoint_GetConfig_Disabled(t *testing.T) {
	ci.Parallel(t)
	srv, _, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	req := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: "ignored!",
		},
	}
	var resp structs.KeyringGetConfigResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Keyring.GetConfig", &req, &resp))
	must.Nil(t, resp.OIDCDiscovery)
}
