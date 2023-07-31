//go:build linux

package proclib

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/proclib/cgroupslib"
)

// LinuxWranglerCG2 is an implementation of ProcessWrangler that leverages
// cgroups v2 on modern Linux systems.
//
// e.g. Ubuntu 22.04 / RHEL 9 and later versions.
type LinuxWranglerCG2 struct {
	task Task
	log  hclog.Logger
}

func newCG2(c *Configs) create {
	cgroupslib.Init(c.Logger.Named("cgv2"))
	return func(task Task) ProcessWrangler {
		return &LinuxWranglerCG2{
			task: task,
			log:  c.Logger,
		}
	}
}

func (w LinuxWranglerCG2) Initialize() error {
	w.log.Info("init cgroup", "task", w.task)
	// create cgroup for the task
	// e.g. mkdir /sys/fs/cgroup/nomad.slice/<scope>
	return cgroupslib.CreateCG2(w.task.AllocID, w.task.Task)
}

func (w *LinuxWranglerCG2) Kill() error {
	w.log.Info("Kill()", "task", w.task)
	return cgroupslib.KillCG2(w.task.AllocID, w.task.Task)
}

func (w *LinuxWranglerCG2) Cleanup() error {
	w.log.Info("Cleanup()", "task", w.task)
	return cgroupslib.DeleteCG2(w.task.AllocID, w.task.Task)
}
