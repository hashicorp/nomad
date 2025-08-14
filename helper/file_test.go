// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package helper

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func Test_ReadFileContent(t *testing.T) {
	ci.Parallel(t)

	tmpDir := t.TempDir()

	rootDir, err := os.OpenRoot(tmpDir)
	must.NoError(t, err)
	t.Cleanup(func() { must.NoError(t, rootDir.Close()) })

	rootFile, err := rootDir.OpenFile("testfile.txt", os.O_CREATE|os.O_RDWR, 0777)
	must.NoError(t, err)

	_, err = rootFile.WriteString("Hello, World!")
	must.NoError(t, err)
	must.NoError(t, rootFile.Close())

	// Reopen the file using os.OpenInRoot to simulate reading from a root
	// file.
	rootFileRead, err := os.OpenInRoot(tmpDir, "testfile.txt")
	data, err := ReadFileContent(rootFileRead)
	must.NoError(t, err)
	must.Eq(t, "Hello, World!", string(data))
}
