// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows
// +build !windows

package host

import (
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// uname returns the syscall like `uname -a`
func uname() string {
	u := &unix.Utsname{}
	err := unix.Uname(u)
	if err != nil {
		return err.Error()
	}

	uname := strings.Join([]string{
		nullStr(u.Machine[:]),
		nullStr(u.Nodename[:]),
		nullStr(u.Release[:]),
		nullStr(u.Sysname[:]),
		nullStr(u.Version[:]),
	}, " ")

	return uname
}

func etcHosts() string {
	return slurp("/etc/hosts")
}

func resolvConf() string {
	return slurp("/etc/resolv.conf")
}

func nullStr(bs []byte) string {
	// find the null byte
	var i int
	var b byte
	for i, b = range bs {
		if b == 0 {
			break
		}
	}

	return string(bs[:i])
}

type df struct {
	s *syscall.Statfs_t
}

func makeDf(path string) (*df, error) {
	var s syscall.Statfs_t
	err := syscall.Statfs(path, &s)
	return &df{s: &s}, err
}

func (d *df) total() uint64 {
	return d.s.Blocks * uint64(d.s.Bsize)
}

func (d *df) available() uint64 {
	return d.s.Bavail * uint64(d.s.Bsize)
}
