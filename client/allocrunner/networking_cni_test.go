// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package allocrunner

import (
	"errors"
	"net"
	"testing"

	"github.com/containerd/go-cni"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/hashicorp/consul/sdk/iptables"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

type mockIPTables struct {
	listCall  [2]string
	listRules []string
	listErr   error

	deleteCall [2]string
	deleteErr  error

	clearCall [2]string
	clearErr  error
}

func (ipt *mockIPTables) List(table, chain string) ([]string, error) {
	ipt.listCall[0], ipt.listCall[1] = table, chain
	return ipt.listRules, ipt.listErr
}

func (ipt *mockIPTables) Delete(table, chain string, rule ...string) error {
	ipt.deleteCall[0], ipt.deleteCall[1] = table, chain
	return ipt.deleteErr
}

func (ipt *mockIPTables) ClearAndDeleteChain(table, chain string) error {
	ipt.clearCall[0], ipt.clearCall[1] = table, chain
	return ipt.clearErr
}

func (ipt *mockIPTables) assert(t *testing.T, jumpChain string) {
	// List assertions
	must.Eq(t, "nat", ipt.listCall[0])
	must.Eq(t, "POSTROUTING", ipt.listCall[1])

	// Delete assertions
	must.Eq(t, "nat", ipt.deleteCall[0])
	must.Eq(t, "POSTROUTING", ipt.deleteCall[1])

	// Clear assertions
	must.Eq(t, "nat", ipt.clearCall[0])
	must.Eq(t, jumpChain, ipt.clearCall[1])
}

func TestCNI_forceCleanup(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		c := cniNetworkConfigurator{logger: testlog.HCLogger(t)}
		ipt := &mockIPTables{
			listRules: []string{
				`-A POSTROUTING -m comment --comment "CNI portfwd requiring masquerade" -j CNI-HOSTPORT-MASQ`,
				`-A POSTROUTING -s 172.17.0.0/16 ! -o docker0 -j MASQUERADE`,
				`-A POSTROUTING -s 172.26.64.216/32 -m comment --comment "name: \"nomad\" id: \"79e8bf2e-a9c8-70ac-8d4e-fa5c4da99fbf\"" -j CNI-f2338c31d4de44472fe99c43`,
				`-A POSTROUTING -s 172.26.64.217/32 -m comment --comment "name: \"nomad\" id: \"2dd71cac-2b1e-ff08-167c-735f7f9f4964\"" -j CNI-5d36f286cfbb35c5776509ec`,
				`-A POSTROUTING -s 172.26.64.218/32 -m comment --comment "name: \"nomad\" id: \"5ff6deb7-9bc1-1491-f20c-e87b15de501d\"" -j CNI-2fe7686eac2fe43714a7b850`,
				`-A POSTROUTING -m mark --mark 0x2000/0x2000 -j MASQUERADE`,
				`-A POSTROUTING -m comment --comment "CNI portfwd masquerade mark" -j MARK --set-xmark 0x2000/0x2000`,
			},
		}
		err := c.forceCleanup(ipt, "2dd71cac-2b1e-ff08-167c-735f7f9f4964")
		must.NoError(t, err)
		ipt.assert(t, "CNI-5d36f286cfbb35c5776509ec")
	})

	t.Run("missing allocation", func(t *testing.T) {
		c := cniNetworkConfigurator{logger: testlog.HCLogger(t)}
		ipt := &mockIPTables{
			listRules: []string{
				`-A POSTROUTING -m comment --comment "CNI portfwd requiring masquerade" -j CNI-HOSTPORT-MASQ`,
				`-A POSTROUTING -s 172.17.0.0/16 ! -o docker0 -j MASQUERADE`,
				`-A POSTROUTING -s 172.26.64.216/32 -m comment --comment "name: \"nomad\" id: \"79e8bf2e-a9c8-70ac-8d4e-fa5c4da99fbf\"" -j CNI-f2338c31d4de44472fe99c43`,
				`-A POSTROUTING -s 172.26.64.217/32 -m comment --comment "name: \"nomad\" id: \"262d57a7-8f85-f3a4-9c3b-120c00ccbff1\"" -j CNI-5d36f286cfbb35c5776509ec`,
				`-A POSTROUTING -s 172.26.64.218/32 -m comment --comment "name: \"nomad\" id: \"5ff6deb7-9bc1-1491-f20c-e87b15de501d\"" -j CNI-2fe7686eac2fe43714a7b850`,
				`-A POSTROUTING -m mark --mark 0x2000/0x2000 -j MASQUERADE`,
				`-A POSTROUTING -m comment --comment "CNI portfwd masquerade mark" -j MARK --set-xmark 0x2000/0x2000`,
			},
		}
		err := c.forceCleanup(ipt, "2dd71cac-2b1e-ff08-167c-735f7f9f4964")
		must.EqError(t, err, "failed to find postrouting rule for alloc 2dd71cac-2b1e-ff08-167c-735f7f9f4964")
	})

	t.Run("list error", func(t *testing.T) {
		c := cniNetworkConfigurator{logger: testlog.HCLogger(t)}
		ipt := &mockIPTables{listErr: errors.New("list error")}
		err := c.forceCleanup(ipt, "2dd71cac-2b1e-ff08-167c-735f7f9f4964")
		must.EqError(t, err, "failed to list iptables rules: list error")
	})

	t.Run("delete error", func(t *testing.T) {
		c := cniNetworkConfigurator{logger: testlog.HCLogger(t)}
		ipt := &mockIPTables{
			deleteErr: errors.New("delete error"),
			listRules: []string{
				`-A POSTROUTING -s 172.26.64.217/32 -m comment --comment "name: \"nomad\" id: \"2dd71cac-2b1e-ff08-167c-735f7f9f4964\"" -j CNI-5d36f286cfbb35c5776509ec`,
			},
		}
		err := c.forceCleanup(ipt, "2dd71cac-2b1e-ff08-167c-735f7f9f4964")
		must.EqError(t, err, "failed to cleanup iptables rules for alloc 2dd71cac-2b1e-ff08-167c-735f7f9f4964")
	})

	t.Run("clear error", func(t *testing.T) {
		c := cniNetworkConfigurator{logger: testlog.HCLogger(t)}
		ipt := &mockIPTables{
			clearErr: errors.New("clear error"),
			listRules: []string{
				`-A POSTROUTING -s 172.26.64.217/32 -m comment --comment "name: \"nomad\" id: \"2dd71cac-2b1e-ff08-167c-735f7f9f4964\"" -j CNI-5d36f286cfbb35c5776509ec`,
			},
		}
		err := c.forceCleanup(ipt, "2dd71cac-2b1e-ff08-167c-735f7f9f4964")
		must.EqError(t, err, "failed to cleanup iptables rules for alloc 2dd71cac-2b1e-ff08-167c-735f7f9f4964")
	})
}

