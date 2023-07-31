//go:build linux

package procstats

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/client/lib/proclib/cgroupslib"
)

func List(cg Cgrouper) *set.Set[ProcessID] {
	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
		return ListCG1(cg)
	default:
		return ListCG2(cg)
	}
}

type Cgrouper interface {
	Cgroup() string
}

func gobble(cg Cgrouper) *set.Set[ProcessID] {
	cgroup := filepath.Join(cg.Cgroup(), "cgroup.procs")
	ed := cgroupslib.OpenPath(cgroup)
	v, err := ed.Read()
	if err != nil {
		return set.New[ProcessID](0)
	}
	fields := strings.Fields(v)
	return set.FromFunc(fields, func(s string) ProcessID {
		i, _ := strconv.Atoi(s)
		return ProcessID(i)
	})
}

func ListCG1(cg Cgrouper) *set.Set[ProcessID] {
	// uses the cpuset cgroup
	return gobble(cg)
}

func ListCG2(cg Cgrouper) *set.Set[ProcessID] {
	return gobble(cg)
}
