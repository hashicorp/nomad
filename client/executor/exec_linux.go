package executor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/args"
	"github.com/hashicorp/nomad/client/driver/environment"
	"github.com/hashicorp/nomad/client/spawn"
	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"
)

var (
	// A mapping of directories on the host OS to attempt to embed inside each
	// task's chroot.
	chrootEnv = map[string]string{
		"/bin":     "/bin",
		"/etc":     "/etc",
		"/lib":     "/lib",
		"/lib32":   "/lib32",
		"/lib64":   "/lib64",
		"/usr/bin": "/usr/bin",
		"/usr/lib": "/usr/lib",
	}
)

func NewExecutor() Executor {
	return &LinuxExecutor{}
}

// Linux executor is designed to run on linux kernel 2.8+.
type LinuxExecutor struct {
	cmd
	user *user.User

	// Isolation configurations.
	groups   *cgroupConfig.Cgroup
	taskName string
	taskDir  string
	allocDir string

	// Spawn process.
	spawn      *spawn.Spawner
	spawnState string
}

func (e *LinuxExecutor) Command() *cmd {
	return &e.cmd
}

func (e *LinuxExecutor) Limit(resources *structs.Resources) error {
	if resources == nil {
		return errNoResources
	}

	return e.configureCgroups(resources)
}

// execLinuxID contains the necessary information to reattach to an executed
// process and cleanup the created cgroups.
type ExecLinuxID struct {
	Groups  *cgroupConfig.Cgroup
	Spawn   *spawn.Spawner
	TaskDir string
}

func (e *LinuxExecutor) Open(id string) error {
	// De-serialize the ID.
	dec := json.NewDecoder(strings.NewReader(id))
	var execID ExecLinuxID
	if err := dec.Decode(&execID); err != nil {
		return fmt.Errorf("Failed to parse id: %v", err)
	}

	// Setup the executor.
	e.groups = execID.Groups
	e.spawn = execID.Spawn
	e.taskDir = execID.TaskDir

	return nil
}

func (e *LinuxExecutor) ID() (string, error) {
	if e.groups == nil || e.spawn == nil || e.taskDir == "" {
		return "", fmt.Errorf("LinuxExecutor not properly initialized.")
	}

	// Build the ID.
	id := ExecLinuxID{
		Groups:  e.groups,
		Spawn:   e.spawn,
		TaskDir: e.taskDir,
	}

	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	if err := enc.Encode(id); err != nil {
		return "", fmt.Errorf("Failed to serialize id: %v", err)
	}

	return buffer.String(), nil
}

// runAs takes a user id as a string and looks up the user. It stores the
// results in the executor and returns an error if the user could not be found.
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
	// Run as "nobody" user so we don't leak root privilege to the spawned
	// process.
	if err := e.runAs("nobody"); err == nil && e.user != nil {
		e.cmd.SetUID(e.user.Uid)
		e.cmd.SetGID(e.user.Gid)
	}

	// Parse the commands arguments and replace instances of Nomad environment
	// variables.
	envVars, err := environment.ParseFromList(e.Cmd.Env)
	if err != nil {
		return err
	}

	parsedPath, err := args.ParseAndReplace(e.cmd.Path, envVars.Map())
	if err != nil {
		return err
	} else if len(parsedPath) != 1 {
		return fmt.Errorf("couldn't properly parse command path: %v", e.cmd.Path)
	}
	e.cmd.Path = parsedPath[0]

	combined := strings.Join(e.Cmd.Args, " ")
	parsed, err := args.ParseAndReplace(combined, envVars.Map())
	if err != nil {
		return err
	}
	e.Cmd.Args = parsed

	spawnState := filepath.Join(e.allocDir, fmt.Sprintf("%s_%s", e.taskName, "exit_status"))
	e.spawn = spawn.NewSpawner(spawnState)
	e.spawn.SetCommand(&e.cmd.Cmd)
	e.spawn.SetChroot(e.taskDir)
	e.spawn.SetLogs(&spawn.Logs{
		Stdout: filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stdout", e.taskName)),
		Stderr: filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stderr", e.taskName)),
		Stdin:  "/dev/null",
	})

	enterCgroup := func(pid int) error {
		// Join the spawn-daemon to the cgroup.
		manager := e.getCgroupManager(e.groups)

		// Apply will place the spawn dameon into the created cgroups.
		if err := manager.Apply(pid); err != nil {
			return fmt.Errorf("Failed to join spawn-daemon to the cgroup (%+v): %v", e.groups, err)
		}

		return nil
	}

	return e.spawn.Spawn(enterCgroup)
}

