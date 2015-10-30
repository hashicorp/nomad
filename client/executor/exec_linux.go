package executor

import (
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
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/args"
	"github.com/hashicorp/nomad/client/driver/environment"
	"github.com/hashicorp/nomad/command"
	"github.com/hashicorp/nomad/helper/discover"
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
	alloc    *allocdir.AllocDir
	taskName string
	taskDir  string

	// Tracking of spawn process.
	spawnChild        *os.Process
	spawnOutputWriter *os.File
	spawnOutputReader *os.File

	// Tracking of user process.
	exitStatusFile string
	userPid        int
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
	Groups         *cgroupConfig.Cgroup
	SpawnPid       int
	UserPid        int
	ExitStatusFile string
	TaskDir        string
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
	e.exitStatusFile = execID.ExitStatusFile
	e.userPid = execID.UserPid
	e.taskDir = execID.TaskDir

	proc, err := os.FindProcess(execID.SpawnPid)
	if proc != nil && err == nil {
		e.spawnChild = proc
	}

	return nil
}

func (e *LinuxExecutor) ID() (string, error) {
	if e.spawnChild == nil {
		return "", fmt.Errorf("Process has finished or was never started")
	}

	// Build the ID.
	id := ExecLinuxID{
		Groups:         e.groups,
		SpawnPid:       e.spawnChild.Pid,
		UserPid:        e.userPid,
		ExitStatusFile: e.exitStatusFile,
		TaskDir:        e.taskDir,
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

	if e.alloc == nil {
		return errors.New("ConfigureTaskDir() must be called before Start()")
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

	return e.spawnDaemon()
}

// spawnDaemon executes a double fork to start the user command with proper
// isolation. Stores the child process for use in Wait.
func (e *LinuxExecutor) spawnDaemon() error {
	bin, err := discover.NomadExecutable()
	if err != nil {
		return fmt.Errorf("Failed to determine the nomad executable: %v", err)
	}

	c := command.DaemonConfig{
		Cmd:            e.cmd.Cmd,
		Chroot:         e.taskDir,
		StdoutFile:     filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stdout", e.taskName)),
		StderrFile:     filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stderr", e.taskName)),
		StdinFile:      "/dev/null",
		ExitStatusFile: e.exitStatusFile,
	}

	// Serialize the cmd and the cgroup configuration so it can be passed to the
	// sub-process.
	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("Failed to serialize daemon configuration: %v", err)
	}

	// Create a pipe to capture stdout.
	if e.spawnOutputReader, e.spawnOutputWriter, err = os.Pipe(); err != nil {
		return err
	}

	// Call ourselves using a hidden flag. The new instance of nomad will join
	// the passed cgroup, forkExec the cmd, and return statuses through stdout.
	escaped := strconv.Quote(buffer.String())
	spawn := exec.Command(bin, "spawn-daemon", escaped)
	spawn.Stdout = e.spawnOutputWriter

	// Capture its Stdin.
	spawnStdIn, err := spawn.StdinPipe()
	if err != nil {
		return err
	}

	if err := spawn.Start(); err != nil {
		fmt.Errorf("Failed to call spawn-daemon on nomad executable: %v", err)
	}

	// Join the spawn-daemon to the cgroup.
	manager := e.getCgroupManager(e.groups)

	// Apply will place the spawn dameon into the created cgroups.
	if err := manager.Apply(spawn.Process.Pid); err != nil {
		errs := new(multierror.Error)
		errs = multierror.Append(errs,
			fmt.Errorf("Failed to join spawn-daemon to the cgroup (%+v): %v", e.groups, err))

		if err := sendAbortCommand(spawnStdIn); err != nil {
			errs = multierror.Append(errs, err)
		}

		return errs
	}

	// Tell it to start.
	if err := sendStartCommand(spawnStdIn); err != nil {
		return err
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

	e.userPid = resp.UserPID
	e.spawnChild = spawn.Process
	return nil
}

// sendStartCommand sends the necessary command to the spawn-daemon to have it
// start the user process.
func sendStartCommand(w io.Writer) error {
	enc := json.NewEncoder(w)
	if err := enc.Encode(true); err != nil {
		return fmt.Errorf("Failed to serialize start command: %v", err)
	}

	return nil
}

// sendAbortCommand sends the necessary command to the spawn-daemon to have it
// abort starting the user process. This should be invoked if the spawn-daemon
// could not be isolated into a cgroup.
func sendAbortCommand(w io.Writer) error {
	enc := json.NewEncoder(w)
	if err := enc.Encode(false); err != nil {
		return fmt.Errorf("Failed to serialize abort command: %v", err)
	}

	return nil
}

// Wait waits til the user process exits and returns an error on non-zero exit
// codes. Wait also cleans up the task directory and created cgroups.
func (e *LinuxExecutor) Wait() error {
	if e.spawnOutputReader != nil {
		e.spawnOutputReader.Close()
	}

	if e.spawnOutputWriter != nil {
		e.spawnOutputWriter.Close()
	}

	errs := new(multierror.Error)
	if err := e.spawnWait(); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("Wait failed on pid %v: %v", e.spawnChild.Pid, err))
	}

	if err := e.destroyCgroup(); err != nil {
		errs = multierror.Append(errs, err)
	}

	if err := e.cleanTaskDir(); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}

// spawnWait waits on the spawn-daemon and can handle the spawn-daemon not being
// a child of this process.
func (e *LinuxExecutor) spawnWait() error {
	// TODO: This needs to be able to wait on non-child processes.
	state, err := e.spawnChild.Wait()
	if err != nil {
		return err
	} else if !state.Success() {
		return fmt.Errorf("exited with non-zero code")
	}

	return nil
}

func (e *LinuxExecutor) Shutdown() error {
	return e.ForceStop()
}

// ForceStop immediately exits the user process and cleans up both the task
// directory and the cgroups.
func (e *LinuxExecutor) ForceStop() error {
	if e.spawnOutputReader != nil {
		e.spawnOutputReader.Close()
	}

	if e.spawnOutputWriter != nil {
		e.spawnOutputWriter.Close()
	}

	errs := new(multierror.Error)
	if e.groups != nil {
		if err := e.destroyCgroup(); err != nil {
			errs = multierror.Append(errs, err)
		}
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

	// Store the file path to save the exit status to.
	e.exitStatusFile = filepath.Join(alloc.AllocDir, fmt.Sprintf("%s_%s", taskName, "exit_status"))

	e.alloc = alloc
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
