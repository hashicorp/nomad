// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

// statically assert network hook implements the expected interfaces
var _ interfaces.RunnerPrerunHook = (*networkHook)(nil)
var _ interfaces.RunnerPostrunHook = (*networkHook)(nil)

type mockNetworkIsolationSetter struct {
	t            *testing.T
	expectedSpec *drivers.NetworkIsolationSpec
	called       bool
}

func (m *mockNetworkIsolationSetter) SetNetworkIsolation(spec *drivers.NetworkIsolationSpec) {
	m.called = true
	test.Eq(m.t, m.expectedSpec, spec)
}

type mockNetworkStatusSetter struct {
	t              *testing.T
	expectedStatus *structs.AllocNetworkStatus
	called         bool
}

func (m *mockNetworkStatusSetter) SetNetworkStatus(status *structs.AllocNetworkStatus) {
	m.called = true
	test.Eq(m.t, m.expectedStatus, status)
}

// Test that the prerun and postrun hooks call the setter with the expected
// NetworkIsolationSpec for group bridge network.
func TestNetworkHook_Prerun_Postrun_group(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Networks = []*structs.NetworkResource{
		{Mode: "bridge"},
	}

	spec := &drivers.NetworkIsolationSpec{
		Mode:   drivers.NetIsolationModeGroup,
		Path:   "test",
		Labels: map[string]string{"abc": "123"},
	}

	destroyCalled := false
	nm := &testutils.MockDriver{
		MockNetworkManager: testutils.MockNetworkManager{
			CreateNetworkF: func(allocID string, req *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
				test.Eq(t, alloc.ID, allocID)
				return spec, false, nil
			},

			DestroyNetworkF: func(allocID string, netSpec *drivers.NetworkIsolationSpec) error {
				destroyCalled = true
				test.Eq(t, alloc.ID, allocID)
				test.Eq(t, spec, netSpec)
				return nil
			},
		},
	}
	setter := &mockNetworkIsolationSetter{
		t:            t,
		expectedSpec: spec,
	}
	statusSetter := &mockNetworkStatusSetter{
		t:              t,
		expectedStatus: nil,
	}

	envBuilder := taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region)
	logger := testlog.HCLogger(t)
	hook := newNetworkHook(logger, setter, alloc, nm, &hostNetworkConfigurator{}, statusSetter, envBuilder.Build())
	must.NoError(t, hook.Prerun())
	must.True(t, setter.called)
	must.False(t, destroyCalled)
	must.NoError(t, hook.Postrun())
	must.True(t, destroyCalled)
}

// Test that prerun and postrun hooks do not expect a NetworkIsolationSpec
func TestNetworkHook_Prerun_Postrun_host(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	alloc.Job.TaskGroups[0].Networks = []*structs.NetworkResource{
		{Mode: "host"},
	}

	destroyCalled := false
	nm := &testutils.MockDriver{
		MockNetworkManager: testutils.MockNetworkManager{
			CreateNetworkF: func(allocID string, req *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
				test.Unreachable(t, test.Sprintf("should not call CreateNetwork for host network"))
				return nil, false, nil
			},

			DestroyNetworkF: func(allocID string, netSpec *drivers.NetworkIsolationSpec) error {
				destroyCalled = true
				test.Nil(t, netSpec)
				return nil
			},
		},
	}
	setter := &mockNetworkIsolationSetter{
		t:            t,
		expectedSpec: nil,
	}
	statusSetter := &mockNetworkStatusSetter{
		t:              t,
		expectedStatus: nil,
	}

	envBuilder := taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region)
	logger := testlog.HCLogger(t)
	hook := newNetworkHook(logger, setter, alloc, nm, &hostNetworkConfigurator{}, statusSetter, envBuilder.Build())
	must.NoError(t, hook.Prerun())
	must.False(t, setter.called)
	must.False(t, destroyCalled)
	must.NoError(t, hook.Postrun())
	must.True(t, destroyCalled)
}
