package structs

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	// note: this is aliased so that it's more noticeable if someone
	// accidentally swaps it out for math/rand via running goimports
	cryptorand "crypto/rand"

	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/uuid"
)

const (
	// SecureVariablesUpsertRPCMethod is the RPC method for upserting
	// secure variables into Nomad state.
	//
	// Args: SecureVariablesUpsertRequest
	// Reply: SecureVariablesUpsertResponse
	SecureVariablesUpsertRPCMethod = "SecureVariables.Upsert"

	// SecureVariablesDeleteRPCMethod is the RPC method for deleting
	// a secure variable by its namespace and path.
	//
	// Args: SecureVariablesDeleteRequest
	// Reply: SecureVariablesDeleteResponse
	SecureVariablesDeleteRPCMethod = "SecureVariables.Delete"

	// SecureVariablesListRPCMethod is the RPC method for listing secure
	// variables within Nomad.
	//
	// Args: SecureVariablesListRequest
	// Reply: SecureVariablesListResponse
	SecureVariablesListRPCMethod = "SecureVariables.List"

	// SecureVariablesGetServiceRPCMethod is the RPC method for fetching a
	// secure variable according to its namepace and path.
	//
	// Args: SecureVariablesByNameRequest
	// Reply: SecureVariablesByNameResponse
	SecureVariablesReadRPCMethod = "SecureVariables.Read"
)

// SecureVariableMetadata is the metadata envelope for a Secure Variable, it
// is the list object and is shared data between an SecureVariableEncrypted and
// a SecureVariableDecrypted object.
type SecureVariableMetadata struct {
	Namespace   string
	Path        string
	CreateTime  time.Time
	CreateIndex uint64
	ModifyIndex uint64
	ModifyTime  time.Time
}

// SecureVariableEncrypted structs are returned from the Encrypter's encrypt
// method. They are the only form that should ever be persisted to storage.
type SecureVariableEncrypted struct {
	SecureVariableMetadata
	SecureVariableData
}

// SecureVariableData is the secret data for a Secure Variable
type SecureVariableData struct {
	Data  []byte // includes nonce
	KeyID string // ID of root key used to encrypt this entry
}

// SecureVariableDecrypted structs are returned from the Encrypter's decrypt
// method. Since they contains sensitive material, they should never be
// persisted to disk.
type SecureVariableDecrypted struct {
	SecureVariableMetadata
	Items SecureVariableItems
}

// SecureVariableItems are the actual secrets stored in a secure variable. They
// are always encrypted and decrypted as a single unit.
type SecureVariableItems map[string]string

func (sv SecureVariableData) Copy() SecureVariableData {
	out := make([]byte, len(sv.Data))
	copy(out, sv.Data)
	return SecureVariableData{
		Data:  out,
		KeyID: sv.KeyID,
	}
}

func (sv SecureVariableEncrypted) Copy() SecureVariableEncrypted {
	return SecureVariableEncrypted{
		SecureVariableMetadata: sv.SecureVariableMetadata,
		SecureVariableData:     sv.SecureVariableData.Copy(),
	}
}

func (sv SecureVariableMetadata) Equals(sv2 SecureVariableMetadata) bool {
	return sv == sv2
}

func (sv SecureVariableDecrypted) Equals(sv2 SecureVariableDecrypted) bool {
	// FIXME: This should be a smarter equality check
	return sv.SecureVariableMetadata.Equals(sv2.SecureVariableMetadata) &&
		len(sv.Items) == len(sv2.Items) &&
		reflect.DeepEqual(sv.Items, sv2.Items)
}

func (sv SecureVariableDecrypted) Copy() SecureVariableDecrypted {
	out := SecureVariableDecrypted{
		SecureVariableMetadata: sv.SecureVariableMetadata,
		Items:                  make(SecureVariableItems, len(sv.Items)),
	}
	for k, v := range sv.Items {
		out.Items[k] = v
	}
	return out
}

func (sv SecureVariableEncrypted) Equals(sv2 SecureVariableEncrypted) bool {
	// FIXME: This should be a smarter equality check
	return sv.SecureVariableMetadata.Equals(sv2.SecureVariableMetadata) &&
		sv.KeyID == sv2.KeyID &&

		reflect.DeepEqual(sv.SecureVariableData, sv2.SecureVariableData)
}

func (sv SecureVariableDecrypted) Validate() error {
	if len(sv.Items) == 0 {
		return errors.New("empty variables are invalid")
	}
	return nil
}

