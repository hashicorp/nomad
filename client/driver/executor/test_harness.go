package executor

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

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
		b := buildExecutor()
		SetCommand(b, name, args)
		return b
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
	e := command("/bin/date", "-invalid")

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
	cmd := fmt.Sprintf(`"%v \"%v\" > %v"`, "/bin/sleep 1 ; echo -n", expected, file)
	e := command("/bin/bash", "-c", cmd)

	if err := e.Limit(constraint); err != nil {
		log.Panicf("Limit() failed: %v", err)
	}

	if err := e.ConfigureTaskDir(task, alloc); err != nil {
		log.Panicf("ConfigureTaskDir(%v, %v) failed: %v", task, alloc, err)
	}

	if err := e.Start(); err != nil {
		log.Panicf("Start() failed: %v", err)
	}

	if err := e.Wait(); err != nil {
		log.Panicf("Wait() failed: %v", err)
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
	e := command("/bin/bash", "-c", "sleep 1 ; echo \"failure\" > "+filePath)

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
	cmd := fmt.Sprintf(`"%v \"%v\" > %v"`, "/bin/sleep 1 ; echo -n", expected, file)
	e := command("/bin/bash", "-c", cmd)

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

	if err := e2.Wait(); err != nil {
		log.Panicf("Wait() failed: %v", err)
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
