package executor

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	lconfigs "github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
)

// setCmdUser takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func setCmdUser(cmd *exec.Cmd, userid string) error {
	u, err := user.Lookup(userid)
	if err != nil {
		return fmt.Errorf("Failed to identify user %v: %v", userid, err)
	}

	// Get the groups the user is a part of
	gidStrings, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("Unable to lookup user's group membership: %v", err)
	}

	gids := make([]uint32, len(gidStrings))
	for _, gidString := range gidStrings {
		u, err := strconv.Atoi(gidString)
		if err != nil {
			return fmt.Errorf("Unable to convert user's group to int %s: %v", gidString, err)
		}

		gids = append(gids, uint32(u))
	}

	// Convert the uid and gid
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert userid to uint32: %s", err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("Unable to convert groupid to uint32: %s", err)
	}

	// Set the command to run as that user and group.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if cmd.SysProcAttr.Credential == nil {
		cmd.SysProcAttr.Credential = &syscall.Credential{}
	}
	cmd.SysProcAttr.Credential.Uid = uint32(uid)
	cmd.SysProcAttr.Credential.Gid = uint32(gid)
	cmd.SysProcAttr.Credential.Groups = gids

	return nil
}

// configureResourceContainer configured the cgroups to be used to track pids
// created by the executor
func (e *UniversalExecutor) configureResourceContainer(pid int) error {
	cfg := &lconfigs.Config{
		Cgroups: &lconfigs.Cgroup{
			Resources: &lconfigs.Resources{},
		},
	}
	for _, device := range specconv.AllowedDevices {
		cfg.Cgroups.Resources.Devices = append(cfg.Cgroups.Resources.Devices, &device.DeviceRule)
	}

	err := configureBasicCgroups(cfg)
	if err != nil {
		// Log this error to help diagnose cases where nomad is run with too few
		// permissions, but don't return an error. There is no separate check for
		// cgroup creation permissions, so this may be the happy path.
		e.logger.Warn("failed to create cgroup",
			"docs", "https://www.nomadproject.io/docs/drivers/raw_exec.html#no_cgroups",
			"error", err)
		return nil
	}
	e.resConCtx.groups = cfg.Cgroups
	return cgroups.EnterPid(cfg.Cgroups.Paths, pid)
}

func (e *UniversalExecutor) getAllPids() (map[int]*nomadPid, error) {
	if e.resConCtx.isEmpty() {
		return getAllPidsByScanning()
	} else {
		return e.resConCtx.getAllPidsByCgroup()
	}
}

// DestroyCgroup kills all processes in the cgroup and removes the cgroup
// configuration from the host. This function is idempotent.
func DestroyCgroup(groups *lconfigs.Cgroup, executorPid int) error {
	mErrs := new(multierror.Error)
	if groups == nil {
		return fmt.Errorf("Can't destroy: cgroup configuration empty")
	}

	// Move the executor into the global cgroup so that the task specific
	// cgroup can be destroyed.
	path, err := cgroups.GetInitCgroupPath("freezer")
	if err != nil {
		return err
	}

	if err := cgroups.EnterPid(map[string]string{"freezer": path}, executorPid); err != nil {
		return err
	}

	// Freeze the Cgroup so that it can not continue to fork/exec.
	groups.Resources.Freezer = lconfigs.Frozen
	freezer := cgroupFs.FreezerGroup{}
	if err := freezer.Set(groups.Paths[freezer.Name()], groups); err != nil {
		return err
	}

	var procs []*os.Process
	pids, err := cgroups.GetAllPids(groups.Paths[freezer.Name()])
	if err != nil {
		multierror.Append(mErrs, fmt.Errorf("error getting pids: %v", err))

		// Unfreeze the cgroup.
		groups.Resources.Freezer = lconfigs.Thawed
		freezer := cgroupFs.FreezerGroup{}
		if err := freezer.Set(groups.Paths[freezer.Name()], groups); err != nil {
			multierror.Append(mErrs, fmt.Errorf("failed to unfreeze cgroup: %v", err))
			return mErrs.ErrorOrNil()
		}
	}

	// Kill the processes in the cgroup
	for _, pid := range pids {
		proc, err := os.FindProcess(pid)
		if err != nil {
			multierror.Append(mErrs, fmt.Errorf("error finding process %v: %v", pid, err))
			continue
		}

		procs = append(procs, proc)
		if e := proc.Kill(); e != nil {
			multierror.Append(mErrs, fmt.Errorf("error killing process %v: %v", pid, e))
		}
	}

	// Unfreeze the cgroug so we can wait.
	groups.Resources.Freezer = lconfigs.Thawed
	if err := freezer.Set(groups.Paths[freezer.Name()], groups); err != nil {
		multierror.Append(mErrs, fmt.Errorf("failed to unfreeze cgroup: %v", err))
		return mErrs.ErrorOrNil()
	}

	// Wait on the killed processes to ensure they are cleaned up.
	for _, proc := range procs {
		// Don't capture the error because we expect this to fail for
		// processes we didn't fork.
		proc.Wait()
	}

	// Remove the cgroup.
	if err := cgroups.RemovePaths(groups.Paths); err != nil {
		multierror.Append(mErrs, fmt.Errorf("failed to delete the cgroup directories: %v", err))
	}
	return mErrs.ErrorOrNil()
}

// withNetworkIsolation calls the passed function the network namespace `spec`
func withNetworkIsolation(f func() error, spec *drivers.NetworkIsolationSpec) error {
	if spec != nil && spec.Path != "" {
		// Get a handle to the target network namespace
		netns, err := ns.GetNS(spec.Path)
		if err != nil {
			return err
		}

		// Start the container in the network namespace
		return netns.Do(func(ns.NetNS) error {
			return f()
		})
	}

	return f()
}
