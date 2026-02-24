// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/rpc"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	wrapping "github.com/hashicorp/go-kms-wrapping/v2"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/auth"
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

// Assert that the Encrypter implements the claimSigner and auth.Encrypter
// interfaces.
var (
	_ claimSigner    = &Encrypter{}
	_ auth.Encrypter = &Encrypter{}
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

	t.Run("legacy wrapper HA", func(t *testing.T) {
		key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
		must.NoError(t, err)

		// create a wrapper file identical to those before we had external KMS
		wrappedKey, err := encrypter.encryptDEK(key, &structs.KEKProviderConfig{})

		writeWrapper := func(i int) {
			var diskWrapper *structs.KeyEncryptionKeyWrapper
			if i == 1 {
				diskWrapper = &structs.KeyEncryptionKeyWrapper{
					Meta:                       key.Meta,
					KeyEncryptionKey:           wrappedKey.KeyEncryptionKey,
					EncryptedDataEncryptionKey: wrappedKey.WrappedDataEncryptionKey.Ciphertext,
					EncryptedRSAKey:            wrappedKey.WrappedRSAKey.Ciphertext,
				}
			} else {
				diskWrapper = &structs.KeyEncryptionKeyWrapper{
					Meta:                       key.Meta,
					KeyEncryptionKey:           []byte{}, // garbage
					EncryptedDataEncryptionKey: wrappedKey.WrappedDataEncryptionKey.Ciphertext,
					EncryptedRSAKey:            wrappedKey.WrappedRSAKey.Ciphertext,
				}
			}

			buf, err := json.Marshal(diskWrapper)
			must.NoError(t, err)
			name := fmt.Sprintf("%s.%d.nks.json", key.Meta.KeyID, i)
			path := filepath.Join(tmpDir, name)
			err = os.WriteFile(path, buf, 0o600)
			must.NoError(t, err)
		}

		writeWrapper(1)
		writeWrapper(0)

		must.NoError(t, encrypter.loadKeystore())
	})

}

// TestEncrypter_loadKeyFromStore_emptyRSA tests a panic seen by some
// operators where the aead key disk file content had an empty RSA block.
func TestEncrypter_loadKeyFromStore_emptyRSA(t *testing.T) {
	ci.Parallel(t)

	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{},
	}

	tmpDir := t.TempDir()

	key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	must.NoError(t, err)

	encrypter, err := NewEncrypter(srv, tmpDir)
	must.NoError(t, err)

	wrappedKey, err := encrypter.encryptDEK(key, &structs.KEKProviderConfig{})
	must.NotNil(t, wrappedKey)
	must.NoError(t, err)

	// Use an artisanally crafted key file.
	kek, err := json.Marshal(wrappedKey.KeyEncryptionKey)
	must.NoError(t, err)

	wrappedDEKCipher, err := json.Marshal(wrappedKey.WrappedDataEncryptionKey.Ciphertext)
	must.NoError(t, err)

	testData := fmt.Sprintf(`
	{
	 "Meta": {
	   "KeyID": %q,
	   "Algorithm": "aes256-gcm",
	   "CreateTime": 1730000000000000000,
	   "CreateIndex": 1555555,
	   "ModifyIndex": 1555555,
	   "State": "active",
	   "PublishTime": 0
	 },
	 "ProviderID": "aead",
	 "WrappedDEK": {
	   "ciphertext": %s,
	   "key_info": {
	     "key_id": %q
	   }
	 },
	 "WrappedRSAKey": {},
	 "KEK": %s
	}
	`, key.Meta.KeyID, wrappedDEKCipher, key.Meta.KeyID, kek)

	path := filepath.Join(tmpDir, key.Meta.KeyID+".nks.json")
	err = os.WriteFile(path, []byte(testData), 0o600)
	must.NoError(t, err)

	unwrappedKey, err := encrypter.loadKeyFromStore(path)
	must.NoError(t, err)
	must.NotNil(t, unwrappedKey)
}

