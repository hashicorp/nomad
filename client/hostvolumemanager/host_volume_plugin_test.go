// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/ci"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

func TestHostVolumePluginMkdir(t *testing.T) {
	ci.Parallel(t)
	tmp := t.TempDir()

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
		volID := "happy"
		target := filepath.Join(tmp, volID)
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
			must.DirMode(t, target, 0o700+os.ModeDir)
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
		volID := "sad"
		// can't mkdir inside a file
		plug.VolumesDir = "host_volume_plugin_test.go"
		t.Cleanup(func() {
			plug.VolumesDir = tmp
		})

		resp, err := plug.Create(timeout(t),
			&cstructs.ClientHostVolumeCreateRequest{
				ID: volID, // minimum required by this plugin
			})
		must.ErrorContains(t, err, "host_volume_plugin_test.go/sad: not a directory")
		must.Nil(t, resp)

		err = plug.Delete(timeout(t),
			&cstructs.ClientHostVolumeDeleteRequest{
				ID: volID,
			})
		must.ErrorContains(t, err, "host_volume_plugin_test.go/sad: not a directory")
	})

	t.Run("happy params", func(t *testing.T) {
		volID := "happy_params"
		target := filepath.Join(tmp, volID)
		currentUser, err := user.Current()
		must.NoError(t, err)
		// run multiple times, should be idempotent
		for range 2 {
			_, err := plug.Create(timeout(t),
				&cstructs.ClientHostVolumeCreateRequest{
					ID: volID,
					Parameters: map[string]string{
						"uid":  currentUser.Uid,
						"gid":  currentUser.Gid,
						"mode": "0400",
					},
				})
			must.NoError(t, err)
			must.DirExists(t, target)
			must.DirMode(t, target, 0o400+os.ModeDir)
		}

		err = plug.Delete(timeout(t),
			&cstructs.ClientHostVolumeDeleteRequest{
				ID: volID,
			})
		must.NoError(t, err)
		must.DirNotExists(t, target)
	})

	t.Run("sad params", func(t *testing.T) {
		volID := "sad_params"
		// test one representative error; decodeMkdirParams has its own tests
		resp, err := plug.Create(timeout(t),
			&cstructs.ClientHostVolumeCreateRequest{
				ID: volID,
				Parameters: map[string]string{
					"mode": "invalid",
				},
			})
		must.ErrorContains(t, err, "invalid value")
		must.Nil(t, resp)
	})
}

func TestDecodeMkdirParams(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name   string
		params map[string]string
		expect HostVolumePluginMkdirParams
		err    string
	}{
		{
			name:   "none ok",
			params: nil,
			expect: HostVolumePluginMkdirParams{
				Uid:  -1,
				Gid:  -1,
				Mode: os.FileMode(0700),
			},
		},
		{
			name: "all ok",
			params: map[string]string{
				"uid":  "1",
				"gid":  "2",
				"mode": "0444",
			},
			expect: HostVolumePluginMkdirParams{
				Uid:  1,
				Gid:  2,
				Mode: os.FileMode(0444),
			},
		},
		{
			name: "invalid mode: bad number",
			params: map[string]string{
				// this is what happens if you put mode=0700 instead of
				// mode="0700" in the HCL spec.
				"mode": "493",
			},
			err: `invalid value for "mode"`,
		},
		{
			name: "invalid mode: string",
			params: map[string]string{
				"mode": "consider the lobster",
			},
			err: `invalid value for "mode"`,
		},
		{
			name: "invalid uid",
			params: map[string]string{
				"uid": "a supposedly fun thing i'll never do again",
			},
			err: `invalid value for "uid"`,
		},
		{
			name: "invalid gid",
			params: map[string]string{
				"gid": "surely you jest",
			},
			err: `invalid value for "gid"`,
		},
		{
			name: "unknown param",
			params: map[string]string{
				"what": "the hell is water?",
			},
			err: `unknown mkdir parameter: "what"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeMkdirParams(tc.params)
			if tc.err != "" {
				must.ErrorContains(t, err, tc.err)
			} else {
				must.NoError(t, err)
				must.Eq(t, tc.expect, got)
			}
		})
	}
}

func TestNewHostVolumePluginExternal(t *testing.T) {
	ci.Parallel(t)
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
	ci.Parallel(t)
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
		must.EqError(t, err, `error fingerprinting plugin "test_plugin_sad.sh": exit status 1`)
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
		must.EqError(t, err, `error creating volume "test-vol-id" with plugin "test_plugin_sad.sh": exit status 1: create: sad plugin is sad`)
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
		must.EqError(t, err, `error deleting volume "test-vol-id" with plugin "test_plugin_sad.sh": exit status 1: delete: sad plugin is sad`)
		logged = getLogs()
		must.StrContains(t, logged, "delete: sad plugin is sad")
		must.StrContains(t, logged, "delete: it tells you all about it in stderr")
	})
}
