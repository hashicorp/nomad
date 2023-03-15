//go:build linux

package cgutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/lib/cpuset"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs"
	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

const (
	DefaultCgroupV1Parent    = "/nomad"
	SharedCpusetCgroupName   = "shared"
	ReservedCpusetCgroupName = "reserved"
)

// NewCpusetManagerV1 creates a  CpusetManager compatible with cgroups.v1
func NewCpusetManagerV1(cgroupParent string, _ []uint16, logger hclog.Logger) CpusetManager {
	if cgroupParent == "" {
		cgroupParent = DefaultCgroupV1Parent
	}

	cgroupParentPath, err := GetCgroupPathHelperV1("cpuset", cgroupParent)
	if err != nil {
		logger.Warn("failed to get cgroup path; disable cpuset management", "error", err)
		return new(NoopCpusetManager)
	}

	// ensures that shared cpuset exists and that the cpuset values are copied from the parent if created
	if err = cpusetEnsureParentV1(filepath.Join(cgroupParentPath, SharedCpusetCgroupName)); err != nil {
		logger.Warn("failed to ensure cgroup parent exists; disable cpuset management", "error", err)
		return new(NoopCpusetManager)
	}

	parentCpus, parentMems, err := getCpusetSubsystemSettingsV1(cgroupParentPath)
	if err != nil {
		logger.Warn("failed to detect parent cpuset settings; disable cpuset management", "error", err)
		return new(NoopCpusetManager)
	}

	parentCpuset, err := cpuset.Parse(parentCpus)
	if err != nil {
		logger.Warn("failed to parse parent cpuset.cpus setting; disable cpuset management", "error", err)
		return new(NoopCpusetManager)
	}

	// ensure the reserved cpuset exists, but only copy the mems from the parent if creating the cgroup
	if err = os.Mkdir(filepath.Join(cgroupParentPath, ReservedCpusetCgroupName), 0755); err != nil {
		if !errors.Is(err, os.ErrExist) {
			logger.Warn("failed to ensure reserved cpuset.cpus interface exists; disable cpuset management", "error", err)
			return new(NoopCpusetManager)
		}
	}

	if err = cgroups.WriteFile(filepath.Join(cgroupParentPath, ReservedCpusetCgroupName), "cpuset.mems", parentMems); err != nil {
		logger.Warn("failed to ensure reserved cpuset.mems interface exists; disable cpuset management", "error", err)
		return new(NoopCpusetManager)
	}

	return &cpusetManagerV1{
		parentCpuset:     parentCpuset,
		cgroupParent:     cgroupParent,
		cgroupParentPath: cgroupParentPath,
		cgroupInfo:       map[string]allocTaskCgroupInfo{},
		logger:           logger,
	}
}

var (
	cpusetReconcileInterval = 30 * time.Second
)

