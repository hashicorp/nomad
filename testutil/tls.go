package testutil

import (
	"crypto"
	"crypto/x509"
	"io/fs"
	"os"
	"testing"

	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/stretchr/testify/require"
)

func ExpectFiles(t *testing.T, caPath, keyPath string) (*x509.Certificate, crypto.Signer) {
	t.Helper()

	require.FileExists(t, caPath)
	require.FileExists(t, keyPath)

	fi, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal("should not happen", err)
	}
	if want, have := fs.FileMode(0600), fi.Mode().Perm(); want != have {
		t.Fatalf("private key file %s: permissions: want: %o; have: %o", keyPath, want, have)
	}

	caData, err := os.ReadFile(caPath)
	require.NoError(t, err)
	keyData, err := os.ReadFile(keyPath)
	require.NoError(t, err)

	ca, err := tlsutil.ParseCert(string(caData))
	require.NoError(t, err)
	require.NotNil(t, ca)

	signer, err := tlsutil.ParseSigner(string(keyData))
	require.NoError(t, err)
	require.NotNil(t, signer)

	return ca, signer
}

// switchToTempDir is meant to be used in a defer statement like:
//
//	defer switchToTempDir(t, testDir)()
//
// This exploits the fact that the body of a defer is evaluated
// EXCEPT for the final function call invocation inline with the code
// where it is found. Only the final evaluation happens in the defer
// at a later time. In this case it means we switch to the temp
// directory immediately and defer switching back in one line of test
// code.
func SwitchToTempDir(t *testing.T, testDir string) func() {
	previousDirectory, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(testDir))
	return func() {
		os.Chdir(previousDirectory)
	}
}
