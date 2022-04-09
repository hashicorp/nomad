package serviceregistration

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
)

func Test_GetAddress(t *testing.T) {
	const HostIP = "127.0.0.1"

	testCases := []struct {
		name string

		// Parameters
		mode      string
		portLabel string
		host      map[string]int // will be converted to structs.Networks
		driver    *drivers.DriverNetwork
		ports     structs.AllocatedPorts
		status    *structs.AllocNetworkStatus

		// Results
		expectedIP   string
		expectedPort int
		expectedErr  string
	}{
		// Valid Configurations
		{
			name:      "ExampleService",
			mode:      structs.AddressModeAuto,
			portLabel: "db",
			host:      map[string]int{"db": 12435},
			driver: &drivers.DriverNetwork{
				PortMap: map[string]int{"db": 6379},
				IP:      "10.1.2.3",
			},
			expectedIP:   HostIP,
			expectedPort: 12435,
		},
		{
			name:      "host",
			mode:      structs.AddressModeHost,
			portLabel: "db",
			host:      map[string]int{"db": 12345},
			driver: &drivers.DriverNetwork{
				PortMap: map[string]int{"db": 6379},
				IP:      "10.1.2.3",
			},
			expectedIP:   HostIP,
			expectedPort: 12345,
		},
		{
			name:      "driver",
			mode:      structs.AddressModeDriver,
			portLabel: "db",
			host:      map[string]int{"db": 12345},
			driver: &drivers.DriverNetwork{
				PortMap: map[string]int{"db": 6379},
				IP:      "10.1.2.3",
			},
			expectedIP:   "10.1.2.3",
			expectedPort: 6379,
		},
		{
			name:      "AutoDriver",
			mode:      structs.AddressModeAuto,
			portLabel: "db",
			host:      map[string]int{"db": 12345},
			driver: &drivers.DriverNetwork{
				PortMap:       map[string]int{"db": 6379},
				IP:            "10.1.2.3",
				AutoAdvertise: true,
			},
			expectedIP:   "10.1.2.3",
			expectedPort: 6379,
		},
		{
			name:      "DriverCustomPort",
			mode:      structs.AddressModeDriver,
			portLabel: "7890",
			host:      map[string]int{"db": 12345},
			driver: &drivers.DriverNetwork{
				PortMap: map[string]int{"db": 6379},
				IP:      "10.1.2.3",
			},
			expectedIP:   "10.1.2.3",
			expectedPort: 7890,
		},

		// Invalid Configurations
		{
			name:        "DriverWithoutNetwork",
			mode:        structs.AddressModeDriver,
			portLabel:   "db",
			host:        map[string]int{"db": 12345},
			driver:      nil,
			expectedErr: "no driver network exists",
		},
		{
			name:      "DriverBadPort",
			mode:      structs.AddressModeDriver,
			portLabel: "bad-port-label",
			host:      map[string]int{"db": 12345},
			driver: &drivers.DriverNetwork{
				PortMap: map[string]int{"db": 6379},
				IP:      "10.1.2.3",
			},
			expectedErr: "invalid port",
		},
		{
			name:      "DriverZeroPort",
			mode:      structs.AddressModeDriver,
			portLabel: "0",
			driver: &drivers.DriverNetwork{
				IP: "10.1.2.3",
			},
			expectedErr: "invalid port",
		},
		{
			name:        "HostBadPort",
			mode:        structs.AddressModeHost,
			portLabel:   "bad-port-label",
			expectedErr: "invalid port",
		},
		{
			name:        "InvalidMode",
			mode:        "invalid-mode",
			portLabel:   "80",
			expectedErr: "invalid address mode",
		},
		{
			name:       "NoPort_AutoMode",
			mode:       structs.AddressModeAuto,
			expectedIP: HostIP,
		},
		{
			name:       "NoPort_HostMode",
			mode:       structs.AddressModeHost,
			expectedIP: HostIP,
		},
		{
			name: "NoPort_DriverMode",
			mode: structs.AddressModeDriver,
			driver: &drivers.DriverNetwork{
				IP: "10.1.2.3",
			},
			expectedIP: "10.1.2.3",
		},

		// Scenarios using port 0.12 networking fields (NetworkStatus, AllocatedPortMapping)
		{
			name:      "ExampleServer_withAllocatedPorts",
			mode:      structs.AddressModeAuto,
			portLabel: "db",
			ports: []structs.AllocatedPortMapping{
				{
					Label:  "db",
					Value:  12435,
					To:     6379,
					HostIP: HostIP,
				},
			},
			status: &structs.AllocNetworkStatus{
				InterfaceName: "eth0",
				Address:       "172.26.0.1",
			},
			expectedIP:   HostIP,
			expectedPort: 12435,
		},
		{
			name:      "Host_withAllocatedPorts",
			mode:      structs.AddressModeHost,
			portLabel: "db",
			ports: []structs.AllocatedPortMapping{
				{
					Label:  "db",
					Value:  12345,
					To:     6379,
					HostIP: HostIP,
				},
			},
			status: &structs.AllocNetworkStatus{
				InterfaceName: "eth0",
				Address:       "172.26.0.1",
			},
			expectedIP:   HostIP,
			expectedPort: 12345,
		},
		{
			name:      "Driver_withAllocatedPorts",
			mode:      structs.AddressModeDriver,
			portLabel: "db",
			ports: []structs.AllocatedPortMapping{
				{
					Label:  "db",
					Value:  12345,
					To:     6379,
					HostIP: HostIP,
				},
			},
			driver: &drivers.DriverNetwork{
				IP: "10.1.2.3",
			},
			status: &structs.AllocNetworkStatus{
				InterfaceName: "eth0",
				Address:       "172.26.0.1",
			},
			expectedIP:   "10.1.2.3",
			expectedPort: 6379,
		},
		{
			name:      "AutoDriver_withAllocatedPorts",
			mode:      structs.AddressModeAuto,
			portLabel: "db",
			ports: []structs.AllocatedPortMapping{
				{
					Label:  "db",
					Value:  12345,
					To:     6379,
					HostIP: HostIP,
				},
			},
			driver: &drivers.DriverNetwork{
				IP:            "10.1.2.3",
				AutoAdvertise: true,
			},
			status: &structs.AllocNetworkStatus{
				InterfaceName: "eth0",
				Address:       "172.26.0.1",
			},
			expectedIP:   "10.1.2.3",
			expectedPort: 6379,
		},
		{
			name:      "DriverCustomPort_withAllocatedPorts",
			mode:      structs.AddressModeDriver,
			portLabel: "7890",
			ports: []structs.AllocatedPortMapping{
				{
					Label:  "db",
					Value:  12345,
					To:     6379,
					HostIP: HostIP,
				},
			},
			driver: &drivers.DriverNetwork{
				IP: "10.1.2.3",
			},
			status: &structs.AllocNetworkStatus{
				InterfaceName: "eth0",
				Address:       "172.26.0.1",
			},
			expectedIP:   "10.1.2.3",
			expectedPort: 7890,
		},
		{
			name:      "Host_MultiHostInterface",
			mode:      structs.AddressModeAuto,
			portLabel: "db",
			ports: []structs.AllocatedPortMapping{
				{
					Label:  "db",
					Value:  12345,
					To:     6379,
					HostIP: "127.0.0.100",
				},
			},
			status: &structs.AllocNetworkStatus{
				InterfaceName: "eth0",
				Address:       "172.26.0.1",
			},
			expectedIP:   "127.0.0.100",
			expectedPort: 12345,
		},
		{
			name:      "Alloc",
			mode:      structs.AddressModeAlloc,
			portLabel: "db",
			ports: []structs.AllocatedPortMapping{
				{
					Label:  "db",
					Value:  12345,
					To:     6379,
					HostIP: HostIP,
				},
			},
			status: &structs.AllocNetworkStatus{
				InterfaceName: "eth0",
				Address:       "172.26.0.1",
			},
			expectedIP:   "172.26.0.1",
			expectedPort: 6379,
		},
		{
			name:      "Alloc no to value",
			mode:      structs.AddressModeAlloc,
			portLabel: "db",
			ports: []structs.AllocatedPortMapping{
				{
					Label:  "db",
					Value:  12345,
					HostIP: HostIP,
				},
			},
			status: &structs.AllocNetworkStatus{
				InterfaceName: "eth0",
				Address:       "172.26.0.1",
			},
			expectedIP:   "172.26.0.1",
			expectedPort: 12345,
		},
		{
			name:      "AllocCustomPort",
			mode:      structs.AddressModeAlloc,
			portLabel: "6379",
			status: &structs.AllocNetworkStatus{
				InterfaceName: "eth0",
				Address:       "172.26.0.1",
			},
			expectedIP:   "172.26.0.1",
			expectedPort: 6379,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Convert host port map into a structs.Networks.
			networks := []*structs.NetworkResource{
				{
					IP:            HostIP,
					ReservedPorts: make([]structs.Port, len(tc.host)),
				},
			}

			i := 0
			for label, port := range tc.host {
				networks[0].ReservedPorts[i].Label = label
				networks[0].ReservedPorts[i].Value = port
				i++
			}

			// Run the GetAddress function.
			actualIP, actualPort, actualErr := GetAddress(
				tc.mode, tc.portLabel, networks, tc.driver, tc.ports, tc.status)

			// Assert the results
			require.Equal(t, tc.expectedIP, actualIP, "IP mismatch")
			require.Equal(t, tc.expectedPort, actualPort, "Port mismatch")
			if tc.expectedErr == "" {
				require.Nil(t, actualErr)
			} else {
				require.Error(t, actualErr)
				require.Contains(t, actualErr.Error(), tc.expectedErr)
			}
		})
	}
}
