package nomad

import (
	"sync"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
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
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)
	id := uuid.Generate()

	// Upsert a new key

	updateReq := &structs.KeyringUpdateRootKeyRequest{
		RootKey: &structs.RootKey{
			Meta: &structs.RootKeyMeta{
				KeyID:     id,
				Algorithm: structs.EncryptionAlgorithmXChaCha20,
				Active:    true,
			},
			Key: []byte{},
		},
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var updateResp structs.KeyringUpdateRootKeyResponse
	var err error

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.EqualError(t, err, structs.ErrPermissionDenied.Error())

	updateReq.AuthToken = rootToken.SecretID
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), updateResp.Index)

	// Get and List don't need a token here because they rely on mTLS role verification
	getReq := &structs.KeyringGetRootKeyRequest{
		KeyID:        id,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	var getResp structs.KeyringGetRootKeyResponse

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Get", getReq, &getResp)
	require.NoError(t, err)
	require.Equal(t, updateResp.Index, getResp.Index)
	require.Equal(t, structs.EncryptionAlgorithmXChaCha20, getResp.Key.Meta.Algorithm)

	// Make a blocking query for List and wait for an Update. Note
	// that List/Get queries don't need ACL tokens in the test server
	// because they always pass the mTLS check

	var wg sync.WaitGroup
	wg.Add(1)
	var listResp structs.KeyringListRootKeyMetaResponse

	go func() {
		defer wg.Done()
		codec := rpcClient(t, srv) // not safe to share across goroutines
		listReq := &structs.KeyringListRootKeyMetaRequest{
			QueryOptions: structs.QueryOptions{
				Region:        "global",
				MinQueryIndex: getResp.Index,
			},
		}
		err = msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
		require.NoError(t, err)
	}()

	updateReq.RootKey.Meta.EncryptionsCount++
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)
	require.NotEqual(t, uint64(0), updateResp.Index)

	// wait for the blocking query to complete and check the response
	wg.Wait()
	require.Greater(t, listResp.Index, getResp.Index)
	require.Len(t, listResp.Keys, 1)

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
	updateReq.RootKey.Meta.Active = false
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Delete", delReq, &delResp)
	require.NoError(t, err)
	require.Greater(t, delResp.Index, getResp.Index)

	listReq := &structs.KeyringListRootKeyMetaRequest{
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	err = msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
	require.NoError(t, err)
	require.Greater(t, listResp.Index, getResp.Index)
	require.Len(t, listResp.Keys, 0)
}

// TestKeyringEndpoint_validateUpdate exercises all the various
// validations we make for the update RPC
func TestKeyringEndpoint_InvalidUpdates(t *testing.T) {

	ci.Parallel(t)
	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)
	id := uuid.Generate()

	// Setup an existing key

	updateReq := &structs.KeyringUpdateRootKeyRequest{
		RootKey: &structs.RootKey{
			Meta: &structs.RootKeyMeta{
				KeyID:     id,
				Algorithm: structs.EncryptionAlgorithmXChaCha20,
				Active:    true,
			},
			Key: []byte{},
		},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: rootToken.SecretID,
		},
	}
	var updateResp structs.KeyringUpdateRootKeyResponse
	err := msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)

	testCases := []struct {
		key            *structs.RootKey
		expectedErrMsg string
	}{
		{
			key:            &structs.RootKey{},
			expectedErrMsg: "root key metadata is required",
		},
		{
			key:            &structs.RootKey{Meta: &structs.RootKeyMeta{}},
			expectedErrMsg: "root key UUID is required",
		},
		{
			key:            &structs.RootKey{Meta: &structs.RootKeyMeta{KeyID: "invalid"}},
			expectedErrMsg: "root key UUID is required",
		},
		{
			key: &structs.RootKey{Meta: &structs.RootKeyMeta{
				KeyID:     id,
				Algorithm: structs.EncryptionAlgorithmAES256GCM,
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
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)
	id := uuid.Generate()

	// Setup an existing key

	updateReq := &structs.KeyringUpdateRootKeyRequest{
		RootKey: &structs.RootKey{
			Meta: &structs.RootKeyMeta{
				KeyID:     id,
				Algorithm: structs.EncryptionAlgorithmXChaCha20,
				Active:    true,
			},
			Key: []byte{},
		},
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: rootToken.SecretID,
		},
	}
	var updateResp structs.KeyringUpdateRootKeyResponse
	err := msgpackrpc.CallWithCodec(codec, "Keyring.Update", updateReq, &updateResp)
	require.NoError(t, err)

	// Rotate the key

	rotateReq := &structs.KeyringRotateRootKeyRequest{
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}
	var rotateResp structs.KeyringRotateRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	require.EqualError(t, err, structs.ErrPermissionDenied.Error())

	rotateReq.AuthToken = rootToken.SecretID
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	require.NoError(t, err)
	require.NotEqual(t, updateResp.Index, rotateResp.Index)

	// Verify we have a new key and the old one is inactive

	listReq := &structs.KeyringListRootKeyMetaRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var listResp structs.KeyringListRootKeyMetaResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
	require.NoError(t, err)

	require.Greater(t, listResp.Index, updateResp.Index)
	require.Len(t, listResp.Keys, 2)
	for _, keyMeta := range listResp.Keys {
		if keyMeta.KeyID == id {
			require.False(t, keyMeta.Active, "expected old key to be inactive")
		} else {
			require.True(t, keyMeta.Active, "expected new key to be inactive")
		}
	}

	// TODO: verify that Encrypter has been updated

}
