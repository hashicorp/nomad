package cgutil

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/lib/cpuset"
	"github.com/hashicorp/nomad/nomad/structs"
)

func NewCpusetManager(cgroupParent string, logger hclog.Logger) CpusetManager {
	if cgroupParent == "" {
		cgroupParent = DefaultCgroupParent
	}
	return &cpusetManager{
		cgroupParent: cgroupParent,
		cgroupInfo:   map[string]allocTaskCgroupInfo{},
		logger:       logger,
	}
}

var (
	cpusetReconcileInterval = 30 * time.Second
)

type cpusetManager struct {
	// cgroupParent relative to the cgroup root. ex. '/nomad'
	cgroupParent string
	// cgroupParentPath is the absolute path to the cgroup parent.
	cgroupParentPath string

	parentCpuset cpuset.CPUSet

	// all exported functions are synchronized
	mu sync.Mutex

	cgroupInfo map[string]allocTaskCgroupInfo

	doneCh   chan struct{}
	signalCh chan struct{}
	logger   hclog.Logger
}

func (c *cpusetManager) AddAlloc(alloc *structs.Allocation) {
	if alloc == nil || alloc.AllocatedResources == nil {
		return
	}
	allocInfo := allocTaskCgroupInfo{}
	for task, resources := range alloc.AllocatedResources.Tasks {
		taskCpuset := cpuset.New(resources.Cpu.ReservedCores...)
		cgroupPath := filepath.Join(c.cgroupParentPath, SharedCpusetCgroupName)
		relativeCgroupPath := filepath.Join(c.cgroupParent, SharedCpusetCgroupName)
		if taskCpuset.Size() > 0 {
			cgroupPath, relativeCgroupPath = c.getCgroupPathsForTask(alloc.ID, task)
		}
		allocInfo[task] = &TaskCgroupInfo{
			CgroupPath:         cgroupPath,
			RelativeCgroupPath: relativeCgroupPath,
			Cpuset:             taskCpuset,
		}
	}
	c.mu.Lock()
	c.cgroupInfo[alloc.ID] = allocInfo
	c.mu.Unlock()
	go c.signalReconcile()
}

func (c *cpusetManager) RemoveAlloc(allocID string) {
	c.mu.Lock()
	delete(c.cgroupInfo, allocID)
	c.mu.Unlock()
	go c.signalReconcile()
}

func (c *cpusetManager) CgroupPathFor(allocID, task string) CgroupPathGetter {
	return func(ctx context.Context) (string, error) {
		c.mu.Lock()
		allocInfo, ok := c.cgroupInfo[allocID]
		if !ok {
			c.mu.Unlock()
			return "", fmt.Errorf("alloc not found for id %q", allocID)
		}

		taskInfo, ok := allocInfo[task]
		c.mu.Unlock()
		if !ok {
			return "", fmt.Errorf("task %q not found", task)
		}

		for {
			if taskInfo.Error != nil {
				break
			}
			if _, err := os.Stat(taskInfo.CgroupPath); os.IsNotExist(err) {
				select {
				case <-ctx.Done():
					return taskInfo.CgroupPath, ctx.Err()
				case <-time.After(100 * time.Millisecond):
					continue
				}
			}
			break
		}

		return taskInfo.CgroupPath, taskInfo.Error
	}

}

type allocTaskCgroupInfo map[string]*TaskCgroupInfo

// Init checks that the cgroup parent and expected child cgroups have been created
// If the cgroup parent is set to /nomad then this will ensure that the /nomad/shared
// cgroup is initialized.
func (c *cpusetManager) Init() error {
	cgroupParentPath, err := getCgroupPathHelper("cpuset", c.cgroupParent)
	if err != nil {
		return err
	}
	c.cgroupParentPath = cgroupParentPath

	// ensures that shared cpuset exists and that the cpuset values are copied from the parent if created
	if err := cpusetEnsureParent(filepath.Join(cgroupParentPath, SharedCpusetCgroupName)); err != nil {
		return err
	}

	parentCpus, parentMems, err := getCpusetSubsystemSettings(cgroupParentPath)
	if err != nil {
		return fmt.Errorf("failed to detect parent cpuset settings: %v", err)
	}
	c.parentCpuset, err = cpuset.Parse(parentCpus)
	if err != nil {
		return fmt.Errorf("failed to parse parent cpuset.cpus setting: %v", err)
	}

	// ensure the reserved cpuset exists, but only copy the mems from the parent if creating the cgroup
	if err := os.Mkdir(filepath.Join(cgroupParentPath, ReservedCpusetCgroupName), 0755); err == nil {
		// cgroup created, leave cpuset.cpus empty but copy cpuset.mems from parent
		if err != nil {
			return err
		}
	} else if !os.IsExist(err) {
		return err
	}

	if err := fscommon.WriteFile(filepath.Join(cgroupParentPath, ReservedCpusetCgroupName), "cpuset.mems", parentMems); err != nil {
		return err
	}

	c.doneCh = make(chan struct{})
	c.signalCh = make(chan struct{})

	c.logger.Info("initialized cpuset cgroup manager", "parent", c.cgroupParent, "cpuset", c.parentCpuset.String())

	go c.reconcileLoop()
	return nil
}

