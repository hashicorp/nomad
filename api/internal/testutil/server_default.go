// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
