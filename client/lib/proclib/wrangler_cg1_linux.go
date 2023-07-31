//go:build linux

package proclib

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/proclib/cgroupslib"
)

// LinuxWranglerCG1 is an implementation of ProcessWrangler that leverages
// cgroups v1 on older Linux systems.
//
// e.g. Ubuntu 20.04 / RHEL 8 and previous versions.
type LinuxWranglerCG1 struct {
	task Task
	log  hclog.Logger
}

func newCG1(c *Configs) create {
	cgroupslib.Init(c.Logger.Named("cgv1"))
	return func(task Task) ProcessWrangler {
		return &LinuxWranglerCG1{
			task: task,
			log:  c.Logger.Named("wrangle_cg1"),
		}
	}
}

func (w *LinuxWranglerCG1) Initialize() error {
	w.log.Info("init cgroups", "task", w.task)
	return cgroupslib.CreateCG1(w.task.AllocID, w.task.Task)
}

func (w *LinuxWranglerCG1) Kill() error {
	err := cgroupslib.FreezeCG1(w.task.AllocID, w.task.Task)
	if err != nil {
		return err
	}

	// ed := cgroupslib.Open

	return cgroupslib.ThawCG1(w.task.AllocID, w.task.Task)
}

func (w *LinuxWranglerCG1) Cleanup() error {
	return nil
}
