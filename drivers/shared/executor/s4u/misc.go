//go:build windows
// +build windows

package s4u

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modadvapi32 = windows.NewLazySystemDLL("advapi32.dll")

	procAllocateLocallyUniqueId = modadvapi32.NewProc("AllocateLocallyUniqueId")
)

func AllocateLocallyUniqueId(result *windows.LUID) error {
	r1, _, e1 := syscall.SyscallN(procAllocateLocallyUniqueId.Addr(), uintptr(unsafe.Pointer(result)))
	if r1 == 0 {
		return e1
	}
	return nil
}