// TestCNI_cniToAllocNet_NoInterfaces asserts an error is returned if cni.Result
// contains no interfaces.
func TestCNI_cniToAllocNet_NoInterfaces(t *testing.T) {
	ci.Parallel(t)

	cniResult := &cni.Result{}

	// Only need a logger
	c := &cniNetworkConfigurator{
		logger: testlog.HCLogger(t),
	}
	allocNet, err := c.cniToAllocNet(cniResult)
	require.Error(t, err)
	require.Nil(t, allocNet)
}

// TestCNI_cniToAllocNet_Fallback asserts if a CNI plugin result lacks an IP on
// its sandbox interface, the first IP found is used.
func TestCNI_cniToAllocNet_Fallback(t *testing.T) {
	ci.Parallel(t)

	cniResult := &cni.Result{
		// Calico's CNI plugin v3.12.3 has been observed to return the following:
		Interfaces: map[string]*cni.Config{
			"cali39179aa3-74": {},
			"eth0": {
				IPConfigs: []*cni.IPConfig{
					{
						IP: net.IPv4(192, 168, 135, 232),
					},
				},
			},
		},
		// cni.Result will return a single empty struct, not an empty slice
		DNS: []types.DNS{{}},
	}

	// Only need a logger
	c := &cniNetworkConfigurator{
		logger: testlog.HCLogger(t),
	}
	allocNet, err := c.cniToAllocNet(cniResult)
	must.NoError(t, err)
	must.NotNil(t, allocNet)
	test.Eq(t, "192.168.135.232", allocNet.Address)
	test.Eq(t, "eth0", allocNet.InterfaceName)
	test.Nil(t, allocNet.DNS)
}

