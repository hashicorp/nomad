// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/taskenv"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	tmock "github.com/stretchr/testify/mock"
)

type mockNetworkIsolationSetter struct {
	tmock.Mock
}

func (m *mockNetworkIsolationSetter) SetNetworkIsolation(spec *drivers.NetworkIsolationSpec) {
	m.Called(spec)
}

type mockNetworkStatus struct {
	tmock.Mock
}

func (m *mockNetworkStatus) SetNetworkStatus(status *structs.AllocNetworkStatus) {
	m.Called(status)
}

func (m *mockNetworkStatus) NetworkStatus() *structs.AllocNetworkStatus {
	args := m.Called()

	if args.Get(0) == nil {
		return nil
	}

	return args.Get(0).(*structs.AllocNetworkStatus)
}

type mockNetworkConfigurator struct {
	tmock.Mock
}

func (m *mockNetworkConfigurator) Setup(ctx context.Context, alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec, created bool) (*structs.AllocNetworkStatus, error) {
	args := m.Called(ctx, alloc, spec, created)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*structs.AllocNetworkStatus), args.Error(1)
}

func (m *mockNetworkConfigurator) Teardown(ctx context.Context, alloc *structs.Allocation, spc *drivers.NetworkIsolationSpec) error {
	args := m.Called()

	return args.Error(0)
}

func TestNetworkHook_Prerun(t *testing.T) {
	ci.Parallel(t)
	tests := []struct {
		name       string
		alloc      func() *structs.Allocation
		setupMocks func(ns *mockNetworkStatus, is *mockNetworkIsolationSetter, nc *mockNetworkConfigurator)
		expErr     error
	}{
		{
			name: "non CNI mode returns nil",
			setupMocks: func(ns *mockNetworkStatus, is *mockNetworkIsolationSetter, nc *mockNetworkConfigurator) {
				// no need to setup mocks, the test will fail if called
			},
			alloc: func() *structs.Allocation {
				m := mock.Alloc()
				m.Job.TaskGroups[0].Networks = []*structs.NetworkResource{
					{Mode: "host"},
				}
				return m
			},
			expErr: nil,
		},
		{
			name: "networkConfigurator returns ErrCNICheckFailed, recovers",
			setupMocks: func(ns *mockNetworkStatus, is *mockNetworkIsolationSetter, nc *mockNetworkConfigurator) {
				nc.On("Setup", tmock.Anything, tmock.Anything, tmock.Anything, tmock.Anything).Return(nil, ErrCNICheckFailed).Once()
				nc.On("Setup", tmock.Anything, tmock.Anything, tmock.Anything, tmock.Anything).Return(&structs.AllocNetworkStatus{
					InterfaceName: "test",
				}, nil).Once()

				is.On("SetNetworkIsolation", tmock.Anything).Return()

				ns.On("SetNetworkStatus", tmock.Anything).Return()
			},
			alloc: func() *structs.Allocation {
				m := mock.Alloc()
				m.Job.TaskGroups[0].Networks = []*structs.NetworkResource{
					{Mode: "bridge"},
				}
				return m
			},
			expErr: nil,
		},
		{
			name: "multiple ErrCNICheckFailed errors",
			setupMocks: func(ns *mockNetworkStatus, is *mockNetworkIsolationSetter, nc *mockNetworkConfigurator) {
				nc.On("Setup", tmock.Anything, tmock.Anything, tmock.Anything, tmock.Anything).Return(nil, ErrCNICheckFailed)

				is.On("SetNetworkIsolation", tmock.Anything).Return()

				ns.On("SetNetworkStatus", tmock.Anything).Return()
			},
			alloc: func() *structs.Allocation {
				m := mock.Alloc()
				m.Job.TaskGroups[0].Networks = []*structs.NetworkResource{
					{Mode: "bridge"},
				}
				return m
			},
			expErr: errors.New("failed to configure networking for alloc"),
		},
		{
			name: "successful cni setup succeeds without error",
			setupMocks: func(ns *mockNetworkStatus, is *mockNetworkIsolationSetter, nc *mockNetworkConfigurator) {
				nc.On("Setup", tmock.Anything, tmock.Anything, tmock.Anything, tmock.Anything).Return(&structs.AllocNetworkStatus{
					InterfaceName: "test",
				}, nil)

				is.On("SetNetworkIsolation", tmock.Anything).Return()

				// should not call NetworkStatus, only SetNetworkStatus
				ns.On("SetNetworkStatus", tmock.Anything).Return()
			},
			alloc: func() *structs.Allocation {
				m := mock.Alloc()
				m.Job.TaskGroups[0].Networks = []*structs.NetworkResource{
					{Mode: "bridge"},
				}
				return m
			},
			expErr: nil,
		},
		{
			name: "nil network setup with saved state succeeds",
			setupMocks: func(ns *mockNetworkStatus, is *mockNetworkIsolationSetter, nc *mockNetworkConfigurator) {
				nc.On("Setup", tmock.Anything, tmock.Anything, tmock.Anything, tmock.Anything).Return(nil, nil)

				is.On("SetNetworkIsolation", tmock.Anything).Return()

				ns.On("SetNetworkStatus", tmock.Anything).Return()
				ns.On("NetworkStatus", tmock.Anything).Return(&structs.AllocNetworkStatus{InterfaceName: "test"})
			},
			alloc: func() *structs.Allocation {
				m := mock.Alloc()
				m.Job.TaskGroups[0].Networks = []*structs.NetworkResource{
					{Mode: "bridge"},
				}
				return m
			},
			expErr: nil,
		},
		{
			name: "nil network setup without saved state errors",
			setupMocks: func(ns *mockNetworkStatus, is *mockNetworkIsolationSetter, nc *mockNetworkConfigurator) {
				nc.On("Setup", tmock.Anything, tmock.Anything, tmock.Anything, tmock.Anything).Return(nil, nil)

				is.On("SetNetworkIsolation", tmock.Anything).Return()

				ns.On("SetNetworkStatus", tmock.Anything).Return()
				ns.On("NetworkStatus", tmock.Anything).Return(nil)
			},
			alloc: func() *structs.Allocation {
				m := mock.Alloc()
				m.Job.TaskGroups[0].Networks = []*structs.NetworkResource{
					{Mode: "bridge"},
				}
				return m
			},
			expErr: errors.New("network already configured but never saved to state"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := testlog.HCLogger(t)
			ns := &mockNetworkStatus{}
			is := &mockNetworkIsolationSetter{}
			nc := &mockNetworkConfigurator{}
			d := &testutils.MockDriver{
				MockNetworkManager: testutils.MockNetworkManager{
					CreateNetworkF: func(allocID string, req *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
						return &drivers.NetworkIsolationSpec{
							Mode: "test",
						}, false, nil
					},

					DestroyNetworkF: func(allocID string, netSpec *drivers.NetworkIsolationSpec) error {
						test.NotNil(t, netSpec)
						return nil
					},
				},
			}
			h := newNetworkHook(logger, is, tc.alloc(), d, nc, ns)
			tc.setupMocks(ns, is, nc)
			err := h.Prerun(&taskenv.TaskEnv{})
			if tc.expErr != nil {
				must.ErrorContains(t, err, tc.expErr.Error())
			} else {
				must.NoError(t, err)
			}
		})
	}
}

