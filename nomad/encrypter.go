package nomad

import (
	"bytes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/nomad/structs"
)

type Encrypter struct {
	ciphers      map[string]cipher.AEAD // map of key IDs to ciphers
	keystorePath string
}

// NewEncrypter loads or creates a new local keystore and returns an
// encryption keyring with the keys it finds.
func NewEncrypter(keystorePath string) *Encrypter {
	err := os.MkdirAll(keystorePath, 0700)
	if err != nil {
		panic(err) // TODO
	}
	encrypter := &Encrypter{
		ciphers:      make(map[string]cipher.AEAD),
		keystorePath: keystorePath,
	}
	if err != nil {
		panic(err) // TODO
	}
	return encrypter
}

// Encrypt takes the serialized map[string][]byte from
// SecureVariable.UnencryptedData, generates an appropriately-sized nonce
// for the algorithm, and encrypts the data with the ciper for the
// CurrentRootKeyID. The buffer returned includes the nonce.
func (e *Encrypter) Encrypt(unencryptedData []byte, keyID string) []byte {
	// TODO: actually encrypt!
	return unencryptedData
}

// Decrypt takes an encrypted buffer and then root key ID. It extracts
// the nonce, decrypts the content, and returns the cleartext data.
func (e *Encrypter) Decrypt(encryptedData []byte, keyID string) ([]byte, error) {
	// TODO: actually decrypt!
	return encryptedData, nil
}

// GenerateNewRootKey returns a new root key and its metadata.
func (e *Encrypter) GenerateNewRootKey(algorithm structs.EncryptionAlgorithm) *structs.RootKey {
	meta := structs.NewRootKeyMeta()
	meta.Algorithm = algorithm
	return &structs.RootKey{
		Meta: meta,
		Key:  []byte{}, // TODO: generate based on algorithm
	}
}

// SaveKeyToStore serializes a root key to the on-disk keystore.
func (e *Encrypter) SaveKeyToStore(rootKey *structs.RootKey) error {
	var buf bytes.Buffer
	enc := codec.NewEncoder(&buf, structs.JsonHandleWithExtensions)
	err := enc.Encode(rootKey)
	if err != nil {
		return err
	}
	path := filepath.Join(e.keystorePath, rootKey.Meta.KeyID+".json")
	err = os.WriteFile(path, buf.Bytes(), 0600)
	if err != nil {
		return err
	}
	return nil
}

// LoadKeyFromStore deserializes a root key from disk.
func (e *Encrypter) LoadKeyFromStore(path string) (*structs.RootKey, error) {

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
		Active:     storedKey.Meta.Active,
		KeyID:      storedKey.Meta.KeyID,
		Algorithm:  storedKey.Meta.Algorithm,
		CreateTime: storedKey.Meta.CreateTime,
	}
	if err = meta.Validate(); err != nil {
		return nil, err
	}

	// Note: we expect to have null bytes for padding, but we don't
	// want to use RawStdEncoding which breaks a lot of command line
	// tools. So we'll truncate the key to the expected length. In
	// theory this value could be vary on Meta.Algorithm but all
	// currently supported algos are the same length.
	const keyLen = 32

	key := make([]byte, keyLen)
	_, err = base64.StdEncoding.Decode(key, []byte(storedKey.Key)[:keyLen])
	if err != nil {
		return nil, fmt.Errorf("could not decode key: %v", err)
	}

	return &structs.RootKey{
		Meta: meta,
		Key:  key,
	}, nil

}
