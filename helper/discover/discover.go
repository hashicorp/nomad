// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discover

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// NomadExecutable checks the current executable, then $GOPATH/bin, and finally
// the CWD, in that order. If it can't be found, an error is returned.
func NomadExecutable() (string, error) {
	nomadExe := "nomad"
	if runtime.GOOS == "windows" {
		nomadExe = "nomad.exe"
	}

	// Check the current executable.
	bin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("Failed to determine the nomad executable: %v", err)
	}

	if _, err := os.Stat(bin); err == nil && isNomad(bin, nomadExe) {
		return bin, nil
	}

	// Check the $PATH
	if bin, err := exec.LookPath(nomadExe); err == nil {
		return bin, nil
	}

	// Check the $GOPATH.
	bin = filepath.Join(os.Getenv("GOPATH"), "bin", nomadExe)
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}

	// Check the CWD.
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Could not find Nomad executable (%v): %v", nomadExe, err)
	}

	bin = filepath.Join(pwd, nomadExe)
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}

	// Check CWD/bin
	bin = filepath.Join(pwd, "bin", nomadExe)
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}

	return "", fmt.Errorf("Could not find Nomad executable (%v)", nomadExe)
}

func isNomad(path, nomadExe string) bool {
	if strings.HasSuffix(path, ".test") || strings.HasSuffix(path, ".test.exe") {
		return false
	}
	return true
}
