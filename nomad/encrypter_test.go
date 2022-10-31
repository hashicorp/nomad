package nomad

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

// TestEncrypter_LoadSave exercises round-tripping keys to disk
func TestEncrypter_LoadSave(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()
	encrypter, err := NewEncrypter(&Server{shutdownCtx: context.Background()}, tmpDir)
	require.NoError(t, err)

	algos := []structs.EncryptionAlgorithm{
		structs.EncryptionAlgorithmAES256GCM,
	}

	for _, algo := range algos {
		t.Run(string(algo), func(t *testing.T) {
			key, err := structs.NewRootKey(algo)
			require.NoError(t, err)
			require.NoError(t, encrypter.saveKeyToStore(key))

			gotKey, err := encrypter.loadKeyFromStore(
				filepath.Join(tmpDir, key.Meta.KeyID+".nks.json"))
			require.NoError(t, err)
			require.NoError(t, encrypter.addCipher(gotKey))
		})
	}
}

// TestEncrypter_Restore exercises the entire reload of a keystore,
// including pairing metadata with key material
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

	// Verify we have a bootstrap key

	listReq := &structs.KeyringListRootKeyMetaRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var listResp structs.KeyringListRootKeyMetaResponse

	require.Eventually(t, func() bool {
		msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
		return len(listResp.Keys) == 1
	}, time.Second*5, time.Second, "expected keyring to be initialized")

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

	require.Eventually(t, func() bool {
		msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
		return len(listResp.Keys) == 5 // 4 new + the bootstrap key
	}, time.Second*5, time.Second, "expected keyring to be restored")

	for _, keyMeta := range listResp.Keys {

		getReq := &structs.KeyringGetRootKeyRequest{
			KeyID: keyMeta.KeyID,
			QueryOptions: structs.QueryOptions{
				Region: "global",
			},
		}
		var getResp structs.KeyringGetRootKeyResponse
		err := msgpackrpc.CallWithCodec(codec, "Keyring.Get", getReq, &getResp)
		require.NoError(t, err)

		gotKey := getResp.Key
		require.Len(t, gotKey.Key, 32)
	}
}

