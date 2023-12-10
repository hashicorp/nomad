// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"os"
	"path/filepath"

	testing "github.com/mitchellh/go-testing-interface"
	"github.com/shoenig/test/must"
)

// MustReadFile returns the contents of the specified file or fails the test.
// Multiple arguments are joined with filepath.Join.
func MustReadFile(t testing.T, path ...string) []byte {
	contents, err := os.ReadFile(filepath.Join(path...))
	must.NoError(t, err)
	return contents
}
