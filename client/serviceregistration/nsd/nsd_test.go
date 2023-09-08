// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nsd

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCheckWatcher struct {
	lock sync.Mutex

	watchCalls   int
	unWatchCalls int
}

func (cw *mockCheckWatcher) Run(_ context.Context) {
	// Run runs async; just assume it ran
}

func (cw *mockCheckWatcher) Watch(_, _, _ string, _ *structs.ServiceCheck, _ serviceregistration.WorkloadRestarter) {
	cw.lock.Lock()
	defer cw.lock.Unlock()
	cw.watchCalls++
}

func (cw *mockCheckWatcher) Unwatch(_ string) {
	cw.lock.Lock()
	defer cw.lock.Unlock()
	cw.unWatchCalls++
}

func (cw *mockCheckWatcher) assert(t *testing.T, watchCalls, unWatchCalls int) {
	cw.lock.Lock()
	defer cw.lock.Unlock()
	test.Eq(t, watchCalls, cw.watchCalls, test.Sprintf("expected %d Watch() calls but got %d", watchCalls, cw.watchCalls))
	test.Eq(t, unWatchCalls, cw.unWatchCalls, test.Sprintf("expected %d Unwatch() calls but got %d", unWatchCalls, cw.unWatchCalls))
}

func TestServiceRegistrationHandler_RegisterWorkload(t *testing.T) {
	testCases := []struct {
		name                 string
		inputCfg             *ServiceRegistrationHandlerCfg
		inputWorkload        *serviceregistration.WorkloadServices
		expectedRPCs         map[string]int
		expectedError        error
		expWatch, expUnWatch int
	}{
		{
			name: "registration disabled",
			inputCfg: &ServiceRegistrationHandlerCfg{
				Enabled:      false,
				CheckWatcher: new(mockCheckWatcher),
			},
			inputWorkload: mockWorkload(),
			expectedRPCs:  map[string]int{},
			expectedError: errors.New(`service registration provider "nomad" not enabled`),
			expWatch:      0,
			expUnWatch:    0,
		},
		{
			name: "registration enabled",
			inputCfg: &ServiceRegistrationHandlerCfg{
				Enabled:      true,
				CheckWatcher: new(mockCheckWatcher),
			},
			inputWorkload: mockWorkload(),
			expectedRPCs:  map[string]int{structs.ServiceRegistrationUpsertRPCMethod: 1},
			expectedError: nil,
			expWatch:      1,
			expUnWatch:    0,
		},
	}

	// Create a logger we can use for all tests.
	log := hclog.NewNullLogger()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Add the mock RPC functionality.
			mockRPC := mockRPC{callCounts: map[string]int{}}
			tc.inputCfg.RPCFn = mockRPC.RPC

			// Create the handler and run the tests.
			h := NewServiceRegistrationHandler(log, tc.inputCfg)

			actualErr := h.RegisterWorkload(tc.inputWorkload)
			require.Equal(t, tc.expectedError, actualErr)
			require.Equal(t, tc.expectedRPCs, mockRPC.calls())
			tc.inputCfg.CheckWatcher.(*mockCheckWatcher).assert(t, tc.expWatch, tc.expUnWatch)
		})
	}
}

func TestServiceRegistrationHandler_RemoveWorkload(t *testing.T) {
	testCases := []struct {
		name                 string
		inputCfg             *ServiceRegistrationHandlerCfg
		inputWorkload        *serviceregistration.WorkloadServices
		expectedRPCs         map[string]int
		expectedError        error
		expWatch, expUnWatch int
	}{
		{
			name: "registration disabled multiple services",
			inputCfg: &ServiceRegistrationHandlerCfg{
				Enabled:      false,
				CheckWatcher: new(mockCheckWatcher),
			},
			inputWorkload: mockWorkload(),
			expectedRPCs:  map[string]int{structs.ServiceRegistrationDeleteByIDRPCMethod: 2},
			expectedError: nil,
			expWatch:      0,
			expUnWatch:    2, // RemoveWorkload works regardless if provider is enabled
		},
		{
			name: "registration enabled multiple services",
			inputCfg: &ServiceRegistrationHandlerCfg{
				Enabled:      true,
				CheckWatcher: new(mockCheckWatcher),
			},
			inputWorkload: mockWorkload(),
			expectedRPCs:  map[string]int{structs.ServiceRegistrationDeleteByIDRPCMethod: 2},
			expectedError: nil,
			expWatch:      0,
			expUnWatch:    2,
		},
	}

	// Create a logger we can use for all tests.
	log := hclog.NewNullLogger()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Add the mock RPC functionality.
			mockRPC := mockRPC{callCounts: map[string]int{}}
			tc.inputCfg.RPCFn = mockRPC.RPC

			// Create the handler and run the tests.
			h := NewServiceRegistrationHandler(log, tc.inputCfg)

			h.RemoveWorkload(tc.inputWorkload)

			require.Eventually(t, func() bool {
				return assert.Equal(t, tc.expectedRPCs, mockRPC.calls())
			}, 100*time.Millisecond, 10*time.Millisecond)
			tc.inputCfg.CheckWatcher.(*mockCheckWatcher).assert(t, tc.expWatch, tc.expUnWatch)
		})
	}
}

