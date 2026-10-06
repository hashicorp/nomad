// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package proclib

import (
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
	"golang.org/x/sys/unix"
)

var _ ProcessWrangler = (*LinuxWranglerCG2)(nil)

func TestProcessWrangler(t *testing.T) {
	ci.Parallel(t)
	testutil.RequireRoot(t)
	if cgroupslib.GetMode() != cgroupslib.CG2 {
		t.Skip("requires cgroup2")
	}

	w, err := New(&Configs{
		Logger:      testlog.HCLogger(t),
		UsableCores: idset.Empty[hw.CoreID](),
	})
	must.NoError(t, err)

	task := Task{
		AllocID: "test-alloc-id",
		Task:    "test-task",
		Cores:   false,
	}
	must.NoError(t, w.Setup(task))

	// Start a sleep command in the cgroup
	cg := cgroupslib.LinuxResourcesPath(task.AllocID, task.Task, task.Cores)
	cgfd, err := unix.Open(cg, unix.O_PATH, 0)
	must.NoError(t, err)
	t.Cleanup(func() {
		_ = unix.Close(cgfd)
	})
	cmd := exec.Command("sleep", "1000")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		UseCgroupFD: true,
		CgroupFD:    cgfd,
	}
	must.NoError(t, cmd.Start())
	exitCh := make(chan error, 1)
	go func() {
		exitCh <- cmd.Wait()
	}()

	select {
	case status := <-exitCh:
		t.Fatalf("sleep exited early. error: %v", status)
	case <-time.After(300 * time.Millisecond):
	}

	// Kill its cgroup
	must.NoError(t, w.Destroy(task))

	// Assert it exited
	select {
	case status := <-exitCh:
		// sleep should exit with error
		must.Error(t, status)
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("sleep did not exit")
	}
}
