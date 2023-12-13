// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskenv

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// TestInterpolateServices asserts that all service
// and check fields are properly interpolated.
func TestInterpolateServices(t *testing.T) {
	ci.Parallel(t)

	services := []*structs.Service{
		{
			Name:      "${name}",
			PortLabel: "${portlabel}",
			Tags:      []string{"${tags}"},
			Meta: map[string]string{
				"meta-key": "${meta}",
			},
			CanaryMeta: map[string]string{
				"canarymeta-key": "${canarymeta}",
			},
			Address: "${address}",
			TaggedAddresses: map[string]string{
				"${ta-key}": "${ta-address}",
			},
			Checks: []*structs.ServiceCheck{
				{
					Name:          "${checkname}",
					Type:          "${checktype}",
					Command:       "${checkcmd}",
					Args:          []string{"${checkarg}"},
					Path:          "${checkstr}",
					Protocol:      "${checkproto}",
					PortLabel:     "${checklabel}",
					InitialStatus: "${checkstatus}",
					Method:        "${checkmethod}",
					Header: map[string][]string{
						"${checkheaderk}": {"${checkheaderv}"},
					},
				},
			},
		},
	}

	env := &TaskEnv{
		EnvMap: map[string]string{
			"name":         "name",
			"portlabel":    "portlabel",
			"tags":         "tags",
			"meta":         "meta-value",
			"address":      "example.com",
			"ta-key":       "public_wan",
			"ta-address":   "1.2.3.4",
			"canarymeta":   "canarymeta-value",
			"checkname":    "checkname",
			"checktype":    "checktype",
			"checkcmd":     "checkcmd",
			"checkarg":     "checkarg",
			"checkstr":     "checkstr",
			"checkpath":    "checkpath",
			"checkproto":   "checkproto",
			"checklabel":   "checklabel",
			"checkstatus":  "checkstatus",
			"checkmethod":  "checkmethod",
			"checkheaderk": "checkheaderk",
			"checkheaderv": "checkheaderv",
		},
	}

	interpolated := InterpolateServices(env, services)

	exp := []*structs.Service{
		{
			Name:      "name",
			PortLabel: "portlabel",
			Tags:      []string{"tags"},
			Meta: map[string]string{
				"meta-key": "meta-value",
			},
			CanaryMeta: map[string]string{
				"canarymeta-key": "canarymeta-value",
			},
			Address: "example.com",
			TaggedAddresses: map[string]string{
				"public_wan": "1.2.3.4",
			},
			Checks: []*structs.ServiceCheck{
				{
					Name:          "checkname",
					Type:          "checktype",
					Command:       "checkcmd",
					Args:          []string{"checkarg"},
					Path:          "checkstr",
					Protocol:      "checkproto",
					PortLabel:     "checklabel",
					InitialStatus: "checkstatus",
					Method:        "checkmethod",
					Header: map[string][]string{
						"checkheaderk": {"checkheaderv"},
					},
				},
			},
		},
	}

	require.Equal(t, exp, interpolated)
}

var testEnv = NewTaskEnv(
	map[string]string{"foo": "bar", "baz": "blah"},
	map[string]string{"foo": "bar", "baz": "blah"},
	nil, nil, "", "")

func TestInterpolate_interpolateMapStringSliceString(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		require.Nil(t, interpolateMapStringSliceString(testEnv, nil))
	})

	t.Run("not nil", func(t *testing.T) {
		require.Equal(t, map[string][]string{
			"a":   {"b"},
			"bar": {"blah", "c"},
		}, interpolateMapStringSliceString(testEnv, map[string][]string{
			"a":      {"b"},
			"${foo}": {"${baz}", "c"},
		}))
	})
}

func TestInterpolate_interpolateMapStringString(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		require.Nil(t, interpolateMapStringString(testEnv, nil))
	})

	t.Run("not nil", func(t *testing.T) {
		require.Equal(t, map[string]string{
			"a":   "b",
			"bar": "blah",
		}, interpolateMapStringString(testEnv, map[string]string{
			"a":      "b",
			"${foo}": "${baz}",
		}))
	})
}

func TestInterpolate_interpolateMapStringInterface(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		require.Nil(t, interpolateMapStringInterface(testEnv, nil))
	})

	t.Run("not nil", func(t *testing.T) {
		require.Equal(t, map[string]interface{}{
			"a":   1,
			"bar": 2,
		}, interpolateMapStringInterface(testEnv, map[string]interface{}{
			"a":      1,
			"${foo}": 2,
		}))
	})
}

