package executor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/command"
	"github.com/hashicorp/nomad/helper/discover"
	"github.com/hashicorp/nomad/nomad/structs"

	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"
)

const (
	cgroupMount = "/sys/fs/cgroup"
)

func NewExecutor() Executor {
	e := LinuxExecutor{}

	// TODO: In a follow-up PR make it so this only happens once per client.
	// Fingerprinting shouldn't happen per task.

	// Check if the process is has root capabilities.
	e.root = syscall.Geteuid() == 0

	// Check if this process can set uid.
	if e.root {
		e.setUidEnabled = true
	}

	// Check that cgroups are available. Must be root to modify it.
	if _, err := os.Stat(cgroupMount); err == nil && e.root {
		e.cgroupEnabled = true
	}

	return &e
}

// Linux executor is designed to run on linux kernel 2.8+.
type LinuxExecutor struct {
	cmd
	user *user.User

	// Finger print capabilities.
	root          bool
	setUidEnabled bool
	cgroupEnabled bool

	// Isolation configurations.
	groups *cgroupConfig.Cgroup

	// Tracking of child process.
	spawnChild        exec.Cmd
	spawnOutputWriter *os.File
	spawnOutputReader *os.File
}

func (e *LinuxExecutor) Limit(resources *structs.Resources) error {
	if resources == nil {
		return nil
	}

	if e.cgroupEnabled {
		e.configureCgroups(resources)
	}

	return nil
}

func (e *LinuxExecutor) configureCgroups(resources *structs.Resources) {
	if !e.cgroupEnabled {
		return
	}

	e.groups = &cgroupConfig.Cgroup{}

	// Groups will be created in a heiarchy according to the resource being
	// constrained, current session, and then this unique name. Restraints are
	// then placed in the corresponding files.
	// Ex: restricting a process to 2048Mhz CPU and 2MB of memory:
	//   $ cat /sys/fs/cgroup/cpu/user/1000.user/4.session/<uuid>/cpu.shares
	//		2028
	//   $ cat /sys/fs/cgroup/memory/user/1000.user/4.session/<uuid>/memory.limit_in_bytes
	//		2097152
	e.groups.Name = structs.GenerateUUID()

	// TODO: verify this is needed for things like network access
	e.groups.AllowAllDevices = true

	if resources.MemoryMB > 0 {
		// Total amount of memory allowed to consume
		e.groups.Memory = int64(resources.MemoryMB * 1024 * 1024)
		// Disable swap to avoid issues on the machine
		e.groups.MemorySwap = int64(-1)
	}

	if resources.CPU > 0.0 {
		// Set the relative CPU shares for this cgroup.
		// The simplest scale is 1 share to 1 MHz so 1024 = 1GHz. This means any
		// given process will have at least that amount of resources, but likely
		// more since it is (probably) rare that the machine will run at 100%
		// CPU. This scale will cease to work if a node is overprovisioned.
		e.groups.CpuShares = int64(resources.CPU)
	}

	if resources.IOPS > 0 {
		e.groups.BlkioThrottleReadIOpsDevice = strconv.FormatInt(int64(resources.IOPS), 10)
		e.groups.BlkioThrottleWriteIOpsDevice = strconv.FormatInt(int64(resources.IOPS), 10)
	}

}

func (e *LinuxExecutor) runAs(userid string) error {
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
	// Try to run as "nobody" user so we don't leak root privilege to the
	// spawned process. Note that we will only do this if we can call SetUID.
	// Otherwise we'll just run the other process as our current (non-root)
	// user. This means we aren't forced to run nomad as root.
	if e.setUidEnabled {
		if err := e.runAs("nobody"); err == nil && e.user != nil {
			e.cmd.SetUID(e.user.Uid)
			e.cmd.SetGID(e.user.Gid)
		}
	}

	return e.spawnDaemon()
}

// spawnCgroup executes a double fork to start the user command with proper
// isolation. Stores the child process for use in Wait.
func (e *LinuxExecutor) spawnDaemon() error {
	bin, err := discover.NomadExecutable()
	if err != nil {
		return fmt.Errorf("Failed to determine the nomad executable: %v", err)
	}

	// Serialize the cmd and the cgroup configuration so it can be passed to the
	// sub-process.
	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)

	// TODO: Do the stdout file handles once there is alloc and task directories
	// set up.
	c := command.DaemonConfig{
		Cmd:        e.cmd.Cmd,
		Groups:     e.groups,
		StdoutFile: "/dev/null",
		StderrFile: "/dev/null",
		StdinFile:  "/dev/null",
	}
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("Failed to serialize daemon configuration: %v", err)
	}

	// Create a pipe to capture Stdout.
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	e.spawnOutputWriter = pw
	e.spawnOutputReader = pr

	// Call ourselves using a hidden flag. The new instance of nomad will join
	// the passed cgroup, forkExec the cmd, and output status codes through
	// Stdout.
	escaped := strconv.Quote(buffer.String())
	spawn := exec.Command(bin, "spawn-daemon", escaped)
	spawn.Stdout = e.spawnOutputWriter

	if err := spawn.Start(); err != nil {
		fmt.Errorf("Failed to call spawn-daemon on nomad executable: %v", err)
	}

	// Parse the response.
	dec := json.NewDecoder(e.spawnOutputReader)
	var resp command.SpawnStartStatus
	if err := dec.Decode(&resp); err != nil {
		return fmt.Errorf("Failed to parse spawn-daemon start response: %v", err)
	}

	if resp.ErrorMsg != "" {
		return fmt.Errorf("Failed to execute user command: %s", resp.ErrorMsg)
	}

	e.spawnChild = *spawn
	return nil
}

