package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestBuildClientAssertionJWT_ClientSecret(t *testing.T) {

	tests := []struct {
		name        string
		config      *structs.ACLAuthMethodConfig
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid client secret",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:     "test-client-id",
				OIDCClientSecret: "1234567890abcdefghijklmnopqrstuvwxyz",
				OIDCEnablePKCE:   true,
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceClientSecret,
					KeyAlgorithm: "HS256",
					Audience:     []string{"test-audience"},
					ExtraHeaders: map[string]string{
						"test-header": "test-value",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid client secret no PKCE",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:     "test-client-id",
				OIDCClientSecret: "1234567890abcdefghijklmnopqrstuvwxyz",
				OIDCEnablePKCE:   false, // Should we default this to true everywhere?
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceClientSecret,
					KeyAlgorithm: "HS256",
					Audience:     []string{"test-audience"},
					ExtraHeaders: map[string]string{
						"test-header": "test-value",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nil config",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:        "test-client-id",
				OIDCClientSecret:    "test-client-secret",
				OIDCClientAssertion: nil,
			},
			wantErr:     true,
			expectedErr: `no auth method config or client assertion`,
		},
		{
			name: "invalid client secret length",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:     "test-client-id",
				OIDCClientSecret: "test-client-secret",
				OIDCEnablePKCE:   true,
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceClientSecret,
					KeyAlgorithm: "HS256",
					Audience:     []string{"test-audience"},
					ExtraHeaders: map[string]string{
						"test-header": "test-value",
					},
				},
			},
			wantErr:     true,
			expectedErr: `invalid secret length for algorithm: "HS256" must be at least 32 bytes long`,
		},
		{
			name: "invalid client secret kid in extra header",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:     "test-client-id",
				OIDCClientSecret: "1234567890abcdefghijklmnopqrstuvwxyz",
				OIDCEnablePKCE:   true,
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceClientSecret,
					KeyAlgorithm: "HS256",
					Audience:     []string{"test-audience"},
					ExtraHeaders: map[string]string{
						"kid": "test-kid",
					},
				},
			},
			wantErr:     true,
			expectedErr: `WithHeaders: "kid" not allowed in WithHeaders; use WithKeyID instead`,
		},
		{
			name: "invalid key algorithm none",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:     "test-client-id",
				OIDCClientSecret: "1234567890abcdefghijklmnopqrstuvwxyz",
				OIDCEnablePKCE:   true,
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceClientSecret,
					KeyAlgorithm: "none",
					Audience:     []string{"test-audience"},
					ExtraHeaders: map[string]string{
						"test-header": "test-value",
					},
				},
			},
			wantErr:     true,
			expectedErr: `unsupported algorithm "none" for client secret`,
		},
		{
			name: "invalid key algorithm None",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:     "test-client-id",
				OIDCClientSecret: "1234567890abcdefghijklmnopqrstuvwxyz",
				OIDCEnablePKCE:   true,
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceClientSecret,
					KeyAlgorithm: "None",
					Audience:     []string{"test-audience"},
					ExtraHeaders: map[string]string{
						"test-header": "test-value",
					},
				},
			},
			wantErr:     true,
			expectedErr: `unsupported algorithm "None" for client secret`,
		},
		// expected non-nil error; got nil
		{
			name: "invalid missing issuer",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:     "test-client-id",
				OIDCClientSecret: "1234567890abcdefghijklmnopqrstuvwxyz",
				OIDCEnablePKCE:   true,
				BoundIssuer:      nil,
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceClientSecret,
					KeyAlgorithm: "HS256",
					ExtraHeaders: map[string]string{
						"test-header": "test-value",
					},
				},
			},
			wantErr:     true,
			expectedErr: "missing issuer",
		},
		// expected non-nil error; got nil
		{
			name: "invalid missing audience with empty discovery url",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:     "test-client-id",
				OIDCClientSecret: "1234567890abcdefghijklmnopqrstuvwxyz",
				OIDCEnablePKCE:   true,
				OIDCDiscoveryURL: "",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceClientSecret,
					KeyAlgorithm: "HS256",
					ExtraHeaders: map[string]string{
						"test-header": "test-value",
					},
				},
			},
			wantErr:     true,
			expectedErr: "missing audience with empty discovery URL",
		},
		// expected non-nil error; got nil
		{
			name: "invalid missing audience",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:     "test-client-id",
				OIDCClientSecret: "1234567890abcdefghijklmnopqrstuvwxyz",
				OIDCEnablePKCE:   true,
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceClientSecret,
					KeyAlgorithm: "HS256",
					ExtraHeaders: map[string]string{
						"test-header": "test-value",
					},
				},
			},
			wantErr:     true,
			expectedErr: "missing audience",
		},
		// expected non-nil error; got nil
		{
			name: "inexistent discovery URL",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:     "test-client-id",
				OIDCClientSecret: "1234567890abcdefghijklmnopqrstuvwxyz",
				OIDCEnablePKCE:   true,
				OIDCDiscoveryURL: "https://test-discovery-url",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceClientSecret,
					KeyAlgorithm: "HS256",
					ExtraHeaders: map[string]string{
						"test-header": "test-value",
					},
				},
			},
			wantErr:     true,
			expectedErr: "inexistent discovery URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.Canonicalize() // inherits clientSecret from OIDCClientAssertion
			tt.config.Validate()     // validates clientSecret length
			jwt, err := BuildClientAssertionJWT(tt.config, nil, "")
			if tt.wantErr {
				must.Error(t, err)
				must.StrContains(t, err.Error(), tt.expectedErr)
			} else {
				must.NoError(t, err)
				must.NotNil(t, jwt)
			}
		})
	}
}

