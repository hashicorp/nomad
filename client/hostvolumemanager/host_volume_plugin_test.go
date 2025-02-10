// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/go-version"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

func TestHostVolumePluginMkdir(t *testing.T) {
	volID := "test-vol-id"
	tmp := t.TempDir()
	target := filepath.Join(tmp, volID)

	plug := &HostVolumePluginMkdir{
		ID:         "test-mkdir-plugin",
		VolumesDir: tmp,
		log:        testlog.HCLogger(t),
	}

	// contexts don't matter here, since they're thrown away by this plugin,
	// but sending timeout contexts anyway, in case the plugin changes later.
	_, err := plug.Fingerprint(timeout(t))
	must.NoError(t, err)

	t.Run("happy", func(t *testing.T) {
		// run multiple times, should be idempotent
		for range 2 {
			resp, err := plug.Create(timeout(t),
				&cstructs.ClientHostVolumeCreateRequest{
					ID: volID, // minimum required by this plugin
				})
			must.NoError(t, err)
			must.Eq(t, &HostVolumePluginCreateResponse{
				Path:      target,
				SizeBytes: 0,
			}, resp)
			must.DirExists(t, target)
		}

		// delete should be idempotent, too
		for range 2 {
			err = plug.Delete(timeout(t),
				&cstructs.ClientHostVolumeDeleteRequest{
					ID: volID,
				})
			must.NoError(t, err)
			must.DirNotExists(t, target)
		}
	})

	t.Run("sad", func(t *testing.T) {
		// can't mkdir inside a file
		plug.VolumesDir = "host_volume_plugin_test.go"

		resp, err := plug.Create(timeout(t),
			&cstructs.ClientHostVolumeCreateRequest{
				ID: volID, // minimum required by this plugin
			})
		must.ErrorContains(t, err, "host_volume_plugin_test.go/test-vol-id: not a directory")
		must.Nil(t, resp)

		err = plug.Delete(timeout(t),
			&cstructs.ClientHostVolumeDeleteRequest{
				ID: volID,
			})
		must.ErrorContains(t, err, "host_volume_plugin_test.go/test-vol-id: not a directory")
	})
}

func TestNewHostVolumePluginExternal(t *testing.T) {
	log := testlog.HCLogger(t)
	var err error

	_, err = NewHostVolumePluginExternal(log, ".", "non-existent", "target", "")
	must.ErrorIs(t, err, ErrPluginNotExists)

	_, err = NewHostVolumePluginExternal(log, ".", "host_volume_plugin_test.go", "target", "")
	must.ErrorIs(t, err, ErrPluginNotExecutable)

	t.Run("unix", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("skipped because windows") // db TODO(1.10.0)
		}
		p, err := NewHostVolumePluginExternal(log,
			"./test_fixtures", "test_plugin.sh", "test-target", "test-pool")
		must.NoError(t, err)
		must.Eq(t, &HostVolumePluginExternal{
			ID:         "test_plugin.sh",
			Executable: "test_fixtures/test_plugin.sh",
			VolumesDir: "test-target",
			PluginDir:  "./test_fixtures",
			NodePool:   "test-pool",
			log:        log,
		}, p)
	})
}

func TestHostVolumePluginExternal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipped because windows") // db TODO(1.10.0)
	}

	volID := "test-vol-id"
	tmp := t.TempDir()
	target := filepath.Join(tmp, volID)

	expectVersion, err := version.NewVersion("0.0.2")
	must.NoError(t, err)

	t.Run("happy", func(t *testing.T) {

		log, getLogs := logRecorder(t)
		plug, err := NewHostVolumePluginExternal(log,
			"./test_fixtures", "test_plugin.sh", tmp, "test-node-pool")
		must.NoError(t, err)

		// fingerprint
		v, err := plug.Fingerprint(timeout(t))
		logged := getLogs()
		must.NoError(t, err, must.Sprintf("logs: %s", logged))
		must.Eq(t, expectVersion, v.Version, must.Sprintf("logs: %s", logged))

		// create
		resp, err := plug.Create(timeout(t),
			&cstructs.ClientHostVolumeCreateRequest{
				Name:                      "test-vol-name",
				ID:                        volID,
				Namespace:                 "test-namespace",
				NodeID:                    "test-node",
				RequestedCapacityMinBytes: 5,
				RequestedCapacityMaxBytes: 10,
				Parameters:                map[string]string{"key": "val"},
			})
		logged = getLogs()
		must.NoError(t, err, must.Sprintf("logs: %s", logged))

		must.Eq(t, &HostVolumePluginCreateResponse{
			Path:      target,
			SizeBytes: 5,
		}, resp)
		must.DirExists(t, target)
		must.StrContains(t, logged, "OPERATION=create") // stderr from `env`
		must.StrContains(t, logged, `stdout="{`)        // stdout from printf

		// delete
		err = plug.Delete(timeout(t),
			&cstructs.ClientHostVolumeDeleteRequest{
				Name:       "test-vol-name",
				ID:         volID,
				HostPath:   resp.Path,
				Namespace:  "test-namespace",
				NodeID:     "test-node",
				Parameters: map[string]string{"key": "val"},
			})
		logged = getLogs()
		must.NoError(t, err, must.Sprintf("logs: %s", logged))
		must.DirNotExists(t, target)
		must.StrContains(t, logged, "OPERATION=delete")  // stderr from `env`
		must.StrContains(t, logged, "removed directory") // stdout from `rm -v`
	})

	t.Run("sad", func(t *testing.T) {

		log, getLogs := logRecorder(t)
		plug, err := NewHostVolumePluginExternal(log, "./test_fixtures", "test_plugin_sad.sh", tmp, "")
		must.NoError(t, err)

		v, err := plug.Fingerprint(timeout(t))
		must.EqError(t, err, `error getting version from plugin "test_plugin_sad.sh": exit status 1`)
		must.Nil(t, v)
		logged := getLogs()
		must.StrContains(t, logged, "fingerprint: sad plugin is sad")
		must.StrContains(t, logged, "fingerprint: it tells you all about it in stderr")

		// reset logger
		log, getLogs = logRecorder(t)
		plug.log = log

		resp, err := plug.Create(timeout(t),
			&cstructs.ClientHostVolumeCreateRequest{
				ID: volID,
			})
		must.EqError(t, err, `error creating volume "test-vol-id" with plugin "test_plugin_sad.sh": exit status 1`)
		must.Nil(t, resp)
		logged = getLogs()
		must.StrContains(t, logged, "create: sad plugin is sad")
		must.StrContains(t, logged, "create: it tells you all about it in stderr")

		log, getLogs = logRecorder(t)
		plug.log = log

		err = plug.Delete(timeout(t),
			&cstructs.ClientHostVolumeDeleteRequest{
				ID: volID,
			})
		must.EqError(t, err, `error deleting volume "test-vol-id" with plugin "test_plugin_sad.sh": exit status 1`)
		logged = getLogs()
		must.StrContains(t, logged, "delete: sad plugin is sad")
		must.StrContains(t, logged, "delete: it tells you all about it in stderr")
	})
}