// Open's behavior is to kill all processes associated with the id and return an
// error. This is done because it is not possible to re-attach to the
// spawn-daemon's stdout to retrieve status messages.
func (e *LinuxExecutor) Open(id string) error {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("Invalid id: %v", id)
	}

	switch parts[0] {
	case "PID":
		pid, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("Invalid id: failed to parse pid %v", parts[1])
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("Failed to find Pid %v: %v", pid, err)
		}

		if err := process.Kill(); err != nil {
			return fmt.Errorf("Failed to kill Pid %v: %v", pid, err)
		}
	case "CGROUP":
		if !e.cgroupEnabled {
			return errors.New("Passed a a cgroup identifier, but cgroups are disabled")
		}

		// De-serialize the cgroup configuration.
		dec := json.NewDecoder(strings.NewReader(parts[1]))
		var groups cgroupConfig.Cgroup
		if err := dec.Decode(&groups); err != nil {
			return fmt.Errorf("Failed to parse cgroup configuration: %v", err)
		}

		e.groups = &groups
		if err := e.destroyCgroup(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid id type: %v", parts[0])
	}

	return errors.New("Could not re-open to id")
}

func (e *LinuxExecutor) Wait() error {
	if e.spawnChild.Process == nil {
		return errors.New("Can not find child to wait on")
	}

	defer e.spawnOutputWriter.Close()
	defer e.spawnOutputReader.Close()

	err := e.spawnChild.Wait()
	if err != nil {
		return fmt.Errorf("Wait failed on pid %v: %v", e.spawnChild.Process.Pid, err)
	}

	// Read the exit status of the spawned process.
	dec := json.NewDecoder(e.spawnOutputReader)
	var resp command.SpawnExitStatus
	if err := dec.Decode(&resp); err != nil {
		return fmt.Errorf("Failed to parse spawn-daemon exit response: %v", err)
	}

	if !resp.Success {
		return errors.New("Task exited with error")
	}

	// If they fork/exec and then exit, wait will return but they will be still
	// running processes so we need to kill the full cgroup.
	if e.cgroupEnabled {
		return e.destroyCgroup()
	}

	return nil
}

// If cgroups are used, the ID is the cgroup structurue. Otherwise, it is the
// PID of the spawn-daemon process. An error is returned if the process was
// never started.
func (e *LinuxExecutor) ID() (string, error) {
	if e.spawnChild.Process != nil {
		if e.cgroupEnabled && e.groups != nil {
			// Serialize the cgroup structure so it can be undone on suabsequent
			// opens.
			var buffer bytes.Buffer
			enc := json.NewEncoder(&buffer)
			if err := enc.Encode(e.groups); err != nil {
				return "", fmt.Errorf("Failed to serialize daemon configuration: %v", err)
			}

			return fmt.Sprintf("CGROUP:%v", buffer.String()), nil
		}

		return fmt.Sprintf("PID:%d", e.spawnChild.Process.Pid), nil
	}

	return "", fmt.Errorf("Process has finished or was never started")
}

func (e *LinuxExecutor) Shutdown() error {
	return e.ForceStop()
}

func (e *LinuxExecutor) ForceStop() error {
	if e.spawnOutputReader != nil {
		e.spawnOutputReader.Close()
	}

	if e.spawnOutputWriter != nil {
		e.spawnOutputWriter.Close()
	}

	// If the task is not running inside a cgroup then just the spawn-daemon child is killed.
	// TODO: Find a good way to kill the children of the spawn-daemon.
	if !e.cgroupEnabled {
		if err := e.spawnChild.Process.Kill(); err != nil {
			return fmt.Errorf("Failed to kill child (%v): %v", e.spawnChild.Process.Pid, err)
		}

		return nil
	}

	return e.destroyCgroup()
}

func (e *LinuxExecutor) destroyCgroup() error {
	if e.groups == nil {
		return errors.New("Can't destroy: cgroup configuration empty")
	}

	manager := cgroupFs.Manager{}
	manager.Cgroups = e.groups
	pids, err := manager.GetPids()
	if err != nil {
		return fmt.Errorf("Failed to get pids in the cgroup %v: %v", e.groups.Name, err)
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

func (e *LinuxExecutor) Command() *cmd {
	return &e.cmd
}