// TestEncrypter_HAFailedWrites exercises the behavior of keyring rotation with
// HA KMS (for Nomad Enterprise) when one of the KMS is down. We want to fail
// the rotation here and not persist the keys.
func TestEncrypter_HAFailedWrites(t *testing.T) {

	srv := &Server{
		logger:      testlog.HCLogger(t),
		config:      &Config{},
		shutdownCtx: t.Context(),
	}

	tmpDir := t.TempDir()
	encrypter, err := NewEncrypter(srv, tmpDir)
	must.NoError(t, err)

	// build two fake Vault transit encryption servers so we can control the
	// failed responses
	newMockVaultServer := func(name string, enabled *atomic.Bool) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("vault %s got %s", name, r.URL)
			if !enabled.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]any{
					"errors": []string{"Vault is sealed or unavailable"},
				})
				return
			}
			if strings.Contains(r.URL.Path, "/transit/encrypt/") {
				var req map[string]any
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				plaintext, _ := req["plaintext"].(string)
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{ // note: not actually encrypted!
						"ciphertext":  fmt.Sprintf("vault:v1:%s", plaintext),
						"key_version": 1,
					},
				})
				return
			}
		}))
	}

	var fakeVault1Enabled atomic.Bool
	fakeVault1Enabled.Store(true)
	fakeVault1 := newMockVaultServer("fake_vault1", &fakeVault1Enabled)
	t.Cleanup(fakeVault1.Close)

	var fakeVault2Enabled atomic.Bool
	fakeVault2Enabled.Store(false)
	fakeVault2 := newMockVaultServer("fake_vault2", &fakeVault2Enabled)
	t.Cleanup(fakeVault2.Close)

	// Nomad CE doesn't support multiple providers, so override the logic in
	// getProviderConfigs; unfortunately this makes causing this test to fail in
	// the correct order flaky, because the map iteration will be random
	encrypter.providerConfigs = map[string]*structs.KEKProviderConfig{
		"transit.fake_vault1": {
			Provider: structs.KEKProviderVaultTransit,
			Name:     "fake_vault1",
			Active:   true,
			Config: map[string]string{
				"address":    fakeVault1.URL,
				"key_name":   "transit_key_name",
				"mount_path": "transit/",
			},
		},
		"transit.fake_vault2": {
			Provider: structs.KEKProviderVaultTransit,
			Name:     "fake_vault2",
			Active:   true,
			Config: map[string]string{
				"address":    fakeVault2.URL,
				"key_name":   "transit_key_name",
				"mount_path": "transit/",
			},
		},
	}

	entries, err := os.ReadDir(tmpDir)
	must.NoError(t, err)
	must.Len(t, 0, entries)

	key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	must.NoError(t, err)
	_, err = encrypter.AddUnwrappedKey(key, false)
	must.ErrorContains(t, err, "Vault is sealed or unavailable")

	// ensure we haven't left any legacy keystore files behind
	entries, err = os.ReadDir(tmpDir)
	must.NoError(t, err)
	must.Len(t, 0, entries)
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
	srv.encrypter.keyringLock.Lock()
	test.MapLen(t, 5, srv.encrypter.keyring)
	for _, keyset := range srv.encrypter.keyring {
		test.Len(t, 32, keyset.rootKey.Key)
		test.Greater(t, 0, len(keyset.rootKey.RSAKey))
	}
	srv.encrypter.keyringLock.Unlock()

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

	srv.encrypter.keyringLock.Lock()
	test.MapLen(t, 5, srv.encrypter.keyring)
	for _, keyset := range srv.encrypter.keyring {
		test.Len(t, 32, keyset.rootKey.Key)
		test.Greater(t, 0, len(keyset.rootKey.RSAKey))
	}
	srv.encrypter.keyringLock.Unlock()

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
		WorkloadIdentityClaims: &structs.WorkloadIdentityClaims{
			Namespace:    "default",
			JobID:        "fakejob",
			AllocationID: uuid.Generate(),
			TaskName:     "faketask",
		},
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
					Name:     "use-vault-config-if-set",
				},
				{
					Provider: "transit",
					Name:     "use-env-if-no-config",
				},
				{
					Provider: "transit",
					Name:     "use-fallback-if-no-env",
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

	t.Setenv("VAULT_SKIP_VERIFY", "")
	fallbackVaultConfig(providers[3], &config.VaultConfig{})
	must.Eq(t, "false", providers[3].Config["tls_skip_verify"])
}

func TestEncrypter_IsReady_noTasks(t *testing.T) {
	ci.Parallel(t)

	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{},
	}

	encrypter, err := NewEncrypter(srv, t.TempDir())
	must.NoError(t, err)

	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	t.Cleanup(timeoutCancel)

	must.NoError(t, encrypter.IsReady(timeoutCtx))
}

