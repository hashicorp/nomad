//go:build linux

package proclib

import (
	"github.com/hashicorp/nomad/client/lib/proclib/cgroupslib"
)

// LinuxWranglerCG2 is an implementation of ProcessWrangler that leverages
// cgroups v2 on modern Linux systems.
//
// e.g. Ubuntu 22.04 / RHEL 9 and later versions.
type LinuxWranglerCG2 struct {
	task Task
}

func newCG2(c *Configs) create {
	cgroupslib.Init(c.Logger.Named("cgv2"))

	return func(task Task) ProcessWrangler {
		nlog.Info("newCG2()", "task", task)
		return &LinuxWranglerCG2{}
	}
}

func (w *LinuxWranglerCG2) Kill() error {
	nlog.Info("Kill()")

	return nil
}

func (w *LinuxWranglerCG2) Cleanup() error {
	nlog.Info("Cleanup()")

	return nil
}
