// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/rpc"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
	"github.com/stretchr/testify/require"
)

var (
	wiHandle = &structs.WIHandle{
		WorkloadIdentifier: "web",
		WorkloadType:       structs.WorkloadTypeTask,
	}
)

type mockSigner struct {
	calls []*structs.IdentityClaims

	nextToken, nextKeyID string
	nextErr              error
}

func (s *mockSigner) SignClaims(c *structs.IdentityClaims) (token, keyID string, err error) {
	s.calls = append(s.calls, c)
	return s.nextToken, s.nextKeyID, s.nextErr
}

// TestEncrypter_LoadSave exercises round-tripping keys to disk
func TestEncrypter_LoadSave(t *testing.T) {
	ci.Parallel(t)

	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{},
	}

	tmpDir := t.TempDir()
	encrypter, err := NewEncrypter(srv, tmpDir)
	must.NoError(t, err)

	algos := []structs.EncryptionAlgorithm{
		structs.EncryptionAlgorithmAES256GCM,
	}

	for _, algo := range algos {
		t.Run(string(algo), func(t *testing.T) {
			key, err := structs.NewUnwrappedRootKey(algo)
			must.Greater(t, 0, len(key.RSAKey))
			must.NoError(t, err)

			_, err = encrypter.wrapRootKey(key, false)
			must.NoError(t, err)

			// startup code path
			gotKey, err := encrypter.loadKeyFromStore(
				filepath.Join(tmpDir, key.Meta.KeyID+".aead.nks.json"))
			must.NoError(t, err)
			must.NoError(t, encrypter.addCipher(gotKey))
			must.Greater(t, 0, len(gotKey.RSAKey))
			_, err = encrypter.wrapRootKey(key, false)
			must.NoError(t, err)

			active, err := encrypter.cipherSetByIDLocked(key.Meta.KeyID)
			must.NoError(t, err)
			must.Greater(t, 0, len(active.rootKey.RSAKey))
		})
	}

	t.Run("legacy aead wrapper", func(t *testing.T) {
		key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
		must.NoError(t, err)

		// create a wrapper file identical to those before we had external KMS
		wrappedKey, err := encrypter.encryptDEK(key, &structs.KEKProviderConfig{})
		diskWrapper := &structs.KeyEncryptionKeyWrapper{
			Meta:                       key.Meta,
			KeyEncryptionKey:           wrappedKey.KeyEncryptionKey,
			EncryptedDataEncryptionKey: wrappedKey.WrappedDataEncryptionKey.Ciphertext,
			EncryptedRSAKey:            wrappedKey.WrappedRSAKey.Ciphertext,
		}

		buf, err := json.Marshal(diskWrapper)
		must.NoError(t, err)

		path := filepath.Join(tmpDir, key.Meta.KeyID+".nks.json")
		err = os.WriteFile(path, buf, 0o600)
		must.NoError(t, err)

		gotKey, err := encrypter.loadKeyFromStore(path)
		must.NoError(t, err)
		must.NoError(t, encrypter.addCipher(gotKey))
		must.Greater(t, 0, len(gotKey.RSAKey))
	})

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
	testutil.WaitForKeyring(t, srv.RPC, "global")
	codec := rpcClient(t, srv)
	nodeID := srv.GetConfig().NodeID

	// Verify we have a bootstrap key

	listReq := &structs.KeyringListRootKeyMetaRequest{
		QueryOptions: structs.QueryOptions{
			Region:    "global",
			AuthToken: rootToken.SecretID,
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
		must.NoError(t, err)
	}

	// Ensure all rotated keys are correct
	srv.encrypter.lock.Lock()
	test.MapLen(t, 5, srv.encrypter.keyring)
	for _, keyset := range srv.encrypter.keyring {
		test.Len(t, 32, keyset.rootKey.Key)
		test.Greater(t, 0, len(keyset.rootKey.RSAKey))
	}
	srv.encrypter.lock.Unlock()

	shutdown()

	srv2, rootToken, shutdown2 := TestACLServer(t, func(c *Config) {
		c.NodeID = nodeID
		c.NodeName = "node1"
		c.NumSchedulers = 0
		c.DevMode = false
		c.DataDir = tmpDir
	})
	defer shutdown2()
	testutil.WaitForKeyring(t, srv2.RPC, "global")
	codec = rpcClient(t, srv2)

	// Verify we've restored all the keys from the old keystore
	listReq.AuthToken = rootToken.SecretID

	require.Eventually(t, func() bool {
		msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
		return len(listResp.Keys) == 5 // 4 new + the bootstrap key
	}, time.Second*5, time.Second, "expected keyring to be restored")

	srv.encrypter.lock.Lock()
	test.MapLen(t, 5, srv.encrypter.keyring)
	for _, keyset := range srv.encrypter.keyring {
		test.Len(t, 32, keyset.rootKey.Key)
		test.Greater(t, 0, len(keyset.rootKey.RSAKey))
	}
	srv.encrypter.lock.Unlock()

	for _, keyMeta := range listResp.Keys {

		getReq := &structs.KeyringGetRootKeyRequest{
			KeyID: keyMeta.KeyID,
			QueryOptions: structs.QueryOptions{
				Region:    "global",
				AuthToken: rootToken.SecretID,
			},
		}
		var getResp structs.KeyringGetRootKeyResponse
		err := msgpackrpc.CallWithCodec(codec, "Keyring.Get", getReq, &getResp)
		must.NoError(t, err)

		gotKey := getResp.Key
		must.Len(t, 32, gotKey.Key)
		test.Greater(t, 0, len(gotKey.RSAKey))
	}
}

