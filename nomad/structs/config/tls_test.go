package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTLSConfig_Merge(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{
		CAFile:   "test-ca-file",
		CertFile: "test-cert-file",
	}

	b := &TLSConfig{
		EnableHTTP:           true,
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               "test-ca-file-2",
		CertFile:             "test-cert-file-2",
		RPCUpgradeMode:       true,
	}

	new := a.Merge(b)
	assert.Equal(b, new)
}

func TestTLS_CertificateInfoIsEqual_TrueWhenEmpty(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{}
	b := &TLSConfig{}
	assert.True(a.CertificateInfoIsEqual(b))
}

func TestTLS_CertificateInfoIsEqual_FalseWhenUnequal(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{CAFile: "abc", CertFile: "def", KeyFile: "ghi"}
	b := &TLSConfig{CAFile: "jkl", CertFile: "def", KeyFile: "ghi"}
	assert.False(a.CertificateInfoIsEqual(b))
}

func TestTLS_CertificateInfoIsEqual_TrueWhenEqual(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{CAFile: "abc", CertFile: "def", KeyFile: "ghi"}
	b := &TLSConfig{CAFile: "abc", CertFile: "def", KeyFile: "ghi"}
	assert.True(a.CertificateInfoIsEqual(b))
}

func TestTLS_Copy(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{CAFile: "abc", CertFile: "def", KeyFile: "ghi"}
	aCopy := a.Copy()
	assert.True(a.CertificateInfoIsEqual(aCopy))
}

// GetKeyLoader should always return an initialized KeyLoader for a TLSConfig
// object
func TestTLS_GetKeyloader(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{}
	assert.NotNil(a.GetKeyLoader())
}
