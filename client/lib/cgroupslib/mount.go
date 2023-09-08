// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/go-set"
)

func detect() Mode {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return OFF
	}
	defer func() {
		_ = f.Close()
	}()
	return scan(f)
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