// TestEncrypter_KeyringBootstrapping exercises key decryption tasks as new
// servers come online and leaders are elected.
func TestEncrypter_KeyringBootstrapping(t *testing.T) {

	ci.Parallel(t)

	srv1, cleanupSRV1 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.NumSchedulers = 0
	})
	t.Cleanup(cleanupSRV1)

	// add two more servers after we've bootstrapped

	srv2, cleanupSRV2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.NumSchedulers = 0
	})
	t.Cleanup(cleanupSRV2)
	srv3, cleanupSRV3 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 3
		c.NumSchedulers = 0
	})
	t.Cleanup(cleanupSRV3)

	servers := []*Server{srv1, srv2, srv3}
	TestJoin(t, servers...)
	testutil.WaitForKeyring(t, srv1.RPC, "global")
	testutil.WaitForKeyring(t, srv2.RPC, "global")
	testutil.WaitForKeyring(t, srv3.RPC, "global")

	var leader *Server

	for _, srv := range servers {
		if ok, _ := srv.getLeader(); ok {
			leader = srv
		}
	}
	must.NotNil(t, leader, must.Sprint("expected there to be a leader"))
	codec := rpcClient(t, leader)
	t.Logf("leader is %s", leader.config.NodeName)

	// Verify we have a bootstrap key

	listReq := &structs.KeyringListRootKeyMetaRequest{
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var listResp structs.KeyringListRootKeyMetaResponse

	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			msgpackrpc.CallWithCodec(codec, "Keyring.List", listReq, &listResp)
			return len(listResp.Keys) == 1
		}),
		wait.Timeout(time.Second*5), wait.Gap(200*time.Millisecond)),
		must.Sprint("expected keyring to be initialized"))

	keyID1 := listResp.Keys[0].KeyID

	// Helper function for checking that a specific key is in the keyring for a
	// specific server
	checkPublicKeyFn := func(codec rpc.ClientCodec, keyID string) bool {
		listPublicReq := &structs.GenericRequest{
			QueryOptions: structs.QueryOptions{
				Region:     "global",
				AllowStale: true,
			},
		}
		var listPublicResp structs.KeyringListPublicResponse
		msgpackrpc.CallWithCodec(codec, "Keyring.ListPublic", listPublicReq, &listPublicResp)
		for _, key := range listPublicResp.PublicKeys {
			if key.KeyID == keyID && len(key.PublicKey) > 0 {
				return true
			}
		}
		return false
	}

	// leader's key should already be available by the time its elected the
	// leader
	must.True(t, checkPublicKeyFn(codec, keyID1))

	// Helper function for checking that a specific key has been
	// replicated to all followers
	checkReplicationFn := func(keyID string) func() bool {
		return func() bool {
			for _, srv := range servers {
				if !checkPublicKeyFn(rpcClient(t, srv), keyID) {
					return false
				}
			}
			return true
		}
	}

	// Assert that the bootstrap key has been replicated to followers
	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(checkReplicationFn(keyID1)),
		wait.Timeout(time.Second*5), wait.Gap(200*time.Millisecond)),
		must.Sprint("expected keys to be replicated to followers after bootstrap"))

	// Assert that key rotations are replicated to followers
	rotateReq := &structs.KeyringRotateRootKeyRequest{
		WriteRequest: structs.WriteRequest{
			Region: "global",
		},
	}
	var rotateResp structs.KeyringRotateRootKeyResponse
	err := msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	must.NoError(t, err)
	keyID2 := rotateResp.Key.KeyID

	getReq := &structs.KeyringGetRootKeyRequest{
		KeyID: keyID2,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}
	var getResp structs.KeyringGetRootKeyResponse
	err = msgpackrpc.CallWithCodec(codec, "Keyring.Get", getReq, &getResp)
	must.NoError(t, err)
	must.NotNil(t, getResp.Key, must.Sprint("expected key to be found on leader"))

	must.True(t, checkPublicKeyFn(codec, keyID1),
		must.Sprint("expected key to be found in leader keystore"))

	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(checkReplicationFn(keyID2)),
		wait.Timeout(time.Second*5), wait.Gap(200*time.Millisecond)),
		must.Sprint("expected keys to be replicated to followers after rotation"))

	// Scenario: simulate a key rotation that doesn't get replicated
	// before a leader election by stopping replication, rotating the
	// key, and triggering a leader election.
	for _, srv := range servers {
		srv.keyringReplicator.stop()
	}

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	must.NoError(t, err)
	keyID3 := rotateResp.Key.KeyID

	err = leader.leadershipTransfer()
	must.NoError(t, err)

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

	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(checkReplicationFn(keyID3)),
		wait.Timeout(time.Second*5), wait.Gap(200*time.Millisecond)),
		must.Sprint("expected keys to be replicated to followers after election"))

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

	servers = []*Server{srv1, srv2, srv3, srv4, srv5}
	TestJoin(t, servers...)
	testutil.WaitForLeader(t, srv4.RPC)
	testutil.WaitForLeader(t, srv5.RPC)

	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(checkReplicationFn(keyID3)),
		wait.Timeout(time.Second*5), wait.Gap(200*time.Millisecond)),
		must.Sprint("expected new servers to get replicated key"))

	// Scenario: reload a snapshot

	t.Logf("taking snapshot of node5")

	snapshot, err := srv5.fsm.Snapshot()
	must.NoError(t, err)

	defer snapshot.Release()

	// Persist so we can read it back
	buf := bytes.NewBuffer(nil)
	sink := &MockSink{buf, false}
	must.NoError(t, snapshot.Persist(sink))
	must.NoError(t, srv5.fsm.Restore(sink))

	// rotate the key

	err = msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp)
	must.NoError(t, err)
	keyID4 := rotateResp.Key.KeyID

	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(checkReplicationFn(keyID4)),
		wait.Timeout(time.Second*5), wait.Gap(200*time.Millisecond)),
		must.Sprint("expected new servers to get replicated keys after snapshot restore"))
}

