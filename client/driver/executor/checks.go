package executor

import (
	"container/heap"
	"fmt"
	"log"
	"os/exec"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	docker "github.com/fsouza/go-dockerclient"

	"github.com/hashicorp/nomad/client/consul"
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

type consulCheck struct {
	check consul.Check
	next  time.Time
	index int
}

type checkHeap struct {
	index map[string]*consulCheck
	heap  checksHeapImp
}

func NewConsulChecksHeap() *checkHeap {
	return &checkHeap{
		index: make(map[string]*consulCheck),
		heap:  make(checksHeapImp, 0),
	}
}

func (c *checkHeap) Push(check consul.Check, next time.Time) error {
	if _, ok := c.index[check.ID()]; ok {
		return fmt.Errorf("check %v already exists", check.ID())
	}

	cCheck := &consulCheck{check, next, 0}

	c.index[check.ID()] = cCheck
	heap.Push(&c.heap, cCheck)
	return nil
}

func (c *checkHeap) Pop() *consulCheck {
	if len(c.heap) == 0 {
		return nil
	}

	cCheck := heap.Pop(&c.heap).(*consulCheck)
	delete(c.index, cCheck.check.ID())
	return cCheck
}

func (c *checkHeap) Peek() *consulCheck {
	if len(c.heap) == 0 {
		return nil
	}
	return c.heap[0]
}

func (c *checkHeap) Contains(check consul.Check) bool {
	_, ok := c.index[check.ID()]
	return ok
}

func (c *checkHeap) Update(check consul.Check, next time.Time) error {
	if cCheck, ok := c.index[check.ID()]; ok {
		cCheck.check = check
		cCheck.next = next
		heap.Fix(&c.heap, cCheck.index)
		return nil
	}

	return fmt.Errorf("heap doesn't contain check %v", check.ID())
}

func (c *checkHeap) Remove(check consul.Check) error {
	if cCheck, ok := c.index[check.ID()]; ok {
		heap.Remove(&c.heap, cCheck.index)
		delete(c.index, check.ID())
		return nil
	}
	return fmt.Errorf("heap doesn't contain check %v", check.ID())
}

func (c *checkHeap) Len() int { return len(c.heap) }

type checksHeapImp []*consulCheck

func (h checksHeapImp) Len() int { return len(h) }

func (h checksHeapImp) Less(i, j int) bool {
	// Two zero times should return false.
	// Otherwise, zero is "greater" than any other time.
	// (To sort it at the end of the list.)
	// Sort such that zero times are at the end of the list.
	iZero, jZero := h[i].next.IsZero(), h[j].next.IsZero()
	if iZero && jZero {
		return false
	} else if iZero {
		return false
	} else if jZero {
		return true
	}

	return h[i].next.Before(h[j].next)
}

func (h checksHeapImp) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *checksHeapImp) Push(x interface{}) {
	n := len(*h)
	check := x.(*consulCheck)
	check.index = n
	*h = append(*h, check)
}

func (h *checksHeapImp) Pop() interface{} {
	old := *h
	n := len(old)
	check := old[n-1]
	check.index = -1 // for safety
	*h = old[0 : n-1]
	return check
}
