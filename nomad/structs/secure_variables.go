package structs

import (
	"fmt"
	"reflect"
	"time"

	// note: this is aliased so that it's more noticeable if someone
	// accidentally swaps it out for math/rand via running goimports
	cryptorand "crypto/rand"

	"golang.org/x/crypto/chacha20poly1305"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
)

// SecureVariable is the metadata envelope for a Secure Variable
type SecureVariable struct {
	Namespace   string
	Path        string
	CreateTime  time.Time
	CreateIndex uint64
	ModifyIndex uint64
	ModifyTime  time.Time

	// reserved for post-1.4.0 work
	// LockIndex      uint64
	// Session        string
	// DeletedAt      time.Time
	// Version        uint64
	// CustomMetaData map[string]string

	EncryptedData   *SecureVariableData `json:"-"`     // removed during serialization
	UnencryptedData map[string]string   `json:"Items"` // empty until serialized
}

// SecureVariableData is the secret data for a Secure Variable
type SecureVariableData struct {
	Data  []byte // includes nonce
	KeyID string // ID of root key used to encrypt this entry
}

func (sv SecureVariableData) Copy() *SecureVariableData {
	out := make([]byte, 0, len(sv.Data))
	copy(out, sv.Data)
	return &SecureVariableData{
		Data:  out,
		KeyID: sv.KeyID,
	}
}

func (sv *SecureVariable) Copy() *SecureVariable {
	if sv == nil {
		return nil
	}
	out := *sv
	if sv.UnencryptedData != nil {
		out.UnencryptedData = make(map[string]string, len(sv.UnencryptedData))
		for k, v := range sv.UnencryptedData {
			out.UnencryptedData[k] = v
		}
	}
	if sv.EncryptedData != nil {
		out.EncryptedData = sv.EncryptedData.Copy()
	}
	return &out
}

func (sv SecureVariable) Equals(sv2 SecureVariable) bool {
	// FIXME: This should be a smarter equality check
	return reflect.DeepEqual(sv, sv2)
}

func (sv SecureVariable) Stub() SecureVariableStub {
	return SecureVariableStub{
		Namespace:   sv.Namespace,
		Path:        sv.Path,
		CreateIndex: sv.CreateIndex,
		CreateTime:  sv.CreateTime,
		ModifyIndex: sv.ModifyIndex,
		ModifyTime:  sv.ModifyTime,
	}
}

// SecureVariableStub is the metadata envelope for a Secure Variable omitting
// the actual data. Intended to be used in list operations.
type SecureVariableStub struct {
	Namespace   string
	Path        string
	CreateTime  time.Time
	CreateIndex uint64
	ModifyIndex uint64
	ModifyTime  time.Time

	// reserved for post-1.4.0 work
	// LockIndex      uint64
	// Session        string
	// DeletedAt      time.Time
	// Version        uint64
	// CustomMetaData map[string]string
}

// SecureVariablesQuota is used to track the total size of secure
// variables entries per namespace. The total length of
// SecureVariable.EncryptedData will be added to the SecureVariablesQuota
// table in the same transaction as a write, update, or delete.
type SecureVariablesQuota struct {
	Namespace   string
	Size        uint64
	CreateIndex uint64
	ModifyIndex uint64
}

type SecureVariablesUpsertRequest struct {
	Data *SecureVariable
	WriteRequest
}

type SecureVariablesUpsertResponse struct {
	WriteMeta
}

type SecureVariablesListRequest struct {
	// TODO: do we need any fields here?
	QueryOptions
}

type SecureVariablesListResponse struct {
	Data []*SecureVariableStub
	QueryMeta
}

type SecureVariablesReadRequest struct {
	Path string
	QueryOptions
}

type SecureVariablesReadResponse struct {
	Data *SecureVariable
	QueryMeta
}

type SecureVariablesDeleteRequest struct {
	Path string
	WriteRequest
}

type SecureVariablesDeleteResponse struct {
	WriteMeta
}

// RootKey is used to encrypt and decrypt secure variables. It is
// never stored in raft.
type RootKey struct {
	Meta *RootKeyMeta
	Key  []byte // serialized to keystore as base64 blob
}