func TestEncrypter_EncryptDecrypt(t *testing.T) {
	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForKeyring(t, srv.RPC, "global")

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
	testutil.WaitForKeyring(t, srv.RPC, "global")

	alloc := mock.Alloc()
	task := alloc.LookupTask("web")

	claims := structs.NewIdentityClaimsBuilder(alloc.Job, alloc, wiHandle, task.Identity).
		WithTask(task).
		Build(time.Now())
	e := srv.encrypter

	out, _, err := e.SignClaims(claims)
	require.NoError(t, err)

	got, err := e.VerifyClaim(out)
	must.NoError(t, err)
	must.NotNil(t, got)
	must.Eq(t, alloc.ID, got.AllocationID)
	must.Eq(t, alloc.JobID, got.JobID)
	must.Eq(t, "web", got.TaskName)

	// By default an issuer should not be set. See _Issuer test.
	must.Eq(t, "", got.Issuer)
}

// TestEncrypter_SignVerify_Issuer asserts that the signer adds an issuer if it
// is configured.
func TestEncrypter_SignVerify_Issuer(t *testing.T) {
	// Set OIDCIssuer to a valid looking (but fake) issuer
	const testIssuer = "https://oidc.test.nomadproject.io"

	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue

		c.OIDCIssuer = testIssuer
	})
	defer shutdown()
	testutil.WaitForKeyring(t, srv.RPC, "global")

	alloc := mock.Alloc()
	task := alloc.LookupTask("web")
	claims := structs.NewIdentityClaimsBuilder(alloc.Job, alloc, wiHandle, task.Identity).
		WithTask(task).
		Build(time.Now())

	e := srv.encrypter

	out, _, err := e.SignClaims(claims)
	require.NoError(t, err)

	got, err := e.VerifyClaim(out)
	must.NoError(t, err)
	must.NotNil(t, got)
	must.Eq(t, alloc.ID, got.AllocationID)
	must.Eq(t, alloc.JobID, got.JobID)
	must.Eq(t, "web", got.TaskName)
	must.Eq(t, testIssuer, got.Issuer)
}

