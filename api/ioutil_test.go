// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"hash"
	"io"
	"math/rand"
	"testing"
	"testing/iotest"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestChecksumValidatingReader(t *testing.T) {
	testutil.Parallel(t)

	data := make([]byte, 4096)
	_, err := rand.Read(data)
	must.NoError(t, err)

	cases := []struct {
		algo string
		hash hash.Hash
	}{
		{"sha-256", sha256.New()},
		{"sha-512", sha512.New()},
	}

	for _, c := range cases {
		t.Run("valid: "+c.algo, func(t *testing.T) {
			_, err = c.hash.Write(data)
			must.NoError(t, err)

			checksum := c.hash.Sum(nil)
			digest := c.algo + "=" + base64.StdEncoding.EncodeToString(checksum)

			r := iotest.HalfReader(bytes.NewReader(data))
			cr, err := newChecksumValidatingReader(io.NopCloser(r), digest)
			must.NoError(t, err)

			_, err = io.Copy(io.Discard, cr)
			must.NoError(t, err)
		})

		t.Run("invalid: "+c.algo, func(t *testing.T) {
			_, err = c.hash.Write(data)
			must.NoError(t, err)

			checksum := c.hash.Sum(nil)
			// mess up checksum
			checksum[0]++
			digest := c.algo + "=" + base64.StdEncoding.EncodeToString(checksum)

			r := iotest.HalfReader(bytes.NewReader(data))
			cr, err := newChecksumValidatingReader(io.NopCloser(r), digest)
			must.NoError(t, err)

			_, err = io.Copy(io.Discard, cr)
			must.ErrorIs(t, err, errMismatchChecksum)
		})
	}
}

func TestChecksumValidatingReader_PropagatesError(t *testing.T) {
	testutil.Parallel(t)

	pr, pw := io.Pipe()
	defer func() { _ = pr.Close() }()
	defer func() { _ = pw.Close() }()

	expectedErr := errors.New("some error")

	go func() {
		_, _ = pw.Write([]byte("some input"))
		_ = pw.CloseWithError(expectedErr)
	}()

	cr, err := newChecksumValidatingReader(pr, "sha-256=aaaa")
	must.NoError(t, err)

	_, err = io.Copy(io.Discard, cr)
	must.ErrorIs(t, err, expectedErr)
}
