package executor

import (
	"testing"

	ps "github.com/mitchellh/go-ps"
)

func TestScanPids(t *testing.T) {
	t.Parallel()
	p1 := NewFakeProcess(2, 5)
	p2 := NewFakeProcess(10, 2)
	p3 := NewFakeProcess(15, 6)
	p4 := NewFakeProcess(3, 10)
	p5 := NewFakeProcess(20, 18)

	nomadPids, err := scanPids(5, []ps.Process{p1, p2, p3, p4, p5})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(nomadPids) != 4 {
		t.Fatalf("expected: 4, actual: %v", len(nomadPids))
	}
}

type FakeProcess struct {
	pid  int
	ppid int
}

func (f FakeProcess) Pid() int {
	return f.pid
}

func (f FakeProcess) PPid() int {
	return f.ppid
}

func (f FakeProcess) Executable() string {
	return "fake"
}

func NewFakeProcess(pid int, ppid int) ps.Process {
	return FakeProcess{pid: pid, ppid: ppid}
}