// TestCNI_cniToAllocNet_Invalid asserts an error is returned if a CNI plugin
// result lacks any IP addresses. This has not been observed, but Nomad still
// must guard against invalid results from external plugins.
func TestCNI_cniToAllocNet_Invalid(t *testing.T) {
	ci.Parallel(t)

	cniResult := &cni.Result{
		Interfaces: map[string]*cni.Config{
			"eth0": {},
			"veth1": {
				IPConfigs: []*cni.IPConfig{},
			},
		},
	}

	// Only need a logger
	c := &cniNetworkConfigurator{
		logger: testlog.HCLogger(t),
	}
	allocNet, err := c.cniToAllocNet(cniResult)
	require.Error(t, err)
	require.Nil(t, allocNet)
}

func TestCNI_setupTproxyArgs(t *testing.T) {
	ci.Parallel(t)

	nodeMeta := map[string]string{
		"connect.transparent_proxy.default_outbound_port": "15001",
		"connect.transparent_proxy.default_uid":           "101",
	}

	nodeAttrs := map[string]string{
		"consul.dns.addr": "192.168.1.117",
		"consul.dns.port": "8600",
	}

	alloc := mock.ConnectAlloc()

	// need to setup the NetworkResource to have the expected port mapping for
	// the services we create
	alloc.AllocatedResources.Shared.Networks = []*structs.NetworkResource{
		{
			Mode: "bridge",
			IP:   "10.0.0.1",
			ReservedPorts: []structs.Port{
				{
					Label: "http",
					Value: 9002,
					To:    9002,
				},
				{
					Label: "health",
					Value: 9001,
					To:    9000,
				},
			},
			DynamicPorts: []structs.Port{
				{
					Label: "connect-proxy-testconnect",
					Value: 25018,
					To:    25018,
				},
			},
		},
	}

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	tg.Networks = []*structs.NetworkResource{{
		Mode: "bridge",
		DNS:  &structs.DNSConfig{},
		ReservedPorts: []structs.Port{ // non-Connect port
			{
				Label:       "http",
				Value:       9002,
				To:          9002,
				HostNetwork: "default",
			},
		},
		DynamicPorts: []structs.Port{ // Connect port
			{
				Label:       "connect-proxy-count-dashboard",
				Value:       0,
				To:          -1,
				HostNetwork: "default",
			},
			{
				Label:       "health",
				Value:       0,
				To:          9000,
				HostNetwork: "default",
			},
		},
	}}
	tg.Services[0].PortLabel = "9002"
	tg.Services[0].Connect.SidecarService.Proxy = &structs.ConsulProxy{
		LocalServiceAddress: "",
		LocalServicePort:    0,
		Upstreams:           []structs.ConsulUpstream{},
		Expose:              &structs.ConsulExposeConfig{},
		Config:              map[string]interface{}{},
	}

	spec := &drivers.NetworkIsolationSpec{
		Mode:        "group",
		Path:        "/var/run/docker/netns/a2ece01ea7bc",
		Labels:      map[string]string{"docker_sandbox_container_id": "4a77cdaad5"},
		HostsConfig: &drivers.HostsConfig{},
	}

	portMaps := getPortMapping(alloc, false)

	testCases := []struct {
		name           string
		cluster        string
		tproxySpec     *structs.ConsulTransparentProxy
		exposeSpec     *structs.ConsulExposeConfig
		nodeAttrs      map[string]string
		expectIPConfig *iptables.Config
		expectErr      string
	}{
		{
			name: "nil tproxy spec returns no error or iptables config",
		},
		{
			name:       "minimal empty tproxy spec returns defaults",
			tproxySpec: &structs.ConsulTransparentProxy{},
			expectIPConfig: &iptables.Config{
				ConsulDNSIP:         "192.168.1.117",
				ConsulDNSPort:       8600,
				ProxyUserID:         "101",
				ProxyInboundPort:    25018,
				ProxyOutboundPort:   15001,
				ExcludeInboundPorts: []string{"9002"},
				NetNS:               "/var/run/docker/netns/a2ece01ea7bc",
			},
		},
		{
			name: "tproxy spec with overrides",
			tproxySpec: &structs.ConsulTransparentProxy{
				UID:                  "1001",
				OutboundPort:         16001,
				ExcludeInboundPorts:  []string{"http", "9000"},
				ExcludeOutboundPorts: []uint16{443, 80},
				ExcludeOutboundCIDRs: []string{"10.0.0.1/8"},
				ExcludeUIDs:          []string{"10", "42"},
				NoDNS:                true,
			},
			expectIPConfig: &iptables.Config{
				ProxyUserID:          "1001",
				ProxyInboundPort:     25018,
				ProxyOutboundPort:    16001,
				ExcludeInboundPorts:  []string{"9000", "9002"},
				ExcludeOutboundCIDRs: []string{"10.0.0.1/8"},
				ExcludeOutboundPorts: []string{"443", "80"},
				ExcludeUIDs:          []string{"10", "42"},
				NetNS:                "/var/run/docker/netns/a2ece01ea7bc",
			},
		},
		{
			name:       "tproxy with exposed checks",
			tproxySpec: &structs.ConsulTransparentProxy{},
			exposeSpec: &structs.ConsulExposeConfig{
				Paths: []structs.ConsulExposePath{{
					Path:          "/v1/example",
					Protocol:      "http",
					LocalPathPort: 9000,
					ListenerPort:  "health",
				}},
			},
			expectIPConfig: &iptables.Config{
				ConsulDNSIP:         "192.168.1.117",
				ConsulDNSPort:       8600,
				ProxyUserID:         "101",
				ProxyInboundPort:    25018,
				ProxyOutboundPort:   15001,
				ExcludeInboundPorts: []string{"9000", "9002"},
				NetNS:               "/var/run/docker/netns/a2ece01ea7bc",
			},
		},
		{
			name:       "tproxy with no consul dns fingerprint",
			nodeAttrs:  map[string]string{},
			tproxySpec: &structs.ConsulTransparentProxy{},
			expectIPConfig: &iptables.Config{
				ProxyUserID:         "101",
				ProxyInboundPort:    25018,
				ProxyOutboundPort:   15001,
				ExcludeInboundPorts: []string{"9002"},
				NetNS:               "/var/run/docker/netns/a2ece01ea7bc",
			},
		},
		{
			name: "tproxy with consul dns disabled",
			nodeAttrs: map[string]string{
				"consul.dns.port": "-1",
				"consul.dns.addr": "192.168.1.117",
			},
			tproxySpec: &structs.ConsulTransparentProxy{},
			expectIPConfig: &iptables.Config{
				ProxyUserID:         "101",
				ProxyInboundPort:    25018,
				ProxyOutboundPort:   15001,
				ExcludeInboundPorts: []string{"9002"},
				NetNS:               "/var/run/docker/netns/a2ece01ea7bc",
			},
		},
		{
			name:    "tproxy for other cluster with default consul dns disabled",
			cluster: "infra",
			nodeAttrs: map[string]string{
				"consul.dns.port":       "-1",
				"consul.dns.addr":       "192.168.1.110",
				"consul.infra.dns.port": "8600",
				"consul.infra.dns.addr": "192.168.1.117",
			},
			tproxySpec: &structs.ConsulTransparentProxy{},
			expectIPConfig: &iptables.Config{
				ConsulDNSIP:         "192.168.1.117",
				ConsulDNSPort:       8600,
				ProxyUserID:         "101",
				ProxyInboundPort:    25018,
				ProxyOutboundPort:   15001,
				ExcludeInboundPorts: []string{"9002"},
				NetNS:               "/var/run/docker/netns/a2ece01ea7bc",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tg.Services[0].Connect.SidecarService.Proxy.TransparentProxy = tc.tproxySpec
			tg.Services[0].Connect.SidecarService.Proxy.Expose = tc.exposeSpec
			tg.Services[0].Cluster = tc.cluster

			c := &cniNetworkConfigurator{
				nodeAttrs: nodeAttrs,
				nodeMeta:  nodeMeta,
				logger:    testlog.HCLogger(t),
			}
			if tc.nodeAttrs != nil {
				c.nodeAttrs = tc.nodeAttrs
			}

			iptablesCfg, err := c.setupTransparentProxyArgs(alloc, spec, portMaps)
			if tc.expectErr == "" {
				must.NoError(t, err)
				must.Eq(t, tc.expectIPConfig, iptablesCfg)
			} else {
				must.EqError(t, err, tc.expectErr)
				must.Nil(t, iptablesCfg)
			}
		})

	}

}
