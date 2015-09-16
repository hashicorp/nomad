package executor

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/nomad/structs"

	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"
)

func NewExecutor() Executor {
	return &LinuxExecutor{}
}

// Linux executor is designed to run on linux kernel 2.8+.
type LinuxExecutor struct {
	cmd
	user    *user.User
	manager *cgroupFs.Manager
}

func (e *LinuxExecutor) Limit(resources *structs.Resources) error {
	if resources == nil {
		return nil
	}
	pid, err := e.Pid()
	if err != nil {
		return fmt.Errorf("Error getting pid: %s", err)
	}
	// TODO limit some things
	if e.manager == nil {
		// accept default paths, cgroups
		e.manager = &cgroupFs.Manager{}
	}

	groups := cgroupConfig.Cgroup{}

	// Groups will be created in a heiarchy according to the resource being
	// constrained, current session, and then this unique name. Restraints are
	// then placed in the corresponding files.
	// Ex: restricting a process to 2048Mhz CPU and 2MB of memory:
	//   $ cat /sys/fs/cgroup/cpu/user/1000.user/4.session/<uuid>/cpu.shares
	//		2028
	//   $ cat /sys/fs/cgroup/memory/user/1000.user/4.session/<uuid>/memory.limit_in_bytes
	//		2097152
	groups.Name = structs.GenerateUUID()

	// TODO: verify this is needed for things like network access
	groups.AllowAllDevices = true

	if resources.MemoryMB > 0 {
		// Total amount of memory allowed to consume
		groups.Memory = int64(resources.MemoryMB * 1024 * 1024)
		// Disable swap to avoid issues on the machine
		groups.MemorySwap = int64(-1)
	}

	if resources.CPU > 0.0 {
		// Set the relative CPU shares for this cgroup.
		// The simplest scale is 1 share to 1 MHz so 1024 = 1GHz. This means any
		// given process will have at least that amount of resources, but likely
		// more since it is (probably) rare that the machine will run at 100%
		// CPU. This scale will cease to work if a node is overprovisioned.
		groups.CpuShares = int64(resources.CPU)
	}

	if resources.IOPS > 0 {
		groups.BlkioThrottleReadIOpsDevice = strconv.FormatInt(int64(resources.IOPS), 10)
		groups.BlkioThrottleWriteIOpsDevice = strconv.FormatInt(int64(resources.IOPS), 10)
	}

	e.manager.Cgroups = &groups
	// Apply will place the pid supplied into the tasks file for each of the
	// created cgroups:
	//  /sys/fs/cgroup/memory/user/1000.user/4.session/<uuid>/tasks
	//
	// Apply requires superuser permissions, and may fail if Nomad is not run with
	// the required permissions
	if err := e.manager.Apply(pid); err != nil {
		return fmt.Errorf("[ERR] Error creating limits for ExecLinux: %s", err)
	}

	return nil
}

func (e *LinuxExecutor) RunAs(userid string) error {
	errs := new(multierror.Error)

	// First, try to lookup the user by uid
	u, err := user.LookupId(userid)
	if err == nil {
		e.user = u
		return nil
	} else {
		errs = multierror.Append(errs, err)
	}

	// Lookup failed, so try by username instead
	u, err = user.Lookup(userid)
	if err == nil {
		e.user = u
		return nil
	} else {
		errs = multierror.Append(errs, err)
	}

	// If we got here we failed to lookup based on id and username, so we'll
	// return those errors.
	return fmt.Errorf("Failed to identify user to run as: %s", errs)
}

func (e *LinuxExecutor) Start() error {
	// If no user has been specified, try to run as "nobody" user so we don't
	// leak root privilege to the spawned process. Note that we will only do
	// this if we can call SetUID. Otherwise we'll just run the other process
	// as our current (non-root) user. This makes testing easier and also means
	// we aren't forced to run nomad as root.
	if e.user == nil && canSetUID() {
		e.RunAs("nobody")
	}

	// Set the user and group this process should run as. If RunAs was called
	// but we are not root this will cause Start to fail. This is intentional.
	if e.user != nil {
		e.cmd.SetUID(e.user.Uid)
		e.cmd.SetGID(e.user.Gid)
	}

	// We don't want to call ourself. We want to call Start on our embedded Cmd.
	return e.cmd.Start()
}

func (e *LinuxExecutor) Open(pid int) error {
	process, err := os.FindProcess(pid)
	// FindProcess doesn't do any checking against the process table so it's
	// unlikely we'll ever see this error.
	if err != nil {
		return fmt.Errorf("Failed to reopen pid %d: %s", pid, err)
	}

	// On linux FindProcess() will return a pid but doesn't actually check to
	// see whether that process is running. We'll send signal 0 to see if the
	// process is alive.
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return fmt.Errorf("Unable to signal pid %d: %s", err)
	}
	e.Process = process
	return nil
}

func (e *LinuxExecutor) Wait() error {
	// We don't want to call ourself. We want to call Start on our embedded Cmd
	return e.cmd.Wait()
}

func (e *LinuxExecutor) Pid() (int, error) {
	if e.cmd.Process != nil {
		return e.cmd.Process.Pid, nil
	} else {
		return 0, fmt.Errorf("Process has finished or was never started")
	}
}

func (e *LinuxExecutor) Shutdown() error {
	return e.ForceStop()
}

func (e *LinuxExecutor) ForceStop() error {
	return e.Process.Kill()
}

func (e *LinuxExecutor) Command() *cmd {
	return &e.cmd
}

// canSetUID will tell us whether we're capable of using SetUID. If we are not
// rootish this command will fail. In that case we'll just run the forked
// process under our own user.
func canSetUID() bool {
	checkroot := Command("true")
	u, err := user.Current()
	if err != nil {
		return false
	}

	// Make sure RunAs is explicitly set so we don't cause infinite recursion.
	checkroot.RunAs(u.Uid)

	err = checkroot.Start()
	if err != nil {
		return false
	}
	return true
}
