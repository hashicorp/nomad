//go:build linux

package cgroupslib

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-set"
	"golang.org/x/sys/unix"
)

const (
	root = "/sys/fs/cgroup"
)

func OpenPath(dir string) Interface {
	return &editor{
		dpath: dir,
	}
}

func OpenFromCpusetCG1(dir, iface string) Interface {
	return &editor{
		dpath: strings.Replace(dir, "cpuset", iface, 1),
	}
}

type Interface interface {
	Read(filename string) (string, error)
	PIDs() (*set.Set[int], error)
	Write(filename, content string) error
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

func Factory(allocID, task string) Lifecycle {
	switch GetMode() {
	case CG1:
		return &lifeCG1{
			allocID: allocID,
			task:    task,
		}
	default:
		return &lifeCG2{
			dpath: pathCG2(allocID, task),
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

type lifeCG1 struct {
	allocID string
	task    string
}

func (l *lifeCG1) Setup() error {
	paths := l.paths()
	for _, p := range paths {
		err := os.MkdirAll(p, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *lifeCG1) Teardown() error {
	paths := l.paths()
	for _, p := range paths {
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
	pids.ForEach(func(pid int) bool {
		_ = unix.Kill(pid, signal)
		return true
	})

	return l.thaw()
}

func (l *lifeCG1) edit(iface string) *editor {
	scope := scopeCG1(l.allocID, l.task)
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
	scope := scopeCG1(l.allocID, l.task)
	ifaces := []string{"freezer", "cpu", "memory", "cpuset"}
	paths := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		paths = append(paths, filepath.Join(
			root, iface, NomadCgroupParent, scope,
		))
	}
	return paths
}

type lifeCG2 struct {
	dpath string
}

func (l *lifeCG2) Setup() error {
	return os.MkdirAll(l.dpath, 0755)
}

func (l *lifeCG2) Teardown() error {
	return os.RemoveAll(l.dpath)
}

func (l *lifeCG2) Kill() error {
	panic("todo")
}

/// -------- helpers

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

func scopeCG1(allocID, task string) string {
	return fmt.Sprintf("%s.%s", allocID, task)
}

func scopeCG2(allocID, task string) string {
	return fmt.Sprintf("%s.%s.scope", allocID, task)
}

func pathCG2(allocID, task string) string {
	return filepath.Join(root, NomadCgroupParent, scopeCG2(allocID, task))
}

/// ------------------- cut below

// func pathTo(parts ...string) string {
// 	switch GetMode() {
// 	case CG1:
// 		parts = append([]string{root}, parts...)
// 		return filepath.Join(parts...)
// 	default:
// 		parts = append([]string{root}, parts...)
// 		return filepath.Join(parts...)
// 	}
// }

// func readRoot(filename string) (string, error) {
// 	switch GetMode() {
// 	case CG1:
// 		panic("todo")
// 	default:
// 		b, err := os.ReadFile(pathTo(filename))
// 		return string(bytes.TrimSpace(b)), err
// 	}
// }

// func writeRoot(filename, content string) error {
// 	switch GetMode() {
// 	case CG1:
// 		panic("todo")
// 	default:
// 		path := pathTo(filename)
// 		return os.WriteFile(path, []byte(content), 0644)
// 	}
// }

// // func OpenParentCG1(iface, filename string) Editor {
// // 	return &Editor1{
// // 		path: filepath.Join("/sys/fs/cgroup", iface, NomadCgroupParent, filename),
// // 	}
// // }

// // // An Editor is used for reading and writing cgroup interface files. Implementations
// // // are provided for cgroups v1 and cgroups v2.
// // type Editor interface {
// // 	// Read the contents of an interface file.
// // 	Read() (string, error)

// // 	// ReadPIDs reads the contents of the interface file, parsing it as a set of
// // 	// process IDs.
// // 	ReadPIDs() (*set.Set[int], error)

// // 	// Write the contents to an interface file.
// // 	Write(string) error
// // }

// // scope returns the name of the scope directory for the task of the
// // given allocation.
// func scope(allocID, task string) string {
// 	switch GetMode() {
// 	case CG1:
// 		return fmt.Sprintf("%s.%s", allocID, task)
// 	default:
// 		return fmt.Sprintf("%s.%s.scope", allocID, task)
// 	}
// }

// // func PathCG2(allocID, task string) string {
// // 	return pathTo(NomadCgroupParent, scope(allocID, task))
// // }

// // // PathsCG1 returns the set of directories in which interface files for the
// // // given allocID:task can be found. The interfaces are always in this order:
// // // [cpuset, freezer, cpu, memory]
// // func PathsCG1(allocID, task string) []string {
// // 	dir := scope(allocID, task)
// // 	return []string{
// // 		// always start with cpuset
// // 		pathTo("cpuset", NomadCgroupParent, dir), // TODO (partitions)
// // 		pathTo("freezer", NomadCgroupParent, dir),
// // 		pathTo("cpu", NomadCgroupParent, dir),
// // 		pathTo("memory", NomadCgroupParent, dir),
// // 	}
// // }

// // func fileCG2(allocID, task, filename string) string {
// // 	return filepath.Join(pathTo(allocID, task, filename))
// // }

// // // CreateCG2 will create the one cgroup directory associated with the given
// // // alloc::task. This cgroup is create under the NomadCgroupParent directory,
// // // e.g.
// // // /sys/fs/cgroup/nomad.slice/<alloc>.<task>.scope/
// // func CreateCG2(allocID, task string) error {
// // 	p := PathCG2(allocID, task)
// // 	return os.MkdirAll(p, 0755)
// // }

// // func KillCG2(allocID, task string) error {
// // 	netlog.Red("kill", "alloc", allocID, "task", task)
// // 	e := Open(fileCG2(allocID, task, "cgroup.kill"))
// // 	return e.Write("1")
// // }

// // func DeleteCG2(allocID, task string) error {
// // 	p := PathCG2(allocID, task)
// // 	return os.RemoveAll(p)
// // }

// // // CreateCG1 will create the minimal set of cgroup directories associated with
// // // the given alloc::task. These cgroups are created under the NomadCgroupParent
// // // directory.
// // // e.g.
// // // /sys/fs/cgroup/freezer/nomad/<alloc>.<task>/
// // // /sys/fs/cgroup/cpuset/nomad/<alloc>.<task>/
// // // /sys/fs/cgroup/cpu/nomad/<alloc>.<task>/
// // // /sys/fs/cgroup/memory/nomad/<alloc>.<task>/
// // func CreateCG1(allocID, task string) error {
// // 	paths := PathsCG1(allocID, task)
// // 	for _, p := range paths {
// // 		if err := os.MkdirAll(p, 0755); err != nil {
// // 			return err
// // 		}
// // 	}
// // 	return nil
// // }

// // func FreezeCG1(allocID, task string) error {
// // 	netlog.Red("freeze cg1", "alloc", allocID, "task", task)
// // 	//
// // 	// do the freezer and stuff
// // 	//
// // 	return nil
// // }

// // func ThawCG1(allocID, task string) error {
// // 	netlog.Red("thaw cg1", "alloc", allocID, "task", task)
// // 	//
// // 	// do the thaw
// // 	//
// // 	return nil
// // }

// // // DeleteCG1 will delete each cgroup path associated with the alloc::task.
// // func DeleteCG1(allocID, task string) error {
// // 	paths := PathsCG1(allocID, task)
// // 	var errs error
// // 	for _, p := range paths {
// // 		if err := os.RemoveAll(p); err != nil {
// // 			errors.Join(errs, err)
// // 		}
// // 	}
// // 	return errs
// // }

// // // OpenScopeFile is useful when you have a complete cgroups v2 scope path,
// // // and want to edit a specific file.
// // func OpenScopeFile(cgroup, filename string) *Editor2 {
// // 	p := filepath.Join(cgroup, filename)
// // 	return &Editor2{
// // 		path: p,
// // 	}
// // }

// // // // OpenPath opens the complete filepath p.
// // // func OpenPath(p string) Editor {
// // // 	switch GetMode() {
// // // 	case CG1:
// // // 		return &Editor1{
// // // 			// todo
// // // 		}
// // // 	default:
// // // 		return &Editor2{
// // // 			path: p,
// // // 		}
// // // 	}
// // // }

// // // // TODO rename OpenCG2
// // // // Open filename, which is the path off of the parent.
// // // // e.g. "<alloc.<task>.scope/cgroup.kill"
// // // func Open(filename string) Editor {
// // // 	return &Editor2{
// // // 		path: filepath.Join(root, NomadCgroupParent, filename),
// // // 	}
// // // }

// // // OpenFromCpusetCG1 opens the given interface file using the cpuset cgroup
// // // path as a basis, since that is the only path we keep track of in the Nomad
// // // client for each task.
// // // func OpenFromCpusetCG1(cg, iface, filename string) Editor {
// // // 	p := strings.Replace(cg, "cpuset", iface, 1)
// // // 	p = filepath.Join(p, filename)
// // // 	return &Editor1{
// // // 		path: p,
// // // 	}
// // // }

// // func OpenCG1(iface, alloc, task, file string) Editor {
// // 	return &Editor1{
// // 		path: filepath.Join(
// // 			root,
// // 			iface,
// // 			NomadCgroupParent,
// // 			scope(alloc, task),
// // 			file,
// // 		),
// // 	}
// // }

// // type Editor1 struct {
// // 	path string // the complete filepath
// // }

// // func (e *Editor1) Read() (string, error) {
// // 	b, err := os.ReadFile(e.path)
// // 	if err != nil {
// // 		return "", err
// // 	}
// // 	return string(bytes.TrimSpace(b)), nil
// // }

// // func (e *Editor1) ReadPIDs() (*set.Set[int], error) {
// // 	b, err := os.ReadFile(e.path)
// // 	if err != nil {
// // 		return nil, err
// // 	}
// // 	tokens := bytes.Fields(bytes.TrimSpace(b))
// // 	return set.FromFunc(tokens, func(b []byte) int {
// // 		i, _ := strconv.Atoi(string(b))
// // 		return i
// // 	}), nil
// // }

// // func (e *Editor1) Write(s string) error {
// // 	return os.WriteFile(e.path, []byte(s), 0644)
// // }

// // type Editor2 struct {
// // 	path string // the complete filepath
// // }

// // func (e *Editor2) Read() (string, error) {
// // 	b, err := os.ReadFile(e.path)
// // 	if err != nil {
// // 		return "", err
// // 	}
// // 	return string(bytes.TrimSpace(b)), nil
// // }

// // func (e *Editor2) ReadPIDs() (*set.Set[int], error) {
// // 	panic("not yet implemented")
// // }

// // func (e *Editor2) Write(s string) error {
// // 	return os.WriteFile(e.path, []byte(s), 0644)
// // }
