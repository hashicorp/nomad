package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/driver/spawn"
	"github.com/hashicorp/nomad/nomad/structs"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
)

// BasicExecutor should work everywhere, and as a result does not include
// any resource restrictions or runas capabilities.
type BasicExecutor struct {
	*ExecutorContext
	cmd      exec.Cmd
	spawn    *spawn.Spawner
	taskName string
	taskDir  string
	allocDir string
}

func NewBasicExecutor(ctx *ExecutorContext) Executor {
	return &BasicExecutor{ExecutorContext: ctx}
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
	e.cmd.Path = e.taskEnv.ReplaceEnv(e.cmd.Path)
	e.cmd.Args = e.taskEnv.ParseAndReplace(e.cmd.Args)
	e.cmd.Env = e.taskEnv.Build().EnvList()

	spawnState := filepath.Join(e.allocDir, fmt.Sprintf("%s_%s", e.taskName, "exit_status"))
	e.spawn = spawn.NewSpawner(spawnState)
	e.spawn.SetCommand(&e.cmd)
	e.spawn.SetLogs(&spawn.Logs{
		Stdout: filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stdout", e.taskName)),
		Stderr: filepath.Join(e.taskDir, allocdir.TaskLocal, fmt.Sprintf("%v.stderr", e.taskName)),
		Stdin:  os.DevNull,
	})

	return e.spawn.Spawn(nil)
}

func (e *BasicExecutor) Open(id string) error {
	var spawn spawn.Spawner
	dec := json.NewDecoder(strings.NewReader(id))
	if err := dec.Decode(&spawn); err != nil {
		return fmt.Errorf("Failed to parse id: %v", err)
	}

	// Setup the executor.
	e.spawn = &spawn
	return e.spawn.Valid()
}

func (e *BasicExecutor) Wait() *cstructs.WaitResult {
	return e.spawn.Wait()
}

func (e *BasicExecutor) ID() (string, error) {
	if e.spawn == nil {
		return "", fmt.Errorf("Process was never started")
	}

	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	if err := enc.Encode(e.spawn); err != nil {
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

	if err := proc.Kill(); err != nil && err.Error() != "os: process already finished" {
		return err
	}
	return nil
}

func (e *BasicExecutor) Command() *exec.Cmd {
	return &e.cmd
}
