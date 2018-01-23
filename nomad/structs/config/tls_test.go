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
