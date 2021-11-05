package testutil

import (
	"os/exec"
	"runtime"
	"syscall"
	"testing"

	"github.com/hashicorp/nomad/client/lib/cgutil"
)

// RequireRoot skips tests unless running on a Unix as root.
func RequireRoot(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("Must run as root on Unix")
	}
}

// RequireConsul skips tests unless a Consul binary is available on $PATH.
func RequireConsul(t *testing.T) {
	_, err := exec.Command("consul", "version").CombinedOutput()
	if err != nil {
		t.Skipf("Test requires Consul: %v", err)
	}
}

// RequireVault skips tests unless a Vault binary is available on $PATH.
func RequireVault(t *testing.T) {
	_, err := exec.Command("vault", "version").CombinedOutput()
	if err != nil {
		t.Skipf("Test requires Vault: %v", err)
	}
}

func ExecCompatible(t *testing.T) {
	if runtime.GOOS != "linux" || syscall.Geteuid() != 0 {
		t.Skip("Test only available running as root on linux")
	}
	CgroupCompatible(t)
}

func JavaCompatible(t *testing.T) {
	if runtime.GOOS == "linux" && syscall.Geteuid() != 0 {
		t.Skip("Test only available when running as root on linux")
	}
}

func QemuCompatible(t *testing.T) {
	// Check if qemu exists
	bin := "qemu-system-x86_64"
	if runtime.GOOS == "windows" {
		bin = "qemu-img"
	}
	_, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Skip("Must have Qemu installed for Qemu specific tests to run")
	}
}

func CgroupCompatible(t *testing.T) {
	mount, err := cgutil.FindCgroupMountpointDir()
	if err != nil || mount == "" {
		t.Skipf("Failed to find cgroup mount: %v %v", mount, err)
	}
}

func MountCompatible(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support mount")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("Must be root to run test")
	}
}
