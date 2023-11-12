// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows
// +build !windows

package host

import (
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
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
	usage *disk.UsageStat
}

func makeDf(path string) (*df, error) {
	usage, err := disk.Usage(path)
	return &df{usage: usage}, err
}

func (d *df) total() uint64 {
	return d.usage.Total
}

func (d *df) available() uint64 {
	return d.usage.Free
}

// mountedPaths produces a list of mounts
func mountedPaths() []string {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return []string{err.Error()}
	}

	var paths []string
	for _, partition := range partitions {
		fsType := partition.Fstype

		switch fsType {
		case "autofs", "binfmt_misc", "cgroup", "debugfs",
			"devpts", "devtmpfs",
			"fusectl", "fuse.lxcfs",
			"hugetlbfs", "mqueue",
			"procfs", "pstore", "rpc_pipefs", "securityfs",
			"sysfs", "tmpfs", "vboxsf", "ptyfs":
			continue
		default:
		}

		paths = append(paths, partition.Mountpoint)
	}

	return paths
}
