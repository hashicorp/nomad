package executor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/environment"
	"github.com/hashicorp/nomad/client/driver/spawn"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/nomad/structs"
)

// The buffer names which the executor uses as file extensions for the logs
const (
	stdout = "stdout"
	stderr = "stderr"
)

var (
	// A mapping of directories on the host OS to attempt to embed inside each
	// task's chroot.
	chrootEnv = map[string]string{
		"/bin":       "/bin",
		"/etc":       "/etc",
		"/lib":       "/lib",
		"/lib32":     "/lib32",
		"/lib64":     "/lib64",
		"/usr/bin":   "/usr/bin",
		"/usr/lib":   "/usr/lib",
		"/usr/share": "/usr/share",
	}
)

func NewExecutor() Executor {
	return NewLinuxExecutor()
}

func NewLinuxExecutor() Executor {
	return &LinuxExecutor{}
}

// Linux executor is designed to run on linux kernel 2.8+.
type LinuxExecutor struct {
	cmd  exec.Cmd
	user *user.User

	// Isolation configurations.
	groups   *cgroupConfig.Cgroup
	taskName string
	taskDir  string
	allocDir string

	// Spawn process.
	spawn *spawn.Spawner
}

func (e *LinuxExecutor) Command() *exec.Cmd {
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
	return e.spawn.Valid()
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

// runAs takes a user id as a string and looks up the user, and sets the command
// to execute as that user.
func (e *LinuxExecutor) runAs(userid string) error {
	u, err := user.Lookup(userid)
	if err != nil {
		return fmt.Errorf("Failed to identify user %v: %v", userid, err)
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
	if e.cmd.SysProcAttr == nil {
		e.cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if e.cmd.SysProcAttr.Credential == nil {
		e.cmd.SysProcAttr.Credential = &syscall.Credential{}
	}
	e.cmd.SysProcAttr.Credential.Uid = uint32(uid)
	e.cmd.SysProcAttr.Credential.Gid = uint32(gid)

	return nil
}

func (e *LinuxExecutor) Start() error {
	// Run as "nobody" user so we don't leak root privilege to the spawned
	// process.
	if err := e.runAs("nobody"); err != nil {
		return err
	}

	// Parse the commands arguments and replace instances of Nomad environment
	// variables.
	envVars, err := environment.ParseFromList(e.cmd.Env)
	if err != nil {
		return err
	}

	e.cmd.Path = args.ReplaceEnv(e.cmd.Path, envVars.Map())
	e.cmd.Args = args.ParseAndReplace(e.cmd.Args, envVars.Map())

	spawnState := filepath.Join(e.allocDir, fmt.Sprintf("%s_%s", e.taskName, "exit_status"))
	e.spawn = spawn.NewSpawner(spawnState)
	e.spawn.SetCommand(&e.cmd)
	e.spawn.SetChroot(e.taskDir)
	e.spawn.SetLogs(&spawn.Logs{
		Stdout: e.logPath(e.taskName, stdout),
		Stderr: e.logPath(e.taskName, stderr),
		Stdin:  os.DevNull,
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
func (e *LinuxExecutor) Wait() *cstructs.WaitResult {
	errs := new(multierror.Error)
	res := e.spawn.Wait()
	if res.Err != nil {
		errs = multierror.Append(errs, res.Err)
	}

	if err := e.destroyCgroup(); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := e.cleanTaskDir(); err != nil {
		errs = multierror.Append(errs, err)
	}

	res.Err = errs.ErrorOrNil()
	return res
}

// Shutdown sends the user process an interrupt signal indicating that it is
// about to be forcefully shutdown in sometime
func (e *LinuxExecutor) Shutdown() error {
	proc, err := os.FindProcess(e.spawn.UserPid)
	if err != nil {
		return fmt.Errorf("Failed to find user processes %v: %v", e.spawn.UserPid, err)
	}

	return proc.Signal(os.Interrupt)
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
	if !e.pathExists(dev) {
		if err := os.Mkdir(dev, 0777); err != nil {
			return fmt.Errorf("Mkdir(%v) failed: %v", dev, err)
		}

		if err := syscall.Mount("", dev, "devtmpfs", syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("Couldn't mount /dev to %v: %v", dev, err)
		}
	}

	// Mount proc
	proc := filepath.Join(taskDir, "proc")
	if !e.pathExists(proc) {
		if err := os.Mkdir(proc, 0777); err != nil {
			return fmt.Errorf("Mkdir(%v) failed: %v", proc, err)
		}

		if err := syscall.Mount("", proc, "proc", syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("Couldn't mount /proc to %v: %v", proc, err)
		}
	}

	// Set the tasks AllocDir environment variable.
	env, err := environment.ParseFromList(e.cmd.Env)
	if err != nil {
		return err
	}
	env.SetAllocDir(filepath.Join("/", allocdir.SharedAllocName))
	env.SetTaskLocalDir(filepath.Join("/", allocdir.TaskLocal))
	e.cmd.Env = env.List()

	return nil
}

// logPath returns the path of the log file for a specific buffer of the task
func (e *LinuxExecutor) logPath(taskName string, bufferName string) string {
	return filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%s.%s", taskName, bufferName))
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
	errs := new(multierror.Error)
	dev := filepath.Join(e.taskDir, "dev")
	if e.pathExists(dev) {
		if err := syscall.Unmount(dev, 0); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to unmount dev (%v): %v", dev, err))
		}

		if err := os.RemoveAll(dev); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to delete dev directory (%v): %v", dev, err))
		}
	}

	// Unmount proc.
	proc := filepath.Join(e.taskDir, "proc")
	if e.pathExists(proc) {
		if err := syscall.Unmount(proc, 0); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to unmount proc (%v): %v", proc, err))
		}

		if err := os.RemoveAll(proc); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("Failed to delete proc directory (%v): %v", dev, err))
		}
	}

	return errs.ErrorOrNil()
}

// Cgroup related functions.

// configureCgroups converts a Nomad Resources specification into the equivalent
// cgroup configuration. It returns an error if the resources are invalid.
func (e *LinuxExecutor) configureCgroups(resources *structs.Resources) error {
	e.groups = &cgroupConfig.Cgroup{}
	e.groups.Resources = &cgroupConfig.Resources{}
	e.groups.Name = structs.GenerateUUID()

	// TODO: verify this is needed for things like network access
	e.groups.Resources.AllowAllDevices = true

	if resources.MemoryMB > 0 {
		// Total amount of memory allowed to consume
		e.groups.Resources.Memory = int64(resources.MemoryMB * 1024 * 1024)
		// Disable swap to avoid issues on the machine
		e.groups.Resources.MemorySwap = int64(-1)
	}

	if resources.CPU < 2 {
		return fmt.Errorf("resources.CPU must be equal to or greater than 2: %v", resources.CPU)
	}

	// Set the relative CPU shares for this cgroup.
	e.groups.Resources.CpuShares = int64(resources.CPU)

	if resources.IOPS != 0 {
		// Validate it is in an acceptable range.
		if resources.IOPS < 10 || resources.IOPS > 1000 {
			return fmt.Errorf("resources.IOPS must be between 10 and 1000: %d", resources.IOPS)
		}

		e.groups.Resources.BlkioWeight = uint16(resources.IOPS)
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

// Logs return a reader where logs of the task are written to
func (e *LinuxExecutor) Logs() (io.Reader, error) {
	var buf bytes.Buffer
	var stdOutLogs *os.File
	var err error
	if stdOutLogs, err = os.Open(e.logPath(e.taskName, stdout)); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(stdOutLogs)

	for scanner.Scan() {
		buf.Write(scanner.Bytes())
	}
	return &buf, scanner.Err()
}
