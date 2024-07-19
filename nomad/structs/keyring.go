// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"time"

	"github.com/go-jose/go-jose/v3"
	wrapping "github.com/hashicorp/go-kms-wrapping/v2"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/crypto"
	"github.com/hashicorp/nomad/helper/uuid"
)

const (
	// PubKeyAlgEdDSA is the JWA (JSON Web Algorithm) for ed25519 public keys
	// used for signatures.
	PubKeyAlgEdDSA = string(jose.EdDSA)

	// PubKeyAlgRS256 is the JWA for RSA public keys used for signatures. Support
	// is required by AWS OIDC IAM Provider.
	PubKeyAlgRS256 = string(jose.RS256)

	// PubKeyUseSig is the JWK (JSON Web Key) "use" parameter value for
	// signatures.
	PubKeyUseSig = "sig"

	// JWKSPath is the path component of the URL to Nomad's JWKS endpoint.
	JWKSPath = "/.well-known/jwks.json"
)

// RootKey is used to encrypt and decrypt variables. It is never stored in raft.
type RootKey struct {
	Meta *RootKeyMeta
	Key  []byte // serialized to keystore as base64 blob

	// RSAKey is the private key used to sign workload identity JWTs with the
	// RS256 algorithm. It is stored in its PKCS #1, ASN.1 DER form. See
	// x509.MarshalPKCS1PrivateKey for details.
	RSAKey []byte
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
		key, err := crypto.Bytes(32)
		if err != nil {
			return nil, fmt.Errorf("failed to generate root key: %w", err)
		}
		rootKey.Key = key
	}

	// Generate RSA key for signing workload identity JWTs with RS256.
	rsaPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate rsa key: %w", err)
	}

	rootKey.RSAKey = x509.MarshalPKCS1PrivateKey(rsaPrivateKey)

	return rootKey, nil
}

func (k *RootKey) Copy() *RootKey {
	return &RootKey{
		Meta:   k.Meta.Copy(),
		Key:    slices.Clone(k.Key),
		RSAKey: slices.Clone(k.RSAKey),
	}
}

// MakeInactive returns a copy of the RootKey with the meta state set to active
func (k *RootKey) MakeActive() *RootKey {
	return &RootKey{
		Meta:   k.Meta.MakeActive(),
		Key:    slices.Clone(k.Key),
		RSAKey: slices.Clone(k.RSAKey),
	}
}

// MakeInactive returns a copy of the RootKey with the meta state set to
// inactive
func (k *RootKey) MakeInactive() *RootKey {
	return &RootKey{
		Meta:   k.Meta.MakeInactive(),
		Key:    slices.Clone(k.Key),
		RSAKey: slices.Clone(k.RSAKey),
	}
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
	PublishTime int64
}

// KEKProviderName enum are the built-in KEK providers.
type KEKProviderName string

const (
	KEKProviderAEAD          KEKProviderName = "aead"
	KEKProviderAWSKMS                        = "awskms"
	KEKProviderAzureKeyVault                 = "azurekeyvault"
	KEKProviderGCPCloudKMS                   = "gcpckms"
	KEKProviderVaultTransit                  = "transit"
)

// KEKProviderConfig is the server configuration for an external KMS provider
// the server will use as a Key Encryption Key (KEK) for encrypting/decrypting
// the DEK.
type KEKProviderConfig struct {
	Provider string            `hcl:",key"`
	Name     string            `hcl:"name"`
	Active   bool              `hcl:"active"`
	Config   map[string]string `hcl:"-" json:"-"`

	// ExtraKeysHCL gets used by HCL to surface unknown keys. The parser will
	// then read these keys to create the Config map, so that we don't need a
	// nested "config" block/map in the config file
	ExtraKeysHCL []string `hcl:",unusedKeys" json:"-"`
}

func (c *KEKProviderConfig) Copy() *KEKProviderConfig {
	return &KEKProviderConfig{
		Provider: c.Provider,
		Active:   c.Active,
		Name:     c.Name,
		Config:   maps.Clone(c.Config),
	}
}

// Merge is used to merge two configurations. Note that Provider and Name should
// always be identical before we merge.
func (c *KEKProviderConfig) Merge(o *KEKProviderConfig) *KEKProviderConfig {
	result := c.Copy()
	result.Active = o.Active
	for k, v := range o.Config {
		result.Config[k] = v
	}
	return result
}

func (c *KEKProviderConfig) ID() string {
	if c.Name == "" {
		return c.Provider
	}
	return c.Provider + "." + c.Name
}

