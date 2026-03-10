// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package testutil

import (
	"os"
)

// gracefulStop performs a platform-specific graceful stop. On non-Windows this
// uses the Go API for SIGINT
func (s *TestServer) gracefulStop() error {
	err := s.cmd.Process.Signal(os.Interrupt)
	return err
}