func TestServiceRegistrationHandler_UpdateWorkload(t *testing.T) {
	testCases := []struct {
		name                 string
		inputCfg             *ServiceRegistrationHandlerCfg
		inputOldWorkload     *serviceregistration.WorkloadServices
		inputNewWorkload     *serviceregistration.WorkloadServices
		expectedRPCs         map[string]int
		expectedError        error
		expWatch, expUnWatch int
	}{
		{
			name: "delete and upsert",
			inputCfg: &ServiceRegistrationHandlerCfg{
				Enabled:      true,
				CheckWatcher: new(mockCheckWatcher),
			},
			inputOldWorkload: mockWorkload(),
			inputNewWorkload: &serviceregistration.WorkloadServices{
				AllocInfo: structs.AllocInfo{
					AllocID: "98ea220b-7ebe-4662-6d74-9868e797717c",
					Task:    "redis",
					Group:   "cache",
					JobID:   "example",
				},
				Canary:            false,
				ProviderNamespace: "default",
				Services: []*structs.Service{
					{
						Name:        "changed-redis-db",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "db",
						Checks: []*structs.ServiceCheck{
							{
								Name:         "changed-check-redis-db",
								CheckRestart: &structs.CheckRestart{Limit: 1},
							},
						},
					},
					{
						Name:        "changed-redis-http",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "http",
						// No check restart this time
					},
				},
				Ports: []structs.AllocatedPortMapping{
					{
						Label:  "db",
						HostIP: "10.10.13.2",
						Value:  23098,
					},
					{
						Label:  "http",
						HostIP: "10.10.13.2",
						Value:  24098,
					},
				},
			},
			expectedRPCs: map[string]int{
				structs.ServiceRegistrationUpsertRPCMethod:     1,
				structs.ServiceRegistrationDeleteByIDRPCMethod: 2,
			},
			expectedError: nil,
			expWatch:      1,
			expUnWatch:    2,
		},
		{
			name: "upsert only",
			inputCfg: &ServiceRegistrationHandlerCfg{
				Enabled:      true,
				CheckWatcher: new(mockCheckWatcher),
			},
			inputOldWorkload: mockWorkload(),
			inputNewWorkload: &serviceregistration.WorkloadServices{
				AllocInfo: structs.AllocInfo{
					AllocID: "98ea220b-7ebe-4662-6d74-9868e797717c",
					Task:    "redis",
					Group:   "cache",
					JobID:   "example",
				},
				Canary:            false,
				ProviderNamespace: "default",
				Services: []*structs.Service{
					{
						Name:        "redis-db",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "db",
						Tags:        []string{"foo"},
						Checks: []*structs.ServiceCheck{
							{
								Name:         "redis-db-check-1",
								CheckRestart: &structs.CheckRestart{Limit: 1},
							},
							{
								Name: "redis-db-check-2",
								// No check restart on this one
							},
						},
					},
					{
						Name:        "redis-http",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "http",
						Tags:        []string{"bar"},
						Checks: []*structs.ServiceCheck{
							{
								Name:         "redis-http-check-1",
								CheckRestart: &structs.CheckRestart{Limit: 1},
							},
							{
								Name:         "redis-http-check-2",
								CheckRestart: &structs.CheckRestart{Limit: 1},
							},
						},
					},
				},
				Ports: []structs.AllocatedPortMapping{
					{
						Label:  "db",
						HostIP: "10.10.13.2",
						Value:  23098,
					},
					{
						Label:  "http",
						HostIP: "10.10.13.2",
						Value:  24098,
					},
				},
			},
			expectedRPCs: map[string]int{
				structs.ServiceRegistrationUpsertRPCMethod: 1,
			},
			expectedError: nil,
			expWatch:      3,
			expUnWatch:    0,
		},
	}

	// Create a logger we can use for all tests.
	log := hclog.NewNullLogger()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Add the mock RPC functionality.
			mockRPC := mockRPC{callCounts: map[string]int{}}
			tc.inputCfg.RPCFn = mockRPC.RPC

			// Create the handler and run the tests.
			h := NewServiceRegistrationHandler(log, tc.inputCfg)

			require.Equal(t, tc.expectedError, h.UpdateWorkload(tc.inputOldWorkload, tc.inputNewWorkload))

			require.Eventually(t, func() bool {
				return assert.Equal(t, tc.expectedRPCs, mockRPC.calls())
			}, 100*time.Millisecond, 10*time.Millisecond)
			tc.inputCfg.CheckWatcher.(*mockCheckWatcher).assert(t, tc.expWatch, tc.expUnWatch)
		})
	}

}