// RootKeyState enum describes the lifecycle of a root key.
type RootKeyState string

const (
	RootKeyStateInactive     RootKeyState = "inactive"
	RootKeyStateActive                    = "active"
	RootKeyStateRekeying                  = "rekeying"
	RootKeyStatePrepublished              = "prepublished"

	// RootKeyStateDeprecated is, itself, deprecated and is no longer in
	// use. For backwards compatibility, any existing keys with this state will
	// be treated as RootKeyStateInactive
	RootKeyStateDeprecated = "deprecated"
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

// IsActive indicates this key is the one currently being used for crypto
// operations (at most one key can be Active)
func (rkm *RootKeyMeta) IsActive() bool {
	return rkm.State == RootKeyStateActive
}

// MakeActive returns a copy of the RootKeyMeta with the state set to active
func (rkm *RootKeyMeta) MakeActive() *RootKeyMeta {
	out := rkm.Copy()
	if out != nil {
		out.State = RootKeyStateActive
		out.PublishTime = 0
	}
	return out
}

// IsRekeying indicates that variables encrypted with this key should be
// rekeyed
func (rkm *RootKeyMeta) IsRekeying() bool {
	return rkm.State == RootKeyStateRekeying
}

// MakeRekeying returns a copy of the RootKeyMeta with the state set to rekeying
func (rkm *RootKeyMeta) MakeRekeying() *RootKeyMeta {
	out := rkm.Copy()
	if out != nil {
		out.State = RootKeyStateRekeying
	}
	return out
}

// MakePrepublished returns a copy of the RootKeyMeta with the state set to
// prepublished at the time t
func (rkm *RootKeyMeta) MakePrepublished(t int64) *RootKeyMeta {
	out := rkm.Copy()
	if out != nil {
		out.PublishTime = t
		out.State = RootKeyStatePrepublished
	}
	return out
}

// IsPrepublished indicates that this key has been published and is pending
// being promoted to active
func (rkm *RootKeyMeta) IsPrepublished() bool {
	return rkm.State == RootKeyStatePrepublished
}

// MakeInactive returns a copy of the RootKeyMeta with the state set to inactive
func (rkm *RootKeyMeta) MakeInactive() *RootKeyMeta {
	out := rkm.Copy()
	if out != nil {
		out.State = RootKeyStateInactive
	}
	return out
}

// IsInactive indicates that this key is no longer being used to encrypt new
// variables or workload identities.
func (rkm *RootKeyMeta) IsInactive() bool {
	return rkm.State == RootKeyStateInactive || rkm.State == RootKeyStateDeprecated
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
		RootKeyStateRekeying, RootKeyStateDeprecated, RootKeyStatePrepublished:
	default:
		return fmt.Errorf("root key state %q is invalid", rkm.State)
	}
	return nil
}

// KeyEncryptionKeyWrapper is the struct that gets serialized for the on-disk
// KMS wrapper. When using the AEAD provider, this struct includes the
// server-specific key-wrapping key. This struct should never be sent over RPC
// or written to Raft.
type KeyEncryptionKeyWrapper struct {
	Meta *RootKeyMeta

	Provider                 string             `json:"Provider,omitempty"`
	ProviderID               string             `json:"ProviderID,omitempty"`
	WrappedDataEncryptionKey *wrapping.BlobInfo `json:"WrappedDEK,omitempty"`
	WrappedRSAKey            *wrapping.BlobInfo `json:"WrappedRSAKey,omitempty"`
	KeyEncryptionKey         []byte             `json:"KEK,omitempty"`

	// These fields were used for AEAD before we added support for external
	// KMS. The wrapped key returned from the go-kms-wrapper library includes
	// the ciphertext but we need all the fields in order to decrypt. We'll
	// leave these fields so we can load keys from older servers.
	EncryptedDataEncryptionKey []byte `json:"DEK,omitempty"`
	EncryptedRSAKey            []byte `json:"RSAKey,omitempty"`
}

// EncryptionAlgorithm chooses which algorithm is used for
// encrypting / decrypting entries with this key
type EncryptionAlgorithm string

const (
	EncryptionAlgorithmAES256GCM EncryptionAlgorithm = "aes256-gcm"
)

// KeyringRotateRootKeyRequest is the argument to the Keyring.Rotate RPC
type KeyringRotateRootKeyRequest struct {
	Algorithm   EncryptionAlgorithm
	Full        bool
	PublishTime int64
	WriteRequest
}

// KeyringRotateRootKeyResponse returns the full key metadata
type KeyringRotateRootKeyResponse struct {
	Key *RootKeyMeta
	WriteMeta
}

