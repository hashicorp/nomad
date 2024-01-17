// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type Exit struct {
	Code      int
	Interrupt int
	Err       error
}

type Waiter interface {
	Wait() *Exit
}

func WaitOnChild(p *os.Process) Waiter {
	return &execWaiter{p: p}
}

type execWaiter struct {
	p *os.Process
}

func (w *execWaiter) Wait() *Exit {
	ps, err := w.p.Wait()
	status := ps.Sys().(syscall.WaitStatus)
	code := ps.ExitCode()
	if code < 0 {
		// just be cool
		code = int(status) + 128
	}
	return &Exit{
		Code:      code,
		Interrupt: int(status),
		Err:       err,
	}
}

// WaitOnOrphan provides a last-ditch effort to wait() on a process that the plugin
// must reattach to. This should never happen, because a Nomad Client restart should
// not trigger task driver plugin restarts - but we implement this ability just in
// case ~ kludgy as it may be.
func WaitOnOrphan(pid int) Waiter {
	return &pidWaiter{pid: pid}
}

type pidWaiter struct {
	pid int
}

func (w *pidWaiter) Wait() *Exit {
	fd, err := openFD(w.pid)
	if err != nil {
		return &Exit{
			Code: 255,
			Err:  err,
		}
	}

	pollFD := []unix.PollFd{{Fd: fd}}
	timeout := -1 // infinite

	// we should check the return is what we expect
	_, _ = unix.Poll(pollFD, timeout)

	// lookup exit code from /proc/<pid>/stat ?
	code, err := codeFromStat(w.pid)
	return &Exit{
		Code: code,
		Err:  err,
	}
}

// codeFromStat reads the exit code of pid from /proc/<pid>/stat
//
// See `man proc`.
// (52) exit_code  %d  (since Linux 3.5)
func codeFromStat(pid int) (int, error) {
	f := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	b, err := os.ReadFile(f)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(b))
	if len(fields) < 52 {
		return 0, fmt.Errorf("failed to read exit code from %q", f)
	}
	code, err := strconv.Atoi(fields[51])
	if err != nil {
		return 0, fmt.Errorf("failed to parse exit code from %q", f)
	}
	if code > 255 {
		// not sure why, read about waitpid
		code -= 255
	}
	return code, nil
}

func openFD(pid int) (int32, error) {
	const syscallNumber = 434
	fd, _, e := syscall.Syscall(syscallNumber, uintptr(pid), uintptr(0), 0)
	if e != 0 {
		return 0, e
	}
	return int32(fd), nil
}
