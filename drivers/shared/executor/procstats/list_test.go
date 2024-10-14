// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package procstats

import (
	"math/rand"
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

func genMockProcs(needles, haystack int) ([]ps.Process, []ProcessID) {

	procs := []ps.Process{mockProc(1, 1), mockProc(42, 1)}
	expect := []ProcessID{42}

	// TODO: make this into a tree structure, not just a linear tree
	for i := 0; i < needles; i++ {
		parent := 42 + i
		pid := parent + 1
		procs = append(procs, mockProc(pid, parent))
		expect = append(expect, pid)
	}

	for i := 0; i < haystack; i++ {
		parent := 200 + i
		pid := parent + 1
		procs = append(procs, mockProc(pid, parent))
	}

	rand.Shuffle(len(procs), func(i, j int) {
		procs[i], procs[j] = procs[j], procs[i]
	})

	return procs, expect
}

func Test_list(t *testing.T) {
	cases := []struct {
		name     string
		needles  int
		haystack int
		expect   int
	}{
		{
			name:     "minimal",
			needles:  2,
			haystack: 10,
			expect:   16,
		},
		{
			name:     "small needles small haystack",
			needles:  5,
			haystack: 200,
			expect:   212,
		},
		{
			name:     "small needles large haystack",
			needles:  10,
			haystack: 1000,
			expect:   1022,
		},
		{
			name:     "moderate needles giant haystack",
			needles:  20,
			haystack: 2000,
			expect:   2042,
		},
	}

	for _, tc := range cases {
		const executorPID = 42
		t.Run(tc.name, func(t *testing.T) {

			procs, expect := genMockProcs(tc.needles, tc.haystack)
			lister := func() ([]ps.Process, error) {
				return procs, nil
			}

			result, examined := list(executorPID, lister)
			must.SliceContainsAll(t, expect, result.Slice(),
				must.Sprintf("exp: %v; got: %v", expect, result),
			)
			must.Eq(t, tc.expect, examined)
		})
	}
}
