package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTLSConfig_Merge(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{
		CAFile:   "test-ca-file",
		CertFile: "test-cert-file",
	}

	b := &TLSConfig{
		EnableHTTP:                  true,
		EnableRPC:                   true,
		VerifyServerHostname:        true,
		CAFile:                      "test-ca-file-2",
		CertFile:                    "test-cert-file-2",
		RPCUpgradeMode:              true,
		TLSCipherSuites:             "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		TLSMinVersion:               "tls12",
		TLSPreferServerCipherSuites: true,
	}

	new := a.Merge(b)
	assert.Equal(b, new)
}

func TestTLS_CertificateInfoIsEqual_TrueWhenEmpty(t *testing.T) {
	require := require.New(t)
	a := &TLSConfig{}
	b := &TLSConfig{}
	isEqual, err := a.CertificateInfoIsEqual(b)
	require.Nil(err)
	require.True(isEqual)
}

func TestTLS_CertificateInfoIsEqual_FalseWhenUnequal(t *testing.T) {
	require := require.New(t)
	const (
		cafile   = "../../../helper/tlsutil/testdata/ca.pem"
		foocert  = "../../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey   = "../../../helper/tlsutil/testdata/nomad-foo-key.pem"
		foocert2 = "../../../helper/tlsutil/testdata/nomad-bad.pem"
		fookey2  = "../../../helper/tlsutil/testdata/nomad-bad-key.pem"
	)

	// Assert that both mismatching certificate and key files are considered
	// unequal
	{
		a := &TLSConfig{
			CAFile:   cafile,
			CertFile: foocert,
			KeyFile:  fookey,
		}
		a.SetChecksum()

		b := &TLSConfig{
			CAFile:   cafile,
			CertFile: foocert2,
			KeyFile:  fookey2,
		}
		isEqual, err := a.CertificateInfoIsEqual(b)
		require.Nil(err)
		require.False(isEqual)
	}

	// Assert that mismatching certificate are considered unequal
	{
		a := &TLSConfig{
			CAFile:   cafile,
			CertFile: foocert,
			KeyFile:  fookey,
		}
		a.SetChecksum()

		b := &TLSConfig{
			CAFile:   cafile,
			CertFile: foocert2,
			KeyFile:  fookey,
		}
		isEqual, err := a.CertificateInfoIsEqual(b)
		require.Nil(err)
		require.False(isEqual)
	}

	// Assert that mismatching keys are considered unequal
	{
		a := &TLSConfig{
			CAFile:   cafile,
			CertFile: foocert,
			KeyFile:  fookey,
		}
		a.SetChecksum()

		b := &TLSConfig{
			CAFile:   cafile,
			CertFile: foocert,
			KeyFile:  fookey2,
		}
		isEqual, err := a.CertificateInfoIsEqual(b)
		require.Nil(err)
		require.False(isEqual)
	}

	// Assert that mismatching empty types are considered unequal
	{
		a := &TLSConfig{}

		b := &TLSConfig{
			CAFile:   cafile,
			CertFile: foocert,
			KeyFile:  fookey2,
		}
		isEqual, err := a.CertificateInfoIsEqual(b)
		require.Nil(err)
		require.False(isEqual)
	}

	// Assert that invalid files return an error
	{
		a := &TLSConfig{
			CAFile:   cafile,
			CertFile: foocert,
			KeyFile:  fookey2,
		}

		b := &TLSConfig{
			CAFile:   cafile,
			CertFile: "invalid_file",
			KeyFile:  fookey2,
		}
		isEqual, err := a.CertificateInfoIsEqual(b)
		require.NotNil(err)
		require.False(isEqual)
	}
}

// Certificate info should be equal when the CA file, certificate file, and key
// file all are equal
func TestTLS_CertificateInfoIsEqual_TrueWhenEqual(t *testing.T) {
	require := require.New(t)
	const (
		cafile  = "../../../helper/tlsutil/testdata/ca.pem"
		foocert = "../../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	a := &TLSConfig{
		CAFile:   cafile,
		CertFile: foocert,
		KeyFile:  fookey,
	}
	a.SetChecksum()

	b := &TLSConfig{
		CAFile:   cafile,
		CertFile: foocert,
		KeyFile:  fookey,
	}
	isEqual, err := a.CertificateInfoIsEqual(b)
	require.Nil(err)
	require.True(isEqual)
}

func TestTLS_Copy(t *testing.T) {
	require := require.New(t)
	const (
		cafile  = "../../../helper/tlsutil/testdata/ca.pem"
		foocert = "../../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	a := &TLSConfig{
		CAFile:                      cafile,
		CertFile:                    foocert,
		KeyFile:                     fookey,
		TLSCipherSuites:             "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
		TLSMinVersion:               "tls12",
		TLSPreferServerCipherSuites: true,
	}
	a.SetChecksum()

	aCopy := a.Copy()
	isEqual, err := a.CertificateInfoIsEqual(aCopy)
	require.Nil(err)
	require.True(isEqual)
}

// GetKeyLoader should always return an initialized KeyLoader for a TLSConfig
// object
func TestTLS_GetKeyloader(t *testing.T) {
	require := require.New(t)
	a := &TLSConfig{}
	require.NotNil(a.GetKeyLoader())
}

func TestTLS_SetChecksum(t *testing.T) {
	require := require.New(t)
	const (
		cafile   = "../../../helper/tlsutil/testdata/ca.pem"
		foocert  = "../../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey   = "../../../helper/tlsutil/testdata/nomad-foo-key.pem"
		foocert2 = "../../../helper/tlsutil/testdata/nomad-bad.pem"
		fookey2  = "../../../helper/tlsutil/testdata/nomad-bad-key.pem"
	)

	a := &TLSConfig{
		CAFile:   cafile,
		CertFile: foocert,
		KeyFile:  fookey,
	}
	a.SetChecksum()
	oldChecksum := a.Checksum

	a.CertFile = foocert2
	a.KeyFile = fookey2

	a.SetChecksum()

	require.NotEqual(oldChecksum, a.Checksum)
}
