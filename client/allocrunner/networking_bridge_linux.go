// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"fmt"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/cni"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers"
)

const (
	// defaultNomadBridgeName is the name of the bridge to use when not set by
	// the client
	defaultNomadBridgeName = "nomad"

	// bridgeNetworkAllocIfPrefix is the prefix that is used for the interface
	// name created inside of the alloc network which is connected to the bridge
	bridgeNetworkAllocIfPrefix = "eth"

	// defaultNomadAllocSubnet is the subnet to use for host local ip address
	// allocation when not specified by the client
	defaultNomadAllocSubnet = "172.26.64.0/20" // end 172.26.79.255
)

// bridgeNetworkConfigurator is a NetworkConfigurator which adds the alloc to a
// shared bridge, configures masquerading for egress traffic and port mapping
// for ingress
type bridgeNetworkConfigurator struct {
	cni             *cniNetworkConfigurator
	allocSubnetIPv6 string
	allocSubnetIPv4 string
	bridgeName      string
	hairpinMode     bool

	newIPTables func(structs.NodeNetworkAF) (IPTablesChain, error)

	logger hclog.Logger
}

func newBridgeNetworkConfigurator(log hclog.Logger, alloc *structs.Allocation, bridgeName, ipv4Range, ipv6Range, cniPath string, hairpinMode, ignorePortMappingHostIP bool, node *structs.Node) (*bridgeNetworkConfigurator, error) {
	b := &bridgeNetworkConfigurator{
		bridgeName:      bridgeName,
		hairpinMode:     hairpinMode,
		allocSubnetIPv4: ipv4Range,
		allocSubnetIPv6: ipv6Range,
		newIPTables:     newIPTablesChain,
		logger:          log,
	}

	if b.bridgeName == "" {
		b.bridgeName = defaultNomadBridgeName
	}

	if b.allocSubnetIPv4 == "" {
		b.allocSubnetIPv4 = defaultNomadAllocSubnet
	}

	var netCfg []byte
	var err error

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	for _, svc := range tg.Services {
		if svc.Connect.HasTransparentProxy() {
			netCfg, err = buildNomadBridgeNetConfig(*b, true)
			if err != nil {
				return nil, err
			}

			break
		}
	}
	if netCfg == nil {
		netCfg, err = buildNomadBridgeNetConfig(*b, false)
		if err != nil {
			return nil, err
		}
	}

	parser := &cniConfParser{
		listBytes: netCfg,
	}

	c, err := newCNINetworkConfiguratorWithConf(log, cniPath, bridgeNetworkAllocIfPrefix, ignorePortMappingHostIP, parser, node)
	if err != nil {
		return nil, err
	}
	b.cni = c

	return b, nil
}

// ensureForwardingRules ensures that a forwarding rule is added to iptables
// to allow traffic inbound to the bridge network
func (b *bridgeNetworkConfigurator) ensureForwardingRules() error {
	if b.allocSubnetIPv6 != "" {
		ip6t, err := b.newIPTables(structs.NodeNetworkAF_IPv6)
		if err != nil {
			return err
		}
		if err = ensureChainRule(ip6t, b.bridgeName, b.allocSubnetIPv6); err != nil {
			return err
		}
	}

	ipt, err := b.newIPTables(structs.NodeNetworkAF_IPv4)
	if err != nil {
		return err
	}

	if err = ensureChainRule(ipt, b.bridgeName, b.allocSubnetIPv4); err != nil {
		return err
	}

	return nil
}

// Setup calls the CNI plugins with the add action
func (b *bridgeNetworkConfigurator) Setup(ctx context.Context, alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec, created bool) (*structs.AllocNetworkStatus, error) {
	if err := b.ensureForwardingRules(); err != nil {
		return nil, fmt.Errorf("failed to initialize table forwarding rules: %v", err)
	}

	return b.cni.Setup(ctx, alloc, spec, created)
}

// Teardown calls the CNI plugins with the delete action
func (b *bridgeNetworkConfigurator) Teardown(ctx context.Context, alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) error {
	return b.cni.Teardown(ctx, alloc, spec)
}

func buildNomadBridgeNetConfig(b bridgeNetworkConfigurator, withConsulCNI bool) ([]byte, error) {
	conf := cni.NewNomadBridgeConflist(cni.NomadBridgeConfig{
		BridgeName:     b.bridgeName,
		AdminChainName: cniAdminChainName,
		IPv4Subnet:     b.allocSubnetIPv4,
		IPv6Subnet:     b.allocSubnetIPv6,
		HairpinMode:    b.hairpinMode,
		ConsulCNI:      withConsulCNI,
	})
	return conf.Json()
}
