// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package ci

import (
	"os"
	"strconv"
	"syscall"
	"testing"
)

// SkipTestWithoutRootAccess will skip test t if it's not running in CI environment
// and test is not running with Root access.
func SkipTestWithoutRootAccess(t *testing.T) {
	ciVar := os.Getenv("CI")
	isCI, err := strconv.ParseBool(ciVar)
	isCI = isCI && err == nil

	if !isCI && syscall.Getuid() != 0 {
		t.Skipf("Skipping test %s. To run this test, you should run it as root user", t.Name())
	}
}
