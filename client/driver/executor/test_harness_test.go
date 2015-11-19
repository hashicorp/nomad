package executor

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestMain(m *testing.M) {
	switch os.Getenv("TEST_MAIN") {
	case "app":
		appMain()
	default:
		os.Exit(m.Run())
	}
}

func appMain() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "no command provided")
		os.Exit(1)
	}
	args := os.Args[1:]
	shift := func() string {
		s := args[0]
		args = args[1:]
		return s
	}
	for len(args) > 0 {
		switch cmd := shift(); cmd {
		case "sleep":
			if len(args) < 1 {
				fmt.Fprintln(os.Stderr, "expected arg for sleep")
				os.Exit(1)
			}
			dur, err := time.ParseDuration(shift())
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not parse sleep time: %v", err)
				os.Exit(1)
			}
			time.Sleep(dur)
		case "echo":
			fmt.Println(strings.Join(args, " "))
			args = args[:0]
		case "write":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "expected two args for write")
				os.Exit(1)
			}
			msg := shift()
			file := shift()
			ioutil.WriteFile(file, []byte(msg), 0666)
		default:
			fmt.Fprintln(os.Stderr, "unknown command:", cmd)
			os.Exit(1)
		}
	}
}

var (
	constraint = &structs.Resources{
		CPU:      250,
		MemoryMB: 256,
		Networks: []*structs.NetworkResource{
			&structs.NetworkResource{
				MBits:        50,
				DynamicPorts: []structs.Port{{Label: "http"}},
			},
		},
	}
)

func mockAllocDir(t *testing.T) (string, *allocdir.AllocDir) {
	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]

	allocDir := allocdir.NewAllocDir(filepath.Join(os.TempDir(), alloc.ID))
	if err := allocDir.Build([]*structs.Task{task}); err != nil {
		log.Panicf("allocDir.Build() failed: %v", err)
	}

	return task.Name, allocDir
}

func testExecutor(t *testing.T, buildExecutor func() Executor, compatible func(*testing.T)) {
	if compatible != nil {
		compatible(t)
	}

	command := func(name string, args ...string) Executor {
		e := buildExecutor()
		SetCommand(e, name, args)
		e.Command().Env = append(os.Environ(), "TEST_MAIN=app")
		return e
	}

	Executor_Start_Invalid(t, command)
	Executor_Start_Wait_Failure_Code(t, command)
	Executor_Start_Wait(t, command)
	Executor_Start_Kill(t, command)
	Executor_Open(t, command, buildExecutor)
}

type buildExecCommand func(name string, args ...string) Executor

func Executor_Start_Invalid(t *testing.T, command buildExecCommand) {
	invalid := "/bin/foobar"
	e := command(invalid, "1")

	if err := e.Limit(constraint); err != nil {
		log.Panicf("Limit() failed: %v", err)
	}

	task, alloc := mockAllocDir(t)
	defer alloc.Destroy()
	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		log.Panicf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err == nil {
		log.Panicf("Start(%v) should have failed", invalid)
	}
}

func Executor_Start_Wait_Failure_Code(t *testing.T, command buildExecCommand) {
	e := command(os.Args[0], "fail")

	if err := e.Limit(constraint); err != nil {
		log.Panicf("Limit() failed: %v", err)
	}

	task, alloc := mockAllocDir(t)
	defer alloc.Destroy()
	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		log.Panicf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err != nil {
		log.Panicf("Start() failed: %v", err)
	}

	if err := e.Wait(); err == nil {
		log.Panicf("Wait() should have failed")
	}
}

