// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package testutil

import (
	"fmt"
	"syscall"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procSetCtrlHandler = kernel32.NewProc("SetConsoleCtrlHandler")
	procGenCtrlEvent   = kernel32.NewProc("GenerateConsoleCtrlEvent")
)

// gracefulStop performs a platform-specific graceful stop. On Windows the Go
// API does not implement SIGINT even though it's supported on Windows via
// CTRL_C_EVENT
func (s *TestServer) gracefulStop() error {
	// note: err is always non-nil from these proc Call methods because it's
	// always populated from GetLastError and you need to check the result
	// returned against the docs.
	pid := s.cmd.Process.Pid
	result, _, err := procSetCtrlHandler.Call(0, 1)
	if result == 0 {
		return fmt.Errorf("failed to modify handlers for ctrl-c on pid %d: %w", pid, err)
	}

	result, _, err = procGenCtrlEvent.Call(syscall.CTRL_C_EVENT, uintptr(pid))
	if result == 0 {
		return fmt.Errorf("failed to send ctrl-C event to pid %d: %w", pid, err)
	}
	return nil
}
