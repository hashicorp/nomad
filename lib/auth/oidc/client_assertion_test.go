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
	"time"

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
			name: "invalid missing audience",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID:     "test-client-id",
				OIDCClientSecret: "1234567890abcdefghijklmnopqrstuvwxyz",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourceClientSecret,
					KeyAlgorithm: "HS256",
					ExtraHeaders: map[string]string{
						"test-header": "test-value",
					},
				},
			},
			wantErr:     true,
			expectedErr: "missing Audience",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.Canonicalize() // inherits ClientSecret from OIDCClientAssertion
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
			name: "valid private key source with pem key",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKey: encodeTestPrivateKey(nomadKey),
						KeyID:  nomadKID,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid private key source with pem key with pem cert",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKey:  encodeTestPrivateKey(nomadKey),
						PemCert: encodeTestCert(nomadCert),
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
						PemKey:      encodeTestPrivateKey(nomadKey),
						PemCertFile: nomadCertPath,
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
			expectedErr: "error reading PemKeyFile",
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
						PemKeyFile: "../../../../../../../../../../../../etc/passwd", // TODO: this gets read, but errors as "invalid key"
						KeyID:      nomadKID,
					},
				},
			},
			wantErr:     true,
			expectedErr: "error parsing PemKeyFile: invalid key:",
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
						PemKey:      encodeTestPrivateKey(nomadKey),
						PemCertFile: "../../../../../../../../../../../../etc/passwd", // TODO: this gets read, but "failed to decode"
					},
				},
			},
			wantErr:     true,
			expectedErr: "failed to decode PemCertFile PEM block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.Canonicalize() // inherits clientSecret from OIDCClientAssertion
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

func TestBuildClientAssertionJWT_PrivateKeyExpiredCert(t *testing.T) {
	nomadKey := generateTestPrivateKey(t)
	nomadInvalidKey := generateInvalidTestPrivateKey(t)
	nomadKID := generateTestKeyID(nomadKey)
	nomadCert := generateTestCertificate(t, nomadKey)
	nomadExpiredCert := generateExpiredTestCertificate(t, nomadKey)

	tests := []struct {
		name        string
		config      *structs.ACLAuthMethodConfig
		wantErr     bool
		expectedErr string
	}{
		{
			name: "invalid PemKeyBase64",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKey:  encodeTestPrivateKey(nomadInvalidKey),
						PemCert: encodeTestCert(nomadCert),
					},
				},
			},
			wantErr:     true,
			expectedErr: "failed to parse private key",
		},
		{
			name: "expired certificate PemCertBase64",
			config: &structs.ACLAuthMethodConfig{
				OIDCClientID: "test-client-id",
				OIDCClientAssertion: &structs.OIDCClientAssertion{
					KeySource:    structs.OIDCKeySourcePrivateKey,
					Audience:     []string{"test-audience"},
					KeyAlgorithm: "RS256",
					PrivateKey: &structs.OIDCClientAssertionKey{
						PemKey:  encodeTestPrivateKey(nomadKey),
						PemCert: encodeTestCert(nomadExpiredCert),
					},
				},
			},
			wantErr:     true,
			expectedErr: "certificate has expired or is not yet valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.Canonicalize() // inherits clientSecret from OIDCClientAssertion
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

func encodeTestPrivateKey(key *rsa.PrivateKey) string {
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}
	return string(pem.EncodeToMemory(block))
}

func generateTestCertificate(t *testing.T, key *rsa.PrivateKey) *x509.Certificate {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	must.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	must.NoError(t, err)

	return cert
}

func encodeTestCert(cert *x509.Certificate) string {
	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}
	return string(pem.EncodeToMemory(block))
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

func generateExpiredTestCertificate(t *testing.T, key *rsa.PrivateKey) *x509.Certificate {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-2 * time.Hour),
		NotAfter:     time.Now().Add(-1 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	must.NoError(t, err)

	cert, err := x509.ParseCertificate(certDER)
	must.NoError(t, err)

	return cert
}

func generateInvalidTestPrivateKey(t *testing.T) *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	must.NoError(t, err)

	// Simulate an invalid key by modifying the key's modulus
	key.N = big.NewInt(0) // This is just a placeholder to simulate an invalid key

	return key
}
