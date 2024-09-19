// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/hashicorp/go-hclog"
	kms "github.com/hashicorp/go-kms-wrapping/v2"
	"github.com/hashicorp/go-kms-wrapping/v2/aead"
	"github.com/hashicorp/go-kms-wrapping/wrappers/awskms/v2"
	"github.com/hashicorp/go-kms-wrapping/wrappers/azurekeyvault/v2"
	"github.com/hashicorp/go-kms-wrapping/wrappers/gcpckms/v2"
	"github.com/hashicorp/go-kms-wrapping/wrappers/transit/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/crypto"
	"github.com/hashicorp/nomad/helper/joseutil"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/raft"
	"golang.org/x/time/rate"
)

const nomadKeystoreExtension = ".nks.json"

type claimSigner interface {
	SignClaims(*structs.IdentityClaims) (string, string, error)
}

var _ claimSigner = &Encrypter{}

// Encrypter is the keyring for encrypting variables and signing workload
// identities.
type Encrypter struct {
	srv             *Server
	log             hclog.Logger
	providerConfigs map[string]*structs.KEKProviderConfig
	keystorePath    string

	// issuer is the OIDC Issuer to use for workload identities if configured
	issuer string

	keyring      map[string]*cipherSet
	decryptTasks map[string]context.CancelFunc
	lock         sync.RWMutex
}

// cipherSet contains the key material for variable encryption and workload
// identity signing. As cipherSets are rotated they are identified by the
// RootKey KeyID although the public key IDs are published with a type prefix to
// disambiguate which signing algorithm to use.
type cipherSet struct {
	rootKey           *structs.UnwrappedRootKey
	cipher            cipher.AEAD
	eddsaPrivateKey   ed25519.PrivateKey
	rsaPrivateKey     *rsa.PrivateKey
	rsaPKCS1PublicKey []byte // PKCS #1 DER encoded public key for JWKS
}

// NewEncrypter loads or creates a new local keystore and returns an encryption
// keyring with the keys it finds.
func NewEncrypter(srv *Server, keystorePath string) (*Encrypter, error) {

	encrypter := &Encrypter{
		srv:             srv,
		log:             srv.logger.Named("keyring"),
		keystorePath:    keystorePath,
		keyring:         make(map[string]*cipherSet),
		issuer:          srv.GetConfig().OIDCIssuer,
		providerConfigs: map[string]*structs.KEKProviderConfig{},
		decryptTasks:    map[string]context.CancelFunc{},
	}

	providerConfigs, err := getProviderConfigs(srv)
	if err != nil {
		return nil, err
	}
	encrypter.providerConfigs = providerConfigs

	err = encrypter.loadKeystore()
	if err != nil {
		return nil, err
	}
	return encrypter, nil
}

// fallbackVaultConfig allows the transit provider to fallback to using the
// default Vault cluster's configuration block, instead of repeating those
// fields
func fallbackVaultConfig(provider *structs.KEKProviderConfig, vaultcfg *config.VaultConfig) {

	setFallback := func(key, fallback, env string) {
		if provider.Config == nil {
			provider.Config = map[string]string{}
		}
		if _, ok := provider.Config[key]; !ok {
			if fallback != "" {
				provider.Config[key] = fallback
			} else {
				provider.Config[key] = os.Getenv(env)
			}
		}
	}

	setFallback("address", vaultcfg.Addr, "VAULT_ADDR")
	setFallback("token", vaultcfg.Token, "VAULT_TOKEN")
	setFallback("tls_ca_cert", vaultcfg.TLSCaPath, "VAULT_CACERT")
	setFallback("tls_client_cert", vaultcfg.TLSCertFile, "VAULT_CLIENT_CERT")
	setFallback("tls_client_key", vaultcfg.TLSKeyFile, "VAULT_CLIENT_KEY")
	setFallback("tls_server_name", vaultcfg.TLSServerName, "VAULT_TLS_SERVER_NAME")

	skipVerify := ""
	if vaultcfg.TLSSkipVerify != nil {
		skipVerify = fmt.Sprintf("%v", *vaultcfg.TLSSkipVerify)
	}
	setFallback("tls_skip_verify", skipVerify, "VAULT_SKIP_VERIFY")
}

