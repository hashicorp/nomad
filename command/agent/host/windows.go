// +build windows

package host

import (
	"os"
	"syscall"
	"unsafe"
)

func uname() string {
	return ""
}

func network() string {
	return ""
}

func resolvConf() string {
	return ""
}

func etcHosts() string {
	return ""
}

func mountedPaths() (disks []string) {
	for _, c := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		d := string(c) + ":\\"
		_, err := os.Stat(d)
		if err == nil {
			disks = append(disks, d)
		}
	}
}

type df struct {
	total int64
	avail int64
}

func makeDf(path string) (*df, error) {
	h := syscall.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	df := &df{}

	c.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))),
		uintptr(unsafe.Pointer(&df.total)),
		uintptr(unsafe.Pointer(&df.avail)))

	return df, nil
}

func (d *df) total() uint64 {
	return uint64(d.total)
}

func (d *df) available() uint64 {
	return uint64(d.avail)
}