// KeyringListRootKeyMetaRequest is the argument to the Keyring.List RPC
type KeyringListRootKeyMetaRequest struct {
	QueryOptions
}

// KeyringListRootKeyMetaRequest is the response value of the List RPC
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
// metadata to the FSM without including the key material.
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

// KeyringListPublicResponse lists public key components of signing keys. Used
// to build a JWKS endpoint.
type KeyringListPublicResponse struct {
	PublicKeys []*KeyringPublicKey

	// RotationThreshold exposes root_key_rotation_threshold so that HTTP
	// endpoints may set a reasonable cache control header informing consumers
	// when to expect a new key.
	RotationThreshold time.Duration

	QueryMeta
}

// KeyringPublicKey is the public key component of a signing key. Used to build
// a JWKS endpoint.
type KeyringPublicKey struct {
	KeyID string

	// PublicKey must be read via GetPublicKey for use with cryptographic
	// functions such as go-jose's Claims(pubKey, claims) as those functions
	// inspect the concrete type to vary behavior.
	PublicKey []byte

	// Algorithm should be the JWT "alg" parameter. So "EdDSA" for Ed25519 public
	// keys used to validate signatures.
	Algorithm string

	// Use should be the JWK "use" parameter as defined in
	// https://datatracker.ietf.org/doc/html/rfc7517#section-4.2.
	//
	// "sig" and "enc" being the two standard values with "sig" being the use for
	// workload identity JWT signing.
	Use string

	// CreateTime + root_key_rotation_threshold = when consumers should look for
	// a new key. Therefore this field can be used for cache control.
	CreateTime int64
}

// GetPublicKey returns the concrete PublicKey type. This *must* be used to
// retrieve the public key as functions such as go-jose's Claims(pubKey,
// claims) inspect pubKey's concrete type.
func (pubKey *KeyringPublicKey) GetPublicKey() (any, error) {
	switch alg := pubKey.Algorithm; alg {

	case PubKeyAlgEdDSA:
		// Convert public key bytes to an ed25519 public key
		return ed25519.PublicKey(pubKey.PublicKey), nil

	case PubKeyAlgRS256:
		// PEM -> rsa.PublickKey
		rsaPubKey, err := x509.ParsePKCS1PublicKey(pubKey.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s public key: %w", alg, err)
		}
		return rsaPubKey, nil

	default:
		return nil, fmt.Errorf("unknown algorithm: %q", alg)
	}
}

// KeyringGetConfigResponse is the response for Keyring.GetConfig RPCs.
type KeyringGetConfigResponse struct {
	OIDCDiscovery *OIDCDiscoveryConfig
}

// OIDCDiscoveryConfig represents the response to OIDC Discovery requests
// usually at: /.well-known/openid-configuration
//
// Only the fields Nomad uses are implemented since many fields in the
// specification are not relevant to Nomad's use case:
// https://openid.net/specs/openid-connect-discovery-1_0.html
type OIDCDiscoveryConfig struct {
	Issuer        string   `json:"issuer"`
	JWKS          string   `json:"jwks_uri"`
	IDTokenAlgs   []string `json:"id_token_signing_alg_values_supported"`
	ResponseTypes []string `json:"response_types_supported"`
	Subjects      []string `json:"subject_types_supported"`
}

// NewOIDCDiscoveryConfig returns a populated OIDCDiscoveryConfig or an error.
func NewOIDCDiscoveryConfig(issuer string) (*OIDCDiscoveryConfig, error) {
	if issuer == "" {
		// url.JoinPath doesn't mind empty strings, so check for it specifically.
		// Likely a programming error as we shouldn't even be trying to create OIDC
		// Discovery configurations without an issuer explicitly set.
		return nil, fmt.Errorf("issuer must not be empty")
	}

	jwksURL, err := url.JoinPath(issuer, JWKSPath)
	if err != nil {
		return nil, fmt.Errorf("error determining jwks path: %w", err)
	}

	disc := &OIDCDiscoveryConfig{
		Issuer: issuer,
		JWKS:   jwksURL,

		// RS256 is required by the OIDC spec and some third parties such as AWS's
		// IAM OIDC Identity Provider. Prior to v1.7 Nomad default to EdDSA so
		// advertise support for backward compatibility.
		IDTokenAlgs: []string{PubKeyAlgRS256, PubKeyAlgEdDSA},

		ResponseTypes: []string{"code"},
		Subjects:      []string{"public"},
	}

	return disc, nil
}