func Executor_Start_Wait(t *testing.T, command buildExecCommand) {
	task, alloc := mockAllocDir(t)
	defer alloc.Destroy()

	taskDir, ok := alloc.TaskDirs[task]
	if !ok {
		log.Panicf("No task directory found for task %v", task)
	}

	expected := "hello world"
	file := filepath.Join(allocdir.TaskLocal, "output.txt")
	absFilePath := filepath.Join(taskDir, file)
	e := command(os.Args[0], "sleep", "1s", "write", expected, file)

	if err := e.Limit(constraint); err != nil {
		log.Panicf("Limit() failed: %v", err)
	}

	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		log.Panicf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err != nil {
		log.Panicf("Start() failed: %v", err)
	}

	if res := e.Wait(); !res.Successful() {
		log.Panicf("Wait() failed: %v", res)
	}

	output, err := ioutil.ReadFile(absFilePath)
	if err != nil {
		log.Panicf("Couldn't read file %v", absFilePath)
	}

	act := string(output)
	if act != expected {
		log.Panicf("Command output incorrectly: want %v; got %v", expected, act)
	}
}

func Executor_Start_Kill(t *testing.T, command buildExecCommand) {
	task, alloc := mockAllocDir(t)
	defer alloc.Destroy()

	taskDir, ok := alloc.TaskDirs[task]
	if !ok {
		log.Panicf("No task directory found for task %v", task)
	}

	filePath := filepath.Join(taskDir, "output")
	e := command(os.Args[0], "sleep", "1s", "write", "failure", filePath)

	if err := e.Limit(constraint); err != nil {
		log.Panicf("Limit() failed: %v", err)
	}

	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		log.Panicf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err != nil {
		log.Panicf("Start() failed: %v", err)
	}

	if err := e.Shutdown(); err != nil {
		log.Panicf("Shutdown() failed: %v", err)
	}

	time.Sleep(1500 * time.Millisecond)

	// Check that the file doesn't exist.
	if _, err := os.Stat(filePath); err == nil {
		log.Panicf("Stat(%v) should have failed: task not killed", filePath)
	}
}

func Executor_Open(t *testing.T, command buildExecCommand, newExecutor func() Executor) {
	task, alloc := mockAllocDir(t)
	defer alloc.Destroy()

	taskDir, ok := alloc.TaskDirs[task]
	if !ok {
		log.Panicf("No task directory found for task %v", task)
	}

	expected := "hello world"
	file := filepath.Join(allocdir.TaskLocal, "output.txt")
	absFilePath := filepath.Join(taskDir, file)
	e := command(os.Args[0], "sleep", "1s", "write", expected, file)

	if err := e.Limit(constraint); err != nil {
		log.Panicf("Limit() failed: %v", err)
	}

	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		log.Panicf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err != nil {
		log.Panicf("Start() failed: %v", err)
	}

	id, err := e.ID()
	if err != nil {
		log.Panicf("ID() failed: %v", err)
	}

	e2 := newExecutor()
	if err := e2.Open(id); err != nil {
		log.Panicf("Open(%v) failed: %v", id, err)
	}

	if res := e2.Wait(); !res.Successful() {
		log.Panicf("Wait() failed: %v", res)
	}

	output, err := ioutil.ReadFile(absFilePath)
	if err != nil {
		log.Panicf("Couldn't read file %v", absFilePath)
	}

	act := string(output)
	if act != expected {
		log.Panicf("Command output incorrectly: want %v; got %v", expected, act)
	}
}

func Executor_Open_Invalid(t *testing.T, command buildExecCommand, newExecutor func() Executor) {
	task, alloc := mockAllocDir(t)
	e := command("echo", "foo")

	if err := e.Limit(constraint); err != nil {
		log.Panicf("Limit() failed: %v", err)
	}

	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		log.Panicf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err != nil {
		log.Panicf("Start() failed: %v", err)
	}

	id, err := e.ID()
	if err != nil {
		log.Panicf("ID() failed: %v", err)
	}

	// Destroy the allocdir which removes the exit code.
	alloc.Destroy()

	e2 := newExecutor()
	if err := e2.Open(id); err == nil {
		log.Panicf("Open(%v) should have failed", id)
	}
}
