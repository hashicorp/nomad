// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	hvm "github.com/hashicorp/nomad/client/hostvolumemanager"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// this is more of a full integration test of:
// fingerprint <- find plugins <- find executables
func TestPluginsHostVolumeFingerprint(t *testing.T) {
	cfg := &config.Config{HostVolumePluginDir: ""}
	node := &structs.Node{Attributes: map[string]string{}}
	req := &FingerprintRequest{Config: cfg, Node: node}
	fp := NewPluginsHostVolumeFingerprint(testlog.HCLogger(t))

	// this fingerprint is not mandatory, so no error should be returned
	for name, path := range map[string]string{
		"empty":        "",
		"non-existent": "/nowhere",
		"impossible":   "dynamic_host_volumes_test.go",
	} {
		t.Run(name, func(t *testing.T) {
			resp := FingerprintResponse{}
			cfg.HostVolumePluginDir = path
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
	cfg.HostVolumePluginDir = tmp

	files := []struct {
		name     string
		contents string
		perm     os.FileMode
	}{
		// only this first one should be detected as a valid plugin
		{"happy-plugin", "#!/usr/bin/env sh\necho '{\"version\": \"0.0.1\"}'", 0700},
		{"not-a-plugin", "#!/usr/bin/env sh\necho 'not a version'", 0700},
		{"unhappy-plugin", "#!/usr/bin/env sh\necho 'sad plugin is sad'; exit 1", 0700},
		{"not-executable", "do not execute me", 0400},
	}
	for _, f := range files {
		must.NoError(t, os.WriteFile(filepath.Join(tmp, f.name), []byte(f.contents), f.perm))
	}
	// directories should be ignored
	must.NoError(t, os.Mkdir(filepath.Join(tmp, "a-directory"), 0700))

	// do the fingerprint
	resp := FingerprintResponse{}
	err := fp.Fingerprint(req, &resp)
	must.NoError(t, err)
	must.Eq(t, map[string]string{
		"plugins.host_volume.mkdir.version":        hvm.HostVolumePluginMkdirVersion, // built-in
		"plugins.host_volume.happy-plugin.version": "0.0.1",
	}, resp.Attributes)

	// do it again after deleting our one good plugin.
	// repeat runs should wipe attributes, so nothing should remain.
	node.Attributes = resp.Attributes
	must.NoError(t, os.Remove(filepath.Join(tmp, "happy-plugin")))

	resp = FingerprintResponse{}
	err = fp.Fingerprint(req, &resp)
	must.NoError(t, err)
	must.Eq(t, map[string]string{
		"plugins.host_volume.happy-plugin.version": "", // empty value means removed

		"plugins.host_volume.mkdir.version": hvm.HostVolumePluginMkdirVersion, // built-in
	}, resp.Attributes)
}
