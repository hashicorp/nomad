// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hostvolumemanager

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-version"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

func TestHostVolumePluginMkdir(t *testing.T) {
	volID := "test-vol-id"
	tmp := t.TempDir()
	target := filepath.Join(tmp, volID)

	plug := &HostVolumePluginMkdir{
		ID:         "test-mkdir-plugin",
		TargetPath: tmp,
		log:        testlog.HCLogger(t),
	}

	// contexts don't matter here, since they're thrown away by this plugin,
	// but sending timeout contexts anyway, in case the plugin changes later.
	_, err := plug.Version(timeout(t))
	must.NoError(t, err)

	t.Run("happy", func(t *testing.T) {
		resp, err := plug.Create(timeout(t),
			&cstructs.ClientHostVolumeCreateRequest{
				ID: volID, // minimum required by this plugin
			})
		must.NoError(t, err)
		must.Eq(t, &HostVolumePluginCreateResponse{
			Path:      target,
			SizeBytes: 0,
			Context:   map[string]string{},
		}, resp)
		must.DirExists(t, target)

		err = plug.Delete(timeout(t),
			&cstructs.ClientHostVolumeDeleteRequest{
				ID: volID,
			})
		must.NoError(t, err)
		must.DirNotExists(t, target)
	})

	t.Run("sad", func(t *testing.T) {
		// can't mkdir inside a file
		plug.TargetPath = "host_volume_plugin_test.go"

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
		plug := &HostVolumePluginExternal{
			ID:         "test-external-plugin",
			Executable: "./test_fixtures/test_plugin.sh",
			TargetPath: tmp,
			log:        log,
		}

		v, err := plug.Version(timeout(t))
		must.NoError(t, err)
		must.Eq(t, expectVersion, v)

		resp, err := plug.Create(timeout(t),
			&cstructs.ClientHostVolumeCreateRequest{
				ID:                        volID,
				NodeID:                    "test-node",
				RequestedCapacityMinBytes: 5,
				RequestedCapacityMaxBytes: 10,
				Parameters:                map[string]string{"key": "val"},
			})
		must.NoError(t, err)

		must.Eq(t, &HostVolumePluginCreateResponse{
			Path:      target,
			SizeBytes: 5,
			Context:   map[string]string{"key": "val"},
		}, resp)
		must.DirExists(t, target)
		logged := getLogs()
		must.StrContains(t, logged, "OPERATION=create") // stderr from `env`
		must.StrContains(t, logged, `stdout="{`)        // stdout from printf

		// reset logger for next call
		log, getLogs = logRecorder(t)
		plug.log = log

		err = plug.Delete(timeout(t),
			&cstructs.ClientHostVolumeDeleteRequest{
				ID:         volID,
				NodeID:     "test-node",
				Parameters: map[string]string{"key": "val"},
			})
		must.NoError(t, err)
		must.DirNotExists(t, target)
		logged = getLogs()
		must.StrContains(t, logged, "OPERATION=delete")  // stderr from `env`
		must.StrContains(t, logged, "removed directory") // stdout from `rm -v`
	})

	t.Run("sad", func(t *testing.T) {

		log, getLogs := logRecorder(t)
		plug := &HostVolumePluginExternal{
			ID:         "test-external-plugin-sad",
			Executable: "./test_fixtures/test_plugin_sad.sh",
			TargetPath: tmp,
			log:        log,
		}

		v, err := plug.Version(timeout(t))
		must.EqError(t, err, `error getting version from plugin "test-external-plugin-sad": exit status 1`)
		must.Nil(t, v)
		logged := getLogs()
		must.StrContains(t, logged, "version: sad plugin is sad")
		must.StrContains(t, logged, "version: it tells you all about it in stderr")

		// reset logger
		log, getLogs = logRecorder(t)
		plug.log = log

		resp, err := plug.Create(timeout(t),
			&cstructs.ClientHostVolumeCreateRequest{
				ID:                        volID,
				NodeID:                    "test-node",
				RequestedCapacityMinBytes: 5,
				RequestedCapacityMaxBytes: 10,
				Parameters:                map[string]string{"key": "val"},
			})
		must.EqError(t, err, `error creating volume "test-vol-id" with plugin "test-external-plugin-sad": exit status 1`)
		must.Nil(t, resp)
		logged = getLogs()
		must.StrContains(t, logged, "create: sad plugin is sad")
		must.StrContains(t, logged, "create: it tells you all about it in stderr")

		log, getLogs = logRecorder(t)
		plug.log = log

		err = plug.Delete(timeout(t),
			&cstructs.ClientHostVolumeDeleteRequest{
				ID:         volID,
				NodeID:     "test-node",
				Parameters: map[string]string{"key": "val"},
			})
		must.EqError(t, err, `error deleting volume "test-vol-id" with plugin "test-external-plugin-sad": exit status 1`)
		logged = getLogs()
		must.StrContains(t, logged, "delete: sad plugin is sad")
		must.StrContains(t, logged, "delete: it tells you all about it in stderr")
	})
}

// timeout provides a context that times out in 1 second
func timeout(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	return ctx
}

// logRecorder is here so we can assert that stdout/stderr appear in logs
func logRecorder(t *testing.T) (hclog.Logger, func() string) {
	t.Helper()
	buf := &bytes.Buffer{}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:            "log-recorder",
		Output:          buf,
		Level:           hclog.Debug,
		IncludeLocation: true,
		DisableTime:     true,
	})
	return logger, func() string {
		bts, err := io.ReadAll(buf)
		test.NoError(t, err)
		return string(bts)
	}
}
