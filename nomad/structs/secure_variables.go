package structs

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
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

	// SecureVariablesApplyRPCMethod is the RPC method for upserting or
	// deleting a secure variable by its namespace and path, with optional
	// conflict detection.
	//
	// Args: SecureVariablesApplyRequest
	// Reply: SecureVariablesApplyResponse
	SecureVariablesApplyRPCMethod = "SecureVariables.Apply"

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

	// maxVariableSize is the maximum size of the unencrypted contents of
	// a variable. This size is deliberately set low and is not
	// configurable, to discourage DoS'ing the cluster
	maxVariableSize = 16384
)

// SecureVariableMetadata is the metadata envelope for a Secure Variable, it
// is the list object and is shared data between an SecureVariableEncrypted and
// a SecureVariableDecrypted object.
type SecureVariableMetadata struct {
	Namespace   string
	Path        string
	CreateIndex uint64
	CreateTime  int64
	ModifyIndex uint64
	ModifyTime  int64
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

func (svi SecureVariableItems) Size() uint64 {
	var out uint64
	for k, v := range svi {
		out += uint64(len(k))
		out += uint64(len(v))
	}
	return out
}

// Equals checks both the metadata and items in a SecureVariableDecrypted
// struct
func (v1 SecureVariableDecrypted) Equals(v2 SecureVariableDecrypted) bool {
	return v1.SecureVariableMetadata.Equals(v2.SecureVariableMetadata) &&
		v1.Items.Equals(v2.Items)
}

// Equals is a convenience method to provide similar equality checking
// syntax for metadata and the SecureVariablesData or SecureVariableItems
// struct
func (sv SecureVariableMetadata) Equals(sv2 SecureVariableMetadata) bool {
	return sv == sv2
}

// Equals performs deep equality checking on the cleartext items
// of a SecureVariableDecrypted. Uses reflect.DeepEqual
func (i1 SecureVariableItems) Equals(i2 SecureVariableItems) bool {
	return reflect.DeepEqual(i1, i2)
}

// Equals checks both the metadata and encrypted data for a
// SecureVariableEncrypted struct
func (v1 SecureVariableEncrypted) Equals(v2 SecureVariableEncrypted) bool {
	return v1.SecureVariableMetadata.Equals(v2.SecureVariableMetadata) &&
		v1.SecureVariableData.Equals(v2.SecureVariableData)
}

// Equals performs deep equality checking on the encrypted data part
// of a SecureVariableEncrypted
func (d1 SecureVariableData) Equals(d2 SecureVariableData) bool {
	return d1.KeyID == d2.KeyID &&
		bytes.Equal(d1.Data, d2.Data)
}

func (sv SecureVariableDecrypted) Copy() SecureVariableDecrypted {
	return SecureVariableDecrypted{
		SecureVariableMetadata: sv.SecureVariableMetadata,
		Items:                  sv.Items.Copy(),
	}
}

func (sv SecureVariableItems) Copy() SecureVariableItems {
	out := make(SecureVariableItems, len(sv))
	for k, v := range sv {
		out[k] = v
	}
	return out
}

func (sv SecureVariableEncrypted) Copy() SecureVariableEncrypted {
	return SecureVariableEncrypted{
		SecureVariableMetadata: sv.SecureVariableMetadata,
		SecureVariableData:     sv.SecureVariableData.Copy(),
	}
}

func (sv SecureVariableData) Copy() SecureVariableData {
	out := make([]byte, len(sv.Data))
	copy(out, sv.Data)
	return SecureVariableData{
		Data:  out,
		KeyID: sv.KeyID,
	}
}

func (sv SecureVariableDecrypted) Validate() error {

	if len(sv.Path) == 0 {
		return fmt.Errorf("variable requires path")
	}
	parts := strings.Split(sv.Path, "/")
	switch {
	case len(parts) == 1 && parts[0] == "nomad":
		return fmt.Errorf("\"nomad\" is a reserved top-level directory path, but you may write variables to \"nomad/jobs\" or below")
	case len(parts) >= 2 && parts[0] == "nomad" && parts[1] != "jobs":
		return fmt.Errorf("only paths at \"nomad/jobs\" or below are valid paths under the top-level \"nomad\" directory")
	}

	if len(sv.Items) == 0 {
		return errors.New("empty variables are invalid")
	}
	if sv.Items.Size() > maxVariableSize {
		return errors.New("variables are limited to 16KiB in total size")
	}
	if sv.Namespace == AllNamespacesSentinel {
		return errors.New("can not target wildcard (\"*\")namespace")
	}
	return nil
}

func (sv *SecureVariableDecrypted) Canonicalize() {
	if sv.Namespace == "" {
		sv.Namespace = DefaultNamespace
	}
}

// GetNamespace returns the secure variable's namespace. Used for pagination.
func (sv *SecureVariableMetadata) Copy() *SecureVariableMetadata {
	var out SecureVariableMetadata = *sv
	return &out
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

// SecureVariablesQuota is used to track the total size of secure variables
// entries per namespace. The total length of SecureVariable.EncryptedData in
// bytes will be added to the SecureVariablesQuota table in the same transaction
// as a write, update, or delete. This tracking effectively caps the maximum
// size of secure variables in a given namespace to MaxInt64 bytes.
type SecureVariablesQuota struct {
	Namespace   string
	Size        int64
	CreateIndex uint64
	ModifyIndex uint64
}

func (svq *SecureVariablesQuota) Copy() *SecureVariablesQuota {
	if svq == nil {
		return nil
	}
	nq := new(SecureVariablesQuota)
	*nq = *svq
	return nq
}

// ---------------------------------------
// RPC and FSM request/response objects

// SVOp constants give possible operations available in a transaction.
type SVOp string

const (
	SVOpSet       SVOp = "set"
	SVOpDelete    SVOp = "delete"
	SVOpDeleteCAS SVOp = "delete-cas"
	SVOpCAS       SVOp = "cas"
)

// SVOpResult constants give possible operations results from a transaction.
type SVOpResult string

const (
	SVOpResultOk       SVOpResult = "ok"
	SVOpResultConflict SVOpResult = "conflict"
	SVOpResultRedacted SVOpResult = "conflict-redacted"
	SVOpResultError    SVOpResult = "error"
)

// SecureVariablesApplyRequest is used by users to operate on the secure variable store
type SecureVariablesApplyRequest struct {
	Op  SVOp                     // Operation to be performed during apply
	Var *SecureVariableDecrypted // Variable-shaped request data
	WriteRequest
}

// SecureVariablesApplyResponse is sent back to the user to inform them of success or failure
type SecureVariablesApplyResponse struct {
	Op       SVOp                     // Operation performed
	Input    *SecureVariableDecrypted // Input supplied
	Result   SVOpResult               // Return status from operation
	Error    error                    // Error if any
	Conflict *SecureVariableDecrypted // Conflicting value if applicable
	Output   *SecureVariableDecrypted // Operation Result if successful; nil for successful deletes
	WriteMeta
}

func (r *SecureVariablesApplyResponse) IsOk() bool {
	return r.Result == SVOpResultOk
}

func (r *SecureVariablesApplyResponse) IsConflict() bool {
	return r.Result == SVOpResultConflict || r.Result == SVOpResultRedacted
}

func (r *SecureVariablesApplyResponse) IsError() bool {
	return r.Result == SVOpResultError
}

func (r *SecureVariablesApplyResponse) IsRedacted() bool {
	return r.Result == SVOpResultRedacted
}

// SVApplyStateRequest is used by the FSM to modify the secure variable store
type SVApplyStateRequest struct {
	Op  SVOp                     // Which operation are we performing
	Var *SecureVariableEncrypted // Which directory entry
	WriteRequest
}

// SVApplyStateResponse is used by the FSM to inform the RPC layer of success or failure
type SVApplyStateResponse struct {
	Op            SVOp                     // Which operation did we performing
	Result        SVOpResult               // What happened (ok, conflict, error)
	Error         error                    // error if any
	Conflict      *SecureVariableEncrypted // conflicting secure variable if applies
	WrittenSVMeta *SecureVariableMetadata  // for making the SecureVariablesApplyResponse
	WriteMeta
}

func (r *SVApplyStateRequest) ErrorResponse(raftIndex uint64, err error) *SVApplyStateResponse {
	return &SVApplyStateResponse{
		Op:        r.Op,
		Result:    SVOpResultError,
		Error:     err,
		WriteMeta: WriteMeta{Index: raftIndex},
	}
}

func (r *SVApplyStateRequest) SuccessResponse(raftIndex uint64, meta *SecureVariableMetadata) *SVApplyStateResponse {
	return &SVApplyStateResponse{
		Op:            r.Op,
		Result:        SVOpResultOk,
		WrittenSVMeta: meta,
		WriteMeta:     WriteMeta{Index: raftIndex},
	}
}

func (r *SVApplyStateRequest) ConflictResponse(raftIndex uint64, cv *SecureVariableEncrypted) *SVApplyStateResponse {
	var cvCopy SecureVariableEncrypted
	if cv != nil {
		// make a copy so that we aren't sending
		// the live state store version
		cvCopy = cv.Copy()
	}
	return &SVApplyStateResponse{
		Op:        r.Op,
		Result:    SVOpResultConflict,
		Conflict:  &cvCopy,
		WriteMeta: WriteMeta{Index: raftIndex},
	}
}

func (r *SVApplyStateResponse) IsOk() bool {
	return r.Result == SVOpResultOk
}

func (r *SVApplyStateResponse) IsConflict() bool {
	return r.Result == SVOpResultConflict
}

func (r *SVApplyStateResponse) IsError() bool {
	// FIXME: This is brittle and requires immense faith that
	// the response is properly managed.
	return r.Result == SVOpResultError
}

// TODO delete everything between the two lines below
// ----------------------------------------------------

type SecureVariablesUpsertRequest struct {
	Data       []*SecureVariableDecrypted
	CheckIndex *uint64
	WriteRequest
}

func (svur *SecureVariablesUpsertRequest) SetCheckIndex(ci uint64) {
	svur.CheckIndex = &ci
}

type SecureVariablesEncryptedUpsertRequest struct {
	Data []*SecureVariableEncrypted
	WriteRequest
}

type SecureVariablesUpsertResponse struct {
	Conflicts []*SecureVariableDecrypted
	WriteMeta
}

type SecureVariablesListRequest struct {
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
	Path       string
	CheckIndex *uint64
	WriteRequest
}

func (svdr *SecureVariablesDeleteRequest) SetCheckIndex(ci uint64) {
	svdr.CheckIndex = &ci
}

type SecureVariablesDeleteResponse struct {
	Conflict *SecureVariableDecrypted
	WriteMeta
}

// ----------------------------------------------------
// TODO delete everything between the two lines above

// ---------------------------------------
// Keyring state and RPC objects

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
	CreateTime  int64
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
	now := time.Now().UTC().UnixNano()
	return &RootKeyMeta{
		KeyID:      uuid.Generate(),
		Algorithm:  EncryptionAlgorithmAES256GCM,
		State:      RootKeyStateInactive,
		CreateTime: now,
	}
}

// RootKeyMetaStub is for serializing root key metadata to the
// keystore, not for the List API. It excludes frequently-changing
// fields such as ModifyIndex so we don't have to sync them to the
// on-disk keystore when the fields are already in raft.
type RootKeyMetaStub struct {
	KeyID      string
	Algorithm  EncryptionAlgorithm
	CreateTime int64
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
