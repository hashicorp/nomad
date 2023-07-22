//go:build linux

package cgroupslib

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// An Editor is used for reading and writing cgroup interface files. Implementations
// are provided for cgroups v1 and cgroups v2.
type Editor interface {
	// Read the contents of an interface file.
	Read() (string, error)

	// Write the contents to an interface file.
	Write(string) error
}

func Join(allocID, task, file string) string {
	return fmt.Sprintf("%s.%s.scope/%s", allocID, task, file)
}

func Open(filepath string) Editor {
	switch GetMode() {
	case CG1:
		return &Editor1{
			Root: "todo",
			File: filepath,
		}
	default:
		return &Editor2{
			File: filepath,
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
	File string
}

func (e *Editor2) Read() (string, error) {
	file := filepath.Join("/sys/fs/cgroup", NomadCgroupParent, e.File)
	b, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimSpace(b)), nil
}

func (e *Editor2) Write(string) error {
	// todo
	return nil
}
