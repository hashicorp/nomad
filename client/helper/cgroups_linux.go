// +build linux
package helper

import (
	"fmt"
	"os"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/nomad/structs"
)

type ResourceConstrainer struct {
	resources *structs.Resources
	pid       int

	groups *cgroupConfig.Cgroup
}

// NewResourceConstrainer creates a cgroup which can be applied to a pid
func NewResourceConstrainer(resources *structs.Resources, pid int) (*ResourceConstrainer, error) {
	rc := ResourceConstrainer{
		resources: resources,
		pid:       pid,
	}
	if err := rc.configureCgroups(); err != nil {
		return nil, err
	}

	return &rc, nil
}

// Apply will place the pid into the created cgroups.
func (r *ResourceConstrainer) Apply() error {
	manager := r.getCgroupManager(r.groups)
	if err := manager.Apply(r.pid); err != nil {
		return fmt.Errorf("failed to join pid to the cgroup (%+v): %v", r.groups, err)
	}

	return nil
}

// Destroy removes the cgroup created for constraining resources for the pid and
// if the pid is still alive then the pid is killed along with the cgroup
func (r *ResourceConstrainer) Destroy() error {
	return r.destroyCgroup()
}

// configureCgroups converts a Nomad Resources specification into the equivalent
// cgroup configuration. It returns an error if the resources are invalid.
func (r *ResourceConstrainer) configureCgroups() error {
	r.groups = &cgroupConfig.Cgroup{}
	r.groups.Resources = &cgroupConfig.Resources{}
	r.groups.Name = structs.GenerateUUID()

	// TODO: verify this is needed for things like network access
	r.groups.Resources.AllowAllDevices = true

	if r.resources.MemoryMB > 0 {
		// Total amount of memory allowed to consume
		r.groups.Resources.Memory = int64(r.resources.MemoryMB * 1024 * 1024)
		// Disable swap to avoid issues on the machine
		r.groups.Resources.MemorySwap = int64(-1)
	}

	if r.resources.CPU < 2 {
		return fmt.Errorf("resources.CPU must be equal to or greater than 2: %v", r.resources.CPU)
	}

	// Set the relative CPU shares for this cgroup.
	r.groups.Resources.CpuShares = int64(r.resources.CPU)

	if r.resources.IOPS != 0 {
		// Validate it is in an acceptable range.
		if r.resources.IOPS < 10 || r.resources.IOPS > 1000 {
			return fmt.Errorf("resources.IOPS must be between 10 and 1000: %d", r.resources.IOPS)
		}

		r.groups.Resources.BlkioWeight = uint16(r.resources.IOPS)
	}

	return nil
}

// destroyCgroup kills all processes in the cgroup and removes the cgroup
// configuration from the host.
func (r *ResourceConstrainer) destroyCgroup() error {
	if r.groups == nil {
		return fmt.Errorf("Can't destroy: cgroup configuration empty")
	}

	manager := r.getCgroupManager(r.groups)
	pids, err := manager.GetPids()
	if err != nil {
		return fmt.Errorf("Failed to get pids in the cgroup %v: %v", r.groups.Name, err)
	}

	errs := new(multierror.Error)
	for _, pid := range pids {
		process, err := os.FindProcess(pid)
		if err != nil {
			multierror.Append(errs, fmt.Errorf("Failed to find Pid %v: %v", pid, err))
			continue
		}

		if err := process.Kill(); err != nil {
			multierror.Append(errs, fmt.Errorf("Failed to kill Pid %v: %v", pid, err))
			continue
		}
	}

	// Remove the cgroup.
	if err := manager.Destroy(); err != nil {
		multierror.Append(errs, fmt.Errorf("Failed to delete the cgroup directories: %v", err))
	}

	if len(errs.Errors) != 0 {
		return fmt.Errorf("Failed to destroy cgroup: %v", errs)
	}

	return nil
}

// getCgroupManager returns the correct libcontainer cgroup manager.
func (r *ResourceConstrainer) getCgroupManager(groups *cgroupConfig.Cgroup) cgroups.Manager {
	var manager cgroups.Manager
	manager = &cgroupFs.Manager{Cgroups: groups}
	if systemd.UseSystemd() {
		manager = &systemd.Manager{Cgroups: groups}
	}
	return manager
}
