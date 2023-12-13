// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/lib/cpuset"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/stretchr/testify/require"
)

func tmpCpusetManagerV1(t *testing.T) (*cpusetManagerV1, func()) {
	mount, err := FindCgroupMountpointDir()
	if err != nil || mount == "" {
		t.Skipf("Failed to find cgroup mount: %v %v", mount, err)
	}

	parent := "/gotest-" + uuid.Short()
	require.NoError(t, cpusetEnsureParentV1(parent))

	parentPath, err := GetCgroupPathHelperV1("cpuset", parent)
	require.NoError(t, err)

	manager := NewCpusetManagerV1(parent, nil, testlog.HCLogger(t)).(*cpusetManagerV1)
	return manager, func() { require.NoError(t, cgroups.RemovePaths(map[string]string{"cpuset": parentPath})) }
}

func TestCpusetManager_V1_Init(t *testing.T) {
	testutil.CgroupsCompatibleV1(t)

	manager, cleanup := tmpCpusetManagerV1(t)
	defer cleanup()
	manager.Init()

	require.DirExists(t, filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName))
	require.FileExists(t, filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName, "cpuset.cpus"))
	sharedCpusRaw, err := os.ReadFile(filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	sharedCpus, err := cpuset.Parse(string(sharedCpusRaw))
	require.NoError(t, err)
	require.Exactly(t, manager.parentCpuset.ToSlice(), sharedCpus.ToSlice())
	require.DirExists(t, filepath.Join(manager.cgroupParentPath, ReservedCpusetCgroupName))
}

func TestCpusetManager_V1_AddAlloc_single(t *testing.T) {
	testutil.CgroupsCompatibleV1(t)

	manager, cleanup := tmpCpusetManagerV1(t)
	defer cleanup()
	manager.Init()

	alloc := mock.Alloc()
	// reserve just one core (the 0th core, which probably exists)
	alloc.AllocatedResources.Tasks["web"].Cpu.ReservedCores = cpuset.New(0).ToSlice()
	manager.AddAlloc(alloc)

	// force reconcile
	manager.reconcileCpusets()

	// check that the 0th core is no longer available in the shared group
	// actual contents of shared group depends on machine core count
	require.DirExists(t, filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName))
	require.FileExists(t, filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName, "cpuset.cpus"))
	sharedCpusRaw, err := os.ReadFile(filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	sharedCpus, err := cpuset.Parse(string(sharedCpusRaw))
	require.NoError(t, err)
	require.NotEmpty(t, sharedCpus.ToSlice())
	require.NotContains(t, sharedCpus.ToSlice(), uint16(0))

	// check that the 0th core is allocated to reserved cgroup
	require.DirExists(t, filepath.Join(manager.cgroupParentPath, ReservedCpusetCgroupName))
	reservedCpusRaw, err := os.ReadFile(filepath.Join(manager.cgroupParentPath, ReservedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	reservedCpus, err := cpuset.Parse(string(reservedCpusRaw))
	require.NoError(t, err)
	require.Exactly(t, alloc.AllocatedResources.Tasks["web"].Cpu.ReservedCores, reservedCpus.ToSlice())

	// check that task cgroup exists and cpuset matches expected reserved cores
	allocInfo, ok := manager.cgroupInfo[alloc.ID]
	require.True(t, ok)
	require.Len(t, allocInfo, 1)
	taskInfo, ok := allocInfo["web"]
	require.True(t, ok)

	require.DirExists(t, taskInfo.CgroupPath)
	taskCpusRaw, err := os.ReadFile(filepath.Join(taskInfo.CgroupPath, "cpuset.cpus"))
	require.NoError(t, err)
	taskCpus, err := cpuset.Parse(string(taskCpusRaw))
	require.NoError(t, err)
	require.Exactly(t, alloc.AllocatedResources.Tasks["web"].Cpu.ReservedCores, taskCpus.ToSlice())
}

func TestCpusetManager_V1_RemoveAlloc(t *testing.T) {
	testutil.CgroupsCompatibleV1(t)

	// This case tests adding 2 allocations, reconciling then removing 1 alloc.
	// It requires the system to have at least 3 cpu cores (one for each alloc),
	// BUT plus another one because writing an empty cpuset causes the cgroup to
	// inherit the parent.
	testutil.MinimumCores(t, 3)

	manager, cleanup := tmpCpusetManagerV1(t)
	defer cleanup()
	manager.Init()

	alloc1 := mock.Alloc()
	alloc1Cpuset := cpuset.New(manager.parentCpuset.ToSlice()[0])
	alloc1.AllocatedResources.Tasks["web"].Cpu.ReservedCores = alloc1Cpuset.ToSlice()
	manager.AddAlloc(alloc1)

	alloc2 := mock.Alloc()
	alloc2Cpuset := cpuset.New(manager.parentCpuset.ToSlice()[1])
	alloc2.AllocatedResources.Tasks["web"].Cpu.ReservedCores = alloc2Cpuset.ToSlice()
	manager.AddAlloc(alloc2)

	//force reconcile
	manager.reconcileCpusets()

	// shared cpuset should not include any expected cores
	sharedCpusRaw, err := os.ReadFile(filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	sharedCpus, err := cpuset.Parse(string(sharedCpusRaw))
	require.NoError(t, err)
	require.False(t, sharedCpus.ContainsAny(alloc1Cpuset.Union(alloc2Cpuset)))

	// reserved cpuset should equal the expected cpus
	reservedCpusRaw, err := os.ReadFile(filepath.Join(manager.cgroupParentPath, ReservedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	reservedCpus, err := cpuset.Parse(string(reservedCpusRaw))
	require.NoError(t, err)
	require.True(t, reservedCpus.Equal(alloc1Cpuset.Union(alloc2Cpuset)))

	// remove first allocation
	alloc1TaskPath := manager.cgroupInfo[alloc1.ID]["web"].CgroupPath
	manager.RemoveAlloc(alloc1.ID)
	manager.reconcileCpusets()

	// alloc1's task reserved cgroup should be removed
	require.NoDirExists(t, alloc1TaskPath)

	// shared cpuset should now include alloc1's cores
	sharedCpusRaw, err = os.ReadFile(filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	sharedCpus, err = cpuset.Parse(string(sharedCpusRaw))
	require.NoError(t, err)
	require.False(t, sharedCpus.ContainsAny(alloc2Cpuset))
	require.True(t, sharedCpus.IsSupersetOf(alloc1Cpuset))

	// reserved cpuset should only include alloc2's cores
	reservedCpusRaw, err = os.ReadFile(filepath.Join(manager.cgroupParentPath, ReservedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	reservedCpus, err = cpuset.Parse(string(reservedCpusRaw))
	require.NoError(t, err)
	require.True(t, reservedCpus.Equal(alloc2Cpuset))

}
