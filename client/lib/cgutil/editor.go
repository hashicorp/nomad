// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package cgutil

import (
	"os"
	"path/filepath"
	"strings"
)

// editor provides a simple mechanism for reading and writing cgroup files.
type editor struct {
	fromRoot string
}

func (e *editor) path(file string) string {
	return filepath.Join(CgroupRoot, e.fromRoot, file)
}

func (e *editor) write(file, content string) error {
	return os.WriteFile(e.path(file), []byte(content), 0o644)
}

func (e *editor) read(file string) (string, error) {
	b, err := os.ReadFile(e.path(file))
	return strings.TrimSpace(string(b)), err
}