func TestNetworkHook_Postrun(t *testing.T) {
	ci.Parallel(t)

	t.Run("network hook with spec calls teardown", func(t *testing.T) {
		destroyCalled := false
		d := &testutils.MockDriver{
			MockNetworkManager: testutils.MockNetworkManager{
				CreateNetworkF: func(allocID string, req *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
					return &drivers.NetworkIsolationSpec{
						Mode: "test",
					}, false, nil
				},

				DestroyNetworkF: func(allocID string, netSpec *drivers.NetworkIsolationSpec) error {
					destroyCalled = true
					return nil
				},
			},
		}

		nc := &mockNetworkConfigurator{}
		nc.On("Teardown", tmock.Anything, tmock.Anything, tmock.Anything).Return(nil)

		testHook := &networkHook{
			spec: &drivers.NetworkIsolationSpec{
				Mode: "test",
			},
			manager:             d,
			networkConfigurator: nc,
			alloc:               mock.Alloc(),
		}

		err := testHook.Postrun()
		must.NoError(t, err)
		must.True(t, destroyCalled)
		must.Len(t, 1, nc.Calls) // Assert teardown was called
	})

	t.Run("network hook without spec does not call teardown", func(t *testing.T) {
		destroyCalled := false
		d := &testutils.MockDriver{
			MockNetworkManager: testutils.MockNetworkManager{
				CreateNetworkF: func(allocID string, req *drivers.NetworkCreateRequest) (*drivers.NetworkIsolationSpec, bool, error) {
					return &drivers.NetworkIsolationSpec{
						Mode: "test",
					}, false, nil
				},

				DestroyNetworkF: func(allocID string, netSpec *drivers.NetworkIsolationSpec) error {
					destroyCalled = true
					return nil
				},
			},
		}

		testHook := &networkHook{
			manager: d,
			alloc:   mock.Alloc(),
		}
		err := testHook.Postrun()
		must.NoError(t, err)
		must.True(t, destroyCalled)
	})
}
