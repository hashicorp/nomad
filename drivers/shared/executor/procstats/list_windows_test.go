// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package procstats

import (
	"testing"

	"github.com/mitchellh/go-ps"
	"github.com/shoenig/test/must"
)

type mockProcess struct {
	pid  int
	ppid int
}

func (p *mockProcess) Pid() int {
	return p.pid
}

func (p *mockProcess) PPid() int {
	return p.ppid
}

func (p *mockProcess) Executable() string {
	return ""
}

func mockProc(pid, ppid int) *mockProcess {
	return &mockProcess{pid: pid, ppid: ppid}
}

var (
	executorOnly = []ps.Process{
		mockProc(1, 1),
		mockProc(42, 1),
	}

	simpleLine = []ps.Process{
		mockProc(1, 1),
		mockProc(50, 42),
		mockProc(42, 1),
		mockProc(51, 50),
		mockProc(101, 100),
		mockProc(60, 51),
		mockProc(100, 1),
	}

	bigTree = []ps.Process{
		mockProc(1, 1),
		mockProc(25, 50),
		mockProc(100, 1),
		mockProc(75, 50),
		mockProc(10, 25),
		mockProc(80, 75),
		mockProc(81, 75),
		mockProc(51, 50),
		mockProc(42, 1),
		mockProc(101, 100),
		mockProc(52, 51),
		mockProc(50, 42),
	}
)

func Test_list(t *testing.T) {
	cases := []struct {
		name  string
		procs []ps.Process
		exp   []ProcessID
	}{
		{
			name:  "executor only",
			procs: executorOnly,
			exp:   []ProcessID{42},
		},
		{
			name:  "simple line",
			procs: simpleLine,
			exp:   []ProcessID{42, 50, 51, 60},
		},
		{
			name:  "big tree",
			procs: bigTree,
			exp:   []ProcessID{42, 50, 25, 75, 10, 80, 81, 51, 52},
		},
	}

	for _, tc := range cases {
		const executorPID = 42
		t.Run(tc.name, func(t *testing.T) {
			lister := func() ([]ps.Process, error) {
				return tc.procs, nil
			}
			result := list(executorPID, lister)
			must.SliceContainsAll(t, tc.exp, result.Slice(),
				must.Sprintf("exp: %v; got: %v", tc.exp, result),
			)
		})
	}
}
