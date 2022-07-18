package nsd

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceRegistrationHandler_RegisterWorkload(t *testing.T) {
	testCases := []struct {
		inputCfg      *ServiceRegistrationHandlerCfg
		inputWorkload *serviceregistration.WorkloadServices
		expectedRPCs  map[string]int
		expectedError error
		name          string
	}{
		{
			inputCfg: &ServiceRegistrationHandlerCfg{
				Enabled: false,
			},
			inputWorkload: mockWorkload(),
			expectedRPCs:  map[string]int{},
			expectedError: errors.New(`service registration provider "nomad" not enabled`),
			name:          "registration disabled",
		},
		{
			inputCfg: &ServiceRegistrationHandlerCfg{
				Enabled: true,
			},
			inputWorkload: mockWorkload(),
			expectedRPCs:  map[string]int{structs.ServiceRegistrationUpsertRPCMethod: 1},
			expectedError: nil,
			name:          "registration enabled",
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
		})
	}
}

func TestServiceRegistrationHandler_RemoveWorkload(t *testing.T) {
	testCases := []struct {
		inputCfg      *ServiceRegistrationHandlerCfg
		inputWorkload *serviceregistration.WorkloadServices
		expectedRPCs  map[string]int
		expectedError error
		name          string
	}{
		{
			inputCfg: &ServiceRegistrationHandlerCfg{
				Enabled: false,
			},
			inputWorkload: mockWorkload(),
			expectedRPCs:  map[string]int{structs.ServiceRegistrationDeleteByIDRPCMethod: 2},
			expectedError: nil,
			name:          "registration disabled multiple services",
		},
		{
			inputCfg: &ServiceRegistrationHandlerCfg{
				Enabled: true,
			},
			inputWorkload: mockWorkload(),
			expectedRPCs:  map[string]int{structs.ServiceRegistrationDeleteByIDRPCMethod: 2},
			expectedError: nil,
			name:          "registration enabled multiple services",
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
		})
	}
}

func TestServiceRegistrationHandler_UpdateWorkload(t *testing.T) {
	testCases := []struct {
		cfg          *ServiceRegistrationHandlerCfg
		oldWorkload  *serviceregistration.WorkloadServices
		newWorkload  *serviceregistration.WorkloadServices
		expectedRPCs map[string]int
		name         string
	}{
		{
			name: "service names modified",
			cfg: &ServiceRegistrationHandlerCfg{
				Enabled: true,
			},
			oldWorkload: mockWorkload(),
			newWorkload: &serviceregistration.WorkloadServices{
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
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
			expectedRPCs: map[string]int{
				structs.ServiceRegistrationUpsertRPCMethod:     1,
				structs.ServiceRegistrationDeleteByIDRPCMethod: 2,
			},
		},
		{
			name: "service tags modified",
			cfg: &ServiceRegistrationHandlerCfg{
				Enabled: true,
			},
			oldWorkload: mockWorkload(),
			newWorkload: &serviceregistration.WorkloadServices{
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
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
			expectedRPCs: map[string]int{
				structs.ServiceRegistrationUpsertRPCMethod:     1,
				structs.ServiceRegistrationDeleteByIDRPCMethod: 2,
			},
		},
		{
			name: "service meta modified",
			cfg: &ServiceRegistrationHandlerCfg{
				Enabled: true,
			},
			oldWorkload: mockWorkload(),
			newWorkload: &serviceregistration.WorkloadServices{
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
				Services: []*structs.Service{
					{
						Name:        "redis-db",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "db",
						Meta:        map[string]string{"a": "1"},
					},
					{
						Name:        "redis-http",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "http",
						Meta:        map[string]string{"b": "2"},
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
		},
	}

	// Create a logger we can use for all tests.
	log := hclog.NewNullLogger()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Add the mock RPC functionality.
			rpc := mockRPC{callCounts: map[string]int{}}
			tc.cfg.RPCFn = rpc.RPC

			// Create the handler and run the tests.
			h := NewServiceRegistrationHandler(log, tc.cfg)

			require.NoError(t, h.UpdateWorkload(tc.oldWorkload, tc.newWorkload))
			require.Eventually(t, func() bool {
				return assert.Equal(t, tc.expectedRPCs, rpc.calls())
			}, 100*time.Millisecond, 10*time.Millisecond)
		})
	}

}

func TestServiceRegistrationHandler_dedupUpdatedWorkload(t *testing.T) {
	testCases := []struct {
		name              string
		oldWorkload       *serviceregistration.WorkloadServices
		newWorkload       *serviceregistration.WorkloadServices
		expectedOldOutput *serviceregistration.WorkloadServices
		expectedNewOutput *serviceregistration.WorkloadServices
	}{
		{
			name:        "service names changed",
			oldWorkload: mockWorkload(),
			newWorkload: &serviceregistration.WorkloadServices{
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
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
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
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
		},
		{
			name:        "tags updated",
			oldWorkload: mockWorkload(),
			newWorkload: &serviceregistration.WorkloadServices{
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
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
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
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
			expectedNewOutput: &serviceregistration.WorkloadServices{
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
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
		},
		{
			name:        "canary tags updated",
			oldWorkload: mockWorkload(),
			newWorkload: &serviceregistration.WorkloadServices{
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
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
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
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
		},
		{
			name:        "meta updated",
			oldWorkload: mockWorkload(),
			newWorkload: &serviceregistration.WorkloadServices{
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
				Services: []*structs.Service{
					{
						Name:        "redis-db",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "db",
						Meta:        map[string]string{"a": "1"},
					},
					{
						Name:        "redis-http",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "http",
						Meta:        map[string]string{"b": "2"},
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
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
				Services:  []*structs.Service{
					// nothing
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
			expectedNewOutput: &serviceregistration.WorkloadServices{
				AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
				Task:      "redis",
				Group:     "cache",
				JobID:     "example",
				Canary:    false,
				Namespace: "default",
				Services: []*structs.Service{
					{
						Name:        "redis-db",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "db",
						Meta:        map[string]string{"a": "1"},
					},
					{
						Name:        "redis-http",
						AddressMode: structs.AddressModeHost,
						PortLabel:   "http",
						Meta:        map[string]string{"b": "2"},
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
		},
	}

	s := &ServiceRegistrationHandler{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOld, actualNew := s.deDupUpdatedWorkload(tc.oldWorkload, tc.newWorkload)
			_ = actualOld
			require.ElementsMatch(t, tc.expectedOldOutput.Services, actualOld.Services)
			require.ElementsMatch(t, tc.expectedNewOutput.Services, actualNew.Services)
		})
	}
}

func mockWorkload() *serviceregistration.WorkloadServices {
	return &serviceregistration.WorkloadServices{
		AllocID:   "98ea220b-7ebe-4662-6d74-9868e797717c",
		Task:      "redis",
		Group:     "cache",
		JobID:     "example",
		Canary:    false,
		Namespace: "default",
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
