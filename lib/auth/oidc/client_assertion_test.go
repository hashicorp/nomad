package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"math/big"
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
	nomadKeyPath := writeTestPrivateKeyToFile(t, nomadKey)
	nomadKID := generateTestKeyID(nomadKey)
	nomadCert := generateTestCertificate(t, nomadKey)
	nomadCertPath := writeTestCertToFile(t, nomadCert)

	tests := []struct {
		name        string
		config      *structs.ACLAuthMethodConfig
		wantErr     bool
		expectedErr string
	}{
		{
			name: "nil config",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientAssertion: nil,
			},
			wantErr: true,
		},
		{
			name: "valid private key source with pem key base64",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKeyBase64: encodeTestPrivateKeyBase64(t, nomadKey),
						KeyID:        nomadKID,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid private key source with pem key base64 with pem cert base64",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKeyBase64:  encodeTestPrivateKeyBase64(t, nomadKey),
						PemCertBase64: encodeTestCertBase64(t, nomadCert),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid private key source with pem cert file",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKeyBase64: encodeTestPrivateKeyBase64(t, nomadKey),
						PemCertFile:  nomadCertPath,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid private key source with pem key file with Key ID",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKeyFile: nomadKeyPath,
						KeyID:      nomadKID,
					},
				},
			},
			wantErr: false,
		},
		// invalid pem key file location
		{
			name: "invalid private key source with pem key file",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKeyFile: nomadKeyPath + "/invalid",
						KeyID:      nomadKID,
					},
				},
			},
			wantErr:     true,
			expectedErr: "invalid path for private key file",
		},
		// path traversal in pem key file
		{
			name: "invalid private key source with pem key file path traversal",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKeyFile: "../../../../../../../../../../../../etc/passwd",
						KeyID:      nomadKID,
					},
				},
			},
			wantErr:     true,
			expectedErr: "invalid path for private key file",
		},
		{
			name: "invalid private key source with pem cert file path traversal",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKeyBase64: encodeTestPrivateKeyBase64(t, nomadKey),
						PemCertFile:  "../../../../../../../../../../../../etc/passwd",
					},
				},
			},
			wantErr:     true,
			expectedErr: "invalid path for private key file",
		},
		{
			name: "invalid private key source with invalid kid ID",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKeyBase64: encodeTestPrivateKeyBase64(t, nomadKey),
						KeyID:        "invalid-kid",
					},
				},
			},
			wantErr:     true,
			expectedErr: "invalid path for private key file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jwt, err := BuildClientAssertionJWT(tt.config, nomadKey, nomadKID)
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

func TestBuildClientAssertionJWT_NomadKey(t *testing.T) {
	nomadKey := generateTestPrivateKey(t)
	nomadKID := generateTestKeyID(nomadKey)

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
			tt.config.Canonicalize() // inherits clientSecret from OIDCClientAssertion
			tt.config.Validate()     // validates clientSecret length
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

func generateTestKeyID(key *rsa.PrivateKey) string {
	keyBytes := x509.MarshalPKCS1PublicKey(&key.PublicKey)
	thumbprint := sha256.Sum256(keyBytes)
	return base64.URLEncoding.EncodeToString(thumbprint[:])
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

func writeTestPrivateKeyToFile(t *testing.T, key *rsa.PrivateKey) string {
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

	return keyPath
}

func encodeTestPrivateKeyBase64(t *testing.T, key *rsa.PrivateKey) string {
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}
	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(block))
}

func generateTestCertificate(t *testing.T, key *rsa.PrivateKey) *x509.Certificate {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	must.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	must.NoError(t, err)

	return cert
}

func encodeTestCertBase64(t *testing.T, cert *x509.Certificate) string {
	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}
	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(block))
}

func writeTestCertToFile(t *testing.T, cert *x509.Certificate) string {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "testcert.pem")

	certFile, err := os.Create(certPath)
	must.NoError(t, err)
	defer certFile.Close()

	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}
	err = pem.Encode(certFile, block)
	must.NoError(t, err)

	return certPath
}
