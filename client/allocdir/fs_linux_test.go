// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocdir

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"golang.org/x/sys/unix"
)

var notFoundErr = fmt.Errorf("not found")

func isMount(path string) error {
	file, err := os.Open("/proc/self/mounts")
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64*1024)
	const max = 100000
	for i := 0; i < max; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return notFoundErr
			}
			return err
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 {
			return fmt.Errorf("unexpected line: %q", line)
		}
		if parts[1] == path {
			// Found it! Make sure it's a tmpfs
			if parts[0] != "tmpfs" {
				return fmt.Errorf("unexpected fs: %q", parts[1])
			}
			return nil
		}
	}
	return fmt.Errorf("exceeded max mount entries (%d)", max)
}

// TestLinuxRootSecretDir asserts secret dir creation and removal are
// idempotent.
func TestLinuxRootSecretDir(t *testing.T) {
	ci.Parallel(t)
	if unix.Geteuid() != 0 {
		t.Skip("Must be run as root")
	}

	secretsDir := filepath.Join(t.TempDir(), TaskSecrets)

	// removing a nonexistent secrets dir should NOT error
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}
	// run twice as it should be idempotent
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}

	// creating a secrets dir should work
	if err := createSecretDir(secretsDir); err != nil {
		t.Fatalf("error creating secrets dir %q: %v", secretsDir, err)
	}
	// creating it again should be a noop (NO error)
	if err := createSecretDir(secretsDir); err != nil {
		t.Fatalf("error creating secrets dir %q: %v", secretsDir, err)
	}

	// ensure it exists and is a directory
	fi, err := os.Lstat(secretsDir)
	if err != nil {
		t.Fatalf("error stat'ing secrets dir %q: %v", secretsDir, err)
	}
	if !fi.IsDir() {
		t.Fatalf("secrets dir %q is not a directory and should be", secretsDir)
	}
	if err := isMount(secretsDir); err != nil {
		t.Fatalf("secrets dir %q is not a mount: %v", secretsDir, err)
	}

	// now remove it
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing secrets dir %q: %v", secretsDir, err)
	}

	// make sure it's gone
	if err := isMount(secretsDir); err != notFoundErr {
		t.Fatalf("error ensuring secrets dir %q isn't mounted: %v", secretsDir, err)
	}

	// removing again should be a noop
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}
}

// TestLinuxUnprivilegedSecretDir asserts secret dir creation and removal are
// idempotent.
func TestLinuxUnprivilegedSecretDir(t *testing.T) {
	ci.Parallel(t)
	if unix.Geteuid() == 0 {
		t.Skip("Must not be run as root")
	}

	secretsDir := filepath.Join(t.TempDir(), TaskSecrets)

	// removing a nonexistent secrets dir should NOT error
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}
	// run twice as it should be idempotent
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}

	// creating a secrets dir should work
	if err := createSecretDir(secretsDir); err != nil {
		t.Fatalf("error creating secrets dir %q: %v", secretsDir, err)
	}
	// creating it again should be a noop (NO error)
	if err := createSecretDir(secretsDir); err != nil {
		t.Fatalf("error creating secrets dir %q: %v", secretsDir, err)
	}

	// ensure it exists and is a directory
	fi, err := os.Lstat(secretsDir)
	if err != nil {
		t.Fatalf("error stat'ing secrets dir %q: %v", secretsDir, err)
	}
	if !fi.IsDir() {
		t.Fatalf("secrets dir %q is not a directory and should be", secretsDir)
	}
	if err := isMount(secretsDir); err != notFoundErr {
		t.Fatalf("error ensuring secrets dir %q isn't mounted: %v", secretsDir, err)
	}

	// now remove it
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing secrets dir %q: %v", secretsDir, err)
	}

	// make sure it's gone
	if _, err := os.Lstat(secretsDir); err == nil {
		t.Fatalf("expected secrets dir %q to be gone but it was found", secretsDir)
	}

	// removing again should be a noop
	if err := removeSecretDir(secretsDir); err != nil {
		t.Fatalf("error removing nonexistent secrets dir %q: %v", secretsDir, err)
	}
}
