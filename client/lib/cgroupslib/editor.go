// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-set/v3"
	"golang.org/x/sys/unix"
)

const (
	root = "/sys/fs/cgroup"
)

func GetDefaultRoot() string {
	return root
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

type editor struct {
	dpath string
}

func (e *editor) Read(filename string) (string, error) {
	path := filepath.Join(e.dpath, filename)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(b)), nil
}

func (e *editor) PIDs() (*set.Set[int], error) {
	path := filepath.Join(e.dpath, "cgroup.procs")
	return getPIDs(path)
}

func (e *editor) Write(filename, content string) error {
	path := filepath.Join(e.dpath, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

// A Factory creates a Lifecycle which is an abstraction over the setup and
// teardown routines used for creating and destroying cgroups used for
// constraining Nomad tasks.
func Factory(allocID, task string, cores bool) Lifecycle {
	switch GetMode() {
	case CG1:
		return &lifeCG1{
			allocID:       allocID,
			task:          task,
			reservedCores: cores,
		}
	default:
		return &lifeCG2{
			dpath: pathCG2(allocID, task, cores),
		}
	}
}

// A Lifecycle manages the lifecycle of the cgroup(s) of a task from the
// perspective of the Nomad client. That is, it creates and deletes the cgroups
// for a task, as well as provides last effort kill semantics for ensuring a
// process cannot stay alive beyond the intent of the client.
type Lifecycle interface {
	Setup() error
	Kill() error
	Teardown() error
}

// -------- cgroups v1 ---------

type lifeCG1 struct {
	allocID       string
	task          string
	reservedCores bool // uses core reservation
}

func (l *lifeCG1) Setup() error {
	paths := l.paths()
	for _, p := range paths {
		err := os.MkdirAll(p, 0755)
		if err != nil {
			return err
		}
		if strings.Contains(p, "/reserve/") {
			if err = l.inheritMems(p); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *lifeCG1) inheritMems(destination string) error {
	parent := filepath.Join(filepath.Dir(destination), "cpuset.mems")
	b, err := os.ReadFile(parent)
	if err != nil {
		return err
	}
	destination = filepath.Join(destination, "cpuset.mems")
	return os.WriteFile(destination, b, 0644)
}

func (l *lifeCG1) Teardown() error {
	paths := l.paths()
	for _, p := range paths {
		if filepath.Base(p) == "share" {
			// avoid removing the share cgroup
			continue
		}
		err := os.RemoveAll(p)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *lifeCG1) Kill() error {
	if err := l.freeze(); err != nil {
		return err
	}

	pids, err := l.pids()
	if err != nil {
		return err
	}

	signal := unix.SignalNum("SIGKILL")
	for pid := range pids.Items() {
		_ = unix.Kill(pid, signal)
	}

	return l.thaw()
}

func (l *lifeCG1) edit(iface string) *editor {
	scope := ScopeCG1(l.allocID, l.task)
	return &editor{
		dpath: filepath.Join(root, iface, NomadCgroupParent, scope),
	}
}

func (l *lifeCG1) freeze() error {
	ed := l.edit("freezer")
	return ed.Write("freezer.state", "FROZEN")
}

func (l *lifeCG1) pids() (*set.Set[int], error) {
	ed := l.edit("freezer")
	return ed.PIDs()
}

func (l *lifeCG1) thaw() error {
	ed := l.edit("freezer")
	return ed.Write("freezer.state", "THAWED")
}

func (l *lifeCG1) paths() []string {
	scope := ScopeCG1(l.allocID, l.task)
	ifaces := []string{"freezer", "cpu", "memory"}
	paths := make([]string, 0, len(ifaces)+1)
	for _, iface := range ifaces {
		paths = append(paths, filepath.Join(
			root, iface, NomadCgroupParent, scope,
		))
	}

	switch partition := GetPartitionFromBool(l.reservedCores); partition {
	case "reserve":
		paths = append(paths, filepath.Join(root, "cpuset", NomadCgroupParent, partition, scope))
	case "share":
		paths = append(paths, filepath.Join(root, "cpuset", NomadCgroupParent, partition))
	}

	return paths
}

// -------- cgroups v2 --------

type lifeCG2 struct {
	dpath string
}

func (l *lifeCG2) edit() *editor {
	return &editor{dpath: l.dpath}
}

func (l *lifeCG2) Setup() error {
	return os.MkdirAll(l.dpath, 0755)
}

func (l *lifeCG2) Teardown() error {
	return os.RemoveAll(l.dpath)
}

func (l *lifeCG2) Kill() error {
	ed := l.edit()
	return ed.Write("cgroup.kill", "1")
}

// -------- helpers ---------

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

func ScopeCG1(allocID, task string) string {
	return fmt.Sprintf("%s.%s", allocID, task)
}

func scopeCG2(allocID, task string) string {
	return fmt.Sprintf("%s.%s.scope", allocID, task)
}

func pathCG2(allocID, task string, cores bool) string {
	partition := GetPartitionFromBool(cores)
	return filepath.Join(root, NomadCgroupParent, partition, scopeCG2(allocID, task))
}
