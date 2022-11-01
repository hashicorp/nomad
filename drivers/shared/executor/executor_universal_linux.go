package executor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/hashicorp/nomad/client/lib/cgutil"
	"github.com/hashicorp/nomad/client/lib/resources"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/users"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
)

// setCmdUser takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func setCmdUser(cmd *exec.Cmd, userid string) error {
	u, err := users.Lookup(userid)
	if err != nil {
		return fmt.Errorf("failed to identify user %v: %v", userid, err)
	}

	// Get the groups the user is a part of
	gidStrings, err := u.GroupIds()
	if err != nil {
		return fmt.Errorf("unable to lookup user's group membership: %v", err)
	}

	gids := make([]uint32, len(gidStrings))
	for _, gidString := range gidStrings {
		u, err := strconv.ParseUint(gidString, 10, 32)
		if err != nil {
			return fmt.Errorf("unable to convert user's group to uint32 %s: %v", gidString, err)
		}

		gids = append(gids, uint32(u))
	}

	// Convert the uid and gid
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("unable to convert userid to uint32: %s", err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("unable to convert groupid to uint32: %s", err)
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
	cfg := &configs.Config{
		Cgroups: &configs.Cgroup{
			Resources: &configs.Resources{},
		},
	}

	// note: this was always here, but not used until cgroups v2 support
	for _, device := range specconv.AllowedDevices {
		cfg.Cgroups.Resources.Devices = append(cfg.Cgroups.Resources.Devices, &device.Rule)
	}

	lookup := func(env []string, name string) (result string) {
		for _, s := range env {
			if strings.HasPrefix(s, name+"=") {
				result = strings.TrimLeft(s, name+"=")
				return
			}
		}
		return
	}

	if cgutil.UseV2 {
		// in v2 we have the definitive cgroup; create and enter it

		// use the task environment variables for determining the cgroup path -
		// not ideal but plumbing the values directly requires grpc protobuf changes
		parent := lookup(e.commandCfg.Env, taskenv.CgroupParent)
		allocID := lookup(e.commandCfg.Env, taskenv.AllocID)
		task := lookup(e.commandCfg.Env, taskenv.TaskName)
		if parent == "" || allocID == "" || task == "" {
			return fmt.Errorf(
				"environment variables %s must be set",
				strings.Join([]string{taskenv.CgroupParent, taskenv.AllocID, taskenv.TaskName}, ","),
			)
		}
		scope := cgutil.CgroupScope(allocID, task)
		path := filepath.Join("/", cgutil.GetCgroupParent(parent), scope)
		cfg.Cgroups.Path = path
		e.containment = resources.Contain(e.logger, cfg.Cgroups)
		return e.containment.Apply(pid)

	} else {
		// in v1 create a freezer cgroup for use by containment

		if err := cgutil.ConfigureBasicCgroups(cfg); err != nil {
			// Log this error to help diagnose cases where nomad is run with too few
			// permissions, but don't return an error. There is no separate check for
			// cgroup creation permissions, so this may be the happy path.
			e.logger.Warn("failed to create cgroup",
				"docs", "https://www.nomadproject.io/docs/drivers/raw_exec.html#no_cgroups",
				"error", err)
			return nil
		}
		path := cfg.Cgroups.Path
		e.logger.Trace("cgroup created, now need to apply", "path", path)
		e.containment = resources.Contain(e.logger, cfg.Cgroups)
		return e.containment.Apply(pid)
	}
}

func (e *UniversalExecutor) getAllPids() (resources.PIDs, error) {
	if e.containment == nil {
		return getAllPidsByScanning()
	}
	return e.containment.GetPIDs(), nil
}

// withNetworkIsolation calls the passed function the network namespace `spec`
func withNetworkIsolation(f func() error, spec *drivers.NetworkIsolationSpec) error {
	if spec != nil && spec.Path != "" {
		// Get a handle to the target network namespace
		netNS, err := ns.GetNS(spec.Path)
		if err != nil {
			return err
		}

		// Start the container in the network namespace
		return netNS.Do(func(ns.NetNS) error {
			return f()
		})
	}
	return f()
}