func (e *Encrypter) loadKeystore() error {

	if err := os.MkdirAll(e.keystorePath, 0o700); err != nil {
		return err
	}

	keyErrors := map[string]error{}

	return filepath.Walk(e.keystorePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("could not read path %s from keystore: %v", path, err)
		}

		// skip over subdirectories and non-key files; they shouldn't
		// be here but there's no reason to fail startup for it if the
		// administrator has left something there
		if path != e.keystorePath && info.IsDir() {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(path, nomadKeystoreExtension) {
			return nil
		}
		idWithIndex := strings.TrimSuffix(filepath.Base(path), nomadKeystoreExtension)
		id, _, _ := strings.Cut(idWithIndex, ".")
		if !helper.IsUUID(id) {
			return nil
		}

		e.lock.RLock()
		_, ok := e.keyring[id]
		e.lock.RUnlock()
		if ok {
			return nil // already loaded this key from another file
		}

		key, err := e.loadKeyFromStore(path)
		if err != nil {
			keyErrors[id] = err
			return fmt.Errorf("could not load key file %s from keystore: %w", path, err)
		}
		if key.Meta.KeyID != id {
			return fmt.Errorf("root key ID %s must match key file %s", key.Meta.KeyID, path)
		}

		err = e.addCipher(key)
		if err != nil {
			return fmt.Errorf("could not add key file %s to keystore: %w", path, err)
		}

		// we loaded this key from at least one KEK configuration, so clear any
		// error from a previous file that we couldn't read from
		delete(keyErrors, id)
		return nil
	})
}

// IsReady blocks until all decrypt tasks are complete, or the context expires.
func (e *Encrypter) IsReady(ctx context.Context) error {
	err := helper.WithBackoffFunc(ctx, time.Millisecond*100, time.Second, func() error {
		e.lock.RLock()
		defer e.lock.RUnlock()
		if len(e.decryptTasks) != 0 {
			keyIDs := []string{}
			for keyID := range e.decryptTasks {
				keyIDs = append(keyIDs, keyID)
			}
			return fmt.Errorf("keyring is not ready - waiting for keys %s",
				strings.Join(keyIDs, ", "))
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// Encrypt encrypts the clear data with the cipher for the active root key, and
// returns the cipher text (including the nonce), and the key ID used to encrypt
// it
func (e *Encrypter) Encrypt(cleartext []byte) ([]byte, string, error) {

	cs, err := e.activeCipherSet()
	if err != nil {
		return nil, "", err
	}

	nonce, err := crypto.Bytes(cs.cipher.NonceSize())
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate key wrapper nonce: %v", err)
	}

	keyID := cs.rootKey.Meta.KeyID
	additional := []byte(keyID) // include the keyID in the signature inputs

	// we use the nonce as the dst buffer so that the ciphertext is appended to
	// that buffer and we always keep the nonce and ciphertext together, and so
	// that we're not tempted to reuse the cleartext buffer which the caller
	// still owns
	ciphertext := cs.cipher.Seal(nonce, nonce, cleartext, additional)
	return ciphertext, keyID, nil
}

// Decrypt takes an encrypted buffer and then root key ID. It extracts
// the nonce, decrypts the content, and returns the cleartext data.
func (e *Encrypter) Decrypt(ciphertext []byte, keyID string) ([]byte, error) {

	ctx, cancel := context.WithTimeout(e.srv.shutdownCtx, time.Second)
	defer cancel()
	ks, err := e.waitForKey(ctx, keyID)
	if err != nil {
		return nil, err
	}

	nonceSize := ks.cipher.NonceSize()
	nonce := ciphertext[:nonceSize] // nonce was stored alongside ciphertext
	additional := []byte(keyID)     // keyID was included in the signature inputs

	return ks.cipher.Open(nil, nonce, ciphertext[nonceSize:], additional)
}

// keyIDHeader is the JWT header for the Nomad Key ID used to sign the
// claim. This name matches the common industry practice for this
// header name.
const keyIDHeader = "kid"

// SignClaims signs the identity claim for the task and returns an encoded JWT
// (including both the claim and its signature) and the key ID of the key used
// to sign it, or an error.
//
// SignClaims adds the Issuer claim prior to signing.
func (e *Encrypter) SignClaims(claims *structs.IdentityClaims) (string, string, error) {

	if claims == nil {
		return "", "", errors.New("cannot sign empty claims")
	}

	cs, err := e.activeCipherSet()
	if err != nil {
		return "", "", err
	}

	// Add Issuer claim from server configuration
	if e.issuer != "" {
		claims.Issuer = e.issuer
	}

	opts := (&jose.SignerOptions{}).WithHeader("kid", cs.rootKey.Meta.KeyID).WithType("JWT")

	var sig jose.Signer
	if cs.rsaPrivateKey != nil {
		// If an RSA key has been created prefer it as it is more widely compatible
		sig, err = jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: cs.rsaPrivateKey}, opts)
		if err != nil {
			return "", "", err
		}
	} else {
		// No RSA key has been created, fallback to ed25519 which always exists
		sig, err = jose.NewSigner(jose.SigningKey{Algorithm: jose.EdDSA, Key: cs.eddsaPrivateKey}, opts)
		if err != nil {
			return "", "", err
		}
	}

	raw, err := jwt.Signed(sig).Claims(claims).CompactSerialize()
	if err != nil {
		return "", "", err
	}

	return raw, cs.rootKey.Meta.KeyID, nil
}

// VerifyClaim accepts a previously-signed encoded claim and validates
// it before returning the claim
func (e *Encrypter) VerifyClaim(tokenString string) (*structs.IdentityClaims, error) {

	token, err := jwt.ParseSigned(tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse signed token: %w", err)
	}

	// Find the Key ID
	keyID, err := joseutil.KeyID(token)
	if err != nil {
		return nil, err
	}

	// Find the key material
	pubKey, err := e.waitForPublicKey(keyID)
	if err != nil {
		return nil, err
	}

	typedPubKey, err := pubKey.GetPublicKey()
	if err != nil {
		return nil, err
	}

	// Validate the claims.
	claims := &structs.IdentityClaims{}
	if err := token.Claims(typedPubKey, claims); err != nil {
		return nil, fmt.Errorf("invalid signature: %w", err)
	}

	//COMPAT Until we can guarantee there are no pre-1.7 JWTs in use we can only
	//       validate the signature and have no further expectations of the
	//       claims.
	expect := jwt.Expected{}
	if err := claims.Validate(expect); err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}

	return claims, nil
}

