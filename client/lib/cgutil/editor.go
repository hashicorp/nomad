// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-set"
)

// editor provides a simple mechanism for reading and writing cgroup files.
type editor struct {
	dpath string
}

func (e *editor) path(file string) string {
	return filepath.Join(CgroupRoot, e.dpath, file)
}

func (e *editor) write(file, content string) error {
	return os.WriteFile(e.path(file), []byte(content), 0o644)
}

func (e *editor) Write(filename, content string) error {
	return e.write(filename, content)
}

func (e *editor) read(file string) (string, error) {
	b, err := os.ReadFile(e.path(file))
	return strings.TrimSpace(string(b)), err
}

func (e *editor) Read(filename string) (string, error) {
	return e.read(filename)
}

// OpenPath creates a handle for modifying cgroup interface files under
// the given directory.
//
// In cgroups v1 this will be like, "<root>/<interface>/<parent>/<scope>".
// In cgroups v2 this will be like, "<root>/<parent>/<scope>".
func OpenPath(dir string) Interface {
	return &editor{
		dpath: dir,
	}
}

// OpenFromFreezerCG1 creates a handle for modifying cgroup interface files
// of the given interface, given a path to the freezer cgroup.
func OpenFromFreezerCG1(orig, iface string) Interface {
	if iface == "cpuset" {
		panic("cannot open cpuset")
	}
	p := strings.Replace(orig, "/freezer/", "/"+iface+"/", 1)
	return OpenPath(p)
}

// An Interface can be used to read and write the interface files of a cgroup.
type Interface interface {
	// Read the content of filename.
	Read(filename string) (string, error)

	// Write content to filename.
	Write(filename, content string) error

	// PIDs returns the set of process IDs listed in the cgroup.procs
	// interface file. We use a set here because the kernel recommends doing
	// so.
	//
	//   This list is not guaranteed to be sorted or free of duplicate TGIDs,
	//   and userspace should sort/uniquify the list if this property is required.
	//
	// https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v1/cgroups.html
	PIDs() (*set.Set[int], error)
}

func (e *editor) PIDs() (*set.Set[int], error) {
	path := filepath.Join(CgroupRoot, e.dpath, "cgroup.procs")
	return getPIDs(path)
}

func getPIDs(file string) (*set.Set[int], error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	tokens := bytes.Fields(bytes.TrimSpace(b))
	result := set.New[int](len(tokens))
	for _, token := range tokens {
		if i, err := strconv.Atoi(string(token)); err == nil {
			result.Insert(i)
		}
	}
	return result, nil
}
