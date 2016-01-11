package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/hpcloud/tail"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/environment"
	"github.com/hashicorp/nomad/client/driver/spawn"
	"github.com/hashicorp/nomad/helper/args"
	"github.com/hashicorp/nomad/nomad/structs"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
)

// BasicExecutor should work everywhere, and as a result does not include
// any resource restrictions or runas capabilities.
type BasicExecutor struct {
	cmd      exec.Cmd
	spawn    *spawn.Spawner
	taskName string
	taskDir  string
	allocDir string
}

func NewBasicExecutor() Executor {
	return &BasicExecutor{}
}

type ExecBasicID struct {
	Spawn    *spawn.Spawner
	TaskDir  string
	TaskName string
	AllocDir string
}

func (e *BasicExecutor) Limit(resources *structs.Resources) error {
	if resources == nil {
		return errNoResources
	}
	return nil
}

func (e *BasicExecutor) ConfigureTaskDir(taskName string, alloc *allocdir.AllocDir) error {
	taskDir, ok := alloc.TaskDirs[taskName]
	if !ok {
		return fmt.Errorf("Couldn't find task directory for task %v", taskName)
	}
	e.cmd.Dir = taskDir

	e.taskDir = taskDir
	e.taskName = taskName
	e.allocDir = alloc.AllocDir
	return nil
}

func (e *BasicExecutor) Start() error {
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
	e.spawn.SetLogs(&spawn.Logs{
		Stdout: e.logPath(e.taskName, stdoutBufExt),
		Stderr: e.logPath(e.taskName, stderrBufExt),
		Stdin:  os.DevNull,
	})

	return e.spawn.Spawn(nil)
}

func (e *BasicExecutor) Open(id string) error {
	var basicID ExecBasicID
	dec := json.NewDecoder(strings.NewReader(id))
	if err := dec.Decode(&basicID); err != nil {
		return fmt.Errorf("Failed to parse id: %v", err)
	}

	// Setup the executor.
	e.spawn = basicID.Spawn
	e.taskDir = basicID.TaskDir
	e.taskName = basicID.TaskName
	e.allocDir = basicID.AllocDir

	return e.spawn.Valid()
}

func (e *BasicExecutor) Wait() *cstructs.WaitResult {
	return e.spawn.Wait()
}

func (e *BasicExecutor) ID() (string, error) {
	if e.spawn == nil {
		return "", fmt.Errorf("Process was never started")
	}

	id := ExecBasicID{
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

func (e *BasicExecutor) Shutdown() error {
	proc, err := os.FindProcess(e.spawn.UserPid)
	if err != nil {
		return fmt.Errorf("Failed to find user processes %v: %v", e.spawn.UserPid, err)
	}

	if runtime.GOOS == "windows" {
		return proc.Kill()
	}

	return proc.Signal(os.Interrupt)
}

func (e *BasicExecutor) ForceStop() error {
	proc, err := os.FindProcess(e.spawn.UserPid)
	if err != nil {
		return fmt.Errorf("Failed to find user processes %v: %v", e.spawn.UserPid, err)
	}

	return proc.Kill()
}

func (e *BasicExecutor) Command() *exec.Cmd {
	return &e.cmd
}

func (e *BasicExecutor) Logs(w io.Writer, follow bool, stdout bool, stderr bool, lines int64) error {
	var to, te *tail.Tail
	var err error
	var wg sync.WaitGroup
	if stdout {
		if to, err = tail.TailFile(e.logPath(e.taskName, stdoutBufExt), tail.Config{Follow: follow}); err != nil {
			return err
		}
		wg.Add(1)
		go e.writeLogLine(w, to.Lines, &wg)
	}

	if stderr {
		if te, err = tail.TailFile(e.logPath(e.taskName, stderrBufExt), tail.Config{Follow: follow}); err != nil {
			return err
		}
		wg.Add(1)
		go e.writeLogLine(w, te.Lines, &wg)
	}
	wg.Wait()
	return nil
}

func (e *BasicExecutor) writeLogLine(w io.Writer, lineCh chan *tail.Line, wg *sync.WaitGroup) {
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

// logPath returns the path of the log file for a specific buffer of the task
func (e *BasicExecutor) logPath(taskName string, bufferName string) string {
	return filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%s.%s", taskName, bufferName))
}
