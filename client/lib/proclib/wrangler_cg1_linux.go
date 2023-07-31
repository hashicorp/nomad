//go:build linux

package proclib

import (
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/proclib/cgroupslib"
	"golang.org/x/sys/unix"
	"oss.indeed.com/go/libtime/decay"
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
	w.log.Info("Kill()", "task", w.task)

	err := cgroupslib.FreezeCG1(w.task.AllocID, w.task.Task)
	if err != nil {
		return err
	}

	// iterate processes in the freezer group and signal them
	paths := cgroupslib.PathsCG1(w.task.AllocID, w.task.Task)
	ed := cgroupslib.OpenPath(filepath.Join(paths[1], "cgroup.procs"))
	pids, err := ed.ReadPIDs()
	if err != nil {
		return err
	}

	// manually issue sigkill to each process
	signal := unix.SignalNum("SIGKILL")
	pids.ForEach(func(pid int) bool {
		_ = unix.Kill(pid, signal)
		return true
	})

	// unthaw the processes so they can die
	return cgroupslib.ThawCG1(w.task.AllocID, w.task.Task)
}

func (w *LinuxWranglerCG1) Cleanup() error {
	w.log.Info("Cleanup()", "task", w.task)

	// need to give the kernel an opportunity to cleanup procs; which could
	// take some time while the procs wake from being thawed only to find they
	// have been issued a kill signal and need to be reaped

	rm := func() (bool, error) {
		err := cgroupslib.DeleteCG1(w.task.AllocID, w.task.Task)
		if err != nil {
			return true, err
		}
		return false, nil
	}

	go func() {
		if err := decay.Backoff(rm, decay.BackoffOptions{
			MaxSleepTime:   30 * time.Second,
			InitialGapSize: 1 * time.Second,
		}); err != nil {
			w.log.Warn("failed to cleanup cgroups", "alloc", w.task.AllocID, "task", w.task.Task, "error", err)
		}
	}()

	return nil
}