// Wait waits til the user process exits and returns an error on non-zero exit
// codes. Wait also cleans up the task directory and created cgroups.
func (e *LinuxExecutor) Wait() error {
	errs := new(multierror.Error)
	code, err := e.spawn.Wait()
	if err != nil {
		errs = multierror.Append(errs, err)
	}

	if code != 0 {
		errs = multierror.Append(errs, fmt.Errorf("Task exited with code: %d", code))
	}

	if err := e.destroyCgroup(); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := e.cleanTaskDir(); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

func (e *LinuxExecutor) Shutdown() error {
	return e.ForceStop()
}

// ForceStop immediately exits the user process and cleans up both the task
// directory and the cgroups.
func (e *LinuxExecutor) ForceStop() error {
	errs := new(multierror.Error)
	if err := e.destroyCgroup(); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := e.cleanTaskDir(); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

// Task Directory related functions.

// ConfigureTaskDir creates the necessary directory structure for a proper
// chroot. cleanTaskDir should be called after.
func (e *LinuxExecutor) ConfigureTaskDir(taskName string, alloc *allocdir.AllocDir) error {
	e.taskName = taskName
	e.allocDir = alloc.AllocDir

	taskDir, ok := alloc.TaskDirs[taskName]
	if !ok {
		fmt.Errorf("Couldn't find task directory for task %v", taskName)
	}
	e.taskDir = taskDir

	if err := alloc.MountSharedDir(taskName); err != nil {
		return err
	}

	if err := alloc.Embed(taskName, chrootEnv); err != nil {
		return err
	}

	// Mount dev
	dev := filepath.Join(taskDir, "dev")
	if err := os.Mkdir(dev, 0777); err != nil {
		return fmt.Errorf("Mkdir(%v) failed: %v", dev, err)
	}

	if err := syscall.Mount("", dev, "devtmpfs", syscall.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("Couldn't mount /dev to %v: %v", dev, err)
	}

	// Mount proc
	proc := filepath.Join(taskDir, "proc")
	if err := os.Mkdir(proc, 0777); err != nil {
		return fmt.Errorf("Mkdir(%v) failed: %v", proc, err)
	}

	if err := syscall.Mount("", proc, "proc", syscall.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("Couldn't mount /proc to %v: %v", proc, err)
	}

	// Set the tasks AllocDir environment variable.
	env, err := environment.ParseFromList(e.Cmd.Env)
	if err != nil {
		return err
	}
	env.SetAllocDir(filepath.Join("/", allocdir.SharedAllocName))
	env.SetTaskLocalDir(filepath.Join("/", allocdir.TaskLocal))
	e.Cmd.Env = env.List()

	return nil
}

// pathExists is a helper function to check if the path exists.
func (e *LinuxExecutor) pathExists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// cleanTaskDir is an idempotent operation to clean the task directory and
// should be called when tearing down the task.
func (e *LinuxExecutor) cleanTaskDir() error {
	// Unmount dev.
	// TODO: This should check if it is a mount.
	errs := new(multierror.Error)
	dev := filepath.Join(e.taskDir, "dev")
	if e.pathExists(dev) {
		if err := syscall.Unmount(dev, 0); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to unmount dev (%v): %v", dev, err))
		}
	}

	// Unmount proc.
	proc := filepath.Join(e.taskDir, "proc")
	if e.pathExists(proc) {
		if err := syscall.Unmount(proc, 0); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to unmount proc (%v): %v", proc, err))
		}
	}

	return errs.ErrorOrNil()
}

// Cgroup related functions.

// configureCgroups converts a Nomad Resources specification into the equivalent
// cgroup configuration. It returns an error if the resources are invalid.
func (e *LinuxExecutor) configureCgroups(resources *structs.Resources) error {
	e.groups = &cgroupConfig.Cgroup{}
	e.groups.Name = structs.GenerateUUID()

	// TODO: verify this is needed for things like network access
	e.groups.AllowAllDevices = true

	if resources.MemoryMB > 0 {
		// Total amount of memory allowed to consume
		e.groups.Memory = int64(resources.MemoryMB * 1024 * 1024)
		// Disable swap to avoid issues on the machine
		e.groups.MemorySwap = int64(-1)
	}

	if resources.CPU < 2 {
		return fmt.Errorf("resources.CPU must be equal to or greater than 2: %v", resources.CPU)
	}

	// Set the relative CPU shares for this cgroup.
	e.groups.CpuShares = int64(resources.CPU)

	if resources.IOPS != 0 {
		// Validate it is in an acceptable range.
		if resources.IOPS < 10 || resources.IOPS > 1000 {
			return fmt.Errorf("resources.IOPS must be between 10 and 1000: %d", resources.IOPS)
		}

		e.groups.BlkioWeight = uint16(resources.IOPS)
	}

	return nil
}

// destroyCgroup kills all processes in the cgroup and removes the cgroup
// configuration from the host.
func (e *LinuxExecutor) destroyCgroup() error {
	if e.groups == nil {
		return errors.New("Can't destroy: cgroup configuration empty")
	}

	manager := e.getCgroupManager(e.groups)
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

// getCgroupManager returns the correct libcontainer cgroup manager.
func (e *LinuxExecutor) getCgroupManager(groups *cgroupConfig.Cgroup) cgroups.Manager {
	var manager cgroups.Manager
	manager = &cgroupFs.Manager{Cgroups: groups}
	if systemd.UseSystemd() {
		manager = &systemd.Manager{Cgroups: groups}
	}
	return manager
}
