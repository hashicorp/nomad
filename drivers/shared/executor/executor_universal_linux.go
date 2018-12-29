package executor

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/helper"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	lconfigs "github.com/opencontainers/runc/libcontainer/configs"
)

// runAs takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func (e *UniversalExecutor) runAs(userid string) error {
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
	if e.childCmd.SysProcAttr == nil {
		e.childCmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if e.childCmd.SysProcAttr.Credential == nil {
		e.childCmd.SysProcAttr.Credential = &syscall.Credential{}
	}
	e.childCmd.SysProcAttr.Credential.Uid = uint32(uid)
	e.childCmd.SysProcAttr.Credential.Gid = uint32(gid)
	e.childCmd.SysProcAttr.Credential.Groups = gids

	e.logger.Debug("setting process user", "user", uid, "group", gid, "additional_groups", gids)

	return nil
}

// configureResourceContainer configured the cgroups to be used to track pids
// created by the executor
func (e *UniversalExecutor) configureResourceContainer(pid int) error {
	cfg := &lconfigs.Config{
		Cgroups: &lconfigs.Cgroup{
			Resources: &lconfigs.Resources{
				AllowAllDevices: helper.BoolToPtr(true),
			},
		},
	}

	configureBasicCgroups(cfg)
	e.resConCtx.groups = cfg.Cgroups
	return cgroups.EnterPid(cfg.Cgroups.Paths, pid)
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