func TestEncrypter_IsReady_eventuallyReady(t *testing.T) {
	ci.Parallel(t)

	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{},
	}

	encrypter, err := NewEncrypter(srv, t.TempDir())
	must.NoError(t, err)

	// Add an initial decryption task to the encrypter. This simulates a key
	// restored from the Raft state (snapshot or trailing logs) as the server is
	// starting.
	encrypter.decryptTasks["id1"] = struct{}{}

	// Generate a timeout value that will be used to create the context passed
	// to the encrypter. Changing this value should not impact the test except
	// for its run length as other trigger values are calculated using this.
	timeout := 2 * time.Second

	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(timeoutCancel)

	// Launch a goroutine to monitor the readiness of the encrypter. Any
	// response is sent on the channel, so we can interrogate it.
	respCh := make(chan error)

	go func() {
		respCh <- encrypter.IsReady(timeoutCtx)
	}()

	// Create a timer at 1/3 the value of the timeout. When this triggers, we
	// add a new decryption task to the encrypter. This simulates Nomad
	// upserting a new key into state which was not part of the original
	// snapshot or trailing logs and therefore should not block the readiness
	// check.
	taskAddTimer, stop := helper.NewSafeTimer(timeout / 3)
	t.Cleanup(stop)

	// Create a timer at half the value of the timeout. When this triggers, we
	// will remove the task from the encrypter simulating it finishing and the
	// encrypter becoming ready.
	taskDeleteTimer, stop := helper.NewSafeTimer(timeout / 2)
	t.Cleanup(stop)

	select {
	case <-taskAddTimer.C:
		encrypter.decryptTasksLock.Lock()
		encrypter.decryptTasks["id2"] = struct{}{}
		encrypter.decryptTasksLock.Unlock()
	case <-taskDeleteTimer.C:
		encrypter.decryptTasksLock.Lock()
		delete(encrypter.decryptTasks, "id1")
		encrypter.decryptTasksLock.Unlock()
	case err := <-respCh:
		must.NoError(t, err)
		encrypter.decryptTasksLock.RLock()
		must.MapLen(t, 1, encrypter.decryptTasks)
		must.MapContainsKey(t, encrypter.decryptTasks, "id2")
		encrypter.decryptTasksLock.RUnlock()
	}
}

func TestEncrypter_IsReady_timeout(t *testing.T) {
	ci.Parallel(t)

	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{},
	}

	encrypter, err := NewEncrypter(srv, t.TempDir())
	must.NoError(t, err)

	// Add some tasks to the encrypter that we will never remove.
	encrypter.decryptTasks["id1"] = struct{}{}
	encrypter.decryptTasks["id2"] = struct{}{}

	// Generate a timeout context that allows the backoff to trigger a few times
	// before being canceled.
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 1*time.Second)
	t.Cleanup(timeoutCancel)

	err = encrypter.IsReady(timeoutCtx)
	must.ErrorContains(t, err, "keys id1, id2")
}

func TestEncrypter_AddWrappedKey_zeroDecryptTaskError(t *testing.T) {
	ci.Parallel(t)

	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{},
	}

	encrypter, err := NewEncrypter(srv, t.TempDir())
	must.NoError(t, err)

	key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	must.NoError(t, err)

	wrappedKey, err := encrypter.wrapRootKey(key, false)
	must.NoError(t, err)

	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(timeoutCancel)

	must.Error(t, encrypter.AddWrappedKey(timeoutCtx, wrappedKey))
	must.MapLen(t, 1, encrypter.decryptTasks)
	must.MapEmpty(t, encrypter.keyring)
}

func TestEncrypter_AddWrappedKey_sameKeyTwice(t *testing.T) {
	ci.Parallel(t)

	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{},
	}

	encrypter, err := NewEncrypter(srv, t.TempDir())
	must.NoError(t, err)

	// Create a valid and correctly formatted key and wrap it.
	key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	must.NoError(t, err)

	wrappedKey, err := encrypter.wrapRootKey(key, true)
	must.NoError(t, err)

	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(timeoutCancel)

	// Add the wrapped key to the encrypter and assert that the key is added to
	// the keyring and no decryption tasks are queued.
	must.NoError(t, encrypter.AddWrappedKey(timeoutCtx, wrappedKey))
	must.MapEmpty(t, encrypter.decryptTasks)
	must.NoError(t, encrypter.IsReady(timeoutCtx))
	must.MapLen(t, 1, encrypter.keyring)
	must.MapContainsKey(t, encrypter.keyring, key.Meta.KeyID)

	timeoutCtx, timeoutCancel = context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(timeoutCancel)

	// Add the same key again and assert that the key is not added to the
	// keyring and no decryption tasks are queued.
	must.NoError(t, encrypter.AddWrappedKey(timeoutCtx, wrappedKey))
	must.MapEmpty(t, encrypter.decryptTasks)
	must.NoError(t, encrypter.IsReady(timeoutCtx))
	must.MapLen(t, 1, encrypter.keyring)
	must.MapContainsKey(t, encrypter.keyring, key.Meta.KeyID)
}