func TestBuildClientAssertionJWT_PrivateKey(t *testing.T) {
	nomadKey := generateTestPrivateKey(t)
	nomadKID := "test-kid"

	tests := []struct {
		name    string
		config  *structs.ACLAuthMethodConfig
		wantErr bool
	}{
		{
			name: "nil config",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientAssertion: nil,
			},
			wantErr: true,
		},
		{
			name: "nomad key source",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceNomad,
					KeyAlgorithm: "RS256",
					Audience:     []string{"test-audience"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwt, err := BuildClientAssertionJWT(tt.config, nomadKey, nomadKID)
			if tt.wantErr {
				must.Error(t, err)
			} else {
				must.NoError(t, err)
				must.NotNil(t, jwt)
			}
		})
	}
}

func TestBuildClientAssertionJWT_NomadKey(t *testing.T) {
	nomadKey := generateTestPrivateKey(t)
	nomadKID := "test-kid"

	tests := []struct {
		name    string
		config  *structs.ACLAuthMethodConfig
		wantErr bool
	}{
		{
			name: "nil config",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientAssertion: nil,
			},
			wantErr: true,
		},
		{
			name: "private key source",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					KeyAlgorithm: "RS256",
					Audience:     []string{"test-audience"},
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKeyBase64: encodeTestPrivateKey(t, nomadKey),
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwt, err := BuildClientAssertionJWT(tt.config, nomadKey, nomadKID)
			if tt.wantErr {
				must.Error(t, err)
			} else {
				must.NoError(t, err)
				must.NotNil(t, jwt)
			}
		})
	}
}
func generateTestPrivateKey(t *testing.T) *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	must.NoError(t, err)

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "testkey.pem")

	keyFile, err := os.Create(keyPath)
	must.NoError(t, err)
	defer keyFile.Close()

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}
	err = pem.Encode(keyFile, block)
	must.NoError(t, err)

	return key
}

func encodeTestPrivateKey(t *testing.T, key *rsa.PrivateKey) string {
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}
	return string(pem.EncodeToMemory(block))
}