// TestEncrypter_KeyringReplication exercises key replication between servers
func TestEncrypter_KeyringReplication(t *testing.T) {

	ci.Parallel(t)

	srv1, cleanupSRV1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.NumSchedulers = 0
	})
	defer cleanupSRV1()

	// add two more servers after we've bootstrapped

	srv2, cleanupSRV2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.NumSchedulers = 0
	})
	defer cleanupSRV2()
	srv3, cleanupSRV3 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.NumSchedulers = 0
	})
	defer cleanupSRV3()

	TestJoin(t, srv1, srv2)
	TestJoin(t, srv1, srv3)

	testutil.WaitForLeader(t, srv1.RPC)
	testutil.WaitForLeader(t, srv2.RPC)
	testutil.WaitForLeader(t, srv3.RPC)

	servers := []*Server{srv1, srv2, srv3}
	var leader *Server

	for _, srv := range servers {
		if ok, _ := srv.getLeader(); ok {
			leader = srv
		}
	}
	require.NotNil(t, leader, "expected there to be a leader")
	codec := rpcClient(t, leader)
	t.Logf("leader is %s", leader.config.NodeName)

	// Verify we have a bootstrap key

	listReq := &structs.KeyringListRootKeyMetaRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var listResp structs.KeyringListRootKeyMetaResponse

	require.Eventually(t, func() bool {
		msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
		return len(listResp.Keys) == 1
	}, time.Second*5, time.Second, "expected keyring to be initialized")

	keyID1 := listResp.Keys[0].KeyID

	keyPath := filepath.Join(leader.GetConfig().DataDir, "keystore",
		keyID1+nomadKeystoreExtension)
	_, err := os.Stat(keyPath)
	require.NoError(t, err, "expected key to be found in leader keystore")

	// Helper function for checking that a specific key has been
	// replicated to followers

	checkReplicationFn := func(keyID string) func() bool {
		return func() bool {
			for _, srv := range servers {
				keyPath := filepath.Join(srv.GetConfig().DataDir, "keystore",
					keyID+nomadKeystoreExtension)
				if _, err := os.Stat(keyPath); err != nil {
					return false
				}
			}
			return true
		}
	}

	// Assert that the bootstrap key has been replicated to followers
	require.Eventually(t, checkReplicationFn(keyID1),
		time.Second*5, time.Second,
		"expected keys to be replicated to followers after bootstrap")

	// Assert that key rotations are replicated to followers

	rotateReq := &structs.KeyringRotateRootKeyRequest{
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}
	var rotateResp structs.KeyringRotateRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	require.NoError(t, err)
	keyID2 := rotateResp.Key.KeyID

	getReq := &structs.KeyringGetRootKeyRequest{
		KeyID: keyID2,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var getResp structs.KeyringGetRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Get", getReq, &getResp)
	require.NoError(t, err)
	require.NotNil(t, getResp.Key, "expected key to be found on leader")

	keyPath = filepath.Join(leader.GetConfig().DataDir, "keystore",
		keyID2+nomadKeystoreExtension)
	_, err = os.Stat(keyPath)
	require.NoError(t, err, "expected key to be found in leader keystore")

	require.Eventually(t, checkReplicationFn(keyID2),
		time.Second*5, time.Second,
		"expected keys to be replicated to followers after rotation")

	// Scenario: simulate a key rotation that doesn't get replicated
	// before a leader election by stopping replication, rotating the
	// key, and triggering a leader election.

	for _, srv := range servers {
		srv.keyringReplicator.stop()
	}

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	require.NoError(t, err)
	keyID3 := rotateResp.Key.KeyID

	err = leader.leadershipTransfer()
	require.NoError(t, err)

	testutil.WaitForLeader(t, leader.RPC)

	for _, srv := range servers {
		if ok, _ := srv.getLeader(); ok {
			t.Logf("new leader is %s", srv.config.NodeName)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Logf("replicating on %s", srv.config.NodeName)
		go srv.keyringReplicator.run(ctx)
	}

	require.Eventually(t, checkReplicationFn(keyID3),
		time.Second*5, time.Second,
		"expected keys to be replicated to followers after election")

	// Scenario: new members join the cluster

	srv4, cleanupSRV4 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 0
		c.NumSchedulers = 0
	})
	defer cleanupSRV4()
	srv5, cleanupSRV5 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 0
		c.NumSchedulers = 0
	})
	defer cleanupSRV5()

	TestJoin(t, srv4, srv5)
	TestJoin(t, srv5, srv1)
	servers = []*Server{srv1, srv2, srv3, srv4, srv5}

	testutil.WaitForLeader(t, srv4.RPC)
	testutil.WaitForLeader(t, srv5.RPC)

	require.Eventually(t, checkReplicationFn(keyID3),
		time.Second*5, time.Second,
		"expected new servers to get replicated keys")

}

func TestEncrypter_EncryptDecrypt(t *testing.T) {
	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	e := srv.encrypter

	cleartext := []byte("the quick brown fox jumps over the lazy dog")
	ciphertext, keyID, err := e.Encrypt(cleartext)
	require.NoError(t, err)

	got, err := e.Decrypt(ciphertext, keyID)
	require.NoError(t, err)
	require.Equal(t, cleartext, got)
}

func TestEncrypter_SignVerify(t *testing.T) {

	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForLeader(t, srv.RPC)

	alloc := mock.Alloc()
	claim := alloc.ToTaskIdentityClaims(nil, "web")
	e := srv.encrypter

	out, _, err := e.SignClaims(claim)
	require.NoError(t, err)

	got, err := e.VerifyClaim(out)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NoError(t, got.Valid())
	require.Equal(t, alloc.ID, got.AllocationID)
	require.Equal(t, alloc.JobID, got.JobID)
	require.Equal(t, "web", got.TaskName)
}