type cpusetManagerV1 struct {
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

func (c *cpusetManagerV1) AddAlloc(alloc *structs.Allocation) {
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

func (c *cpusetManagerV1) RemoveAlloc(allocID string) {
	c.mu.Lock()
	delete(c.cgroupInfo, allocID)
	c.mu.Unlock()
	go c.signalReconcile()
}

func (c *cpusetManagerV1) CgroupPathFor(allocID, task string) CgroupPathGetter {
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

		timer, stop := helper.NewSafeTimer(0)
		defer stop()

		for {

			if taskInfo.Error != nil {
				break
			}

			if _, err := os.Stat(taskInfo.CgroupPath); os.IsNotExist(err) {
				select {
				case <-ctx.Done():
					return taskInfo.CgroupPath, ctx.Err()
				case <-timer.C:
					timer.Reset(100 * time.Millisecond)
					continue
				}
			}
			break
		}

		return taskInfo.CgroupPath, taskInfo.Error
	}

}

// task name -> task cgroup info
type allocTaskCgroupInfo map[string]*TaskCgroupInfo

// Init checks that the cgroup parent and expected child cgroups have been created
// If the cgroup parent is set to /nomad then this will ensure that the /nomad/shared
// cgroup is initialized.
func (c *cpusetManagerV1) Init() {
	c.doneCh = make(chan struct{})
	c.signalCh = make(chan struct{})
	c.logger.Info("initialized cpuset cgroup manager", "parent", c.cgroupParent, "cpuset", c.parentCpuset.String())
	go c.reconcileLoop()
}

func (c *cpusetManagerV1) reconcileLoop() {
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

func (c *cpusetManagerV1) reconcileCpusets() {
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
	files, err := os.ReadDir(c.reservedCpusetPath())
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
		_, parentMems, err := getCpusetSubsystemSettingsV1(filepath.Dir(info.CgroupPath))
		if err != nil {
			c.logger.Error("failed to read parent cgroup settings for task", "path", info.CgroupPath, "error", err)
			info.Error = err
			continue
		}
		if err := cgroups.WriteFile(info.CgroupPath, "cpuset.mems", parentMems); err != nil {
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
// must hold a lock on cpusetManagerV1.mu before calling
func (_ *cpusetManagerV1) setCgroupCpusetCPUs(path, cpus string) error {
	currentCpusRaw, err := cgroups.ReadFile(path, "cpuset.cpus")
	if err != nil {
		return err
	}

	if cpus != strings.TrimSpace(currentCpusRaw) {
		if err := cgroups.WriteFile(path, "cpuset.cpus", cpus); err != nil {
			return err
		}
	}
	return nil
}

func (c *cpusetManagerV1) signalReconcile() {
	select {
	case c.signalCh <- struct{}{}:
	case <-c.doneCh:
	}
}

func (c *cpusetManagerV1) getCgroupPathsForTask(allocID, task string) (absolute, relative string) {
	return filepath.Join(c.reservedCpusetPath(), fmt.Sprintf("%s-%s", allocID, task)),
		filepath.Join(c.cgroupParent, ReservedCpusetCgroupName, fmt.Sprintf("%s-%s", allocID, task))
}

func (c *cpusetManagerV1) sharedCpusetPath() string {
	return filepath.Join(c.cgroupParentPath, SharedCpusetCgroupName)
}

func (c *cpusetManagerV1) reservedCpusetPath() string {
	return filepath.Join(c.cgroupParentPath, ReservedCpusetCgroupName)
}

func getCPUsFromCgroupV1(group string) ([]uint16, error) {
	cgroupPath, err := GetCgroupPathHelperV1("cpuset", group)
	if err != nil {
		return nil, err
	}

	cgroup := &configs.Cgroup{
		Path:      group,
		Resources: new(configs.Resources),
	}

	paths := map[string]string{
		"cpuset": cgroupPath,
	}

	man, err := fs.NewManager(cgroup, paths)
	if err != nil {
		return nil, err
	}

	stats, err := man.GetStats()
	if err != nil {
		return nil, err
	}

	return stats.CPUSetStats.CPUs, nil
}

// cpusetEnsureParentV1 makes sure that the parent directories of current
// are created and populated with the proper cpus and mems files copied
// from their respective parent. It does that recursively, starting from
// the top of the cpuset hierarchy (i.e. cpuset cgroup mount point).
func cpusetEnsureParentV1(current string) error {
	var st unix.Statfs_t

	parent := filepath.Dir(current)
	err := unix.Statfs(parent, &st)
	if err == nil && st.Type != unix.CGROUP_SUPER_MAGIC {
		return nil
	}
	// Treat non-existing directory as cgroupfs as it will be created,
	// and the root cpuset directory obviously exists.
	if err != nil && err != unix.ENOENT {
		return &os.PathError{Op: "statfs", Path: parent, Err: err}
	}

	if err := cpusetEnsureParentV1(parent); err != nil {
		return err
	}
	if err := os.Mkdir(current, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	return cpusetCopyIfNeededV1(current, parent)
}

// cpusetCopyIfNeededV1 copies the cpuset.cpus and cpuset.mems from the parent
// directory to the current directory if the file's contents are 0
func cpusetCopyIfNeededV1(current, parent string) error {
	currentCpus, currentMems, err := getCpusetSubsystemSettingsV1(current)
	if err != nil {
		return err
	}
	parentCpus, parentMems, err := getCpusetSubsystemSettingsV1(parent)
	if err != nil {
		return err
	}

	if isEmptyCpusetV1(currentCpus) {
		if err := cgroups.WriteFile(current, "cpuset.cpus", parentCpus); err != nil {
			return err
		}
	}
	if isEmptyCpusetV1(currentMems) {
		if err := cgroups.WriteFile(current, "cpuset.mems", parentMems); err != nil {
			return err
		}
	}
	return nil
}

func getCpusetSubsystemSettingsV1(parent string) (cpus, mems string, err error) {
	if cpus, err = cgroups.ReadFile(parent, "cpuset.cpus"); err != nil {
		return
	}
	if mems, err = cgroups.ReadFile(parent, "cpuset.mems"); err != nil {
		return
	}
	return cpus, mems, nil
}

func isEmptyCpusetV1(str string) bool {
	return str == "" || str == "\n"
}

func GetCgroupPathHelperV1(subsystem, cgroup string) (string, error) {
	mnt, root, err := cgroups.FindCgroupMountpointAndRoot("", subsystem)
	if err != nil {
		return "", err
	}

	// This is needed for nested containers, because in /proc/self/cgroup we
	// see paths from host, which don't exist in container.
	relCgroup, err := filepath.Rel(root, cgroup)
	if err != nil {
		return "", err
	}

	result := filepath.Join(mnt, relCgroup)
	return result, nil
}
