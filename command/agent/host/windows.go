// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows
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
	return disks
}

type df struct {
	size  int64
	avail int64
}

func makeDf(path string) (*df, error) {
	h, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return nil, err
	}

	c, err := h.FindProc("GetDiskFreeSpaceExW")
	if err != nil {
		return nil, err
	}

	df := &df{}

	c.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))),
		uintptr(unsafe.Pointer(&df.size)),
		uintptr(unsafe.Pointer(&df.avail)))

	return df, nil
}

func (d *df) total() uint64 {
	return uint64(d.size)
}

func (d *df) available() uint64 {
	return uint64(d.avail)
}
