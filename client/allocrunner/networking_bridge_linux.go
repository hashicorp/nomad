// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package allocrunner

import (
	"context"
	"fmt"
	"github.com/coreos/go-iptables/iptables"
	hclog "github.com/hashicorp/go-hclog"
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

	// defaultNomadAllocSubnetIPv6 is the subnet to use for host local ipv6 address
	// allocation when not specified by the client
	defaultNomadAllocSubnetIPv6 = "fd00::/8" // Unique local address

	// cniAdminChainName is the name of the admin iptables chain used to allow
	// forwarding traffic to allocations
	cniAdminChainName = "NOMAD-ADMIN"
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

	logger hclog.Logger
}

func newBridgeNetworkConfigurator(log hclog.Logger, alloc *structs.Allocation, bridgeName, ip4Range string, ipv6Range string, hairpinMode bool, cniPath string, ignorePortMappingHostIP bool, node *structs.Node) (*bridgeNetworkConfigurator, error) {
	b := &bridgeNetworkConfigurator{
		bridgeName:      bridgeName,
		hairpinMode:     hairpinMode,
		allocSubnetIPv4: ip4Range,
		allocSubnetIPv6: ipv6Range,
		logger:          log,
	}
	if b.bridgeName == "" {
		b.bridgeName = defaultNomadBridgeName
	}

	if b.allocSubnetIPv4 == "" {
		b.allocSubnetIPv4 = defaultNomadAllocSubnet
	}
	if b.allocSubnetIPv6 == "" {
		b.allocSubnetIPv6 = defaultNomadAllocSubnetIPv6
	}

	var netCfg []byte

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	for _, svc := range tg.Services {
		if svc.Connect.HasTransparentProxy() {
			netCfg = buildNomadBridgeNetConfig(*b, true)
			break
		}
	}
	if netCfg == nil {
		netCfg = buildNomadBridgeNetConfig(*b, false)
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
		ip6t, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv6), iptables.Timeout(5))
		if err != nil {
			return err
		}
		if err = ensureChain(ip6t, "filter", cniAdminChainName); err != nil {
			return err
		}

		if err := appendChainRule(ip6t, cniAdminChainName, b.generateAdminChainRule("ipv6")); err != nil {
			return err
		}
	}
	if b.allocSubnetIPv4 != "" {
		ipt, err := iptables.New()
		if err != nil {
			return err
		}

		if err = ensureChain(ipt, "filter", cniAdminChainName); err != nil {
			return err
		}

		if err := appendChainRule(ipt, cniAdminChainName, b.generateAdminChainRule("ipv4")); err != nil {
			return err
		}
	}

	return nil
}

// ensureChain ensures that the given chain exists, creating it if missing
func ensureChain(ipt *iptables.IPTables, table, chain string) error {
	chains, err := ipt.ListChains(table)
	if err != nil {
		return fmt.Errorf("failed to list iptables chains: %v", err)
	}
	for _, ch := range chains {
		if ch == chain {
			return nil
		}
	}

	err = ipt.NewChain(table, chain)

	// if err is for chain already existing return as it is possible another
	// goroutine created it first
	if e, ok := err.(*iptables.Error); ok && e.ExitStatus() == 1 {
		return nil
	}

	return err
}

// appendChainRule adds the given rule to the chain
func appendChainRule(ipt *iptables.IPTables, chain string, rule []string) error {
	exists, err := ipt.Exists("filter", chain, rule...)
	if !exists && err == nil {
		err = ipt.Append("filter", chain, rule...)
	}
	return err
}

// generateAdminChainRule builds the iptables rule that is inserted into the
// CNI admin chain to ensure traffic forwarding to the bridge network
func (b *bridgeNetworkConfigurator) generateAdminChainRule(ipProtocol string) []string {
	if ipProtocol == "ipv6" {
		return []string{"-o", b.bridgeName, "-d", b.allocSubnetIPv6, "-j", "ACCEPT"}
	} else {
		return []string{"-o", b.bridgeName, "-d", b.allocSubnetIPv4, "-j", "ACCEPT"}
	}

}

// Setup calls the CNI plugins with the add action
func (b *bridgeNetworkConfigurator) Setup(ctx context.Context, alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) (*structs.AllocNetworkStatus, error) {
	if err := b.ensureForwardingRules(); err != nil {
		return nil, fmt.Errorf("failed to initialize table forwarding rules: %v", err)
	}

	return b.cni.Setup(ctx, alloc, spec)
}

// Teardown calls the CNI plugins with the delete action
func (b *bridgeNetworkConfigurator) Teardown(ctx context.Context, alloc *structs.Allocation, spec *drivers.NetworkIsolationSpec) error {
	return b.cni.Teardown(ctx, alloc, spec)
}

func buildNomadBridgeNetConfig(b bridgeNetworkConfigurator, withConsulCNI bool) []byte {
	var consulCNI string
	if withConsulCNI {
		consulCNI = consulCNIBlock
	}

	return []byte(fmt.Sprintf(nomadCNIConfigTemplate,
		b.bridgeName,
		b.hairpinMode,
		b.allocSubnetIPv4,
		b.allocSubnetIPv6,
		cniAdminChainName,
		cniAdminChainName,
		consulCNI,
	))
}

// Update website/content/docs/networking/cni.mdx when the bridge configuration
// is modified. If CNI plugins are added or versions need to be updated for new
// fields, add a new constraint to nomad/job_endpoint_hooks.go
const nomadCNIConfigTemplate = `{
	"cniVersion": "0.4.0",
	"name": "nomad",
	"plugins": [
		{
			"type": "loopback"
		},
		{
			"type": "bridge",
			"bridge": %q,
			"ipMasq": true,
			"isGateway": true,
			"forceAddress": true,
			"hairpinMode": %v,
			"ipam": {
				"type": "host-local",
				"ranges": [
					[
						{
							"subnet": %q
						}
					],
					[
						{ 
							"subnet": %q
						}
					]
				],
				"routes": [
					{ "dst": "0.0.0.0/0" },
                    { "dst" : "::/0" }
				]
			}
		},
		{
			"type": "firewall",
			"backend": "iptables",
			"iptablesAdminChainName": %q
		},
		{
			"type": "firewall",
			"backend": "ip6tables",
			"iptablesAdminChainName": %q
		},
		{
			"type": "portmap",
			"capabilities": {"portMappings": true},
			"snat": true
		}%s
	]
}
`

const consulCNIBlock = `,
		{
			"type": "consul-cni",
			"log_level": "debug"
		}
`
