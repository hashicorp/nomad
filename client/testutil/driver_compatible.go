package testutil

import (
	"runtime"
	"syscall"
	"testing"
)

func ExecCompatible(t *testing.T) {
	if runtime.GOOS != "windows" && syscall.Geteuid() != 0 {
		t.Skip("Must be root on non-windows environments to run test")
	}
}
