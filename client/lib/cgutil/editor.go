//go:build linux

package cgutil

import (
	"os"
	"path/filepath"
	"strings"
)

// editor provides a simple mechanism for reading and writing cgroup files
// of a particular cgroup.
type editor struct {
	fromRoot string
}

// dir returns the absolute path to the directory of the cgroup
func (e *editor) dir() string {
	return filepath.Join(CgroupRoot, e.fromRoot)
}

// path returns the absolute path to cgroup interface file.
func (e *editor) path(file string) string {
	return filepath.Join(CgroupRoot, e.fromRoot, file)
}

// create ensures the directory for the cgroup exists.
func (e *editor) create() error {
	return os.MkdirAll(e.dir(), 0o755)
}

// write content to file.
func (e *editor) write(file, content string) error {
	return os.WriteFile(e.path(file), []byte(content), 0o644)
}

// read the content of file.
func (e *editor) read(file string) (string, error) {
	b, err := os.ReadFile(e.path(file))
	return strings.TrimSpace(string(b)), err
}
