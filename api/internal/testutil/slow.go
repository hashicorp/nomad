// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"os"
	"strconv"
	"syscall"
	"testing"
)

// Copy of ci/slow.go for API.

// SkipSlow skips a slow test unless NOMAD_SLOW_TEST is set to a true value.
func SkipSlow(t *testing.T, reason string) {
	value := os.Getenv("NOMAD_SLOW_TEST")
	run, err := strconv.ParseBool(value)
	if !run || err != nil {
		t.Skipf("Skipping slow test: %s", reason)
	}
}

// Parallel runs t in parallel.
//
// The API package has been vetted to be concurrency safe (ish).
func Parallel(t *testing.T) {
	t.Parallel() // :)
}

func RequireRoot(t *testing.T) {
	t.Helper()
	if syscall.Getuid() != 0 {
		t.Skip("test requires root")
	}
}
