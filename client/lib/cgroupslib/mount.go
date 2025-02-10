// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hashicorp/go-set/v3"
)

// detect tries to detect which cgroups version we have by looking at the mount
// and whether Nomad owns the cgroup.
// - For cgroups v1 this requires root.
// - For cgroups v2 we look for root or whether we're the owner of the slice.
// - All other cases, including any file permission errors, return OFF.
func detect() Mode {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return OFF
	}
	defer func() {
		_ = f.Close()
	}()

	mode := scan(f)

	if mode == CG1 && os.Geteuid() > 0 {
		return OFF
	}

	if mode == CG2 {
		if !functionalCgroups2("cgroup.controllers") {
			return OFF
		}
		uid := os.Geteuid()
		if uid > 0 {
			// allow for cgroup delegation if we own the slice
			cgPath := filepathCG("nomad.slice")
			fi, err := os.Stat(cgPath)
			if err != nil {
				return OFF
			}
			if uid != int(fi.Sys().(*syscall.Stat_t).Uid) {
				return OFF
			}
		}
	}

	return mode
}

func functionalCgroups2(controllersFile string) bool {
	requiredCgroup2Controllers := []string{"cpuset", "cpu", "io", "memory", "pids"}

	controllersRootPath := filepath.Join(root, controllersFile)
	content, err := os.ReadFile(controllersRootPath)
	if err != nil {
		return false
	}

	rootSubtreeControllers := set.From[string](strings.Fields(string(content)))
	return rootSubtreeControllers.ContainsSlice(requiredCgroup2Controllers)
}

func scan(in io.Reader) Mode {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		tokens := set.From(strings.Fields(scanner.Text()))
		if tokens.Contains("/sys/fs/cgroup") {
			if tokens.Contains("tmpfs") {
				return CG1
			}
			if tokens.Contains("cgroup2") {
				return CG2
			}
		}
	}
	return OFF
}
