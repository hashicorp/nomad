//Copyright IBM Corp. 2015, 2026
//SPDX-License-Identifier: BUSL-1.1

package example

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

// newDynamicTestDevice returns an FsDevice in dynamic mode that exposes the
// given device IDs and writes its device files to a temporary directory.
func newDynamicTestDevice(t *testing.T, ids ...string) *FsDevice {
	t.Helper()

	d := NewExampleDevice(testlog.HCLogger(t))
	d.pluginMode = dynamicMode
	d.deviceDir = t.TempDir()
	d.dynamicFileInfo.DeviceFiles = make(map[string]time.Time)
	for _, id := range ids {
		d.devices[id] = true
	}
	return d
}

// TestCollectDynamicStats_MultipleExpiredFiles asserts that a single stats
// pass can remove more than one expired dynamic device file, as happens after
// a reservation of two or more devices reaches its TTL.
func TestCollectDynamicStats_MultipleExpiredFiles(t *testing.T) {
	ci.Parallel(t)

	d := newDynamicTestDevice(t, "dev0", "dev1")

	_, err := d.Reserve([]string{"dev0", "dev1"})
	must.NoError(t, err)

	// Backdate both files so they expire in the same pass.
	expired := time.Now().Add(-2 * dynamicFileTTL * time.Second)
	for name := range d.dynamicFileInfo.DeviceFiles {
		d.dynamicFileInfo.DeviceFiles[name] = expired
	}

	done := make(chan error, 1)
	go func() {
		_, err := d.collectDynamicStats()
		done <- err
	}()

	select {
	case err := <-done:
		must.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("collectDynamicStats did not return with two expired files")
	}

	must.MapEmpty(t, d.dynamicFileInfo.DeviceFiles)
	for _, name := range []string{"dev0", "dev1"} {
		_, err := os.Stat(filepath.Join(d.deviceDir, name))
		must.ErrorIs(t, err, fs.ErrNotExist)
	}
}

// TestCollectDynamicStats_ConcurrentReserve asserts that stats collection and
// device reservation can run at the same time. Run with -race.
func TestCollectDynamicStats_ConcurrentReserve(t *testing.T) {
	ci.Parallel(t)

	d := newDynamicTestDevice(t, "dev0", "dev1")

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 50 {
			_, err := d.Reserve([]string{"dev0", "dev1"})
			test.NoError(t, err)
		}
	})
	for range 50 {
		_, err := d.collectDynamicStats()
		must.NoError(t, err)
	}
	wg.Wait()
}

func TestSanitizeDir(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		expectedErr string
	}{
		{
			name:        "relative segment",
			path:        "../home/fine",
			expectedErr: ".. is not allowed in dir when running in dynamic mode",
		},
		{
			name:        "disallowed segment",
			path:        "C:\\WINDOWS\\Teams",
			expectedErr: "WINDOWS is not allowed in dir when running in dynamic mode",
		},
		{
			name:        "disallowed segment",
			path:        "./home",
			expectedErr: ". is not allowed in dir when running in dynamic mode",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := sanitizeDir(tc.path)
			if tc.expectedErr != "" {
				must.NotNil(t, err)
				must.Eq(t, tc.expectedErr, err.Error())
			}
		})
	}
}