func TestServiceRegistrationHandler_dedupUpdatedWorkload(t *testing.T) {
	testCases := []struct {
		inputOldWorkload  *serviceregistration.WorkloadServices
		inputNewWorkload  *serviceregistration.WorkloadServices
		expectedOldOutput *serviceregistration.WorkloadServices
		expectedNewOutput *serviceregistration.WorkloadServices
		name              string
	}{
		{
			inputOldWorkload: mockWorkload(),
			inputNewWorkload: &serviceregistration.WorkloadServices{
				AllocInfo: structs.AllocInfo{
					AllocID: "98ea220b-7ebe-4662-6d74-9868e797717c",
					Task:    "redis",
					Group:   "cache",
					JobID:   "example",
				},
				Canary:            false,
				ProviderNamespace: "default",
				Services: []*structs.Service{
					{
						Name:        "changed-redis-db",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "db",
					},
					{
						Name:        "changed-redis-http",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "http",
					},
				},
				Ports: []structs.AllocatedPortMapping{
					{
						Label:  "db",
						HostIP: "10.10.13.2",
						Value:  23098,
					},
					{
						Label:  "http",
						HostIP: "10.10.13.2",
						Value:  24098,
					},
				},
			},
			expectedOldOutput: mockWorkload(),
			expectedNewOutput: &serviceregistration.WorkloadServices{
				AllocInfo: structs.AllocInfo{
					AllocID: "98ea220b-7ebe-4662-6d74-9868e797717c",
					Task:    "redis",
					Group:   "cache",
					JobID:   "example",
				},
				Canary:            false,
				ProviderNamespace: "default",
				Services: []*structs.Service{
					{
						Name:        "changed-redis-db",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "db",
					},
					{
						Name:        "changed-redis-http",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "http",
					},
				},
				Ports: []structs.AllocatedPortMapping{
					{
						Label:  "db",
						HostIP: "10.10.13.2",
						Value:  23098,
					},
					{
						Label:  "http",
						HostIP: "10.10.13.2",
						Value:  24098,
					},
				},
			},
			name: "service names changed",
		},
		{
			inputOldWorkload: mockWorkload(),
			inputNewWorkload: &serviceregistration.WorkloadServices{
				AllocInfo: structs.AllocInfo{
					AllocID: "98ea220b-7ebe-4662-6d74-9868e797717c",
					Task:    "redis",
					Group:   "cache",
					JobID:   "example",
				},
				Canary:            false,
				ProviderNamespace: "default",
				Services: []*structs.Service{
					{
						Name:        "redis-db",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "db",
						Tags:        []string{"foo"},
					},
					{
						Name:        "redis-http",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "http",
						Tags:        []string{"bar"},
					},
				},
				Ports: []structs.AllocatedPortMapping{
					{
						Label:  "db",
						HostIP: "10.10.13.2",
						Value:  23098,
					},
					{
						Label:  "http",
						HostIP: "10.10.13.2",
						Value:  24098,
					},
				},
			},
			expectedOldOutput: &serviceregistration.WorkloadServices{
				AllocInfo: structs.AllocInfo{
					AllocID: "98ea220b-7ebe-4662-6d74-9868e797717c",
					Task:    "redis",
					Group:   "cache",
					JobID:   "example",
				},
				Canary:            false,
				ProviderNamespace: "default",
				Services:          []*structs.Service{},
				Ports: []structs.AllocatedPortMapping{
					{
						Label:  "db",
						HostIP: "10.10.13.2",
						Value:  23098,
					},
					{
						Label:  "http",
						HostIP: "10.10.13.2",
						Value:  24098,
					},
				},
			},
			expectedNewOutput: &serviceregistration.WorkloadServices{
				AllocInfo: structs.AllocInfo{
					AllocID: "98ea220b-7ebe-4662-6d74-9868e797717c",
					Task:    "redis",
					Group:   "cache",
					JobID:   "example",
				},
				Canary:            false,
				ProviderNamespace: "default",
				Services: []*structs.Service{
					{
						Name:        "redis-db",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "db",
						Tags:        []string{"foo"},
					},
					{
						Name:        "redis-http",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "http",
						Tags:        []string{"bar"},
					},
				},
				Ports: []structs.AllocatedPortMapping{
					{
						Label:  "db",
						HostIP: "10.10.13.2",
						Value:  23098,
					},
					{
						Label:  "http",
						HostIP: "10.10.13.2",
						Value:  24098,
					},
				},
			},
			name: "tags updated",
		},
		{
			inputOldWorkload: mockWorkload(),
			inputNewWorkload: &serviceregistration.WorkloadServices{
				AllocInfo: structs.AllocInfo{
					AllocID: "98ea220b-7ebe-4662-6d74-9868e797717c",
					Task:    "redis",
					Group:   "cache",
					JobID:   "example",
				},
				Canary:            false,
				ProviderNamespace: "default",
				Services: []*structs.Service{
					{
						Name:        "redis-db",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "dbs",
					},
					{
						Name:        "redis-http",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "https",
					},
				},
				Ports: []structs.AllocatedPortMapping{
					{
						Label:  "dbs",
						HostIP: "10.10.13.2",
						Value:  23098,
					},
					{
						Label:  "https",
						HostIP: "10.10.13.2",
						Value:  24098,
					},
				},
			},
			expectedOldOutput: mockWorkload(),
			expectedNewOutput: &serviceregistration.WorkloadServices{
				AllocInfo: structs.AllocInfo{
					AllocID: "98ea220b-7ebe-4662-6d74-9868e797717c",
					Task:    "redis",
					Group:   "cache",
					JobID:   "example",
				},
				Canary:            false,
				ProviderNamespace: "default",
				Services: []*structs.Service{
					{
						Name:        "redis-db",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "dbs",
					},
					{
						Name:        "redis-http",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "https",
					},
				},
				Ports: []structs.AllocatedPortMapping{
					{
						Label:  "dbs",
						HostIP: "10.10.13.2",
						Value:  23098,
					},
					{
						Label:  "https",
						HostIP: "10.10.13.2",
						Value:  24098,
					},
				},
			},
			name: "canary tags updated",
		},
	}

	s := &ServiceRegistrationHandler{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOld, actualNew := s.dedupUpdatedWorkload(tc.inputOldWorkload, tc.inputNewWorkload)
			require.ElementsMatch(t, tc.expectedOldOutput.Services, actualOld.Services)
			require.ElementsMatch(t, tc.expectedNewOutput.Services, actualNew.Services)
		})
	}
}

