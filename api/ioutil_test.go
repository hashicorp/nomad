package api

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"math/rand"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
)

func TestChecksumValidatingReader(t *testing.T) {
	data := make([]byte, 4096)
	_, err := rand.Read(data)
	require.NoError(t, err)

	cases := []struct {
		algo string
		hash hash.Hash
	}{
		{"sha-256", sha256.New()},
		{"sha-512", sha512.New()},
	}

	for _, c := range cases {
		t.Run("valid: "+c.algo, func(t *testing.T) {
			_, err := c.hash.Write(data)
			require.NoError(t, err)

			checksum := c.hash.Sum(nil)
			digest := c.algo + "=" + base64.StdEncoding.EncodeToString(checksum)

			r := iotest.HalfReader(bytes.NewReader(data))
			cr, err := newChecksumValidatingReader(ioutil.NopCloser(r), digest)
			require.NoError(t, err)

			_, err = io.Copy(ioutil.Discard, cr)
			require.NoError(t, err)
		})

		t.Run("invalid: "+c.algo, func(t *testing.T) {
			_, err := c.hash.Write(data)
			require.NoError(t, err)

			checksum := c.hash.Sum(nil)
			// mess up checksum
			checksum[0]++
			digest := c.algo + "=" + base64.StdEncoding.EncodeToString(checksum)

			r := iotest.HalfReader(bytes.NewReader(data))
			cr, err := newChecksumValidatingReader(ioutil.NopCloser(r), digest)
			require.NoError(t, err)

			_, err = io.Copy(ioutil.Discard, cr)
			require.Error(t, err)
			require.Equal(t, errMismatchChecksum, err)
		})
	}
}

func TestChecksumValidatingReader_PropagatesError(t *testing.T) {

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	expectedErr := fmt.Errorf("some error")

	go func() {
		pw.Write([]byte("some input"))
		pw.CloseWithError(expectedErr)
	}()

	cr, err := newChecksumValidatingReader(pr, "sha-256=aaaa")
	require.NoError(t, err)

	_, err = io.Copy(ioutil.Discard, cr)
	require.Error(t, err)
	require.Equal(t, expectedErr, err)
}