func TestEncrypter_SignVerify_AlgNone(t *testing.T) {

	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0 // Prevent automatic dequeue
	})
	defer shutdown()
	testutil.WaitForKeyring(t, srv.RPC, "global")

	alloc := mock.Alloc()
	task := alloc.LookupTask("web")
	claims := structs.NewIdentityClaimsBuilder(alloc.Job, alloc, wiHandle, task.Identity).
		WithTask(task).
		Build(time.Now())

	e := srv.encrypter

	keyset, err := e.activeCipherSet()
	must.NoError(t, err)
	keyID := keyset.rootKey.Meta.KeyID

	// the go-jose library rightfully doesn't acccept alg=none, so we'll forge a
	// JWT with alg=none and some attempted claims

	bodyData, err := json.Marshal(claims)
	must.NoError(t, err)
	body := make([]byte, base64.StdEncoding.EncodedLen(len(bodyData)))
	base64.StdEncoding.Encode(body, bodyData)

	// Try without a key ID
	headerData := []byte(`{"alg":"none","typ":"JWT"}`)
	header := make([]byte, base64.StdEncoding.EncodedLen(len(headerData)))
	base64.StdEncoding.Encode(header, headerData)

	badJWT := fmt.Sprintf("%s.%s.", string(header), string(body))

	got, err := e.VerifyClaim(badJWT)
	must.Error(t, err)
	must.ErrorContains(t, err, "missing key ID header")
	must.Nil(t, got)

	// Try with a valid key ID
	headerData = []byte(fmt.Sprintf(`{"alg":"none","kid":"%s","typ":"JWT"}`, keyID))
	header = make([]byte, base64.StdEncoding.EncodedLen(len(headerData)))
	base64.StdEncoding.Encode(header, headerData)

	badJWT = fmt.Sprintf("%s.%s.", string(header), string(body))

	got, err = e.VerifyClaim(badJWT)
	must.Error(t, err)
	must.ErrorContains(t, err, "invalid signature")
	must.Nil(t, got)
}

// TestEncrypter_Upgrade17 simulates upgrading from 1.6 -> 1.7 does not break
// old (ed25519) or new (rsa) signing keys.
func TestEncrypter_Upgrade17(t *testing.T) {

	ci.Parallel(t)

	srv, shutdown := TestServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})
	t.Cleanup(shutdown)
	testutil.WaitForKeyring(t, srv.RPC, "global")
	codec := rpcClient(t, srv)

	initKey, err := srv.State().GetActiveRootKey(nil)
	must.NoError(t, err)

	wr := structs.WriteRequest{
		Namespace: "default",
		Region:    "global",
	}

	// Delete the initialization key because it's a newer WrappedRootKey from
	// 1.9, which isn't under test here.
	_, _, err = srv.raftApply(
		structs.WrappedRootKeysDeleteRequestType, structs.KeyringDeleteRootKeyRequest{
			KeyID:        initKey.KeyID,
			WriteRequest: wr,
		})
	must.NoError(t, err)

	// Fake life as a 1.6 server by writing only ed25519 keys
	oldRootKey, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	must.NoError(t, err)

	oldRootKey = oldRootKey.MakeActive()

	// Remove RSAKey to mimic 1.6
	oldRootKey.RSAKey = nil

	// Add to keyring
	_, err = srv.encrypter.AddUnwrappedKey(oldRootKey, false)
	must.NoError(t, err)

	// Write a legacy key metadata to Raft
	req := structs.KeyringUpdateRootKeyMetaRequest{
		RootKeyMeta:  oldRootKey.Meta,
		WriteRequest: wr,
	}
	_, _, err = srv.raftApply(structs.RootKeyMetaUpsertRequestType, req)
	must.NoError(t, err)

	// Create a 1.6 style workload identity
	claims := &structs.IdentityClaims{
		Namespace:    "default",
		JobID:        "fakejob",
		AllocationID: uuid.Generate(),
		TaskName:     "faketask",
	}

	// Sign the claims and assert they were signed with EdDSA (the 1.6 signing
	// algorithm)
	oldRawJWT, oldKeyID, err := srv.encrypter.SignClaims(claims)
	must.NoError(t, err)
	must.Eq(t, oldRootKey.Meta.KeyID, oldKeyID)

	oldJWT, err := jwt.ParseSigned(oldRawJWT)
	must.NoError(t, err)

	foundKeyID := false
	foundAlg := false
	for _, h := range oldJWT.Headers {
		if h.KeyID != "" {
			// Should only have one key id header
			must.False(t, foundKeyID)
			foundKeyID = true
			must.Eq(t, oldKeyID, h.KeyID)
		}

		if h.Algorithm != "" {
			// Should only have one alg header
			must.False(t, foundAlg)
			foundAlg = true
			must.Eq(t, structs.PubKeyAlgEdDSA, h.Algorithm)
		}
	}
	must.True(t, foundKeyID)
	must.True(t, foundAlg)

	_, err = srv.encrypter.VerifyClaim(oldRawJWT)
	must.NoError(t, err)

	// !! Mimic an upgrade by rotating to get a new RSA key !!
	rotateReq := &structs.KeyringRotateRootKeyRequest{
		WriteRequest: wr,
	}
	var rotateResp structs.KeyringRotateRootKeyResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Keyring.Rotate", rotateReq, &rotateResp))

	newRawJWT, newKeyID, err := srv.encrypter.SignClaims(claims)
	must.NoError(t, err)
	must.NotEq(t, oldRootKey.Meta.KeyID, newKeyID)
	must.Eq(t, rotateResp.Key.KeyID, newKeyID)

	newJWT, err := jwt.ParseSigned(newRawJWT)
	must.NoError(t, err)

	foundKeyID = false
	foundAlg = false
	for _, h := range newJWT.Headers {
		if h.KeyID != "" {
			// Should only have one key id header
			must.False(t, foundKeyID)
			foundKeyID = true
			must.Eq(t, newKeyID, h.KeyID)
		}

		if h.Algorithm != "" {
			// Should only have one alg header
			must.False(t, foundAlg)
			foundAlg = true
			must.Eq(t, structs.PubKeyAlgRS256, h.Algorithm)
		}
	}
	must.True(t, foundKeyID)
	must.True(t, foundAlg)

	_, err = srv.encrypter.VerifyClaim(newRawJWT)
	must.NoError(t, err)

	// Ensure that verifying the old JWT still works
	_, err = srv.encrypter.VerifyClaim(oldRawJWT)
	must.NoError(t, err)
}

