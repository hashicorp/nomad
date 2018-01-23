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

func TestTLS_Equals_TrueWhenEmpty(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{}
	b := &TLSConfig{}
	assert.True(a.Equals(b))
}

func TestTLS_Equals_FalseWhenUnequal(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{CAFile: "abc", CertFile: "def", KeyFile: "ghi"}
	b := &TLSConfig{CAFile: "jkl", CertFile: "def", KeyFile: "ghi"}
	assert.False(a.Equals(b))
}

func TestTLS_Equals_TrueWhenEqual(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{CAFile: "abc", CertFile: "def", KeyFile: "ghi"}
	b := &TLSConfig{CAFile: "abc", CertFile: "def", KeyFile: "ghi"}
	assert.True(a.Equals(b))
}

func TestTLS_Copy(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{CAFile: "abc", CertFile: "def", KeyFile: "ghi"}
	aCopy := a.Copy()
	assert.True(a.Equals(aCopy))
}

// GetKeyLoader should always return an initialized KeyLoader for a TLSConfig
// object
func TestTLS_GetKeyloader(t *testing.T) {
	assert := assert.New(t)
	a := &TLSConfig{}
	assert.NotNil(a.GetKeyLoader())
}
