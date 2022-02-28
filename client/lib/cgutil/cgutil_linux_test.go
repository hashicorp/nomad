//go:build linux

package cgutil

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs2"
	"github.com/stretchr/testify/require"
)

func TestUtil_GetCgroupParent(t *testing.T) {
	ci.Parallel(t)

	t.Run("v1", func(t *testing.T) {
		testutil.CgroupsCompatibleV1(t)
		t.Run("default", func(t *testing.T) {
			exp := "/nomad"
			parent := GetCgroupParent("")
			require.Equal(t, exp, parent)
		})

		t.Run("configured", func(t *testing.T) {
			exp := "/bar"
			parent := GetCgroupParent("/bar")
			require.Equal(t, exp, parent)
		})
	})

	t.Run("v2", func(t *testing.T) {
		testutil.CgroupsCompatibleV2(t)
		t.Run("default", func(t *testing.T) {
			exp := "nomad.slice"
			parent := GetCgroupParent("")
			require.Equal(t, exp, parent)
		})

		t.Run("configured", func(t *testing.T) {
			exp := "abc.slice"
			parent := GetCgroupParent("abc.slice")
			require.Equal(t, exp, parent)
		})
	})
}

func TestUtil_CreateCPUSetManager(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)

	t.Run("v1", func(t *testing.T) {
		testutil.CgroupsCompatibleV1(t)
		parent := "/" + uuid.Short()
		manager := CreateCPUSetManager(parent, logger)
		err := manager.Init([]uint16{0})
		require.NoError(t, err)
		require.NoError(t, cgroups.RemovePath(filepath.Join(CgroupRoot, parent)))
	})

	t.Run("v2", func(t *testing.T) {
		testutil.CgroupsCompatibleV2(t)
		parent := uuid.Short() + ".slice"
		manager := CreateCPUSetManager(parent, logger)
		err := manager.Init([]uint16{0})
		require.NoError(t, err)
		require.NoError(t, cgroups.RemovePath(filepath.Join(CgroupRoot, parent)))
	})
}

func TestUtil_GetCPUsFromCgroup(t *testing.T) {
	ci.Parallel(t)

	t.Run("v2", func(t *testing.T) {
		testutil.CgroupsCompatibleV2(t)
		cpus, err := GetCPUsFromCgroup("system.slice") // thanks, systemd!
		require.NoError(t, err)
		require.NotEmpty(t, cpus)
	})
}

func create(t *testing.T, name string) {
	mgr, err := fs2.NewManager(nil, filepath.Join(CgroupRoot, name), rootless)
	require.NoError(t, err)
	err = mgr.Apply(CreationPID)
	require.NoError(t, err)
}

func cleanup(t *testing.T, name string) {
	err := cgroups.RemovePath(name)
	require.NoError(t, err)
}

func TestUtil_CopyCpuset(t *testing.T) {
	ci.Parallel(t)

	t.Run("v2", func(t *testing.T) {
		testutil.CgroupsCompatibleV2(t)
		source := uuid.Short() + ".scope"
		create(t, source)
		defer cleanup(t, source)
		require.NoError(t, cgroups.WriteFile(filepath.Join(CgroupRoot, source), "cpuset.cpus", "0-1"))

		destination := uuid.Short() + ".scope"
		create(t, destination)
		defer cleanup(t, destination)

		err := CopyCpuset(
			filepath.Join(CgroupRoot, source),
			filepath.Join(CgroupRoot, destination),
		)
		require.NoError(t, err)

		value, readErr := cgroups.ReadFile(filepath.Join(CgroupRoot, destination), "cpuset.cpus")
		require.NoError(t, readErr)
		require.Equal(t, "0-1", strings.TrimSpace(value))
	})
}
