package executor

import (
	"fmt"
	"log"
	"os/exec"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	docker "github.com/fsouza/go-dockerclient"

	cstructs "github.com/hashicorp/nomad/client/driver/structs"
)

type DockerScriptCheck struct {
	id          string
	containerID string
	client      *docker.Client
	logger      *log.Logger
	cmd         string
	args        []string
}

func (d *DockerScriptCheck) Run() *cstructs.CheckResult {
	execOpts := docker.CreateExecOptions{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          append([]string{d.cmd}, d.args...),
		Container:    d.containerID,
	}
	var (
		exec    *docker.Exec
		err     error
		execRes *docker.ExecInspect
		time    = time.Now()
	)
	if exec, err = d.client.CreateExec(execOpts); err != nil {
		return &cstructs.CheckResult{Err: err}
	}

	output, _ := circbuf.NewBuffer(int64(cstructs.CheckBufSize))
	startOpts := docker.StartExecOptions{
		Detach:       false,
		Tty:          false,
		OutputStream: output,
		ErrorStream:  output,
	}

	if err = d.client.StartExec(exec.ID, startOpts); err != nil {
		return &cstructs.CheckResult{Err: err}
	}
	if execRes, err = d.client.InspectExec(exec.ID); err != nil {
		return &cstructs.CheckResult{Err: err}
	}
	return &cstructs.CheckResult{
		ExitCode:  execRes.ExitCode,
		Output:    string(output.Bytes()),
		Timestamp: time,
	}
}

func (d *DockerScriptCheck) ID() string {
	return d.id
}

type ExecScriptCheck struct {
	id      string
	cmd     string
	args    []string
	taskDir string

	FSIsolation bool
}

func (e *ExecScriptCheck) Run() *cstructs.CheckResult {
	buf, _ := circbuf.NewBuffer(int64(cstructs.CheckBufSize))
	cmd := exec.Command(e.cmd, e.args...)
	cmd.Stdout = buf
	cmd.Stderr = buf
	e.setChroot(cmd)
	ts := time.Now()
	if err := cmd.Start(); err != nil {
		return &cstructs.CheckResult{Err: err}
	}
	errCh := make(chan error, 2)
	go func() {
		errCh <- cmd.Wait()
	}()
	for {
		select {
		case err := <-errCh:
			if err == nil {
				return &cstructs.CheckResult{ExitCode: 0, Output: string(buf.Bytes()), Timestamp: ts}
			}
			exitCode := 1
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					exitCode = status.ExitStatus()
				}
			}
			return &cstructs.CheckResult{ExitCode: exitCode, Output: string(buf.Bytes()), Timestamp: ts}
		case <-time.After(30 * time.Second):
			errCh <- fmt.Errorf("timed out after waiting 30s")
		}
	}
	return nil
}

func (e *ExecScriptCheck) ID() string {
	return e.id
}
