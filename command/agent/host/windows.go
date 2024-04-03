// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows
// +build windows

package host

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
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
	size       uint64 // "systemFree" less quotas
	avail      uint64
	systemFree uint64
}

func makeDf(path string) (*df, error) {
	df := &df{}
	err := windows.GetDiskFreeSpaceEx(
		syscall.StringToUTF16Ptr(path),
		&df.avail, &df.size, &df.systemFree)

	return df, err
}

func (d *df) total() uint64 {
	return d.size
}

func (d *df) available() uint64 {
	return d.avail
}
