//go:build linux

package proclib

import (
	"github.com/hashicorp/nomad/client/lib/proclib/cgroupslib"
)

// Configs is used to pass along values from client configuration that are
// build-tag specific.
type Configs struct {
	// ParentCgroup can be set in Nomad client config. By default this value
	// is "/nomad" on cgroups v1, and "nomad.slice" in cgroups v2.
	ParentCgroup string
}

func New(configs *Configs) *Wranglers {
	nlog.Info("New() Wranglers", "parent_cgroup", configs.ParentCgroup)

	w := &Wranglers{
		m: make(map[Task]ProcessWrangler),
	}

	if cgroupslib.IsV2() {
		w.create = newCG2(configs)
	} else {
		w.create = newCG1(configs)
	}

	return nil
}

func (w *Wranglers) SetAttributes(m map[string]string) {
	// todo
}