// AddUnwrappedKey stores the key in the keystore and creates a new cipher for
// it. This is called in the RPC handlers on the leader and from the legacy
// KeyringReplicator.
func (e *Encrypter) AddUnwrappedKey(rootKey *structs.UnwrappedRootKey, isUpgraded bool) (*structs.RootKey, error) {

	// note: we don't lock the keyring here but inside addCipher
	// instead, so that we're not holding the lock while performing
	// local disk writes
	if err := e.addCipher(rootKey); err != nil {
		return nil, err
	}
	return e.wrapRootKey(rootKey, isUpgraded)
}

// AddWrappedKey creates decryption tasks for keys we've previously stored in
// Raft. It's only called as a goroutine by the FSM Apply for WrappedRootKeys,
// but it returns an error for ease of testing.
func (e *Encrypter) AddWrappedKey(ctx context.Context, wrappedKeys *structs.RootKey) error {

	logger := e.log.With("key_id", wrappedKeys.KeyID)

	e.lock.Lock()

	_, err := e.cipherSetByIDLocked(wrappedKeys.KeyID)
	if err == nil {

		// key material for each key ID is immutable so nothing to do, but we
		// can cancel and remove any running decrypt tasks
		if cancel, ok := e.decryptTasks[wrappedKeys.KeyID]; ok {
			cancel()
			delete(e.decryptTasks, wrappedKeys.KeyID)
		}
		e.lock.Unlock()
		return nil
	}

	if cancel, ok := e.decryptTasks[wrappedKeys.KeyID]; ok {
		// stop any previous tasks for this same key ID under the assumption
		// they're broken or being superceded, but don't remove the CancelFunc
		// from the map yet so that other callers don't think we can continue
		cancel()
	}

	e.lock.Unlock()

	completeCtx, cancel := context.WithCancel(ctx)

	var mErr *multierror.Error

	decryptTasks := 0
	for _, wrappedKey := range wrappedKeys.WrappedKeys {
		providerID := wrappedKey.ProviderID
		if providerID == "" {
			providerID = string(structs.KEKProviderAEAD)
		}

		provider, ok := e.providerConfigs[providerID]
		if !ok {
			err := fmt.Errorf("no such KMS provider %q configured", providerID)
			mErr = multierror.Append(mErr, err)
			continue
		}

		wrapper, err := e.newKMSWrapper(provider, wrappedKeys.KeyID, wrappedKey.KeyEncryptionKey)
		if err != nil {
			// the errors that bubble up from this library can be a bit opaque, so
			// make sure we wrap them with as much context as possible
			err := fmt.Errorf("unable to create KMS wrapper for provider %q: %w", providerID, err)
			mErr = multierror.Append(mErr, err)
			continue
		}

		// fan-out decryption tasks for HA in Nomad Enterprise. we can use the
		// key whenever any one provider returns a successful decryption
		go e.decryptWrappedKeyTask(completeCtx, cancel, wrapper, provider, wrappedKeys.Meta(), wrappedKey)
		decryptTasks++
	}

	e.lock.Lock()
	defer e.lock.Unlock()

	e.decryptTasks[wrappedKeys.KeyID] = cancel

	err = mErr.ErrorOrNil()
	if err != nil {
		if decryptTasks == 0 {
			cancel()
		}

		logger.Error("root key cannot be decrypted", "error", err)
		return err
	}

	return nil
}

