// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows

package testutil

import (
	"fmt"
	"syscall"
)

// gracefulStop performs a platform-specific graceful stop. On Windows the Go
// API does not implement SIGINT even though it's supported on Windows via
// CTRL_C_EVENT
func (s *TestServer) gracefulStop() error {
	kernel32, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return fmt.Errorf("could not load kernel dll: %w", err)
	}
	proc, err := kernel32.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return fmt.Errorf("could not load GenerateConsoleCtrlEvent procedure: %w", err)
	}
	pid := s.cmd.Process.Pid
	result, _, err := proc.Call(syscall.CTRL_C_EVENT, uintptr(pid))
	if result == 0 {
		// note: err is always non-nil because Call always populates it from
		// GetLastError and you need to check the result returned against the
		// docs. from
		// https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent
		// "If the function fails, the return value is zero. To get extended
		// error information, call GetLastError."
		return fmt.Errorf("failed to send ctrl-C event to pid %d: %w", pid, err)
	}

	return nil
}
