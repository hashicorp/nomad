// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package process

import (
	"strings"
	"syscall"
)

type Signaler interface {
	Signal(s string) error
}

func parse(s string) syscall.Signal {
	switch strings.ToLower(s) {
	case "sighup":
		return syscall.SIGHUP
	case "sigint":
		return syscall.SIGINT
	case "sigquit":
		return syscall.SIGQUIT
	case "sigtrap":
		return syscall.SIGTRAP
	case "sigabrt":
		return syscall.SIGABRT
	case "sigkill":
		return syscall.SIGKILL
	case "sigusr1":
		return syscall.SIGUSR1
	case "sigusr2":
		return syscall.SIGUSR2
	case "sigalrm":
		return syscall.SIGALRM
	case "sigterm":
		return syscall.SIGTERM
	case "sigstop":
		return syscall.SIGSTOP
	case "sigpwr":
		return syscall.SIGPWR
	default:
		return syscall.SIGSTOP
	}
}

func Interrupts(pid int) Signaler {
	return &sysSignal{pid: pid}
}

type sysSignal struct {
	pid int
}

func (sig *sysSignal) Signal(signal string) error {
	s := parse(signal)
	return syscall.Kill(-sig.pid, s)
}
