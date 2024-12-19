// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"crypto/x509"
	"io/fs"
	"os"
	"testing"

	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/shoenig/test/must"
)

// Assert CA file exists and is a valid CA Returns the CA
func IsValidCertificate(t *testing.T, caPath string) *x509.Certificate {
	t.Helper()

	must.FileExists(t, caPath)
	caData, err := os.ReadFile(caPath)
	must.NoError(t, err)

	ca, err := tlsutil.ParseCert(string(caData))
	must.NoError(t, err)
	must.NotNil(t, ca)

	return ca
}

// Assert key file exists and is a valid signer returns a bool
func IsValidSigner(t *testing.T, keyPath string) bool {
	t.Helper()

	must.FileExists(t, keyPath)
	fi, err := os.Stat(keyPath)
	must.NoError(t, err)
	if want, have := fs.FileMode(0600), fi.Mode().Perm(); want != have {
		t.Fatalf("private key file %s: permissions: want: %o; have: %o", keyPath, want, have)
	}

	keyData, err := os.ReadFile(keyPath)
	must.NoError(t, err)

	signer, err := tlsutil.ParseSigner(string(keyData))
	must.NoError(t, err)
	must.NotNil(t, signer)
	return true
}
