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
	"github.com/hashicorp/nomad/client/driver/args"
	"github.com/hashicorp/nomad/client/driver/environment"
	"github.com/hashicorp/nomad/client/driver/spawn"
	"github.com/hashicorp/nomad/nomad/structs"
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

// TODO: Have raw_exec use this as well.
func NewBasicExecutor() Executor {
	return &BasicExecutor{}
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
		fmt.Errorf("Couldn't find task directory for task %v", taskName)
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
	combined := strings.Join(e.cmd.Args, " ")
	parsed, err := args.ParseAndReplace(combined, envVars.Map())
	if err != nil {
		return err
	}
	e.cmd.Args = parsed

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

func (e *BasicExecutor) Wait() error {
	code, err := e.spawn.Wait()
	if err != nil {
		return err
	}

	if code != 0 {
		return fmt.Errorf("Task exited with code: %d", code)
	}

	return nil
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

	return proc.Kill()
}

func (e *BasicExecutor) Command() *exec.Cmd {
	return &e.cmd
}
