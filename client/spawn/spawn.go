package spawn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/docker/docker/vendor/src/gopkg.in/fsnotify.v1"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/command"
	"github.com/hashicorp/nomad/helper/discover"
)

// Spawner is used to start a user command in an isolated fashion that is
// resistent to Nomad agent failure.
type Spawner struct {
	spawn     *os.Process
	SpawnPid  int
	SpawnPpid int
	StateFile string

	// User configuration
	UserCmd *exec.Cmd
	Logs    *Logs
	Chroot  string
}

// Logs is used to define the filepaths the user command's logs should be
// redirected to. The files do not need to exist.
type Logs struct {
	Stdin, Stdout, Stderr string
}

// NewSpawner takes a path to a state file. This state file can be used to
// create a new Spawner that can be used to wait on the exit status of a
// process even through Nomad restarts.
func NewSpawner(stateFile string) *Spawner {
	return &Spawner{StateFile: stateFile}
}

// SetCommand sets the user command to spawn.
func (s *Spawner) SetCommand(cmd *exec.Cmd) {
	s.UserCmd = cmd
}

// SetLogs sets the redirection of user command log files.
func (s *Spawner) SetLogs(l *Logs) {
	s.Logs = l
}

// SetChroot puts the user command into a chroot.
func (s *Spawner) SetChroot(root string) {
	s.Chroot = root
}