func TestEncrypter_AddWrappedKey_sameKeyConcurrent(t *testing.T) {
	ci.Parallel(t)

	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{},
	}

	encrypter, err := NewEncrypter(srv, t.TempDir())
	must.NoError(t, err)

	// Create a valid and correctly formatted key and wrap it.
	key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	must.NoError(t, err)

	wrappedKey, err := encrypter.wrapRootKey(key, true)
	must.NoError(t, err)

	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(timeoutCancel)

	// Define the number of concurrent calls to AddWrappedKey. Changing this
	// value should not affect the correctness of the test.
	concurrentNum := 10

	// Create a channel to receive the responses from the concurrent calls to
	// AddWrappedKey. The channel is buffered to ensure that the launched
	// routines can send to it without blocking.
	respCh := make(chan error, concurrentNum)

	// Create a channel to control when the concurrent calls to AddWrappedKey
	// are triggered. When the channel is closed, all waiting routines will
	// unblock within 0.001 ms of each other.
	startCh := make(chan struct{})

	// Launch the concurrent calls to AddWrappedKey and wait till they have all
	// triggered and responded before moving on. The timeout ensures this test
	// won't deadlock or hang indefinitely.
	var wg sync.WaitGroup
	wg.Add(concurrentNum)

	for i := 0; i < concurrentNum; i++ {
		go func() {
			<-startCh
			respCh <- encrypter.AddWrappedKey(timeoutCtx, wrappedKey)
			wg.Done()
		}()
	}

	close(startCh)
	wg.Wait()

	// Gather the responses and ensure the encrypter state is as we expect.
	var respNum int

	for {
		select {
		case resp := <-respCh:
			must.NoError(t, resp)
			if respNum++; respNum == concurrentNum {
				must.NoError(t, encrypter.IsReady(timeoutCtx))
				must.MapEmpty(t, encrypter.decryptTasks)
				must.MapLen(t, 1, encrypter.keyring)
				must.MapContainsKey(t, encrypter.keyring, key.Meta.KeyID)
				return
			}
		case <-timeoutCtx.Done():
			must.NoError(t, timeoutCtx.Err())
		}
	}
}

func TestEncrypter_decryptWrappedKeyTask_successful(t *testing.T) {
	ci.Parallel(t)

	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{},
	}

	key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	must.NoError(t, err)

	encrypter, err := NewEncrypter(srv, t.TempDir())
	must.NoError(t, err)

	wrappedKey, err := encrypter.encryptDEK(key, &structs.KEKProviderConfig{})
	must.NotNil(t, wrappedKey)
	must.NoError(t, err)

	// Purposely empty the RSA key, but do not nil it, so we can test for a
	// panic where the key doesn't contain the ciphertext.
	wrappedKey.WrappedRSAKey = &wrapping.BlobInfo{}

	provider, ok := encrypter.providerConfigs[string(structs.KEKProviderAEAD)]
	must.True(t, ok)
	must.NotNil(t, provider)

	KMSWrapper, err := encrypter.newKMSWrapper(provider, key.Meta.KeyID, wrappedKey.KeyEncryptionKey)
	must.NoError(t, err)
	must.NotNil(t, KMSWrapper)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	respCh := make(chan *cipherSet)

	go encrypter.decryptWrappedKeyTask(ctx, KMSWrapper, key.Meta, wrappedKey, respCh)

	select {
	case <-ctx.Done():
		t.Fatal("timed out waiting for decryptWrappedKeyTask to complete")
	case cipherResp := <-respCh:
		must.NotNil(t, cipherResp)
	}
}

