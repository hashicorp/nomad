package testutil

import (
	"runtime"
	"syscall"
	"testing"
        "os/exec"
)

func ExecCompatible(t *testing.T) {
	if runtime.GOOS != "windows" && syscall.Geteuid() != 0 {
		t.Skip("Must be root on non-windows environments to run test")
	}
}

func QemuCompatible(t *testing.T) {
	if runtime.GOOS != "windows" && syscall.Geteuid() != 0 {
		t.Skip("Must be root on non-windows environments to run test")
	}
}

func RktCompatible(t *testing.T) bool {
        if runtime.GOOS == "windows" || syscall.Geteuid() != 0 {
                t.Skip("Must be root on non-windows environments to run test")
        }
        // else see if rkt exists
        _, err := exec.Command("rkt", "version").CombinedOutput()
        return err == nil
}

func MountCompatible(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support mount")
	}

	if syscall.Geteuid() != 0 {
		t.Skip("Must be root to run test")
	}
}
