// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
	"github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/test/must"
)

func Test_computeMemory(t *testing.T) {
	cases := []struct {
		memory    int64
		memoryMax int64
		expSoft   int64
		expHard   int64
	}{
		{
			// typical case; only 'memory' is set and that is used as the hard
			// memory limit
			memory:    100,
			memoryMax: 0,
			expSoft:   0,
			expHard:   mbToBytes(100),
		},
		{
			// oversub case; both 'memory' and 'memory_max' are set and used as
			// the soft and hard memory limits
			memory:    100,
			memoryMax: 200,
			expSoft:   mbToBytes(100),
			expHard:   mbToBytes(200),
		},
		{
			// special oversub case; 'memory' is set and 'memory_max' is set to
			// -1; which indicates there should be no hard limit (i.e. -1 / max)
			memory:    100,
			memoryMax: memoryNoLimit,
			expSoft:   mbToBytes(100),
			expHard:   memoryNoLimit,
		},
	}

	for _, tc := range cases {
		name := fmt.Sprintf("(%d,%d)", tc.memory, tc.memoryMax)
		t.Run(name, func(t *testing.T) {
			command := &ExecCommand{
				Resources: &drivers.Resources{
					NomadResources: &structs.AllocatedTaskResources{
						Memory: structs.AllocatedMemoryResources{
							MemoryMB:    tc.memory,
							MemoryMaxMB: tc.memoryMax,
						},
					},
				},
			}
			hard, soft := (*UniversalExecutor)(nil).computeMemory(command)
			must.Eq(t, tc.expSoft, soft)
			must.Eq(t, tc.expHard, hard)
		})
	}
}

func TestExecutor_InvalidCgroup(t *testing.T) {
	ci.Parallel(t)
	testutil.CgroupsCompatible(t)

	factory := universalFactory
	testExecCmd := testExecutorCommand(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "sleep"
	execCmd.Args = []string{"infinity"}

	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
		execCmd.OverrideCgroupV1 = map[string]string{
			"pid": "custom/path",
		}
	case cgroupslib.CG2:
		execCmd.OverrideCgroupV2 = "custom.slice/test.scope"
	}

	factory.configureExecCmd(t, execCmd)
	defer allocDir.Destroy()
	executor := factory.new(testlog.HCLogger(t), compute)
	defer executor.Shutdown("", 0)

	_, err := executor.Launch(execCmd)
	must.ErrorContains(t, err, "unable to configure cgroups: no such file or directory")

}

func TestUniversalExecutor_setOomAdj(t *testing.T) {
	ci.Parallel(t)

	factory := universalFactory
	testExecCmd := testExecutorCommand(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "sleep"
	execCmd.Args = []string{"infinity"}
	execCmd.OOMScoreAdj = 1000

	factory.configureExecCmd(t, execCmd)
	defer allocDir.Destroy()
	executor := factory.new(testlog.HCLogger(t), compute)
	defer executor.Shutdown("", 0)

	p, err := executor.Launch(execCmd)
	must.NoError(t, err)

	oomScore, err := os.ReadFile(fmt.Sprintf("/proc/%d/oom_score_adj", p.Pid))
	must.NoError(t, err)

	oomScoreInt, _ := strconv.Atoi(strings.TrimSuffix(string(oomScore), "\n"))
	must.Eq(t, execCmd.OOMScoreAdj, int32(oomScoreInt))
}

func TestUniversalExecutor_cg1_no_executor_pid(t *testing.T) {
	testutil.CgroupsCompatibleV1(t)
	ci.Parallel(t)

	factory := universalFactory
	testExecCmd := testExecutorCommand(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "sleep"
	execCmd.Args = []string{"infinity"}

	factory.configureExecCmd(t, execCmd)
	defer allocDir.Destroy()
	executor := factory.new(testlog.HCLogger(t), compute)
	defer executor.Shutdown("", 0)

	p, err := executor.Launch(execCmd)
	must.NoError(t, err)

	alloc := filepath.Base(allocDir.AllocDirPath())

	ifaces := []string{"cpu", "memory", "freezer"}
	for _, iface := range ifaces {
		cgroup := fmt.Sprintf("/sys/fs/cgroup/%s/nomad/%s.web/cgroup.procs", iface, alloc)

		content, err := os.ReadFile(cgroup)
		must.NoError(t, err)

		// ensure only 1 pid (sleep) is present in this  cgroup
		pids := strings.Fields(string(content))
		must.SliceLen(t, 1, pids)
		must.Eq(t, pids[0], strconv.Itoa(p.Pid))
	}
}

func TestUniversalExecutor_CgroupV2_DiskThrottle(t *testing.T) {
	testutil.CgroupsCompatibleV2(t)
	ci.Parallel(t)

	factory := universalFactory
	testExecCmd := testExecutorCommand(t)
	execCmd, allocDir := testExecCmd.command, testExecCmd.allocDir
	execCmd.Cmd = "sleep"
	execCmd.Args = []string{"3"}

	execCmd.Resources.NomadResources.DiskThrottles = []*structs.DiskThrottle{
		{
			Major:     8,
			Minor:     0,
			ReadBps:   2097152,
			WriteIops: 150,
		},
	}
	factory.configureExecCmd(t, execCmd)
	defer allocDir.Destroy()
	executor := factory.new(testlog.HCLogger(t), compute)
	defer executor.Shutdown("", 0)

	_, err := executor.Launch(execCmd)
	must.NoError(t, err)
	time.Sleep(1000 * time.Millisecond)

	alloc := filepath.Base(allocDir.AllocDirPath())
	cgroupPath := execCmd.OverrideCgroupV2
	if cgroupPath == "" {
		cgroupPath = filepath.Join("nomad.slice", "share.slice", alloc+".web.scope")
	}
	ioMaxFile := filepath.Join(cgroupslib.GetDefaultRoot(), cgroupPath, "io.max")
	content, err := os.ReadFile(ioMaxFile)
	must.NoError(t, err)

	stringContent := strings.TrimSpace(string(content))
	must.True(t, strings.Contains(stringContent, "8:0"))
	must.True(t, strings.Contains(stringContent, "rbps=2097152"))
	must.True(t, strings.Contains(stringContent, "wiops=150"))

	_, err = executor.Wait(context.Background())
	must.NoError(t, err)
}
