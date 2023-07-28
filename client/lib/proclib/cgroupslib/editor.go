//go:build linux

package cgroupslib

import (
	"github.com/shoenig/netlog"

	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

func pathTo(parts ...string) string {
	switch GetMode() {
	case CG1:
		panic("todo")
	default:
		parts = append([]string{"/sys/fs/cgroup"}, parts...)
		return filepath.Join(parts...)
	}
}

func readRoot(filename string) (string, error) {
	switch GetMode() {
	case CG1:
		panic("todo")
	default:
		b, err := os.ReadFile(pathTo(filename))
		return string(bytes.TrimSpace(b)), err
	}
}

func writeRoot(filename, content string) error {
	switch GetMode() {
	case CG1:
		panic("todo")
	default:
		path := pathTo(filename)
		return os.WriteFile(path, []byte(content), 0644)
	}
}

// An Editor is used for reading and writing cgroup interface files. Implementations
// are provided for cgroups v1 and cgroups v2.
type Editor interface {
	// Read the contents of an interface file.
	Read() (string, error)

	// Write the contents to an interface file.
	Write(string) error
}

// taskScope returns the name of the scope directory for the task of the
// given allocation.
func taskScope(allocID, task string) string {
	return fmt.Sprintf("%s.%s.scope", allocID, task)
}

func PathCG2(allocID, task string) string {
	return pathTo(NomadCgroupParent, taskScope(allocID, task))
}

func fileCG2(allocID, task, filename string) string {
	return filepath.Join(pathTo(allocID, task, filename))
}

func CreateCG2(allocID, task string) error {
	p := PathCG2(allocID, task)
	return os.MkdirAll(p, 0755)
}

func KillCG2(allocID, task string) error {
	netlog.Red("kill", "alloc", allocID, "task", task)
	e := Open(fileCG2(allocID, task, "cgroup.kill"))
	return e.Write("1")
}

func DeleteCG2(allocID, task string) error {
	p := PathCG2(allocID, task)
	return os.RemoveAll(p)
}

// OpenScopeFile is useful when you have a complete cgroups v2 scope path,
// and want to edit a specific file.
func OpenScopeFile(cgroup, filename string) *Editor2 {
	p := filepath.Join(cgroup, filename)
	return &Editor2{
		path: p,
	}
}

// OpenPath opens the complete filepath p.
func OpenPath(p string) Editor {
	switch GetMode() {
	case CG1:
		return &Editor1{
			// todo
		}
	default:
		return &Editor2{
			path: p,
		}
	}
}

// TODO rename "OpenFile"
//
// Open filename, which is the path off of the parent.
func Open(filename string) Editor {
	switch GetMode() {
	case CG1:
		return &Editor1{
			Root: "todo",
			File: "todo",
		}
	default:
		return &Editor2{
			path: filepath.Join("/sys/fs/cgroup", NomadCgroupParent, filename),
		}
	}
}

type Editor1 struct {
	Root   string
	Parent string
	File   string
}

func (e *Editor1) Read() (string, error) {
	// todo
	return "", nil
}

func (e *Editor1) Write(string) error {
	// todo
	return nil
}

type Editor2 struct {
	path string // the complete filepath
}

func (e *Editor2) Read() (string, error) {
	b, err := os.ReadFile(e.path)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(b)), nil
}

func (e *Editor2) Write(s string) error {
	return os.WriteFile(e.path, []byte(s), 0644)
}
