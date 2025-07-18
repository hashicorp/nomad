// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestPluginsSecretsFingerprint(t *testing.T) {
	fp := NewPluginsSecretsFingerprint(testlog.HCLogger(t))

	node := &structs.Node{Attributes: map[string]string{}}
	cfg := &config.Config{CommonPluginsDir: ""}
	req := &FingerprintRequest{Config: cfg, Node: node}

	for name, path := range map[string]string{
		"empty":        "",
		"non-existent": "/nowhere",
		"impossible":   "secrets_plugins_test.go",
	} {
		t.Run(name, func(t *testing.T) {
			resp := FingerprintResponse{}
			cfg.CommonPluginsDir = path
			err := fp.Fingerprint(req, &resp)
			must.NoError(t, err)
			must.True(t, resp.Detected) // always true due to "mkdir" built-in
		})
	}

	if runtime.GOOS == "windows" {
		t.Skip("test scripts not built for windows") // db TODO(1.10.0)
	}

	// happy path: dir exists. this one will contain a single valid plugin.
	tmp := t.TempDir()
	secretsDir := filepath.Join(tmp, "secrets")
	os.Mkdir(secretsDir, 0777)
	cfg.CommonPluginsDir = tmp

	files := []struct {
		name     string
		contents string
		perm     os.FileMode
	}{
		// only this first one should be detected as a valid plugin
		{"happy-plugin", "#!/usr/bin/env sh\necho '{\"type\": \"secrets\", \"version\": \"0.0.1\"}'", 0700},
		{"not-a-plugin", "#!/usr/bin/env sh\necho 'not a version'", 0700},
		{"unhappy-plugin", "#!/usr/bin/env sh\necho 'sad plugin is sad'; exit 1", 0700},
		{"not-executable", "do not execute me", 0400},
	}
	for _, f := range files {
		must.NoError(t, os.WriteFile(filepath.Join(secretsDir, f.name), []byte(f.contents), f.perm))
	}
	// directories should be ignored
	must.NoError(t, os.Mkdir(filepath.Join(secretsDir, "a-directory"), 0700))

	// do the fingerprint
	resp := &FingerprintResponse{}
	err := fp.Fingerprint(req, resp)
	must.NoError(t, err)
	must.Eq(t, map[string]string{
		"plugins.secrets.happy-plugin.version": "0.0.1",
		"plugins.secrets.nomad.version":        "1.0.0",
		"plugins.secrets.vault.version":        "1.0.0",
	}, resp.Attributes)

	// do it again after deleting our one good plugin.
	// repeat runs should wipe attributes, so nothing should remain.
	node.Attributes = resp.Attributes
	must.NoError(t, os.Remove(filepath.Join(secretsDir, "happy-plugin")))

	resp = &FingerprintResponse{}
	err = fp.Fingerprint(req, resp)
	must.NoError(t, err)
	must.Eq(t, map[string]string{
		"plugins.secrets.happy-plugin.version": "", // empty value means removed
		"plugins.secrets.nomad.version":        "1.0.0",
		"plugins.secrets.vault.version":        "1.0.0",
	}, resp.Attributes)
}
