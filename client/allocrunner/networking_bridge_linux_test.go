// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/coreos/go-iptables/iptables"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func Test_buildNomadBridgeNetConfig(t *testing.T) {
	ci.Parallel(t)
	testCases := []struct {
		name          string
		withConsulCNI bool
		b             *bridgeNetworkConfigurator
	}{
		{
			name: "empty",
			b:    &bridgeNetworkConfigurator{},
		},

		{
			name: "ipv6",
			b: &bridgeNetworkConfigurator{
				bridgeName:      defaultNomadBridgeName,
				allocSubnetIPv6: "3fff:cab0:0d13::/120",
				allocSubnetIPv4: defaultNomadAllocSubnet,
			},
		},
		{
			name: "hairpin",
			b: &bridgeNetworkConfigurator{
				bridgeName:      defaultNomadBridgeName,
				allocSubnetIPv4: defaultNomadAllocSubnet,
				hairpinMode:     true,
			},
		},
		{
			name: "bad_input",
			b: &bridgeNetworkConfigurator{
				bridgeName:      `bad"`,
				allocSubnetIPv4: defaultNomadAllocSubnet,
				hairpinMode:     true,
			},
		},
		{
			name:          "consul-cni",
			withConsulCNI: true,
			b: &bridgeNetworkConfigurator{
				bridgeName:      defaultNomadBridgeName,
				allocSubnetIPv4: defaultNomadAllocSubnet,
				hairpinMode:     true,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bCfg, err := buildNomadBridgeNetConfig(*tc.b, tc.withConsulCNI)
			must.NoError(t, err)

			// Validate that the JSON created is rational
			must.True(t, json.Valid(bCfg))

			// and that it matches golden expectations
			goldenFile := filepath.Join("test_fixtures", tc.name+".conflist.json")
			expect, err := os.ReadFile(goldenFile)
			must.NoError(t, err)
			must.Eq(t, string(expect), string(bCfg)+"\n")
		})
	}
}

func TestBridgeNetworkConfigurator_newIPTables_default(t *testing.T) {
	t.Parallel()

	b, err := newBridgeNetworkConfigurator(hclog.Default(),
		mock.MinAlloc(),
		"", "", "", "",
		false, false,
		mock.Node())
	must.NoError(t, err)

	for family, expect := range map[structs.NodeNetworkAF]iptables.Protocol{
		"ipv6":  iptables.ProtocolIPv6,
		"ipv4":  iptables.ProtocolIPv4,
		"other": iptables.ProtocolIPv4,
	} {
		t.Run(string(family), func(t *testing.T) {
			mgr, err := b.newIPTables(family)
			must.NoError(t, err)
			ipt := mgr.(*iptables.IPTables)
			must.Eq(t, expect, ipt.Proto(), must.Sprint("unexpected ip family"))
		})
	}
}

func TestBridgeNetworkConfigurator_ensureForwardingRules(t *testing.T) {
	t.Parallel()

	newMockIPTables := func(b *bridgeNetworkConfigurator) (*mockIPTablesChain, *mockIPTablesChain) {
		ipt := &mockIPTablesChain{}
		ip6t := &mockIPTablesChain{}
		b.newIPTables = func(fam structs.NodeNetworkAF) (IPTablesChain, error) {
			switch fam {
			case "ipv6":
				return ip6t, nil
			case "ipv4":
				return ipt, nil
			}
			return nil, fmt.Errorf("unknown fam %q in newMockIPTables", fam)
		}
		return ipt, ip6t
	}

	cases := []struct {
		name                 string
		bridgeName, ip4, ip6 string
		expectIP4Rules       []string
		expectIP6Rules       []string
		ip4Err, ip6Err       error
	}{
		{
			name:           "defaults",
			expectIP4Rules: []string{"-o", defaultNomadBridgeName, "-d", defaultNomadAllocSubnet, "-j", "ACCEPT"},
		},
		{
			name:           "configured",
			bridgeName:     "golden-gate",
			ip4:            "a.b.c.d/z",
			ip6:            "aa:bb:cc:dd/z",
			expectIP4Rules: []string{"-o", "golden-gate", "-d", "a.b.c.d/z", "-j", "ACCEPT"},
			expectIP6Rules: []string{"-o", "golden-gate", "-d", "aa:bb:cc:dd/z", "-j", "ACCEPT"},
		},
		{
			name:   "ip4error",
			ip4Err: errors.New("test ip4error"),
		},
		{
			name:   "ip6error",
			ip6:    "aa:bb:cc:dd/z",
			ip6Err: errors.New("test ip6error"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := newBridgeNetworkConfigurator(hclog.Default(),
				mock.MinAlloc(),
				tc.bridgeName, tc.ip4, tc.ip6, "",
				false, false,
				mock.Node())
			must.NoError(t, err)

			ipt, ip6t := newMockIPTables(b)
			ipt.newChainErr = tc.ip4Err
			ip6t.newChainErr = tc.ip6Err

			// method under test
			err = b.ensureForwardingRules()

			if tc.ip6Err != nil {
				must.ErrorIs(t, err, tc.ip6Err)
				return
			}
			if tc.ip4Err != nil {
				must.ErrorIs(t, err, tc.ip4Err)
				return
			}
			must.NoError(t, err)

			must.Eq(t, ipt.chain, cniAdminChainName)
			must.Eq(t, ipt.table, "filter")
			must.Eq(t, ipt.rules, tc.expectIP4Rules)

			if tc.expectIP6Rules != nil {
				must.Eq(t, ip6t.chain, cniAdminChainName)
				must.Eq(t, ip6t.table, "filter")
				must.Eq(t, ip6t.rules, tc.expectIP6Rules)
			} else {
				must.Eq(t, "", ip6t.chain, must.Sprint("expect empty ip6tables chain"))
				must.Eq(t, "", ip6t.table, must.Sprint("expect empty ip6tables table"))
				must.Len(t, 0, ip6t.rules, must.Sprint("expect empty ip6tables rules"))
			}
		})
	}
}
