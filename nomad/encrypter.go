package nomad

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// note: this is aliased so that it's more noticeable if someone
	// accidentally swaps it out for math/rand via running goimports
	cryptorand "crypto/rand"

	jwt "github.com/golang-jwt/jwt/v4"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	"golang.org/x/time/rate"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

const nomadKeystoreExtension = ".nks.json"

// Encrypter is the keyring for secure variables.
type Encrypter struct {
	srv          *Server
	keystorePath string

	keyring map[string]*keyset
	lock    sync.RWMutex
}

type keyset struct {
	rootKey    *structs.RootKey
	cipher     cipher.AEAD
	privateKey ed25519.PrivateKey
}

// NewEncrypter loads or creates a new local keystore and returns an
// encryption keyring with the keys it finds.
func NewEncrypter(srv *Server, keystorePath string) (*Encrypter, error) {
	err := os.MkdirAll(keystorePath, 0700)
	if err != nil {
		return nil, err
	}
	encrypter, err := encrypterFromKeystore(keystorePath)
	if err != nil {
		return nil, err
	}
	encrypter.srv = srv
	return encrypter, nil
}

func encrypterFromKeystore(keystoreDirectory string) (*Encrypter, error) {

	encrypter := &Encrypter{
		keyring:      make(map[string]*keyset),
		keystorePath: keystoreDirectory,
	}

	err := filepath.Walk(keystoreDirectory, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("could not read path %s from keystore: %v", path, err)
		}

		// skip over subdirectories and non-key files; they shouldn't
		// be here but there's no reason to fail startup for it if the
		// administrator has left something there
		if path != keystoreDirectory && info.IsDir() {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(path, nomadKeystoreExtension) {
			return nil
		}
		id := strings.TrimSuffix(filepath.Base(path), nomadKeystoreExtension)
		if !helper.IsUUID(id) {
			return nil
		}

		key, err := encrypter.loadKeyFromStore(path)
		if err != nil {
			return fmt.Errorf("could not load key file %s from keystore: %v", path, err)
		}
		if key.Meta.KeyID != id {
			return fmt.Errorf("root key ID %s must match key file %s", key.Meta.KeyID, path)
		}

		err = encrypter.AddKey(key)
		if err != nil {
			return fmt.Errorf("could not add key file %s to keystore: %v", path, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return encrypter, nil
}

// Encrypt encrypts the clear data with the cipher for the current
// root key, and returns the cipher text (including the nonce), and
// the key ID used to encrypt it
func (e *Encrypter) Encrypt(cleartext []byte) ([]byte, string, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	keyset, err := e.activeKeySetLocked()
	if err != nil {
		return nil, "", err
	}

	nonceSize := keyset.cipher.NonceSize()
	nonce := make([]byte, nonceSize)
	n, err := cryptorand.Read(nonce)
	if err != nil {
		return nil, "", err
	}
	if n < nonceSize {
		return nil, "", fmt.Errorf("failed to encrypt: entropy exhausted")
	}

	keyID := keyset.rootKey.Meta.KeyID
	additional := []byte(keyID) // include the keyID in the signature inputs

	// we use the nonce as the dst buffer so that the ciphertext is
	// appended to that buffer and we always keep the nonce and
	// ciphertext together, and so that we're not tempted to reuse
	// the cleartext buffer which the caller still owns
	ciphertext := keyset.cipher.Seal(nonce, nonce, cleartext, additional)
	return ciphertext, keyID, nil
}

// Decrypt takes an encrypted buffer and then root key ID. It extracts
// the nonce, decrypts the content, and returns the cleartext data.
func (e *Encrypter) Decrypt(ciphertext []byte, keyID string) ([]byte, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	keyset, err := e.keysetByIDLocked(keyID)
	if err != nil {
		return nil, err
	}

	nonceSize := keyset.cipher.NonceSize()
	nonce := ciphertext[:nonceSize] // nonce was stored alongside ciphertext
	additional := []byte(keyID)     // keyID was included in the signature inputs

	return keyset.cipher.Open(nil, nonce, ciphertext[nonceSize:], additional)
}

// keyIDHeader is the JWT header for the Nomad Key ID used to sign the
// claim. This name matches the common industry practice for this
// header name.
const keyIDHeader = "kid"

// SignClaims signs the identity claim for the task and returns an
// encoded JWT with both the claim and its signature
func (e *Encrypter) SignClaims(claim *structs.IdentityClaims) (string, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	keyset, err := e.activeKeySetLocked()
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(&jwt.SigningMethodEd25519{}, claim)
	token.Header[keyIDHeader] = keyset.rootKey.Meta.KeyID

	tokenString, err := token.SignedString(keyset.privateKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// VerifyClaim accepts a previously-signed encoded claim and validates
// it before returning the claim
func (e *Encrypter) VerifyClaim(tokenString string) (*structs.IdentityClaims, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	token, err := jwt.ParseWithClaims(tokenString, &structs.IdentityClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Method.Alg())
		}
		raw := token.Header[keyIDHeader]
		if raw == nil {
			return nil, fmt.Errorf("missing key ID header")
		}
		keyID := raw.(string)
		keyset, err := e.keysetByIDLocked(keyID)
		if err != nil {
			return nil, err
		}
		return keyset.privateKey.Public(), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %v", err)
	}

	claims, ok := token.Claims.(*structs.IdentityClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("failed to verify token: invalid token")
	}
	return claims, nil
}

// AddKey stores the key in the keystore and creates a new cipher for it.
func (e *Encrypter) AddKey(rootKey *structs.RootKey) error {

	// note: we don't lock the keyring here but inside addCipher
	// instead, so that we're not holding the lock while performing
	// local disk writes
	if err := e.addCipher(rootKey); err != nil {
		return err
	}
	if err := e.saveKeyToStore(rootKey); err != nil {
		return err
	}
	return nil
}

// addCipher stores the key in the keyring and creates a new cipher for it.
func (e *Encrypter) addCipher(rootKey *structs.RootKey) error {

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

	privateKey := ed25519.NewKeyFromSeed(rootKey.Key)

	e.lock.Lock()
	defer e.lock.Unlock()
	e.keyring[rootKey.Meta.KeyID] = &keyset{
		rootKey:    rootKey,
		cipher:     aead,
		privateKey: privateKey,
	}
	return nil
}

// GetKey retrieves the key material by ID from the keyring
func (e *Encrypter) GetKey(keyID string) ([]byte, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	keyset, err := e.keysetByIDLocked(keyID)
	if err != nil {
		return nil, err
	}
	return keyset.rootKey.Key, nil
}

// activeKeySetLocked returns the keyset that belongs to the key marked as
// active in the state store (so that it's consistent with raft). The
// called must read-lock the keyring
func (e *Encrypter) activeKeySetLocked() (*keyset, error) {
	store := e.srv.fsm.State()
	keyMeta, err := store.GetActiveRootKeyMeta(nil)
	if err != nil {
		return nil, err
	}

	return e.keysetByIDLocked(keyMeta.KeyID)
}

// keysetByIDLocked returns the keyset for the specified keyID. The
// caller must read-lock the keyring
func (e *Encrypter) keysetByIDLocked(keyID string) (*keyset, error) {
	keyset, ok := e.keyring[keyID]
	if !ok {
		return nil, fmt.Errorf("no such key %s in keyring", keyID)
	}
	return keyset, nil
}

// RemoveKey removes a key by ID from the keyring
func (e *Encrypter) RemoveKey(keyID string) error {
	// TODO: should the server remove the serialized file here?
	// TODO: given that it's irreversible, should the server *ever*
	// remove the serialized file?
	e.lock.Lock()
	defer e.lock.Unlock()
	delete(e.keyring, keyID)
	return nil
}

// saveKeyToStore serializes a root key to the on-disk keystore.
func (e *Encrypter) saveKeyToStore(rootKey *structs.RootKey) error {
	var buf bytes.Buffer
	enc := codec.NewEncoder(&buf, structs.JsonHandleWithExtensions)
	err := enc.Encode(rootKey)
	if err != nil {
		return err
	}
	path := filepath.Join(e.keystorePath, rootKey.Meta.KeyID+nomadKeystoreExtension)
	err = os.WriteFile(path, buf.Bytes(), 0600)
	if err != nil {
		return err
	}
	return nil
}

// loadKeyFromStore deserializes a root key from disk.
func (e *Encrypter) loadKeyFromStore(path string) (*structs.RootKey, error) {

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	storedKey := &struct {
		Meta *structs.RootKeyMetaStub
		Key  string
	}{}

	if err := json.Unmarshal(raw, storedKey); err != nil {
		return nil, err
	}

	meta := &structs.RootKeyMeta{
		State:      storedKey.Meta.State,
		KeyID:      storedKey.Meta.KeyID,
		Algorithm:  storedKey.Meta.Algorithm,
		CreateTime: storedKey.Meta.CreateTime,
	}
	if err = meta.Validate(); err != nil {
		return nil, err
	}

	key, err := base64.StdEncoding.DecodeString(storedKey.Key)
	if err != nil {
		return nil, fmt.Errorf("could not decode key: %v", err)
	}

	return &structs.RootKey{
		Meta: meta,
		Key:  key,
	}, nil
}

type KeyringReplicator struct {
	srv       *Server
	encrypter *Encrypter
	logger    log.Logger
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

func (krr *KeyringReplicator) run(ctx context.Context) {
	limiter := rate.NewLimiter(replicationRateLimit, int(replicationRateLimit))
	krr.logger.Debug("starting encryption key replication")
	defer krr.logger.Debug("exiting key replication")

	retryErrTimer, stop := helper.NewSafeTimer(time.Second * 1)
	defer stop()

START:
	store := krr.srv.fsm.State()

	for {
		select {
		case <-krr.srv.shutdownCtx.Done():
			return
		case <-ctx.Done():
			return
		default:
			// Rate limit how often we attempt replication
			limiter.Wait(ctx)

			ws := store.NewWatchSet()
			iter, err := store.RootKeyMetas(ws)
			if err != nil {
				krr.logger.Error("failed to fetch keyring", "error", err)
				goto ERR_WAIT
			}
			for {
				raw := iter.Next()
				if raw == nil {
					break
				}
				keyMeta := raw.(*structs.RootKeyMeta)
				keyID := keyMeta.KeyID
				if _, err := krr.encrypter.GetKey(keyID); err == nil {
					// the key material is immutable so if we've already got it
					// we can safely return early
					continue
				}

				krr.logger.Trace("replicating new key", "id", keyID)

				getReq := &structs.KeyringGetRootKeyRequest{
					KeyID: keyID,
					QueryOptions: structs.QueryOptions{
						Region: krr.srv.config.Region,
					},
				}
				getResp := &structs.KeyringGetRootKeyResponse{}
				err := krr.srv.RPC("Keyring.Get", getReq, getResp)

				if err != nil || getResp.Key == nil {
					// Key replication needs to tolerate leadership
					// flapping. If a key is rotated during a
					// leadership transition, it's possible that the
					// new leader has not yet replicated the key from
					// the old leader before the transition. Ask all
					// the other servers if they have it.
					krr.logger.Debug("failed to fetch key from current leader",
						"key", keyID, "error", err)
					getReq.AllowStale = true
					for _, peer := range krr.getAllPeers() {
						err = krr.srv.forwardServer(peer, "Keyring.Get", getReq, getResp)
						if err == nil {
							break
						}
					}
					if getResp.Key == nil {
						krr.logger.Error("failed to fetch key from any peer",
							"key", keyID, "error", err)
						goto ERR_WAIT
					}
				}
				err = krr.encrypter.AddKey(getResp.Key)
				if err != nil {
					krr.logger.Error("failed to add key", "key", keyID, "error", err)
					goto ERR_WAIT
				}
				krr.logger.Trace("added key", "key", keyID)
			}
		}
	}

ERR_WAIT:
	// TODO: what's the right amount of backoff here? should this be
	// part of our configuration?
	retryErrTimer.Reset(1 * time.Second)

	select {
	case <-retryErrTimer.C:
		goto START
	case <-ctx.Done():
		return
	}

}

// TODO: move this method into Server?
func (krr *KeyringReplicator) getAllPeers() []*serverParts {
	krr.srv.peerLock.RLock()
	defer krr.srv.peerLock.RUnlock()
	peers := make([]*serverParts, 0, len(krr.srv.localPeers))
	for _, peer := range krr.srv.localPeers {
		peers = append(peers, peer.Copy())
	}
	return peers
}
