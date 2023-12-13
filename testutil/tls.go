// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"crypto/x509"
	"io/fs"
	"os"
	"testing"

	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/stretchr/testify/require"
)

// Assert CA file exists and is a valid CA Returns the CA
func IsValidCertificate(t *testing.T, caPath string) *x509.Certificate {
	t.Helper()

	require.FileExists(t, caPath)
	caData, err := os.ReadFile(caPath)
	require.NoError(t, err)

	ca, err := tlsutil.ParseCert(string(caData))
	require.NoError(t, err)
	require.NotNil(t, ca)

	return ca
}

// Assert key file exists and is a valid signer returns a bool
func IsValidSigner(t *testing.T, keyPath string) bool {
	t.Helper()

	require.FileExists(t, keyPath)
	fi, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal("should not happen", err)
	}
	if want, have := fs.FileMode(0600), fi.Mode().Perm(); want != have {
		t.Fatalf("private key file %s: permissions: want: %o; have: %o", keyPath, want, have)
	}

	keyData, err := os.ReadFile(keyPath)
	require.NoError(t, err)

	signer, err := tlsutil.ParseSigner(string(keyData))
	require.NoError(t, err)
	require.NotNil(t, signer)

	return true
}
