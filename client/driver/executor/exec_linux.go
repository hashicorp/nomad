package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/hpcloud/tail"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/environment"
	"github.com/hashicorp/nomad/client/driver/spawn"
	cstructs "github.com/hashicorp/nomad/client/driver/structs"
	"github.com/hashicorp/nomad/client/helper"
	"github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/nomad/structs"
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
	rc        *helper.ResourceConstrainer
	resources *structs.Resources
	taskName  string
	taskDir   string
	allocDir  string

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
	e.resources = resources
	return nil
}

// execLinuxID contains the necessary information to reattach to an executed
// process and cleanup the created cgroups.
type ExecLinuxID struct {
	RC       *helper.ResourceConstrainer
	Spawn    *spawn.Spawner
	TaskDir  string
	TaskName string
	AllocDir string
}

func (e *LinuxExecutor) Open(id string) error {
	// De-serialize the ID.
	dec := json.NewDecoder(strings.NewReader(id))
	var execID ExecLinuxID
	if err := dec.Decode(&execID); err != nil {
		return fmt.Errorf("Failed to parse id: %v", err)
	}

	// Setup the executor.
	e.spawn = execID.Spawn
	e.taskDir = execID.TaskDir
	e.rc = execID.RC
	e.taskName = execID.TaskName
	e.allocDir = execID.AllocDir
	return e.spawn.Valid()
}

func (e *LinuxExecutor) ID() (string, error) {
	if e.rc == nil || e.spawn == nil || e.taskDir == "" {
		return "", fmt.Errorf("LinuxExecutor not properly initialized.")
	}

	// Build the ID.
	id := ExecLinuxID{
		RC:       e.rc,
		Spawn:    e.spawn,
		TaskDir:  e.taskDir,
		TaskName: e.taskName,
		AllocDir: e.allocDir,
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
		Stdout: e.logPath(e.taskName, stdoutBufExt),
		Stderr: e.logPath(e.taskName, stderrBufExt),
		Stdin:  os.DevNull,
	})

	enterCgroup := func(pid int) error {
		rc, err := helper.NewResourceConstrainer(e.resources, pid)
		if err != nil {
			return fmt.Errorf("failed to create cgroup for spawn daemon: %v", err)
		}
		e.rc = rc
		if err := e.rc.Apply(); err != nil {
			return fmt.Errorf("failed to apply resource constraint on spawn daemon: %v", err)
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

	if err := e.rc.Destroy(); err != nil {
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
	if err := e.rc.Destroy(); err != nil {
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

// Logs return a reader where logs of the task are written to
func (e *LinuxExecutor) Logs(w io.Writer, follow bool, stderr bool, stdout bool, lines int64) error {
	var to, te *tail.Tail
	var err error
	var wg sync.WaitGroup
	if stdout {
		if to, err = tail.TailFile(e.logPath(e.taskName, stdoutBufExt), tail.Config{Follow: follow}); err != nil {
			return err
		}
		wg.Add(1)
		go e.writeLog(w, to.Lines, &wg)
	}

	if stderr {
		if te, err = tail.TailFile(e.logPath(e.taskName, stderrBufExt), tail.Config{Follow: follow}); err != nil {
			return err
		}
		wg.Add(1)
		go e.writeLog(w, te.Lines, &wg)
	}
	wg.Wait()
	return nil
}

func (e *LinuxExecutor) writeLog(w io.Writer, lineCh chan *tail.Line, wg *sync.WaitGroup) {
	var l *tail.Line
	var more bool
	for {
		if l, more = <-lineCh; !more {
			wg.Done()
			return
		}
		w.Write([]byte(l.Text))
	}
}