// NewRootKey returns a new root key and its metadata.
func NewRootKey(algorithm EncryptionAlgorithm) (*RootKey, error) {
	meta := NewRootKeyMeta()
	meta.Algorithm = algorithm

	rootKey := &RootKey{
		Meta: meta,
	}

	switch algorithm {
	case EncryptionAlgorithmAES256GCM:
		key := make([]byte, 32)
		if _, err := cryptorand.Read(key); err != nil {
			return nil, err
		}
		rootKey.Key = key

	case EncryptionAlgorithmXChaCha20:
		key := make([]byte, chacha20poly1305.KeySize)
		if _, err := cryptorand.Read(key); err != nil {
			return nil, err
		}
		rootKey.Key = key
	}

	return rootKey, nil
}

// RootKeyMeta is the metadata used to refer to a RootKey. It is
// stored in raft.
type RootKeyMeta struct {
	Active           bool
	KeyID            string // UUID
	Algorithm        EncryptionAlgorithm
	EncryptionsCount uint64
	CreateTime       time.Time
	CreateIndex      uint64
	ModifyIndex      uint64
}

// NewRootKeyMeta returns a new RootKeyMeta with default values
func NewRootKeyMeta() *RootKeyMeta {
	return &RootKeyMeta{
		KeyID:      uuid.Generate(),
		Algorithm:  EncryptionAlgorithmXChaCha20,
		CreateTime: time.Now(),
	}
}

// RootKeyMetaStub is for serializing root key metadata to the
// keystore, not for the List API. It excludes frequently-changing
// fields such as EncryptionsCount or ModifyIndex so we don't have to
// sync them to the on-disk keystore when the fields are already in
// raft.
type RootKeyMetaStub struct {
	KeyID      string
	Algorithm  EncryptionAlgorithm
	CreateTime time.Time
	Active     bool
}

func (rkm *RootKeyMeta) Stub() *RootKeyMetaStub {
	if rkm == nil {
		return nil
	}
	return &RootKeyMetaStub{
		KeyID:      rkm.KeyID,
		Algorithm:  rkm.Algorithm,
		CreateTime: rkm.CreateTime,
		Active:     rkm.Active,
	}

}
func (rkm *RootKeyMeta) Copy() *RootKeyMeta {
	if rkm == nil {
		return nil
	}
	out := *rkm
	return &out
}

func (rkm *RootKeyMeta) Validate() error {
	if rkm == nil {
		return fmt.Errorf("root key metadata is required")
	}
	if rkm.KeyID == "" || !helper.IsUUID(rkm.KeyID) {
		return fmt.Errorf("root key UUID is required")
	}
	if rkm.Algorithm == "" {
		return fmt.Errorf("root key algorithm is required")
	}
	return nil
}

// EncryptionAlgorithm chooses which algorithm is used for
// encrypting / decrypting entries with this key
type EncryptionAlgorithm string

const (
	EncryptionAlgorithmXChaCha20 EncryptionAlgorithm = "xchacha20"
	EncryptionAlgorithmAES256GCM EncryptionAlgorithm = "aes256-gcm"
)

type KeyringRotateRootKeyRequest struct {
	Algorithm EncryptionAlgorithm
	Full      bool
	WriteRequest
}

// KeyringRotateRootKeyResponse returns the full key metadata
type KeyringRotateRootKeyResponse struct {
	Key *RootKeyMeta
	WriteMeta
}

type KeyringListRootKeyMetaRequest struct {
	// TODO: do we need any fields here?
	QueryOptions
}

type KeyringListRootKeyMetaResponse struct {
	Keys []*RootKeyMeta
	QueryMeta
}

// KeyringUpdateRootKeyRequest is used internally for key replication
// only and for keyring restores. The RootKeyMeta will be extracted
// for applying to the FSM with the KeyringUpdateRootKeyMetaRequest
// (see below)
type KeyringUpdateRootKeyRequest struct {
	RootKey *RootKey
	WriteRequest
}

type KeyringUpdateRootKeyResponse struct {
	WriteMeta
}

// KeyringGetRootKeyRequest is used internally for key replication
// only and for keyring restores.
type KeyringGetRootKeyRequest struct {
	KeyID string
	QueryOptions
}

type KeyringGetRootKeyResponse struct {
	Key *RootKey
	QueryMeta
}

// KeyringUpdateRootKeyMetaRequest is used internally for key
// replication so that we have a request wrapper for writing the
// metadata to the FSM without including the key material
type KeyringUpdateRootKeyMetaRequest struct {
	RootKeyMeta *RootKeyMeta
	WriteRequest
}

type KeyringUpdateRootKeyMetaResponse struct {
	WriteMeta
}

type KeyringDeleteRootKeyRequest struct {
	KeyID string
	WriteRequest
}

type KeyringDeleteRootKeyResponse struct {
	WriteMeta
}