// decryptWrappedKeyTask attempts to decrypt a wrapped key. It blocks until
// successful or until the context is canceled (another task completes or the
// server shuts down). The error returned is only for testing and diagnostics.
func (e *Encrypter) decryptWrappedKeyTask(ctx context.Context, cancel context.CancelFunc, wrapper kms.Wrapper, provider *structs.KEKProviderConfig, meta *structs.RootKeyMeta, wrappedKey *structs.WrappedKey) error {

	var key []byte
	var rsaKey []byte

	minBackoff := time.Second
	maxBackoff := time.Second * 5

	err := helper.WithBackoffFunc(ctx, minBackoff, maxBackoff, func() error {
		wrappedDEK := wrappedKey.WrappedDataEncryptionKey
		var err error
		key, err = wrapper.Decrypt(e.srv.shutdownCtx, wrappedDEK)
		if err != nil {
			err := fmt.Errorf("%w (root key): %w", ErrDecryptFailed, err)
			e.log.Error(err.Error(), "key_id", meta.KeyID)
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = helper.WithBackoffFunc(ctx, minBackoff, maxBackoff, func() error {
		var err error

		// Decrypt RSAKey for Workload Identity JWT signing if one exists. Prior to
		// 1.7 an ed25519 key derived from the root key was used instead of an RSA
		// key.
		if wrappedKey.WrappedRSAKey != nil {
			rsaKey, err = wrapper.Decrypt(e.srv.shutdownCtx, wrappedKey.WrappedRSAKey)
			if err != nil {
				err := fmt.Errorf("%w (rsa key): %w", ErrDecryptFailed, err)
				e.log.Error(err.Error(), "key_id", meta.KeyID)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	rootKey := &structs.UnwrappedRootKey{
		Meta:   meta,
		Key:    key,
		RSAKey: rsaKey,
	}

	err = helper.WithBackoffFunc(ctx, minBackoff, maxBackoff, func() error {
		err := e.addCipher(rootKey)
		if err != nil {
			err := fmt.Errorf("could not add cipher: %w", err)
			e.log.Error(err.Error(), "key_id", meta.KeyID)
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	e.lock.Lock()
	defer e.lock.Unlock()
	cancel()
	delete(e.decryptTasks, meta.KeyID)
	return nil
}

// addCipher creates a new cipherSet for the key and stores them in the keyring
func (e *Encrypter) addCipher(rootKey *structs.UnwrappedRootKey) error {

	if rootKey == nil || rootKey.Meta == nil {
		return fmt.Errorf("missing metadata")
	}
	var aead cipher.AEAD

	switch rootKey.Meta.Algorithm {
	case structs.EncryptionAlgorithmAES256GCM:
		block, err := aes.NewCipher(rootKey.Key)
		if err != nil {
			return fmt.Errorf("could not create cipher: %v", err)
		}
		aead, err = cipher.NewGCM(block)
		if err != nil {
			return fmt.Errorf("could not create cipher: %v", err)
		}
	default:
		return fmt.Errorf("invalid algorithm %s", rootKey.Meta.Algorithm)
	}

	ed25519Key := ed25519.NewKeyFromSeed(rootKey.Key)

	cs := cipherSet{
		rootKey:         rootKey,
		cipher:          aead,
		eddsaPrivateKey: ed25519Key,
	}

	// Unmarshal RSAKey for Workload Identity JWT signing if one exists. Prior to
	// 1.7 only the ed25519 key was used.
	if len(rootKey.RSAKey) > 0 {
		rsaKey, err := x509.ParsePKCS1PrivateKey(rootKey.RSAKey)
		if err != nil {
			return fmt.Errorf("error parsing rsa key: %w", err)
		}

		cs.rsaPrivateKey = rsaKey
		cs.rsaPKCS1PublicKey = x509.MarshalPKCS1PublicKey(&rsaKey.PublicKey)
	}

	e.lock.Lock()
	defer e.lock.Unlock()
	e.keyring[rootKey.Meta.KeyID] = &cs
	return nil
}

// waitForKey retrieves the key material by ID from the keyring, retrying with
// geometric backoff until the context expires.
func (e *Encrypter) waitForKey(ctx context.Context, keyID string) (*cipherSet, error) {
	var ks *cipherSet

	err := helper.WithBackoffFunc(ctx, 50*time.Millisecond, 100*time.Millisecond,
		func() error {
			e.lock.RLock()
			defer e.lock.RUnlock()
			var err error
			ks, err = e.cipherSetByIDLocked(keyID)
			if err != nil {
				return err
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	if ks == nil {
		return nil, fmt.Errorf("no such key")
	}
	return ks, nil
}

// GetKey retrieves the key material by ID from the keyring.
func (e *Encrypter) GetKey(keyID string) (*structs.UnwrappedRootKey, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	ks, err := e.cipherSetByIDLocked(keyID)
	if err != nil {
		return nil, err
	}
	if ks == nil {
		return nil, fmt.Errorf("no such key")
	}

	return ks.rootKey, nil
}

// activeCipherSetLocked returns the cipherSet that belongs to the key marked as
// active in the state store (so that it's consistent with raft).
//
// If a key is rotated immediately following a leader election, plans that are
// in-flight may get signed before the new leader has decrypted the key. Allow
// for a short timeout-and-retry to avoid rejecting plans
func (e *Encrypter) activeCipherSet() (*cipherSet, error) {
	store := e.srv.fsm.State()
	key, err := store.GetActiveRootKey(nil)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, fmt.Errorf("keyring has not been initialized yet")
	}

	ctx, cancel := context.WithTimeout(e.srv.shutdownCtx, time.Second)
	defer cancel()
	return e.waitForKey(ctx, key.KeyID)
}

// cipherSetByIDLocked returns the cipherSet for the specified keyID. The
// caller must read-lock the keyring
func (e *Encrypter) cipherSetByIDLocked(keyID string) (*cipherSet, error) {
	cipherSet, ok := e.keyring[keyID]
	if !ok {
		return nil, fmt.Errorf("no such key %q in keyring", keyID)
	}
	return cipherSet, nil
}

// RemoveKey removes a key by ID from the keyring
func (e *Encrypter) RemoveKey(keyID string) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	delete(e.keyring, keyID)
	return nil
}

// wrapRootKey encrypts the key for every KEK provider and returns a RootKey
// with wrapped keys. On legacy clusters, this also serializes the wrapped key
// to the on-disk keystore.
func (e *Encrypter) wrapRootKey(rootKey *structs.UnwrappedRootKey, isUpgraded bool) (*structs.RootKey, error) {

	wrappedKeys := structs.NewRootKey(rootKey.Meta)

	for _, provider := range e.providerConfigs {
		if !provider.Active {
			continue
		}
		wrappedKey, err := e.encryptDEK(rootKey, provider)
		if err != nil {
			return nil, err
		}

		switch {
		case isUpgraded && provider.Provider == string(structs.KEKProviderAEAD):
			// nothing to do but don't want to hit next case

		case isUpgraded:
			wrappedKey.KeyEncryptionKey = nil

		case provider.Provider == string(structs.KEKProviderAEAD): // !isUpgraded
			kek := wrappedKey.KeyEncryptionKey
			wrappedKey.KeyEncryptionKey = nil
			e.writeKeyToDisk(rootKey.Meta, provider, wrappedKey, kek)

		default: // !isUpgraded
			wrappedKey.KeyEncryptionKey = nil
			e.writeKeyToDisk(rootKey.Meta, provider, wrappedKey, nil)
		}

		wrappedKeys.WrappedKeys = append(wrappedKeys.WrappedKeys, wrappedKey)

	}
	return wrappedKeys, nil
}

// encryptDEK encrypts the DEKs (one for encryption and one for signing) with
// the KMS provider and returns a WrappedKey built from the provider's
// kms.BlobInfo. This includes the cleartext KEK for the AEAD provider.
func (e *Encrypter) encryptDEK(rootKey *structs.UnwrappedRootKey, provider *structs.KEKProviderConfig) (*structs.WrappedKey, error) {
	if provider == nil {
		panic("can't encrypt DEK without a provider")
	}
	var kek []byte
	var err error
	if provider.Provider == string(structs.KEKProviderAEAD) || provider.Provider == "" {
		kek, err = crypto.Bytes(32)
		if err != nil {
			return nil, fmt.Errorf("failed to generate key wrapper key: %w", err)
		}
	}
	wrapper, err := e.newKMSWrapper(provider, rootKey.Meta.KeyID, kek)
	if err != nil {
		return nil, fmt.Errorf("unable to create key wrapper: %w", err)
	}

	rootBlob, err := wrapper.Encrypt(e.srv.shutdownCtx, rootKey.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt root key: %w", err)
	}

	kekWrapper := &structs.WrappedKey{
		Provider:                 provider.Provider,
		ProviderID:               provider.ID(),
		WrappedDataEncryptionKey: rootBlob,
		WrappedRSAKey:            &kms.BlobInfo{},
		KeyEncryptionKey:         kek,
	}

	// Only cipherSets created after 1.7.0 will contain an RSA key.
	if len(rootKey.RSAKey) > 0 {
		rsaBlob, err := wrapper.Encrypt(e.srv.shutdownCtx, rootKey.RSAKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt rsa key: %w", err)
		}
		kekWrapper.WrappedRSAKey = rsaBlob
	}

	return kekWrapper, nil
}

func (e *Encrypter) writeKeyToDisk(
	meta *structs.RootKeyMeta, provider *structs.KEKProviderConfig,
	wrappedKey *structs.WrappedKey, kek []byte) error {

	// the on-disk keystore flattens the keys wrapped for the individual
	// KMS providers out to their own files
	diskWrapper := &structs.KeyEncryptionKeyWrapper{
		Meta:                     meta,
		Provider:                 provider.Name,
		ProviderID:               provider.ID(),
		WrappedDataEncryptionKey: wrappedKey.WrappedDataEncryptionKey,
		WrappedRSAKey:            wrappedKey.WrappedRSAKey,
		KeyEncryptionKey:         kek,
	}

	buf, err := json.Marshal(diskWrapper)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.%s%s",
		meta.KeyID, provider.ID(), nomadKeystoreExtension)
	path := filepath.Join(e.keystorePath, filename)
	err = os.WriteFile(path, buf, 0o600)
	if err != nil {
		return err
	}
	return nil
}

// loadKeyFromStore deserializes a root key from disk.
func (e *Encrypter) loadKeyFromStore(path string) (*structs.UnwrappedRootKey, error) {

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	kekWrapper := &structs.KeyEncryptionKeyWrapper{}
	if err := json.Unmarshal(raw, kekWrapper); err != nil {
		return nil, err
	}

	meta := kekWrapper.Meta
	if err = meta.Validate(); err != nil {
		return nil, err
	}

	if kekWrapper.ProviderID == "" {
		kekWrapper.ProviderID = string(structs.KEKProviderAEAD)
	}
	provider, ok := e.providerConfigs[kekWrapper.ProviderID]
	if !ok {
		return nil, fmt.Errorf("no such provider %q configured", kekWrapper.ProviderID)
	}

	// the errors that bubble up from this library can be a bit opaque, so make
	// sure we wrap them with as much context as possible
	wrapper, err := e.newKMSWrapper(provider, meta.KeyID, kekWrapper.KeyEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("unable to create key wrapper: %w", err)
	}
	wrappedDEK := kekWrapper.WrappedDataEncryptionKey
	if wrappedDEK == nil {
		// older KEK wrapper versions with AEAD-only have the key material in a
		// different field
		wrappedDEK = &kms.BlobInfo{Ciphertext: kekWrapper.EncryptedDataEncryptionKey}
	}
	key, err := wrapper.Decrypt(e.srv.shutdownCtx, wrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("%w (root key): %w", ErrDecryptFailed, err)
	}

	// Decrypt RSAKey for Workload Identity JWT signing if one exists. Prior to
	// 1.7 an ed25519 key derived from the root key was used instead of an RSA
	// key.
	var rsaKey []byte
	if kekWrapper.WrappedRSAKey != nil {
		rsaKey, err = wrapper.Decrypt(e.srv.shutdownCtx, kekWrapper.WrappedRSAKey)
		if err != nil {
			return nil, fmt.Errorf("%w (rsa key): %w", ErrDecryptFailed, err)
		}
	} else if len(kekWrapper.EncryptedRSAKey) > 0 {
		// older KEK wrapper versions with AEAD-only have the key material in a
		// different field
		rsaKey, err = wrapper.Decrypt(e.srv.shutdownCtx, &kms.BlobInfo{
			Ciphertext: kekWrapper.EncryptedRSAKey})
		if err != nil {
			return nil, fmt.Errorf("%w (rsa key): %w", ErrDecryptFailed, err)
		}
	}

	return &structs.UnwrappedRootKey{
		Meta:   meta,
		Key:    key,
		RSAKey: rsaKey,
	}, nil
}

var ErrDecryptFailed = errors.New("unable to decrypt wrapped key")

// waitForPublicKey returns the public signing key for the requested key id or
// an error if the key could not be found. It blocks up to 1s for key material
// to be decrypted so that Workload Identities signed by a brand-new key can be
// verified for stale RPCs made to followers that might not have yet decrypted
// the key received via Raft
func (e *Encrypter) waitForPublicKey(keyID string) (*structs.KeyringPublicKey, error) {
	ctx, cancel := context.WithTimeout(e.srv.shutdownCtx, 1*time.Second)
	defer cancel()
	ks, err := e.waitForKey(ctx, keyID)
	if err != nil {
		return nil, err
	}

	pubKey := &structs.KeyringPublicKey{
		KeyID:      keyID,
		Use:        structs.PubKeyUseSig,
		CreateTime: ks.rootKey.Meta.CreateTime,
	}

	if ks.rsaPrivateKey != nil {
		pubKey.PublicKey = ks.rsaPKCS1PublicKey
		pubKey.Algorithm = structs.PubKeyAlgRS256
	} else {
		pubKey.PublicKey = ks.eddsaPrivateKey.Public().(ed25519.PublicKey)
		pubKey.Algorithm = structs.PubKeyAlgEdDSA
	}

	return pubKey, nil
}

// GetPublicKey returns the public signing key for the requested key id or an
// error if the key could not be found.
func (e *Encrypter) GetPublicKey(keyID string) (*structs.KeyringPublicKey, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	ks, err := e.cipherSetByIDLocked(keyID)
	if err != nil {
		return nil, err
	}

	pubKey := &structs.KeyringPublicKey{
		KeyID:      keyID,
		Use:        structs.PubKeyUseSig,
		CreateTime: ks.rootKey.Meta.CreateTime,
	}

	if ks.rsaPrivateKey != nil {
		pubKey.PublicKey = ks.rsaPKCS1PublicKey
		pubKey.Algorithm = structs.PubKeyAlgRS256
	} else {
		pubKey.PublicKey = ks.eddsaPrivateKey.Public().(ed25519.PublicKey)
		pubKey.Algorithm = structs.PubKeyAlgEdDSA
	}

	return pubKey, nil
}

// newKMSWrapper returns a go-kms-wrapping interface the caller can use to
// encrypt the RootKey with a key encryption key (KEK).
func (e *Encrypter) newKMSWrapper(provider *structs.KEKProviderConfig, keyID string, kek []byte) (kms.Wrapper, error) {
	var wrapper kms.Wrapper

	// note: adding support for another provider from go-kms-wrapping is a
	// matter of adding the dependency and another case here, but the remaining
	// third-party providers add significantly to binary size

	switch provider.Provider {
	case structs.KEKProviderAWSKMS:
		wrapper = awskms.NewWrapper()
	case structs.KEKProviderAzureKeyVault:
		wrapper = azurekeyvault.NewWrapper()
	case structs.KEKProviderGCPCloudKMS:
		wrapper = gcpckms.NewWrapper()
	case structs.KEKProviderVaultTransit:
		wrapper = transit.NewWrapper()

	default: // "aead"
		wrapper := aead.NewWrapper()
		wrapper.SetConfig(context.Background(),
			aead.WithAeadType(kms.AeadTypeAesGcm),
			aead.WithHashType(kms.HashTypeSha256),
			kms.WithKeyId(keyID),
		)
		err := wrapper.SetAesGcmKeyBytes(kek)
		if err != nil {
			return nil, err
		}
		return wrapper, nil
	}

	config, ok := e.providerConfigs[provider.ID()]
	if ok {
		_, err := wrapper.SetConfig(context.Background(), kms.WithConfigMap(config.Config))
		if err != nil {
			return nil, err
		}
	}
	return wrapper, nil
}

// KeyringReplicator supports the legacy (pre-1.9.0) keyring management where
// wrapped keys were stored outside of Raft.
//
// COMPAT(1.12.0) - remove in 1.12.0 LTS
type KeyringReplicator struct {
	srv       *Server
	encrypter *Encrypter
	logger    hclog.Logger
	stopFn    context.CancelFunc
}

func NewKeyringReplicator(srv *Server, e *Encrypter) *KeyringReplicator {
	ctx, cancel := context.WithCancel(context.Background())
	repl := &KeyringReplicator{
		srv:       srv,
		encrypter: e,
		logger:    srv.logger.Named("keyring.replicator"),
		stopFn:    cancel,
	}
	go repl.run(ctx)
	return repl
}

// stop is provided for testing
func (krr *KeyringReplicator) stop() {
	krr.stopFn()
}

const keyringReplicationRate = 5

func (krr *KeyringReplicator) run(ctx context.Context) {
	krr.logger.Debug("starting encryption key replication")
	defer krr.logger.Debug("exiting key replication")

	limiter := rate.NewLimiter(keyringReplicationRate, keyringReplicationRate)

	for {
		select {
		case <-krr.srv.shutdownCtx.Done():
			return
		case <-ctx.Done():
			return
		default:
			err := limiter.Wait(ctx)
			if err != nil {
				continue // rate limit exceeded
			}

			store := krr.srv.fsm.State()
			iter, err := store.RootKeys(nil)
			if err != nil {
				krr.logger.Error("failed to fetch keyring", "error", err)
				continue
			}
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}

				wrappedKeys := raw.(*structs.RootKey)
				if key, err := krr.encrypter.GetKey(wrappedKeys.KeyID); err == nil && len(key.Key) > 0 {
					// the key material is immutable so if we've already got it
					// we can move on to the next key
					continue
				}

				err := krr.replicateKey(ctx, wrappedKeys)
				if err != nil {
					// don't break the loop on an error, as we want to make sure
					// we've replicated any keys we can. the rate limiter will
					// prevent this case from sending excessive RPCs
					krr.logger.Error(err.Error(), "key", wrappedKeys.KeyID)
				}

			}

		}
	}

}

// replicateKey replicates a single key from peer servers that was present in
// the state store but missing from the keyring. Returns an error only if no
// peers have this key.
func (krr *KeyringReplicator) replicateKey(ctx context.Context, wrappedKeys *structs.RootKey) error {
	keyID := wrappedKeys.KeyID
	krr.logger.Debug("replicating new key", "id", keyID)

	var err error
	getReq := &structs.KeyringGetRootKeyRequest{
		KeyID: keyID,
		QueryOptions: structs.QueryOptions{
			Region:        krr.srv.config.Region,
			MinQueryIndex: wrappedKeys.ModifyIndex - 1,
		},
	}
	getResp := &structs.KeyringGetRootKeyResponse{}

	// Servers should avoid querying themselves
	_, leaderID := krr.srv.raft.LeaderWithID()
	if leaderID != raft.ServerID(krr.srv.GetConfig().NodeID) {
		err = krr.srv.RPC("Keyring.Get", getReq, getResp)
	}

	if err != nil || getResp.Key == nil {
		// Key replication needs to tolerate leadership flapping. If a key is
		// rotated during a leadership transition, it's possible that the new
		// leader has not yet replicated the key from the old leader before the
		// transition. Ask all the other servers if they have it.
		krr.logger.Warn("failed to fetch key from current leader, trying peers",
			"key", keyID, "error", err)
		getReq.AllowStale = true

		cfg := krr.srv.GetConfig()
		self := fmt.Sprintf("%s.%s", cfg.NodeName, cfg.Region)

		for _, peer := range krr.getAllPeers() {
			if peer.Name == self {
				continue
			}

			krr.logger.Trace("attempting to replicate key from peer",
				"id", keyID, "peer", peer.Name)
			err = krr.srv.forwardServer(peer, "Keyring.Get", getReq, getResp)
			if err == nil && getResp.Key != nil {
				break
			}
		}
	}

	if getResp.Key == nil {
		krr.logger.Error("failed to fetch key from any peer",
			"key", keyID, "error", err)
		return fmt.Errorf("failed to fetch key from any peer: %v", err)
	}

	isClusterUpgraded := ServersMeetMinimumVersion(
		krr.srv.serf.Members(), krr.srv.Region(), minVersionKeyringInRaft, true)

	// In the legacy replication, we toss out the wrapped key because it's
	// always persisted to disk
	_, err = krr.srv.encrypter.AddUnwrappedKey(getResp.Key, isClusterUpgraded)
	if err != nil {
		return fmt.Errorf("failed to add key to keyring: %v", err)
	}

	krr.logger.Debug("added key", "key", keyID)
	return nil
}

func (krr *KeyringReplicator) getAllPeers() []*serverParts {
	krr.srv.peerLock.RLock()
	defer krr.srv.peerLock.RUnlock()
	peers := make([]*serverParts, 0, len(krr.srv.localPeers))
	for _, peer := range krr.srv.localPeers {
		peers = append(peers, peer.Copy())
	}
	return peers
}