func (c *cpusetManager) reconcileLoop() {
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for {
		select {
		case <-c.doneCh:
			c.logger.Debug("shutting down reconcile loop")
			return
		case <-c.signalCh:
			timer.Reset(500 * time.Millisecond)
		case <-timer.C:
			c.reconcileCpusets()
			timer.Reset(cpusetReconcileInterval)
		}
	}
}

func (c *cpusetManager) reconcileCpusets() {
	c.mu.Lock()
	defer c.mu.Unlock()
	sharedCpuset := cpuset.New(c.parentCpuset.ToSlice()...)
	reservedCpuset := cpuset.New()
	taskCpusets := map[string]*TaskCgroupInfo{}
	for _, alloc := range c.cgroupInfo {
		for _, task := range alloc {
			if task.Cpuset.Size() == 0 {
				continue
			}
			sharedCpuset = sharedCpuset.Difference(task.Cpuset)
			reservedCpuset = reservedCpuset.Union(task.Cpuset)
			taskCpusets[task.CgroupPath] = task
		}
	}

	// look for reserved cpusets which we don't know about and remove
	files, err := ioutil.ReadDir(c.reservedCpusetPath())
	if err != nil {
		c.logger.Error("failed to list files in reserved cgroup path during reconciliation", "path", c.reservedCpusetPath(), "error", err)
	}
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		path := filepath.Join(c.reservedCpusetPath(), f.Name())
		if _, ok := taskCpusets[path]; ok {
			continue
		}
		c.logger.Debug("removing reserved cpuset cgroup", "path", path)
		err := cgroups.RemovePaths(map[string]string{"cpuset": path})
		if err != nil {
			c.logger.Error("removal of existing cpuset cgroup failed", "path", path, "error", err)
		}
	}

	if err := c.setCgroupCpusetCPUs(c.sharedCpusetPath(), sharedCpuset.String()); err != nil {
		c.logger.Error("could not write shared cpuset.cpus", "path", c.sharedCpusetPath(), "cpuset.cpus", sharedCpuset.String(), "error", err)
	}
	if err := c.setCgroupCpusetCPUs(c.reservedCpusetPath(), reservedCpuset.String()); err != nil {
		c.logger.Error("could not write reserved cpuset.cpus", "path", c.reservedCpusetPath(), "cpuset.cpus", reservedCpuset.String(), "error", err)
	}
	for _, info := range taskCpusets {
		if err := os.Mkdir(info.CgroupPath, 0755); err != nil && !os.IsExist(err) {
			c.logger.Error("failed to create new cgroup path for task", "path", info.CgroupPath, "error", err)
			info.Error = err
			continue
		}

		// copy cpuset.mems from parent
		_, parentMems, err := getCpusetSubsystemSettings(filepath.Dir(info.CgroupPath))
		if err != nil {
			c.logger.Error("failed to read parent cgroup settings for task", "path", info.CgroupPath, "error", err)
			info.Error = err
			continue
		}
		if err := fscommon.WriteFile(info.CgroupPath, "cpuset.mems", parentMems); err != nil {
			c.logger.Error("failed to write cgroup cpuset.mems setting for task", "path", info.CgroupPath, "mems", parentMems, "error", err)
			info.Error = err
			continue
		}
		if err := c.setCgroupCpusetCPUs(info.CgroupPath, info.Cpuset.String()); err != nil {
			c.logger.Error("failed to write cgroup cpuset.cpus settings for task", "path", info.CgroupPath, "cpus", info.Cpuset.String(), "error", err)
			info.Error = err
			continue
		}
	}
}

// setCgroupCpusetCPUs will compare an existing cpuset.cpus value with an expected value, overwriting the existing if different
// must hold a lock on cpusetManager.mu before calling
func (_ *cpusetManager) setCgroupCpusetCPUs(path, cpus string) error {
	currentCpusRaw, err := fscommon.ReadFile(path, "cpuset.cpus")
	if err != nil {
		return err
	}

	if cpus != strings.TrimSpace(currentCpusRaw) {
		if err := fscommon.WriteFile(path, "cpuset.cpus", cpus); err != nil {
			return err
		}
	}
	return nil
}

func (c *cpusetManager) signalReconcile() {
	select {
	case c.signalCh <- struct{}{}:
	case <-c.doneCh:
	}
}

func (c *cpusetManager) getCpuset(group string) (cpuset.CPUSet, error) {
	man := cgroupFs.NewManager(
		&configs.Cgroup{
			Path: filepath.Join(c.cgroupParent, group),
		},
		map[string]string{"cpuset": filepath.Join(c.cgroupParentPath, group)},
		false,
	)
	stats, err := man.GetStats()
	if err != nil {
		return cpuset.CPUSet{}, err
	}
	return cpuset.New(stats.CPUSetStats.CPUs...), nil
}

func (c *cpusetManager) getCgroupPathsForTask(allocID, task string) (absolute, relative string) {
	return filepath.Join(c.reservedCpusetPath(), fmt.Sprintf("%s-%s", allocID, task)),
		filepath.Join(c.cgroupParent, ReservedCpusetCgroupName, fmt.Sprintf("%s-%s", allocID, task))
}

func (c *cpusetManager) sharedCpusetPath() string {
	return filepath.Join(c.cgroupParentPath, SharedCpusetCgroupName)
}

func (c *cpusetManager) reservedCpusetPath() string {
	return filepath.Join(c.cgroupParentPath, ReservedCpusetCgroupName)
}
