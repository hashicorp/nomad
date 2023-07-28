//go:build linux

package cgroupslib

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set"
)

func Init(log hclog.Logger) {
	switch GetMode() {
	case CG1:
	case CG2:
		// minimum controllers must be set first
		s, err := readRoot("cgroup.subtree_control")
		if err != nil {
			log.Error("failed to create nomad cgroup", "error", err)
			return
		}

		required := set.From([]string{"cpuset", "cpu", "io", "memory", "pids"})
		enabled := set.From(strings.Fields(s))
		needed := required.Difference(enabled)

		if needed.Size() == 0 {
			log.Debug("top level nomad.slice cgroup already exists")
			return // already setup
		}

		sb := new(strings.Builder)
		for _, controller := range needed.List() {
			sb.WriteString("+" + controller + " ")
		}

		activation := strings.TrimSpace(sb.String())
		if err = writeRoot("cgroup.subtree_control", activation); err != nil {
			log.Error("failed to create nomad cgroup", "error", err)
			return
		}

		nomadSlice := filepath.Join("/sys/fs/cgroup", NomadCgroupParent)
		if err := os.MkdirAll(nomadSlice, 0755); err != nil {
			log.Error("failed to create nomad cgroup", "error", err)
			return
		}

		log.Debug("top level nomad.slice cgroup initialized", "controllers", needed)
	}
}
