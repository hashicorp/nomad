// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

		// Inputs
		advertise string
		mode      string
		portLabel string
		host      map[string]int // will be converted to structs.Networks
		driver    *drivers.DriverNetwork
		ports     structs.AllocatedPorts
		status    *structs.AllocNetworkStatus

		// Expectations
		expIP   string
		expPort int
		expErr  string
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
			expIP:   HostIP,
			expPort: 12435,
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
			expIP:   HostIP,
			expPort: 12345,
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
			expIP:   "10.1.2.3",
			expPort: 6379,
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
			expIP:   "10.1.2.3",
			expPort: 6379,
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
			expIP:   "10.1.2.3",
			expPort: 7890,
		},

		// Invalid Configurations
		{
			name:      "DriverWithoutNetwork",
			mode:      structs.AddressModeDriver,
			portLabel: "db",
			host:      map[string]int{"db": 12345},
			driver:    nil,
			expErr:    "no driver network exists",
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
			expErr: "invalid port",
		},
		{
			name:      "DriverZeroPort",
			mode:      structs.AddressModeDriver,
			portLabel: "0",
			driver: &drivers.DriverNetwork{
				IP: "10.1.2.3",
			},
			expErr: "invalid port",
		},
		{
			name:      "HostBadPort",
			mode:      structs.AddressModeHost,
			portLabel: "bad-port-label",
			expErr:    "invalid port",
		},
		{
			name:      "InvalidMode",
			mode:      "invalid-mode",
			portLabel: "80",
			expErr:    "invalid address mode",
		},
		{
			name:  "NoPort_AutoMode",
			mode:  structs.AddressModeAuto,
			expIP: HostIP,
		},
		{
			name:  "NoPort_HostMode",
			mode:  structs.AddressModeHost,
			expIP: HostIP,
		},
		{
			name: "NoPort_DriverMode",
			mode: structs.AddressModeDriver,
			driver: &drivers.DriverNetwork{
				IP: "10.1.2.3",
			},
			expIP: "10.1.2.3",
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
			expIP:   HostIP,
			expPort: 12435,
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
			expIP:   HostIP,
			expPort: 12345,
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
			expIP:   "10.1.2.3",
			expPort: 6379,
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
			expIP:   "10.1.2.3",
			expPort: 6379,
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
			expIP:   "10.1.2.3",
			expPort: 7890,
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
			expIP:   "127.0.0.100",
			expPort: 12345,
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
			expIP:   "172.26.0.1",
			expPort: 6379,
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
			expIP:   "172.26.0.1",
			expPort: 12345,
		},
		{
			name:      "AllocCustomPort",
			mode:      structs.AddressModeAlloc,
			portLabel: "6379",
			status: &structs.AllocNetworkStatus{
				InterfaceName: "eth0",
				Address:       "172.26.0.1",
			},
			expIP:   "172.26.0.1",
			expPort: 6379,
		},
		// Cases for setting the address field
		{
			name:      "Address",
			mode:      structs.AddressModeAuto,
			advertise: "example.com",
			expIP:     "example.com",
			expPort:   0,
		},
		{
			name:      "Address with numeric port",
			mode:      structs.AddressModeAuto,
			advertise: "example.com",
			portLabel: "8080",
			expIP:     "example.com",
			expPort:   8080,
		},
		{
			name:      "Address with mapped port",
			mode:      structs.AddressModeAuto,
			portLabel: "web",
			advertise: "example.com",
			ports: []structs.AllocatedPortMapping{
				{
					Label:  "web",
					Value:  12345,
					HostIP: HostIP,
				},
			},
			expIP:   "example.com",
			expPort: 12345,
		},
		{
			name:      "Address with invalid mapped port",
			mode:      structs.AddressModeAuto,
			advertise: "example.com",
			portLabel: "foobar",
			expErr:    `invalid port: "foobar": not a valid port mapping or numeric port`,
		},
		{
			name:      "Address with host mode",
			mode:      structs.AddressModeHost,
			advertise: "example.com",
			expErr:    `cannot use custom advertise address with "host" address mode`,
		},
		{
			name:      "Address with driver mode",
			mode:      structs.AddressModeDriver,
			advertise: "example.com",
			expErr:    `cannot use custom advertise address with "driver" address mode`,
		},
		{
			name:      "Address with alloc mode",
			mode:      structs.AddressModeAlloc,
			advertise: "example.com",
			expErr:    `cannot use custom advertise address with "alloc" address mode`,
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
				tc.advertise,
				tc.mode,
				tc.portLabel,
				networks,
				tc.driver,
				tc.ports,
				tc.status,
			)

			// Assert the results
			require.Equal(t, tc.expIP, actualIP, "IP mismatch")
			require.Equal(t, tc.expPort, actualPort, "Port mismatch")
			if tc.expErr == "" {
				require.Nil(t, actualErr)
			} else {
				require.Error(t, actualErr)
				require.Contains(t, actualErr.Error(), tc.expErr)
			}
		})
	}
}
