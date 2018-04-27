package testutil

import (
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"testing"
)

// RequireRoot skips tests unless running on a Unix as root.
func RequireRoot(t *testing.T) {
	if syscall.Geteuid() != 0 {
		t.Skip("Must run as root on Unix")
	}
}

func ExecCompatible(t *testing.T) {
	if runtime.GOOS != "linux" || syscall.Geteuid() != 0 {
		t.Skip("Test only available running as root on linux")
	}
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

var rktExists bool
var rktOnce sync.Once

func RktCompatible(t *testing.T) {
	if runtime.GOOS != "linux" || syscall.Geteuid() != 0 {
		t.Skip("Must be root on Linux to run test")
	}

	// else see if rkt exists
	rktOnce.Do(func() {
		_, err := exec.Command("rkt", "version").CombinedOutput()
		if err == nil {
			rktExists = true
		}
	})

	if !rktExists {
		t.Skip("Must have rkt installed for rkt specific tests to run")
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
