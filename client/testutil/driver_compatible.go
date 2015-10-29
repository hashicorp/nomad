package testutil

import (
	"os/exec"
	"runtime"
	"syscall"
	"testing"
)

func ExecCompatible(t *testing.T) {
	if runtime.GOOS != "linux" || syscall.Geteuid() != 0 {
		t.Skip("Test only available running as root on linux")
	}
}

func QemuCompatible(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Must be on non-windows environments to run test")
	}
	// else see if qemu exists
	_, err := exec.Command("qemu-system-x86_64", "-version").CombinedOutput()
	if err != nil {
		t.Skip("Must have Qemu installed for Qemu specific tests to run")
	}
}

func RktCompatible(t *testing.T) {
	if runtime.GOOS == "windows" || syscall.Geteuid() != 0 {
		t.Skip("Must be root on non-windows environments to run test")
	}
	// else see if rkt exists
	_, err := exec.Command("rkt", "version").CombinedOutput()
	if err != nil {
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
