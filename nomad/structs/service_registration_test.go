package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceRegistration_Copy(t *testing.T) {
	sr := &ServiceRegistration{
		ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
		ServiceName: "example-cache",
		Namespace:   "default",
		NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
		Datacenter:  "dc1",
		JobID:       "example",
		AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
		Tags:        []string{"foo"},
		Address:     "192.168.13.13",
		Port:        23813,
	}
	newSR := sr.Copy()
	require.True(t, sr.Equals(newSR))
}

func TestServiceRegistration_Equal(t *testing.T) {
	testCases := []struct {
		serviceReg1    *ServiceRegistration
		serviceReg2    *ServiceRegistration
		expectedOutput bool
		name           string
	}{
		{
			serviceReg1: nil,
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			expectedOutput: false,
			name:           "nil service registration composed",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2:    nil,
			expectedOutput: false,
			name:           "nil service registration func input",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-group-2873cf75-42e5-7c45-ca1c-415f3e18be3dcache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			expectedOutput: false,
			name:           "ID not equal",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "platform-example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			expectedOutput: false,
			name:           "service name not equal",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "ba991c17-7ce5-9c20-78b7-311e63578583",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			expectedOutput: false,
			name:           "node ID not equal",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc2",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			expectedOutput: false,
			name:           "datacenter not equal",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "platform-example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			expectedOutput: false,
			name:           "job ID not equal",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "ba991c17-7ce5-9c20-78b7-311e63578583",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			expectedOutput: false,
			name:           "alloc ID not equal",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "platform",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			expectedOutput: false,
			name:           "namespace not equal",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "10.10.13.13",
				Port:        23813,
			},
			expectedOutput: false,
			name:           "address not equal",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        33813,
			},
			expectedOutput: false,
			name:           "port not equal",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"canary"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			expectedOutput: false,
			name:           "tags not equal",
		},
		{
			serviceReg1: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			serviceReg2: &ServiceRegistration{
				ID:          "_nomad-task-2873cf75-42e5-7c45-ca1c-415f3e18be3d-group-cache-example-cache-db",
				ServiceName: "example-cache",
				Namespace:   "default",
				NodeID:      "17a6d1c0-811e-2ca9-ded0-3d5d6a54904c",
				Datacenter:  "dc1",
				JobID:       "example",
				AllocID:     "2873cf75-42e5-7c45-ca1c-415f3e18be3d",
				Tags:        []string{"foo"},
				Address:     "192.168.13.13",
				Port:        23813,
			},
			expectedOutput: true,
			name:           "both equal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := tc.serviceReg1.Equals(tc.serviceReg2)
			require.Equal(t, tc.expectedOutput, actualOutput)
		})
	}
}

func TestServiceRegistrationListRequest_StaleReadSupport(t *testing.T) {
	req := &ServiceRegistrationListRequest{}
	require.True(t, req.IsRead())
}

func TestServiceRegistrationByNameRequest_StaleReadSupport(t *testing.T) {
	req := &ServiceRegistrationByNameRequest{}
	require.True(t, req.IsRead())
}
