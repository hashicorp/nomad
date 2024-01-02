// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testutil

import (
	"os/exec"
	"runtime"
	"syscall"
	"testing"
)

// RequireRoot skips tests unless:
// - running as root
func RequireRoot(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("Test requires root")
	}
}

// RequireNonRoot skips tests unless:
// - running as non-root
func RequireNonRoot(t *testing.T) {
	if syscall.Geteuid() == 0 {
		t.Skip("Test requires non-root")
	}
}

// RequireConsul skips tests unless:
// - "consul" executable is detected on $PATH
func RequireConsul(t *testing.T) {
	_, err := exec.Command("consul", "version").CombinedOutput()
	if err != nil {
		t.Skipf("Test requires Consul: %v", err)
	}
}

// RequireVault skips tests unless:
// - "vault" executable is detected on $PATH
func RequireVault(t *testing.T) {
	_, err := exec.Command("vault", "version").CombinedOutput()
	if err != nil {
		t.Skipf("Test requires Vault: %v", err)
	}
}

// RequireLinux skips tests unless:
// - running on Linux
func RequireLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test requires Linux")
	}
}

// ExecCompatible skips tests unless:
// - running as root
// - running on Linux
func ExecCompatible(t *testing.T) {
	if runtime.GOOS != "linux" || syscall.Geteuid() != 0 {
		t.Skip("Test requires root on Linux")
	}
}

// JavaCompatible skips tests unless:
// - "java" executable is detected on $PATH
// - running as root
// - running on Linux
func JavaCompatible(t *testing.T) {
	_, err := exec.Command("java", "-version").CombinedOutput()
	if err != nil {
		t.Skipf("Test requires Java: %v", err)
	}

	if runtime.GOOS != "linux" || syscall.Geteuid() != 0 {
		t.Skip("Test requires root on Linux")
	}
}

// QemuCompatible skips tests unless:
// - "qemu-system-x86_64" executable is detected on $PATH (!windows)
// - "qemu-img" executable is detected on on $PATH (windows)
func QemuCompatible(t *testing.T) {
	// Check if qemu exists
	bin := "qemu-system-x86_64"
	if runtime.GOOS == "windows" {
		bin = "qemu-img"
	}
	_, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Skipf("Test requires QEMU (%s)", bin)
	}
}

// MountCompatible skips tests unless:
// - not running as windows
// - running as root
func MountCompatible(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Test requires not using Windows")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("Test requires root")
	}
}

// MinimumCores skips tests unless:
// - system has at least cores available CPU cores
func MinimumCores(t *testing.T, cores int) {
	available := runtime.NumCPU()
	if available < cores {
		t.Skipf("Test requires at least %d cores, only %d available", cores, available)
	}
}