func (sv *SecureVariableDecrypted) Canonicalize() {
	if sv.Namespace == "" {
		sv.Namespace = DefaultNamespace
	}
}

// GetNamespace returns the secure variable's namespace. Used for pagination.
func (sv SecureVariableMetadata) GetNamespace() string {
	return sv.Namespace
}

// GetID returns the secure variable's path. Used for pagination.
func (sv SecureVariableMetadata) GetID() string {
	return sv.Path
}

// GetCreateIndex returns the secure variable's create index. Used for pagination.
func (sv SecureVariableMetadata) GetCreateIndex() uint64 {
	return sv.CreateIndex
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
	Data []*SecureVariableDecrypted
	WriteRequest
}

type SecureVariablesEncryptedUpsertRequest struct {
	Data []*SecureVariableEncrypted
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
	Data []*SecureVariableMetadata
	QueryMeta
}

type SecureVariablesReadRequest struct {
	Path string
	QueryOptions
}

type SecureVariablesReadResponse struct {
	Data *SecureVariableDecrypted
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
		const keyBytes = 32
		key := make([]byte, keyBytes)
		n, err := cryptorand.Read(key)
		if err != nil {
			return nil, err
		}
		if n < keyBytes {
			return nil, fmt.Errorf("failed to generate key: entropy exhausted")
		}
		rootKey.Key = key
	}

	return rootKey, nil
}

// RootKeyMeta is the metadata used to refer to a RootKey. It is
// stored in raft.
type RootKeyMeta struct {
	KeyID       string // UUID
	Algorithm   EncryptionAlgorithm
	CreateTime  time.Time
	CreateIndex uint64
	ModifyIndex uint64
	State       RootKeyState
}

// RootKeyState enum describes the lifecycle of a root key.
type RootKeyState string

const (
	RootKeyStateInactive   RootKeyState = "inactive"
	RootKeyStateActive                  = "active"
	RootKeyStateRekeying                = "rekeying"
	RootKeyStateDeprecated              = "deprecated"
)

// NewRootKeyMeta returns a new RootKeyMeta with default values
func NewRootKeyMeta() *RootKeyMeta {
	return &RootKeyMeta{
		KeyID:      uuid.Generate(),
		Algorithm:  EncryptionAlgorithmAES256GCM,
		State:      RootKeyStateInactive,
		CreateTime: time.Now(),
	}
}

// RootKeyMetaStub is for serializing root key metadata to the
// keystore, not for the List API. It excludes frequently-changing
// fields such as ModifyIndex so we don't have to sync them to the
// on-disk keystore when the fields are already in raft.
type RootKeyMetaStub struct {
	KeyID      string
	Algorithm  EncryptionAlgorithm
	CreateTime time.Time
	State      RootKeyState
}

// Active indicates his key is the one currently being used for
// crypto operations (at most one key can be Active)
func (rkm *RootKeyMeta) Active() bool {
	return rkm.State == RootKeyStateActive
}

func (rkm *RootKeyMeta) SetActive() {
	rkm.State = RootKeyStateActive
}

// Rekeying indicates that variables encrypted with this key should be
// rekeyed
func (rkm *RootKeyMeta) Rekeying() bool {
	return rkm.State == RootKeyStateRekeying
}

func (rkm *RootKeyMeta) SetRekeying() {
	rkm.State = RootKeyStateRekeying
}

func (rkm *RootKeyMeta) SetInactive() {
	rkm.State = RootKeyStateInactive
}

// Deprecated indicates that variables encrypted with this key
// have been rekeyed
func (rkm *RootKeyMeta) Deprecated() bool {
	return rkm.State == RootKeyStateDeprecated
}

func (rkm *RootKeyMeta) SetDeprecated() {
	rkm.State = RootKeyStateDeprecated
}

func (rkm *RootKeyMeta) Stub() *RootKeyMetaStub {
	if rkm == nil {
		return nil
	}
	return &RootKeyMetaStub{
		KeyID:      rkm.KeyID,
		Algorithm:  rkm.Algorithm,
		CreateTime: rkm.CreateTime,
		State:      rkm.State,
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
	switch rkm.State {
	case RootKeyStateInactive, RootKeyStateActive,
		RootKeyStateRekeying, RootKeyStateDeprecated:
	default:
		return fmt.Errorf("root key state %q is invalid", rkm.State)
	}
	return nil
}

// EncryptionAlgorithm chooses which algorithm is used for
// encrypting / decrypting entries with this key
type EncryptionAlgorithm string

const (
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
	Rekey   bool
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
	Rekey       bool
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
