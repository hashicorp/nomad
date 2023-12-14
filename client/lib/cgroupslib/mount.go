// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"bufio"
	"io"
	"os"
	"slices"
	"strings"
	"path/filepath"

	"github.com/hashicorp/go-set/v2"
)


func detect() Mode {
	if os.Geteuid() > 0 {
		return OFF
	}

	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return OFF
	}
	defer func() {
		_ = f.Close()
	}()

	mode := scan(f)
	if mode == CG2 && !functionalCgroups2() {
		return OFF
	}
	return mode
}

func functionalCgroups2() bool {
	const controllersFile = "cgroup.controllers"
	requiredCgroup2Controllers := []string{"cpuset", "cpu", "io", "memory", "pids"}

	controllersRootPath := filepath.Join(root, controllersFile)
	content, err := os.ReadFile(controllersRootPath)
	if err != nil {
		return false
	}
	rootSubtreeControllers := strings.Split(strings.TrimSpace(string(content)), " ")

	for _, controller := range requiredCgroup2Controllers {
		if !slices.Contains(rootSubtreeControllers, controller) {
			return false
		}
	}
	return true
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
