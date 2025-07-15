// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package deploymentwatcher

import (
	"sync"
	"testing"

	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

type mockBackend struct {
	index uint64
	state *state.StateStore
	l     sync.Mutex
	calls map[string]int
}

func newMockBackend(t *testing.T) *mockBackend {
	m := &mockBackend{
		index: 10000,
		state: state.TestStateStore(t),
		calls: map[string]int{},
	}
	return m
}

func (m *mockBackend) nextIndex() uint64 {
	m.l.Lock()
	defer m.l.Unlock()
	i := m.index
	m.index++
	return i
}

func (m *mockBackend) trackCall(method string) {
	m.l.Lock()
	defer m.l.Unlock()
	m.calls[method]++
}

func (m *mockBackend) assertCalls(t *testing.T, method string, expect int) {
	t.Helper()
	m.l.Lock()
	defer m.l.Unlock()
	must.Eq(t, expect, m.calls[method],
		must.Sprintf("expected %d calls for method=%s. got=%+v", expect, method, m.calls))
}

func (m *mockBackend) UpdateAllocDesiredTransition(u *structs.AllocUpdateDesiredTransitionRequest) (uint64, error) {
	m.trackCall("UpdateAllocDesiredTransition")
	i := m.nextIndex()
	return i, m.state.UpdateAllocsDesiredTransitions(structs.MsgTypeTestSetup, i, u.Allocs, u.Evals)
}

func (m *mockBackend) UpsertJob(job *structs.Job) (uint64, error) {
	m.trackCall("UpsertJob")
	i := m.nextIndex()
	return i, m.state.UpsertJob(structs.MsgTypeTestSetup, i, nil, job)
}

func (m *mockBackend) UpdateDeploymentStatus(u *structs.DeploymentStatusUpdateRequest) (uint64, error) {
	m.trackCall("UpdateDeploymentStatus")
	i := m.nextIndex()
	return i, m.state.UpdateDeploymentStatus(structs.MsgTypeTestSetup, i, u)
}

func (m *mockBackend) UpdateDeploymentPromotion(req *structs.ApplyDeploymentPromoteRequest) (uint64, error) {
	m.trackCall("UpdateDeploymentPromotion")
	i := m.nextIndex()
	return i, m.state.UpdateDeploymentPromotion(structs.MsgTypeTestSetup, i, req)
}

func (m *mockBackend) UpdateDeploymentAllocHealth(req *structs.ApplyDeploymentAllocHealthRequest) (uint64, error) {
	m.trackCall("UpdateDeploymentAllocHealth")
	i := m.nextIndex()
	return i, m.state.UpdateDeploymentAllocHealth(structs.MsgTypeTestSetup, i, req)
}