// Spawn does a double-fork to start and isolate the user command. It takes a
// call-back that is invoked with the pid of the intermediary process. If the
// call back returns an error, the user command is not started and the spawn is
// cancelled. This can be used to put the process into a cgroup or jail and
// cancel starting the user process if that was not successful. An error is
// returned if the call-back returns an error or the user-command couldn't be
// started.
func (s *Spawner) Spawn(cb func(pid int) error) error {
	bin, err := discover.NomadExecutable()
	if err != nil {
		return fmt.Errorf("Failed to determine the nomad executable: %v", err)
	}

	exitFile, err := os.OpenFile(s.StateFile, os.O_CREATE|os.O_WRONLY, 0666)
	defer exitFile.Close()
	if err != nil {
		return fmt.Errorf("Error opening file to store exit status: %v", err)
	}

	config, err := s.spawnConfig()
	if err != nil {
		return err
	}

	spawn := exec.Command(bin, "spawn-daemon", config)

	// Capture stdout
	spawnStdout, err := spawn.StdoutPipe()
	defer spawnStdout.Close()
	if err != nil {
		return fmt.Errorf("Failed to capture spawn-daemon stdout: %v", err)
	}

	// Capture stdin.
	spawnStdin, err := spawn.StdinPipe()
	defer spawnStdin.Close()
	if err != nil {
		return fmt.Errorf("Failed to capture spawn-daemon stdin: %v", err)
	}

	if err := spawn.Start(); err != nil {
		return fmt.Errorf("Failed to call spawn-daemon on nomad executable: %v", err)
	}

	if cb != nil {
		cbErr := cb(spawn.Process.Pid)
		if cbErr != nil {
			errs := new(multierror.Error)
			errs = multierror.Append(errs, cbErr)
			if err := s.sendAbortCommand(spawnStdin); err != nil {
				errs = multierror.Append(errs, err)
			}

			return errs
		}
	}

	if err := s.sendStartCommand(spawnStdin); err != nil {
		return err
	}

	respCh := make(chan command.SpawnStartStatus, 1)
	errCh := make(chan error, 1)

	go func() {
		var resp command.SpawnStartStatus
		dec := json.NewDecoder(spawnStdout)
		if err := dec.Decode(&resp); err != nil {
			errCh <- fmt.Errorf("Failed to parse spawn-daemon start response: %v", err)
		}
		respCh <- resp
	}()

	select {
	case err := <-errCh:
		return err
	case resp := <-respCh:
		if resp.ErrorMsg != "" {
			return fmt.Errorf("Failed to execute user command: %s", resp.ErrorMsg)
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timed out waiting for response")
	}

	// Store the spawn process.
	s.spawn = spawn.Process
	s.SpawnPpid = os.Getpid()
	return nil
}

// spawnConfig returns a serialized config to pass to the Nomad spawn-daemon
// command.
func (s *Spawner) spawnConfig() (string, error) {
	if s.UserCmd == nil {
		return "", fmt.Errorf("Must specify user command")
	}

	config := command.DaemonConfig{
		Cmd:            *s.UserCmd,
		Chroot:         s.Chroot,
		ExitStatusFile: s.StateFile,
	}

	if s.Logs != nil {
		config.StdoutFile = s.Logs.Stdout
		config.StdinFile = s.Logs.Stdin
		config.StderrFile = s.Logs.Stderr
	}

	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	if err := enc.Encode(config); err != nil {
		return "", fmt.Errorf("Failed to serialize configuration: %v", err)
	}

	return strconv.Quote(buffer.String()), nil
}

// sendStartCommand sends the necessary command to the spawn-daemon to have it
// start the user process.
func (s *Spawner) sendStartCommand(w io.Writer) error {
	enc := json.NewEncoder(w)
	if err := enc.Encode(true); err != nil {
		return fmt.Errorf("Failed to serialize start command: %v", err)
	}

	return nil
}

// sendAbortCommand sends the necessary command to the spawn-daemon to have it
// abort starting the user process. This should be invoked if the spawn-daemon
// could not be isolated into a cgroup.
func (s *Spawner) sendAbortCommand(w io.Writer) error {
	enc := json.NewEncoder(w)
	if err := enc.Encode(false); err != nil {
		return fmt.Errorf("Failed to serialize abort command: %v", err)
	}

	return nil
}

// Wait returns the exit code of the user process or an error if the wait
// failed.
func (s *Spawner) Wait() (int, error) {
	if os.Getpid() == s.SpawnPpid {
		return s.waitAsParent()
	}

	return s.waitOnStatusFile()
}

// waitAsParent waits on the process if the current process was the spawner.
func (s *Spawner) waitAsParent() (int, error) {
	if s.SpawnPpid != os.Getpid() {
		return -1, fmt.Errorf("not the parent. Spawner parent is %v; current pid is %v", s.SpawnPpid, os.Getpid())
	}

	// Try to reattach to the spawn.
	if s.spawn == nil {
		// If it can't be reattached, it means the spawn process has exited so
		// we should just read its exit file.
		var err error
		if s.spawn, err = os.FindProcess(s.SpawnPid); err != nil {
			return s.waitOnStatusFile()
		}
	}

	if _, err := s.spawn.Wait(); err != nil {
		return -1, err
	}

	return s.waitOnStatusFile()
}

// waitOnStatusFile uses OS level file watching APIs to wait on the status file
// and returns the exit code and possibly an error.
func (s *Spawner) waitOnStatusFile() (int, error) {
	// Set up a watcher for the exit status file.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return -1, fmt.Errorf("Failed to create file watcher to read exit code: %v", err)
	}

	if err := watcher.Add(s.StateFile); err != nil {
		return -1, fmt.Errorf("Failed to watch %v to read exit code: %v", s.StateFile, err)
	}

	// Stat to check if it is there to avoid a race condition.
	stat, err := os.Stat(s.StateFile)
	if err != nil {
		return -1, fmt.Errorf("Failed to Stat exit status file %v: %v", s.StateFile, err)
	}

	// If there is data it means that the file has already been written.
	if stat.Size() > 0 {
		return s.readExitCode()
	}

	// Store the mod time as a way to heartbeat. If the file doesn't get touched
	// then we know the spawner has died. This avoids an infinite loop.
	prevModTime := stat.ModTime()

	// Wait on watcher.
	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				stat, err := os.Stat(s.StateFile)
				if err != nil {
					return -1, fmt.Errorf("Failed to Stat exit status file %v: %v", s.StateFile, err)
				}

				if stat.Size() > 0 {
					return s.readExitCode()
				}
			}
		case err := <-watcher.Errors:
			return -1, fmt.Errorf("Failed to watch %v for an exit code: %v", s.StateFile, err)
		case <-time.After(5 * time.Second):
			stat, err := os.Stat(s.StateFile)
			if err != nil {
				return -1, fmt.Errorf("Failed to Stat exit status file %v: %v", s.StateFile, err)
			}

			modTime := stat.ModTime()
			if modTime.Equal(prevModTime) {
				return -1, fmt.Errorf("Task is dead and exit code unreadable")
			}

			prevModTime = modTime
		}
	}
}

// readExitCode parses the state file and returns the exit code of the task. It
// returns an error if the file can't be read.
func (s *Spawner) readExitCode() (int, error) {
	f, err := os.Open(s.StateFile)
	defer f.Close()
	if err != nil {
		return -1, fmt.Errorf("Failed to open %v to read exit code: %v", s.StateFile, err)
	}

	stat, err := f.Stat()
	if err != nil {
		return -1, fmt.Errorf("Failed to stat file %v: %v", s.StateFile, err)
	}

	if stat.Size() == 0 {
		return -1, fmt.Errorf("Empty state file: %v", s.StateFile)
	}

	var exitStatus command.SpawnExitStatus
	dec := json.NewDecoder(f)
	if err := dec.Decode(&exitStatus); err != nil {
		return -1, fmt.Errorf("Failed to parse exit status from %v: %v", s.StateFile, err)
	}

	return exitStatus.ExitCode, nil
}