func TestInterpolate_interpolateConnect(t *testing.T) {
	ci.Parallel(t)

	e := map[string]string{
		"tag1":              "_tag1",
		"port1":             "12345",
		"address1":          "1.2.3.4",
		"destination1":      "_dest1",
		"datacenter1":       "_datacenter1",
		"localbindaddress1": "127.0.0.2",
		"path1":             "_path1",
		"protocol1":         "_protocol1",
		"port2":             "_port2",
		"config1":           "_config1",
		"driver1":           "_driver1",
		"user1":             "_user1",
		"config2":           "_config2",
		"env1":              "_env1",
		"env2":              "_env2",
		"mode1":             "_mode1",
		"device1":           "_device1",
		"cidr1":             "10.0.0.0/64",
		"ip1":               "1.1.1.1",
		"server1":           "10.0.0.1",
		"search1":           "10.0.0.2",
		"option1":           "10.0.0.3",
		"port3":             "_port3",
		"network1":          "_network1",
		"port4":             "_port4",
		"network2":          "_network2",
		"resource1":         "_resource1",
		"meta1":             "_meta1",
		"meta2":             "_meta2",
		"signal1":           "_signal1",
		"bind1":             "_bind1",
		"address2":          "10.0.0.4",
		"config3":           "_config3",
		"protocol2":         "_protocol2",
		"service1":          "_service1",
		"host1":             "_host1",
	}
	env := NewTaskEnv(e, e, nil, nil, "", "")

	connect := &structs.ConsulConnect{
		Native: false,
		SidecarService: &structs.ConsulSidecarService{
			Tags: []string{"${tag1}", "tag2"},
			Port: "${port1}",
			Proxy: &structs.ConsulProxy{
				LocalServiceAddress: "${address1}",
				LocalServicePort:    10000,
				Upstreams: []structs.ConsulUpstream{{
					DestinationName:  "${destination1}",
					Datacenter:       "${datacenter1}",
					LocalBindPort:    10001,
					LocalBindAddress: "${localbindaddress1}",
					Config:           map[string]any{"${config1}": 1},
				}},
				Expose: &structs.ConsulExposeConfig{
					Paths: []structs.ConsulExposePath{{
						Path:          "${path1}",
						Protocol:      "${protocol1}",
						ListenerPort:  "${port2}",
						LocalPathPort: 10002,
					}},
				},
				Config: map[string]interface{}{
					"${config1}": 1,
					"port":       "${port1}",
				},
			},
		},
		SidecarTask: &structs.SidecarTask{
			Name:   "name", // not interpolated by taskenv
			Driver: "${driver1}",
			User:   "${user1}",
			Config: map[string]interface{}{"${config2}": 2},
			Env:    map[string]string{"${env1}": "${env2}"},
			Resources: &structs.Resources{
				CPU:      1,
				MemoryMB: 2,
				DiskMB:   3,
				IOPS:     4,
				Networks: structs.Networks{{
					Mode:   "${mode1}",
					Device: "${device1}",
					CIDR:   "${cidr1}",
					IP:     "${ip1}",
					MBits:  1,
					DNS: &structs.DNSConfig{
						Servers:  []string{"${server1}"},
						Searches: []string{"${search1}"},
						Options:  []string{"${option1}"},
					},
					ReservedPorts: []structs.Port{{
						Label:       "${port3}",
						Value:       9000,
						To:          9000,
						HostNetwork: "${network1}",
					}},
					DynamicPorts: []structs.Port{{
						Label:       "${port4}",
						Value:       9001,
						To:          9001,
						HostNetwork: "${network2}",
					}},
				}},
				Devices: structs.ResourceDevices{{
					Name: "${resource1}",
				}},
			},
			Meta:        map[string]string{"${meta1}": "${meta2}"},
			KillTimeout: pointer.Of(1 * time.Second),
			LogConfig: &structs.LogConfig{
				MaxFiles:      1,
				MaxFileSizeMB: 2,
			},
			ShutdownDelay: pointer.Of(2 * time.Second),
			KillSignal:    "${signal1}",
		},
		Gateway: &structs.ConsulGateway{
			Proxy: &structs.ConsulGatewayProxy{
				ConnectTimeout:                  pointer.Of(3 * time.Second),
				EnvoyGatewayBindTaggedAddresses: true,
				EnvoyGatewayBindAddresses: map[string]*structs.ConsulGatewayBindAddress{
					"${bind1}": {
						Address: "${address2}",
						Port:    8000,
					},
				},
				EnvoyGatewayNoDefaultBind: true,
				Config: map[string]interface{}{
					"${config3}": 4,
				},
			},
			Ingress: &structs.ConsulIngressConfigEntry{
				TLS: &structs.ConsulGatewayTLSConfig{
					Enabled: true,
				},
				Listeners: []*structs.ConsulIngressListener{{
					Protocol: "${protocol2}",
					Port:     8001,
					Services: []*structs.ConsulIngressService{{
						Name:  "${service1}",
						Hosts: []string{"${host1}", "host2"},
					}},
				}},
			},
		},
	}

	interpolateConnect(env, connect)

	require.Equal(t, &structs.ConsulConnect{
		Native: false,
		SidecarService: &structs.ConsulSidecarService{
			Tags: []string{"_tag1", "tag2"},
			Port: "12345",
			Proxy: &structs.ConsulProxy{
				LocalServiceAddress: "1.2.3.4",
				LocalServicePort:    10000,
				Upstreams: []structs.ConsulUpstream{{
					DestinationName:  "_dest1",
					Datacenter:       "_datacenter1",
					LocalBindPort:    10001,
					LocalBindAddress: "127.0.0.2",
					Config:           map[string]any{"_config1": 1},
				}},
				Expose: &structs.ConsulExposeConfig{
					Paths: []structs.ConsulExposePath{{
						Path:          "_path1",
						Protocol:      "_protocol1",
						ListenerPort:  "_port2",
						LocalPathPort: 10002,
					}},
				},
				Config: map[string]interface{}{
					"_config1": 1,
					"port":     "12345",
				},
			},
		},
		SidecarTask: &structs.SidecarTask{
			Name:   "name", // not interpolated by InterpolateServices
			Driver: "_driver1",
			User:   "_user1",
			Config: map[string]interface{}{"_config2": 2},
			Env:    map[string]string{"_env1": "_env2"},
			Resources: &structs.Resources{
				CPU:      1,
				MemoryMB: 2,
				DiskMB:   3,
				IOPS:     4,
				Networks: structs.Networks{{
					Mode:   "_mode1",
					Device: "_device1",
					CIDR:   "10.0.0.0/64",
					IP:     "1.1.1.1",
					MBits:  1,
					DNS: &structs.DNSConfig{
						Servers:  []string{"10.0.0.1"},
						Searches: []string{"10.0.0.2"},
						Options:  []string{"10.0.0.3"},
					},
					ReservedPorts: []structs.Port{{
						Label:       "_port3",
						Value:       9000,
						To:          9000,
						HostNetwork: "_network1",
					}},
					DynamicPorts: []structs.Port{{
						Label:       "_port4",
						Value:       9001,
						To:          9001,
						HostNetwork: "_network2",
					}},
				}},
				Devices: structs.ResourceDevices{{
					Name: "_resource1",
				}},
			},
			Meta:        map[string]string{"_meta1": "_meta2"},
			KillTimeout: pointer.Of(1 * time.Second),
			LogConfig: &structs.LogConfig{
				MaxFiles:      1,
				MaxFileSizeMB: 2,
			},
			ShutdownDelay: pointer.Of(2 * time.Second),
			KillSignal:    "_signal1",
		},
		Gateway: &structs.ConsulGateway{
			Proxy: &structs.ConsulGatewayProxy{
				ConnectTimeout:                  pointer.Of(3 * time.Second),
				EnvoyGatewayBindTaggedAddresses: true,
				EnvoyGatewayBindAddresses: map[string]*structs.ConsulGatewayBindAddress{
					"_bind1": {
						Address: "10.0.0.4",
						Port:    8000,
					},
				},
				EnvoyGatewayNoDefaultBind: true,
				Config: map[string]interface{}{
					"_config3": 4,
				},
			},
			Ingress: &structs.ConsulIngressConfigEntry{
				TLS: &structs.ConsulGatewayTLSConfig{
					Enabled: true,
				},
				Listeners: []*structs.ConsulIngressListener{{
					Protocol: "_protocol2",
					Port:     8001,
					Services: []*structs.ConsulIngressService{{
						Name:  "_service1",
						Hosts: []string{"_host1", "host2"},
					}},
				}},
			},
		},
	}, connect)
}