func TestEncrypter_TransitConfigFallback(t *testing.T) {
	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{
			VaultConfigs: map[string]*config.VaultConfig{structs.VaultDefaultCluster: {
				Addr:          "https://localhost:8203",
				TLSCaPath:     "/etc/certs/ca",
				TLSCertFile:   "/var/certs/vault.crt",
				TLSKeyFile:    "/var/certs/vault.key",
				TLSSkipVerify: pointer.Of(true),
				TLSServerName: "foo",
				Token:         "vault-token",
			}},
			KEKProviderConfigs: []*structs.KEKProviderConfig{
				{
					Provider: "transit",
					Name:     "no-fallback",
					Config: map[string]string{
						"address":         "https://localhost:8203",
						"token":           "vault-token",
						"tls_ca_cert":     "/etc/certs/ca",
						"tls_client_cert": "/var/certs/vault.crt",
						"tls_client_key":  "/var/certs/vault.key",
						"tls_server_name": "foo",
						"tls_skip_verify": "true",
					},
				},
				{
					Provider: "transit",
					Name:     "fallback-to-vault-block",
				},
				{
					Provider: "transit",
					Name:     "fallback-to-env",
				},
			},
		},
	}

	providers := srv.config.KEKProviderConfigs
	expect := maps.Clone(providers[0].Config)

	fallbackVaultConfig(providers[0], srv.config.GetDefaultVault())
	must.Eq(t, expect, providers[0].Config, must.Sprint("expected no change"))

	fallbackVaultConfig(providers[1], srv.config.GetDefaultVault())
	must.Eq(t, expect, providers[1].Config, must.Sprint("expected fallback to vault block"))

	t.Setenv("VAULT_ADDR", "https://localhost:8203")
	t.Setenv("VAULT_TOKEN", "vault-token")
	t.Setenv("VAULT_CACERT", "/etc/certs/ca")
	t.Setenv("VAULT_CLIENT_CERT", "/var/certs/vault.crt")
	t.Setenv("VAULT_CLIENT_KEY", "/var/certs/vault.key")
	t.Setenv("VAULT_TLS_SERVER_NAME", "foo")
	t.Setenv("VAULT_SKIP_VERIFY", "true")

	fallbackVaultConfig(providers[2], &config.VaultConfig{})
	must.Eq(t, expect, providers[2].Config, must.Sprint("expected fallback to env"))
}
