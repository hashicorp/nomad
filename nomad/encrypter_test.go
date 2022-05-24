package nomad

import (
	"path/filepath"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

// TestEncrypter_LoadSave exercises round-tripping keys to disk
func TestEncrypter_LoadSave(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()
	encrypter, err := NewEncrypter(tmpDir)
	require.NoError(t, err)

	algos := []structs.EncryptionAlgorithm{
		structs.EncryptionAlgorithmAES256GCM,
		structs.EncryptionAlgorithmXChaCha20,
	}

	for _, algo := range algos {
		t.Run(string(algo), func(t *testing.T) {
			key, err := structs.NewRootKey(algo)
			require.NoError(t, err)
			require.NoError(t, encrypter.SaveKeyToStore(key))

			gotKey, err := encrypter.LoadKeyFromStore(
				filepath.Join(tmpDir, key.Meta.KeyID+".json"))
			require.NoError(t, err)
			require.NoError(t, encrypter.AddKey(gotKey))
		})
	}
}

// TestEncrypter_Restore exercises the entire reload of a keystore,
// including pairing metadfata with key material
func TestEncrypter_Restore(t *testing.T) {

	ci.Parallel(t)

	// use a known tempdir so that we can restore from it
	tmpDir := t.TempDir()

	srv, rootToken, shutdown := TestACLServer(t, func(c *Config) {
		c.NodeName = "node1"
		c.NumSchedulers = 0
		c.DevMode = false
		c.DataDir = tmpDir
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	nodeID := srv.GetConfig().NodeID

	// Send a few key rotations to add keys

	rotateReq := &structs.KeyringRotateRootKeyRequest{
		WriteRequest: structs.WriteRequest{
			Region:    "global",
			AuthToken: rootToken.SecretID,
		},
	}
	var rotateResp structs.KeyringRotateRootKeyResponse
	for i := 0; i < 4; i++ {
		err := msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
		require.NoError(t, err)
	}

	shutdown()

	srv2, rootToken, shutdown2 := TestACLServer(t, func(c *Config) {
		c.NodeID = nodeID
		c.NodeName = "node1"
		c.NumSchedulers = 0
		c.DevMode = false
		c.DataDir = tmpDir
	})
	defer shutdown2()
	testutil.WaitForLeader(t, srv2.RPC)
	codec = rpcClient(t, srv2)

	// Verify we've restored all the keys from the old keystore

	listReq := &structs.KeyringListRootKeyMetaRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var listResp structs.KeyringListRootKeyMetaResponse
	err := msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
	require.NoError(t, err)
	require.Len(t, listResp.Keys, 4)
}