func mockWorkload() *serviceregistration.WorkloadServices {
	return &serviceregistration.WorkloadServices{
		AllocInfo: structs.AllocInfo{
			AllocID: "98ea220b-7ebe-4662-6d74-9868e797717c",
			Task:    "redis",
			Group:   "cache",
			JobID:   "example",
		},
		Canary:            false,
		ProviderNamespace: "default",
		Services: []*structs.Service{
			{
				Name:        "redis-db",
				AddressMode: structs.AddressModeHost,
				PortLabel:   "db",
			},
			{
				Name:        "redis-http",
				AddressMode: structs.AddressModeHost,
				PortLabel:   "http",
				Checks: []*structs.ServiceCheck{
					{
						Name:     "check1",
						Type:     "http",
						Interval: 5 * time.Second,
						Timeout:  1 * time.Second,
						CheckRestart: &structs.CheckRestart{
							Limit: 1,
							Grace: 1,
						},
					},
				},
			},
		},
		Ports: []structs.AllocatedPortMapping{
			{
				Label:  "db",
				HostIP: "10.10.13.2",
				Value:  23098,
			},
			{
				Label:  "http",
				HostIP: "10.10.13.2",
				Value:  24098,
			},
		},
	}
}

// mockRPC mocks and tracks RPC calls made for testing.
type mockRPC struct {

	// callCounts tracks how many times each RPC method has been called. The
	// lock should be used to access this.
	callCounts map[string]int
	l          sync.RWMutex
}

// calls returns the mapping counting the number of calls made to each RPC
// method.
func (mr *mockRPC) calls() map[string]int {
	mr.l.RLock()
	defer mr.l.RUnlock()
	return mr.callCounts
}

// RPC mocks the server RPCs, acting as though any request succeeds.
func (mr *mockRPC) RPC(method string, _, _ interface{}) error {
	switch method {
	case structs.ServiceRegistrationUpsertRPCMethod, structs.ServiceRegistrationDeleteByIDRPCMethod:
		mr.l.Lock()
		mr.callCounts[method]++
		mr.l.Unlock()
		return nil
	default:
		return fmt.Errorf("unexpected RPC method: %v", method)
	}
}
