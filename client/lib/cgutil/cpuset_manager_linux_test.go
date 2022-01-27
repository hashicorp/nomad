package cgutil

import (
	"io/ioutil"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/hashicorp/nomad/lib/cpuset"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/opencontainers/runc/libcontainer/cgroups"

	"github.com/hashicorp/nomad/helper/uuid"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/helper/testlog"
)

func tmpCpusetManager(t *testing.T) (manager *cpusetManager, cleanup func()) {
	if runtime.GOOS != "linux" || syscall.Geteuid() != 0 {
		t.Skip("Test only available running as root on linux")
	}
	mount, err := FindCgroupMountpointDir()
	if err != nil || mount == "" {
		t.Skipf("Failed to find cgroup mount: %v %v", mount, err)
	}

	parent := "/gotest-" + uuid.Short()
	require.NoError(t, cpusetEnsureParent(parent))

	manager = &cpusetManager{
		cgroupParent: parent,
		cgroupInfo:   map[string]allocTaskCgroupInfo{},
		logger:       testlog.HCLogger(t),
	}

	parentPath, err := getCgroupPathHelper("cpuset", parent)
	require.NoError(t, err)

	return manager, func() { require.NoError(t, cgroups.RemovePaths(map[string]string{"cpuset": parentPath})) }
}

func TestCpusetManager_Init(t *testing.T) {
	manager, cleanup := tmpCpusetManager(t)
	defer cleanup()
	require.NoError(t, manager.Init())

	require.DirExists(t, filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName))
	require.FileExists(t, filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName, "cpuset.cpus"))
	sharedCpusRaw, err := ioutil.ReadFile(filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	sharedCpus, err := cpuset.Parse(string(sharedCpusRaw))
	require.NoError(t, err)
	require.Exactly(t, manager.parentCpuset.ToSlice(), sharedCpus.ToSlice())
	require.DirExists(t, filepath.Join(manager.cgroupParentPath, ReservedCpusetCgroupName))
}

func TestCpusetManager_AddAlloc_single(t *testing.T) {
	manager, cleanup := tmpCpusetManager(t)
	defer cleanup()
	require.NoError(t, manager.Init())

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
	sharedCpusRaw, err := ioutil.ReadFile(filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	sharedCpus, err := cpuset.Parse(string(sharedCpusRaw))
	require.NoError(t, err)
	require.NotEmpty(t, sharedCpus.ToSlice())
	require.NotContains(t, sharedCpus.ToSlice(), uint16(0))

	// check that the 0th core is allocated to reserved cgroup
	require.DirExists(t, filepath.Join(manager.cgroupParentPath, ReservedCpusetCgroupName))
	reservedCpusRaw, err := ioutil.ReadFile(filepath.Join(manager.cgroupParentPath, ReservedCpusetCgroupName, "cpuset.cpus"))
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
	taskCpusRaw, err := ioutil.ReadFile(filepath.Join(taskInfo.CgroupPath, "cpuset.cpus"))
	require.NoError(t, err)
	taskCpus, err := cpuset.Parse(string(taskCpusRaw))
	require.NoError(t, err)
	require.Exactly(t, alloc.AllocatedResources.Tasks["web"].Cpu.ReservedCores, taskCpus.ToSlice())
}

func TestCpusetManager_AddAlloc_subset(t *testing.T) {
	t.Skip("todo: add test for #11933")
}

func TestCpusetManager_AddAlloc_all(t *testing.T) {
	// cgroupsv2 changes behavior of writing empty cpuset.cpu, which is what
	// happens to the /shared group when one or more allocs consume all available
	// cores.
	t.Skip("todo: add test for #11933")
}

func TestCpusetManager_RemoveAlloc(t *testing.T) {
	manager, cleanup := tmpCpusetManager(t)
	defer cleanup()
	require.NoError(t, manager.Init())

	// this case tests adding 2 allocs, reconciling then removing 1 alloc
	// it requires the system to have atleast 2 cpu cores (one for each alloc)
	if manager.parentCpuset.Size() < 2 {
		t.Skip("test requires atleast 2 cpu cores")
	}

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
	sharedCpusRaw, err := ioutil.ReadFile(filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	sharedCpus, err := cpuset.Parse(string(sharedCpusRaw))
	require.NoError(t, err)
	require.False(t, sharedCpus.ContainsAny(alloc1Cpuset.Union(alloc2Cpuset)))

	// reserved cpuset should equal the expected cpus
	reservedCpusRaw, err := ioutil.ReadFile(filepath.Join(manager.cgroupParentPath, ReservedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	reservedCpus, err := cpuset.Parse(string(reservedCpusRaw))
	require.NoError(t, err)
	require.True(t, reservedCpus.Equals(alloc1Cpuset.Union(alloc2Cpuset)))

	// remove first allocation
	alloc1TaskPath := manager.cgroupInfo[alloc1.ID]["web"].CgroupPath
	manager.RemoveAlloc(alloc1.ID)
	manager.reconcileCpusets()

	// alloc1's task reserved cgroup should be removed
	require.NoDirExists(t, alloc1TaskPath)

	// shared cpuset should now include alloc1's cores
	sharedCpusRaw, err = ioutil.ReadFile(filepath.Join(manager.cgroupParentPath, SharedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	sharedCpus, err = cpuset.Parse(string(sharedCpusRaw))
	require.NoError(t, err)
	require.False(t, sharedCpus.ContainsAny(alloc2Cpuset))
	require.True(t, sharedCpus.IsSupersetOf(alloc1Cpuset))

	// reserved cpuset should only include alloc2's cores
	reservedCpusRaw, err = ioutil.ReadFile(filepath.Join(manager.cgroupParentPath, ReservedCpusetCgroupName, "cpuset.cpus"))
	require.NoError(t, err)
	reservedCpus, err = cpuset.Parse(string(reservedCpusRaw))
	require.NoError(t, err)
	require.True(t, reservedCpus.Equals(alloc2Cpuset))

}
