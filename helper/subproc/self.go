// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package subproc

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	// executable is the executable of this process
	executable string
)

func init() {
	s, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("failed to detect executable: %v", err))
	}

	// when running tests, we need to use the real nomad binary,
	// and make sure you recompile between changes!
	if strings.HasSuffix(s, ".test") {
		if s, err = exec.LookPath("nomad"); err != nil {
			panic(fmt.Sprintf("failed to find nomad binary: %v", err))
		}
	}
	executable = s
}

// Self returns the path to the executable of this process.
func Self() string {
	return executable
}
