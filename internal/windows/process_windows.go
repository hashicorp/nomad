package windows

import (
	"errors"
	"log"
	"os"
	"syscall"
)

func TerminateProcess(pid int) error {
	log.Printf("Terminating Process: %d", pid)
	h, e := syscall.OpenProcess(syscall.PROCESS_TERMINATE, false, uint32(pid))
	if e != nil {
		return os.NewSyscallError("OpenProcess", e)
	}
	if h <= 0 {
		return errors.New("OpenProcess: returned non existent pid")
	}
	defer syscall.CloseHandle(h)
	e = syscall.TerminateProcess(h, uint32(1))
	return os.NewSyscallError("TerminateProcess", e)
}
