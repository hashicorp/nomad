// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package allocrunner

import (
	"errors"
	"net"
	"testing"

	"github.com/containerd/go-cni"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
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

// TestCNI_cniToAllocNet_NoInterfaces asserts an error is returned if CNIResult
// contains no interfaces.
func TestCNI_cniToAllocNet_NoInterfaces(t *testing.T) {
	ci.Parallel(t)

	cniResult := &cni.CNIResult{}

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

	// Calico's CNI plugin v3.12.3 has been observed to return the
	// following:
	cniResult := &cni.CNIResult{
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
	}

	// Only need a logger
	c := &cniNetworkConfigurator{
		logger: testlog.HCLogger(t),
	}
	allocNet, err := c.cniToAllocNet(cniResult)
	require.NoError(t, err)
	require.NotNil(t, allocNet)
	assert.Equal(t, "192.168.135.232", allocNet.Address)
	assert.Equal(t, "eth0", allocNet.InterfaceName)
	assert.Nil(t, allocNet.DNS)
}

// TestCNI_cniToAllocNet_Invalid asserts an error is returned if a CNI plugin
// result lacks any IP addresses. This has not been observed, but Nomad still
// must guard against invalid results from external plugins.
func TestCNI_cniToAllocNet_Invalid(t *testing.T) {
	ci.Parallel(t)

	cniResult := &cni.CNIResult{
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
