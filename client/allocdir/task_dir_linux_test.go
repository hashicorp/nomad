// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package allocdir

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"golang.org/x/sys/unix"
)

// TestMountSecretsDirs_RemountsAfterUnmount verifies that MountSecretsDirs
// re-establishes the tmpfs mounts when they have been lost (e.g. after a host
// reboot).
func TestMountSecretsDirs_RemountsAfterUnmount(t *testing.T) {
	ci.Parallel(t)

	if unix.Geteuid() != 0 {
		t.Skip("must be run as root")
	}

	tmp := t.TempDir()
	logger := testlog.HCLogger(t)
	d := NewAllocDir(logger, tmp, tmp, "test-alloc")
	defer d.Destroy()

	task := &structs.Task{
		Name:      "web",
		Resources: &structs.Resources{DiskMB: 2},
	}
	must.NoError(t, d.Build())
	td := d.NewTaskDir(task)

	// This is the first call, so directories do not yet exist and both tmpfs
	// mounts are created.
	must.NoError(t, td.MakeSecretsDirs())

	_, err := isMount(td.SecretsDir)
	must.NoError(t, err)

	_, err = isMount(td.PrivateDir)
	must.NoError(t, err)

	// Write something to the tmpds mounts to test that subsequent calls to
	// MakeSecretsDirs do not destroy the data.
	must.NoError(t, os.WriteFile(filepath.Join(td.SecretsDir, "testfile"), []byte("testdata"), 0644))
	must.NoError(t, os.WriteFile(filepath.Join(td.PrivateDir, "testfile"), []byte("testdata"), 0644))

	// Perform a second call to ensure that this is a no-op and does not error
	// when the tmpfs mounts already exist.
	must.NoError(t, td.MakeSecretsDirs())

	// Verify that the tmpfs mounts still exist and that the data is still
	// there.
	_, err = isMount(td.SecretsDir)
	must.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(td.SecretsDir, "testfile"))
	must.NoError(t, err)
	must.Eq(t, "testdata", string(content))

	_, err = isMount(td.PrivateDir)
	must.NoError(t, err)

	content, err = os.ReadFile(filepath.Join(td.PrivateDir, "testfile"))
	must.NoError(t, err)
	must.Eq(t, "testdata", string(content))

	// Simulate a host reboot by unmounting only. The directory itself remains
	// on disk, as it would after a reboot that drops the tmpfs mounts.
	must.NoError(t, syscall.Unmount(td.SecretsDir, 0))
	must.NoError(t, syscall.Unmount(td.PrivateDir, 0))

	// Verify that the tmpfs mounts are gone.
	_, err = isMount(td.SecretsDir)
	must.ErrorIs(t, err, notFoundErr)

	_, err = isMount(td.PrivateDir)
	must.ErrorIs(t, err, notFoundErr)

	// This call simulates what happens via taskDirHook.Prestart on the restore
	// path. The directories still exist on disk but their tmpfs mounts are
	// gone. MountSecretsDirs must re-establish the mounts.
	must.NoError(t, td.MakeSecretsDirs())

	_, err = isMount(td.SecretsDir)
	must.NoError(t, err)

	_, err = isMount(td.PrivateDir)
	must.NoError(t, err)
}
