// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/hashicorp/nomad/testutil"
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
				return spec, true, nil
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

// TestNetworkHook_Prerun_Postrun_ExistingNetNS tests that the prerun and
// postrun hooks call the Setup and Destroy with the expected behaviors when the
// network namespace already exists (typical of agent restarts and host reboots)
func TestNetworkHook_Prerun_Postrun_ExistingNetNS(t *testing.T) {
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
	isolationSetter := &mockNetworkIsolationSetter{t: t, expectedSpec: spec}
	statusSetter := &mockNetworkStatusSetter{t: t, expectedStatus: nil}

	callCounts := testutil.NewCallCounter()

	nm := &testutils.MockDriver{
		MockNetworkManager: testutils.MockNetworkManager{
			CreateNetworkF: func(allocID string, req *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
				test.Eq(t, alloc.ID, allocID)
				callCounts.Inc("CreateNetwork")
				return spec, false, nil
			},

			DestroyNetworkF: func(allocID string, netSpec *drivers.NetworkIsolationSpec) error {
				test.Eq(t, alloc.ID, allocID)
				test.Eq(t, spec, netSpec)
				callCounts.Inc("DestroyNetwork")
				return nil
			},
		},
	}

	fakePlugin := newMockCNIPlugin()

	configurator := &cniNetworkConfigurator{
		nodeAttrs: map[string]string{
			"plugins.cni.version.bridge": "1.6.1",
		},
		nodeMeta: map[string]string{},
		logger:   testlog.HCLogger(t),
		cni:      fakePlugin,
		nsOpts:   &nsOpts{},
	}

	envBuilder := taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region)

	testCases := []struct {
		name                             string
		cniVersion                       string
		checkErrs                        []error
		setupErrs                        []string
		expectPrerunCreateNetworkCalls   int
		expectPrerunDestroyNetworkCalls  int
		expectCheckCalls                 int
		expectSetupCalls                 int
		expectPostrunDestroyNetworkCalls int
		expectPrerunError                string
	}{
		{
			name:                             "good check",
			cniVersion:                       "1.6.1",
			expectPrerunCreateNetworkCalls:   1,
			expectPrerunDestroyNetworkCalls:  0,
			expectCheckCalls:                 1,
			expectSetupCalls:                 0,
			expectPostrunDestroyNetworkCalls: 1,
		},
		{
			name:                             "initial check fails",
			cniVersion:                       "1.6.1",
			checkErrs:                        []error{fmt.Errorf("whatever")},
			expectPrerunCreateNetworkCalls:   2,
			expectPrerunDestroyNetworkCalls:  1,
			expectCheckCalls:                 2,
			expectSetupCalls:                 0,
			expectPostrunDestroyNetworkCalls: 2,
		},
		{
			name:       "check fails twice",
			cniVersion: "1.6.1",
			checkErrs: []error{
				fmt.Errorf("whatever"),
				fmt.Errorf("whatever"),
			},
			expectPrerunCreateNetworkCalls:   2,
			expectPrerunDestroyNetworkCalls:  1,
			expectCheckCalls:                 2,
			expectSetupCalls:                 0,
			expectPostrunDestroyNetworkCalls: 2,
			expectPrerunError:                "failed to configure networking for alloc: network namespace already exists but was misconfigured: whatever",
		},
		{
			name:                             "old CNI version skips check",
			cniVersion:                       "1.2.0",
			expectPrerunCreateNetworkCalls:   1,
			expectPrerunDestroyNetworkCalls:  0,
			expectCheckCalls:                 0,
			expectSetupCalls:                 0,
			expectPostrunDestroyNetworkCalls: 1,
		},
	}

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {
			callCounts.Reset()
			fakePlugin.counter.Reset()
			fakePlugin.checkErrors = tc.checkErrs
			configurator.nodeAttrs["plugins.cni.version.bridge"] = tc.cniVersion
			hook := newNetworkHook(testlog.HCLogger(t), isolationSetter,
				alloc, nm, configurator, statusSetter, envBuilder.Build())

			err := hook.Prerun()
			if tc.expectPrerunError == "" {
				must.NoError(t, err)
			} else {
				must.EqError(t, err, tc.expectPrerunError)
			}

			test.Eq(t, tc.expectPrerunDestroyNetworkCalls,
				callCounts.Get()["DestroyNetwork"], test.Sprint("DestroyNetwork calls after prerun"))
			test.Eq(t, tc.expectPrerunCreateNetworkCalls,
				callCounts.Get()["CreateNetwork"], test.Sprint("CreateNetwork calls after prerun"))

			test.Eq(t, tc.expectCheckCalls, fakePlugin.counter.Get()["Check"], test.Sprint("Check calls"))
			test.Eq(t, tc.expectSetupCalls, fakePlugin.counter.Get()["Setup"], test.Sprint("Setup calls"))

			must.NoError(t, hook.Postrun())
			test.Eq(t, tc.expectPostrunDestroyNetworkCalls,
				callCounts.Get()["DestroyNetwork"], test.Sprint("DestroyNetwork calls after postrun"))

		})

	}
}