func TestEncrypter_decryptWrappedKeyTask_contextCancel(t *testing.T) {
	ci.Parallel(t)

	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{},
	}

	encrypter, err := NewEncrypter(srv, t.TempDir())
	must.NoError(t, err)

	// Create a valid and correctly formatted key and wrap it.
	key, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	must.NoError(t, err)

	wrappedKey, err := encrypter.encryptDEK(key, &structs.KEKProviderConfig{})
	must.NotNil(t, wrappedKey)
	must.NoError(t, err)

	// Prepare the KMS wrapper and the response channel, so we can call
	// decryptWrappedKeyTask. Use a buffered channel, so the decrypt task does
	// not block on a send.
	provider, ok := encrypter.providerConfigs[string(structs.KEKProviderAEAD)]
	must.True(t, ok)
	must.NotNil(t, provider)

	kmsWrapper, err := encrypter.newKMSWrapper(provider, key.Meta.KeyID, wrappedKey.KeyEncryptionKey)
	must.NoError(t, err)
	must.NotNil(t, kmsWrapper)

	respCh := make(chan *cipherSet, 1)

	// Generate a context and immediately cancel it.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Ensure we receive an error indicating we hit the context done case and
	// check no cipher response was sent.
	err = encrypter.decryptWrappedKeyTask(ctx, kmsWrapper, key.Meta, wrappedKey, respCh)
	must.ErrorContains(t, err, "operation cancelled")
	must.Eq(t, 0, len(respCh))

	// Recreate the response channel so that it is no longer buffered. The
	// decrypt task should now block on attempting to send to it.
	respCh = make(chan *cipherSet)

	// Generate a new context and an error channel so we can gather the response
	// of decryptWrappedKeyTask running inside a goroutine.
	ctx, cancel = context.WithCancel(context.Background())

	errorCh := make(chan error, 1)

	// Launch the decryptWrappedKeyTask routine.
	go func() {
		err := encrypter.decryptWrappedKeyTask(ctx, kmsWrapper, key.Meta, wrappedKey, respCh)
		errorCh <- err
	}()

	// Roughly ensure the decrypt task is running for enough time to get past
	// the cipher generation. This is so that when we cancel the context, we
	// have passed the helper.Backoff functions, which are also designed to exit
	// and return if the context is canceled. As Tim correctly pointed out; this
	// "is about giving this test a fighting chance to be testing the thing we
	// think it is".
	//
	// Canceling the context should cause the routine to exit and send an error
	// which we can check to ensure we correctly unblock.
	timer, timerStop := helper.NewSafeTimer(500 * time.Millisecond)
	defer timerStop()

	<-timer.C
	cancel()

	timer, timerStop = helper.NewSafeTimer(200 * time.Millisecond)
	defer timerStop()

	select {
	case <-timer.C:
		t.Fatal("timed out waiting for decryptWrappedKeyTask to send its error")
	case err := <-errorCh:
		must.ErrorContains(t, err, "context canceled")
	}
}

func TestEncrypter_AddWrappedKey_noWrappedKeys(t *testing.T) {
	ci.Parallel(t)

	srv := &Server{
		logger: testlog.HCLogger(t),
		config: &Config{},
	}

	encrypter, err := NewEncrypter(srv, t.TempDir())
	must.NoError(t, err)

	// Fake life as a 1.6 server by writing only ed25519 keys and removing the
	// RSAKey. Add this to the encrypter as if we are loading it as part of the
	// FSM restore process which actions the trailing logs.
	oldKey, err := structs.NewUnwrappedRootKey(structs.EncryptionAlgorithmAES256GCM)
	must.NoError(t, err)

	oldKey.RSAKey = nil
	wrappedOldKey := structs.NewRootKey(oldKey.Meta)

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	t.Cleanup(shutdownCancel)

	// Add the wrapped key and then wait for the encrypter to become ready. If
	// it reaches the timeout, it means the encrypter has added a decryption
	// task to its internal tracking without launching any decryptWrappedKeyTask
	// routines that can remove this task entry.
	must.NoError(t, encrypter.AddWrappedKey(shutdownCtx, wrappedOldKey))

	timeoutCtx, timeoutCancel := context.WithTimeout(shutdownCtx, 3*time.Second)
	t.Cleanup(timeoutCancel)

	must.NoError(t, encrypter.IsReady(timeoutCtx))
	must.MapLen(t, 0, encrypter.decryptTasks)
}
