package client

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
)

type MockAllocStateUpdater struct {
	Count  int
	Allocs []*structs.Allocation
	Err    error
}

func (m *MockAllocStateUpdater) Update(alloc *structs.Allocation) error {
	m.Count += 1
	m.Allocs = append(m.Allocs, alloc)
	return m.Err
}

func testAllocRunner() (*MockAllocStateUpdater, *AllocRunner) {
	logger := testLogger()
	conf := DefaultConfig()
	conf.StateDir = "/tmp"
	upd := &MockAllocStateUpdater{}
	alloc := mock.Alloc()
	ar := NewAllocRunner(logger, conf, upd.Update, alloc)
	return upd, ar
}

func TestAllocRunner_SimpleRun(t *testing.T) {
}

func TestAllocRunner_Update(t *testing.T) {
}

func TestAllocRunner_Destroy(t *testing.T) {
}

func TestAllocRunner_SaveRestoreState(t *testing.T) {
}
