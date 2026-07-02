// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/shoenig/test/must"
)

func TestFileRotator_OffsetManifestRecordsFileBases(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	must.NoError(t, os.MkdirAll(dir, 0755))
	rotator, err := NewFileRotator(dir, "stdout", 3, 3, hclog.NewNullLogger())
	must.NoError(t, err)

	n, err := rotator.Write([]byte("abcdef"))
	must.NoError(t, err)
	must.Eq(t, 6, n)
	must.NoError(t, rotator.Close())

	raw, err := os.ReadFile(filepath.Join(filepath.Dir(dir), OffsetManifestFile("stdout")))
	must.NoError(t, err)

	var manifest OffsetManifest
	must.NoError(t, json.Unmarshal(raw, &manifest))
	must.Eq(t, int64(0), manifest.Files["0"].Base)
	must.Eq(t, int64(3), manifest.Files["1"].Base)
}
