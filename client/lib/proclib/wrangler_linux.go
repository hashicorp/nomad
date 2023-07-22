//go:build linux

package proclib

import (
	"github.com/hashicorp/nomad/client/lib/proclib/cgroupslib"
)

func New(configs *Configs) *Wranglers {
	configs.Log(nlog)

	w := &Wranglers{
		configs: configs,
		m:       make(map[Task]ProcessWrangler),
	}

	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
		w.create = newCG1(configs)
	case cgroupslib.CG2:
		w.create = newCG2(configs)
	case cgroupslib.OFF:
		panic("must enable cgroups v1 or v2")
	}

	return nil
}
